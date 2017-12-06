/*
 * Copyright (C) 2016 Red Hat, Inc.
 *
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 *
 */

#include <stddef.h>
#include <linux/if_packet.h>
#include <linux/ip.h>
#include <linux/ipv6.h>
#include <linux/icmp.h>
#include <linux/icmpv6.h>
#include <linux/udp.h>
#include <linux/tcp.h>
#include <linux/in.h>

#include "bpf.h"
#include "flow.h"

#define DEBUG 1

// Fowler/Noll/Vo hash
#define FNV_BASIS ((__u64)14695981039346656037U)
#define FNV_PRIME ((__u64)1099511628211U)

#define IP_MF     0x2000
#define IP_OFFSET	0x1FFF

MAP(u64_config_values) {
	.type = BPF_MAP_TYPE_ARRAY,
	.key_size = sizeof(__u32),
	.value_size = sizeof(__u64),
	.max_entries = 1,
};

MAP(flow_table) {
	.type = BPF_MAP_TYPE_HASH,
	.key_size = sizeof(__u64),
	.value_size = sizeof(struct flow),
	.max_entries = 500000,
};

static inline void update_hash_byte(__u64 *key, __u8 byte)
{
	*key ^= (__u64)byte;
	*key *= FNV_PRIME;
}

static inline void update_hash_half(__u64 *key, __u16 half)
{
	update_hash_byte(key, (half >> 8) & 0xff);
	update_hash_byte(key, half & 0xff);
}

static inline void update_hash_word(__u64 *key, __u32 word)
{
	update_hash_half(key, (word >> 16) & 0xffff);
	update_hash_half(key, word & 0xffff);
}

static inline void fill_payload_bucket(struct __sk_buff *skb, int offset, __u8 *bucket, int bsize) {
	for (int i = 0; i != bsize; i++) {
		bucket[i] = load_byte(skb, offset + i);
	}
}

static void fill_payload(struct __sk_buff *skb, int offset, struct flow *flow, int len)
{
	// TODO add more data
	fill_payload_bucket(skb, offset, flow->payload, 30);
}

static void fill_transport(struct __sk_buff *skb, __u8 protocol, int offset,
	struct flow *flow, int len)
{
	struct transport_layer *layer = &flow->transport_layer;

	layer->protocol = protocol;
	layer->port_src = load_half(skb, offset);
	layer->port_dst = load_half(skb, offset + sizeof(__be16));

	__u64 hash_src = 0;
	update_hash_half(&hash_src, layer->port_src);

	__u64 hash_dst = 0;
	update_hash_half(&hash_dst, layer->port_dst);

	switch (protocol) {
		case IPPROTO_UDP:
			//len -= sizeof(struct udphdr);
			fill_payload(skb, offset, flow, len);
			break;
		case IPPROTO_TCP:
			//len -= sizeof(struct tcphdr);
			fill_payload(skb, offset, flow, len);
			break;
	}

	layer->_hash = FNV_BASIS ^ hash_src ^ hash_dst;

	flow->layers |= TRANSPORT_LAYER;
}

static void fill_icmpv4(struct __sk_buff *skb, int offset, struct flow *flow)
{
	struct icmp_layer *layer = &flow->icmp_layer;

	layer->kind = load_byte(skb, offset + offsetof(struct icmphdr, type));
	layer->code = load_byte(skb, offset + offsetof(struct icmphdr, code));

	__u64 hash = 0;
	update_hash_byte(&hash, layer->code);

	switch (layer->kind) {
		case ICMP_ECHO:
		case ICMP_ECHOREPLY:
			update_hash_byte(&hash, ICMP_ECHO|ICMP_ECHOREPLY);

			layer->id = load_half(skb, offset + offsetof(struct icmphdr, un.echo.id));

			update_hash_byte(&hash, layer->id);
			break;
	}

	layer->_hash = FNV_BASIS ^ hash;

	flow->layers |= ICMP_LAYER;
}

static void fill_icmpv6(struct __sk_buff *skb, int offset, struct flow *flow)
{
	struct icmp_layer *layer = &flow->icmp_layer;

	layer->kind = load_byte(skb, offset + offsetof(struct icmp6hdr, icmp6_type));
	layer->code = load_byte(skb, offset + offsetof(struct icmp6hdr, icmp6_code));

	__u64 hash = 0;
	update_hash_byte(&hash, layer->code);

	switch (layer->kind) {
		case ICMPV6_ECHO_REQUEST:
		case ICMPV6_ECHO_REPLY:
			update_hash_byte(&hash, ICMPV6_ECHO_REQUEST|ICMPV6_ECHO_REPLY);

			layer->id = load_half(skb, offset + offsetof(struct icmp6hdr, icmp6_dataun.u_echo.identifier));

			update_hash_byte(&hash, layer->id);
			break;
	}

	layer->_hash = FNV_BASIS ^ hash;

	flow->layers |= ICMP_LAYER;
}

static void fill_word(__u32 src, __u8 *dst, int offset)
{
	dst[offset] = (src >> 24) & 0xff;
	dst[offset + 1] = (src >> 16) & 0xff;
	dst[offset + 2] = (src >> 8) & 0xff;
	dst[offset + 3] = src & 0xff;
}

static inline void fill_ipv4(struct __sk_buff *skb, int offset, __u8 *dst, __u64 *hash)
{
	__u32 w = load_word(skb, offset);
	fill_word(w, dst, 12);
	update_hash_word(hash, w);
}

static inline void fill_ipv6(struct __sk_buff *skb, int offset, __u8 *dst, __u64 *hash)
{
	__u32 w = load_word(skb, offset);
	fill_word(w, dst, 0);
	update_hash_word(hash, w);

	w = load_word(skb, offset + 4);
	fill_word(w, dst, 4);
	update_hash_word(hash, w);

	w = load_word(skb, offset + 8);
	fill_word(w, dst, 8);
	update_hash_word(hash, w);

	w = load_word(skb, offset + 12);
	fill_word(w, dst, 12);
	update_hash_word(hash, w);
}

static void fill_network(struct __sk_buff *skb, __u16 protocol, int offset,
	struct flow *flow)
{
	struct network_layer *layer = &flow->network_layer;

	int len = skb->len - sizeof(struct ethhdr);
	int frag = 0;

	__u64 hash_src = 0;
	__u64 hash_dst = 0;

	layer->protocol = protocol;
	switch (protocol) {
		case ETH_P_IP:
			frag = load_half(skb, offset + offsetof(struct iphdr, frag_off)) & (IP_MF | IP_OFFSET);
			if (frag) {
				// TODO report fragment
				return;
			}

			protocol = load_byte(skb, offset + offsetof(struct iphdr, protocol));
			fill_ipv4(skb, offset + offsetof(struct iphdr, saddr), layer->ip_src, &hash_src);
			fill_ipv4(skb, offset + offsetof(struct iphdr, daddr), layer->ip_dst, &hash_dst);
			break;
		case ETH_P_IPV6:
			protocol = load_byte(skb, offset + offsetof(struct ipv6hdr, nexthdr));
			fill_ipv6(skb, offset + offsetof(struct ipv6hdr, saddr), layer->ip_src, &hash_src);
			fill_ipv6(skb, offset + offsetof(struct ipv6hdr, daddr), layer->ip_dst, &hash_dst);
			break;
		default:
			return;
	}

	__u8 verlen = load_byte(skb, offset);
	offset += (verlen & 0xF) << 2;

	len -= (verlen & 0xF) << 2;

	switch (protocol) {
		case IPPROTO_GRE:
			// TODO
			break;
		case IPPROTO_SCTP:
			// TODO
		case IPPROTO_UDP:
		case IPPROTO_TCP:
			fill_transport(skb, protocol, offset, flow, len);
			break;
		case IPPROTO_ICMP:
			fill_icmpv4(skb, offset, flow);
			break;
		case IPPROTO_ICMPV6:
			fill_icmpv6(skb, offset, flow);
			break;
		default:
			fill_payload(skb, offset, flow, len);
	}

	layer->_hash = FNV_BASIS ^ hash_src ^ hash_dst ^ protocol;

	flow->layers |= NETWORK_LAYER;
}

static inline void fill_haddr(struct __sk_buff *skb, int offset,
	unsigned char *mac)
{
	mac[0] = load_byte(skb, offset);
	mac[1] = load_byte(skb, offset + 1);
	mac[2] = load_byte(skb, offset + 2);
	mac[3] = load_byte(skb, offset + 3);
	mac[4] = load_byte(skb, offset + 4);
	mac[5] = load_byte(skb, offset + 5);
}

static void fill_link(struct __sk_buff *skb, int offset, struct flow *flow)
{
	struct link_layer *layer = &flow->link_layer;

	fill_haddr(skb, offset + offsetof(struct ethhdr, h_source), layer->mac_src);
	fill_haddr(skb, offset + offsetof(struct ethhdr, h_dest), layer->mac_dst);

	update_hash_half(&layer->_hash_src, layer->mac_src[0] << 8 | layer->mac_src[1]);
	update_hash_half(&layer->_hash_src, layer->mac_src[2] << 8 | layer->mac_src[3]);
	update_hash_half(&layer->_hash_src, layer->mac_src[3] << 8 | layer->mac_src[5]);

	__u64 hash_dst = 0;
	update_hash_half(&hash_dst, layer->mac_dst[0] << 8 | layer->mac_dst[1]);
	update_hash_half(&hash_dst, layer->mac_dst[2] << 8 | layer->mac_dst[3]);
	update_hash_half(&hash_dst, layer->mac_dst[3] << 8 | layer->mac_dst[5]);

	layer->_hash = FNV_BASIS ^ layer->_hash_src ^ hash_dst;

	flow->layers |= LINK_LAYER;
}

static void update_metrics(struct __sk_buff *skb, struct flow *flow, __u64 tm, int ab)
{
	struct link_layer *layer = &flow->link_layer;

	if (ab) {
		__sync_fetch_and_add(&flow->metrics.ab_packets, 1);
		__sync_fetch_and_add(&flow->metrics.ab_bytes, skb->len);
	} else {
		__sync_fetch_and_add(&flow->metrics.ba_packets, 1);
		__sync_fetch_and_add(&flow->metrics.ba_bytes, skb->len);
	}
}

static void fill_flow(struct __sk_buff *skb, struct flow *flow)
{
	flow->ifindex = skb->ifindex;

	fill_link(skb, 0, flow);

	__u16 protocol = load_half(skb, offsetof(struct ethhdr, h_proto));
	switch (protocol) {
	case ETH_P_8021Q:
		// TODO
		break;
	case ETH_P_IP:
	case ETH_P_IPV6:
		fill_network(skb, (__u16)protocol, ETH_HLEN, flow);
		break;
	}

	flow->key = flow->link_layer._hash ^ flow->network_layer._hash ^
		flow->transport_layer._hash ^ flow->icmp_layer._hash;
}

SOCKET(flow_table)
int bpf_flow_table(struct __sk_buff *skb)
{
	__u64 tm = bpf_ktime_get_ns();

	__u32 key = START_TIME_NS;
	__u32 *sns = bpf_map_lookup_element(&u64_config_values, &key);
	if (sns != NULL && *sns == 0) {
		bpf_map_update_element(&u64_config_values, &key, &tm, BPF_ANY);
	}

	struct flow flow = {.rtt = 0}, *prev;
	fill_flow(skb, &flow);

	prev = bpf_map_lookup_element(&flow_table, &flow.key);
	if (prev) {
		update_metrics(skb, prev, tm,
			flow.link_layer._hash_src == prev->link_layer._hash_src);
		__sync_fetch_and_add(&prev->last, tm - prev->last);

		if (prev->rtt == 0) {
			__sync_fetch_and_add(&prev->rtt, tm - prev->start);
		}
	} else {
		update_metrics(skb, &flow, tm, 1);
		__sync_fetch_and_add(&flow.start, tm);
		__sync_fetch_and_add(&flow.last, tm);

		bpf_map_update_element(&flow_table, &flow.key, &flow, BPF_ANY);
	}

	return 0;
}
char _license[] LICENSE = "GPL";
