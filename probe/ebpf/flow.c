/*
 * Copyright (C) 2016 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy ofthe License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specificlanguage governing permissions and
 * limitations under the License.
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

// Fowler/Noll/Vo hash
#define FNV_BASIS ((__u64)14695981039346656037U)
#define FNV_PRIME ((__u64)1099511628211U)

#define IP_MF     0x2000
#define IP_OFFSET	0x1FFF

#define MAX_VLAN_LAYERS 5
struct vlan {
	__u16		tci;
	__u16		ethertype;
};

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

static inline void add_layer(struct flow *flow, __u8 layer) {
	if (flow->layers_path & (LAYERS_PATH_MASK << ((LAYERS_PATH_LEN-1)*LAYERS_PATH_SHIFT))) {
		return;
	}
	flow->layers_path = (flow->layers_path << LAYERS_PATH_SHIFT) | layer;
}

static inline void fill_transport(struct __sk_buff *skb, __u8 protocol, int offset,
	struct flow *flow, int len, __u64 tm)
{
	struct transport_layer *layer = &flow->transport_layer;

	layer->protocol = protocol;
	layer->port_src = load_half(skb, offset);
	layer->port_dst = load_half(skb, offset + sizeof(__be16));

	__u64 hash_src = 0;
	update_hash_half(&hash_src, layer->port_src);

	__u64 hash_dst = 0;
	update_hash_half(&hash_dst, layer->port_dst);

	__u8 flags = 0;

	switch (protocol) {
        case IPPROTO_SCTP:
			add_layer(flow, SCTP_LAYER);
            break;
		case IPPROTO_UDP:
            add_layer(flow, UDP_LAYER);
			break;
		case IPPROTO_TCP:
            add_layer(flow, TCP_LAYER);
			flags = load_byte(skb, offset + 14);
			layer->ab_syn = (flags & 0x02) > 0 ? tm : 0;
			layer->ab_fin = (flags & 0x01) > 0 ? tm : 0;
			layer->ab_rst = (flags & 0x04) > 0 ? tm : 0;
			break;
	}

	layer->_hash = FNV_BASIS ^ hash_src ^ hash_dst;

	flow->layers_info |= TRANSPORT_LAYER_INFO;
}

static inline void fill_icmpv4(struct __sk_buff *skb, int offset, struct flow *flow)
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

    add_layer(flow, ICMP4_LAYER);
	flow->layers_info |= ICMP_LAYER_INFO;
}

static inline void fill_icmpv6(struct __sk_buff *skb, int offset, struct flow *flow)
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

    add_layer(flow, ICMP6_LAYER);
	flow->layers_info |= ICMP_LAYER_INFO;
}

static inline void fill_word(__u32 src, __u8 *dst, int offset)
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

static inline void fill_network(struct __sk_buff *skb, __u16 netproto, int offset,
	struct flow *flow, __u64 tm)
{
	struct network_layer *layer = &flow->network_layer;

	int len = skb->len - sizeof(struct ethhdr);
	int frag = 0;

	__u8 transproto = 0;

	__u64 hash_src = 0;
	__u64 hash_dst = 0;

	layer->protocol = netproto;
	switch (netproto) {
		case ETH_P_IP:
			frag = load_half(skb, offset + offsetof(struct iphdr, frag_off)) & (IP_MF | IP_OFFSET);
			if (frag) {
				// TODO report fragment
				return;
			}

			transproto = load_byte(skb, offset + offsetof(struct iphdr, protocol));
			fill_ipv4(skb, offset + offsetof(struct iphdr, saddr), layer->ip_src, &hash_src);
			fill_ipv4(skb, offset + offsetof(struct iphdr, daddr), layer->ip_dst, &hash_dst);
			break;
		case ETH_P_IPV6:
			transproto = load_byte(skb, offset + offsetof(struct ipv6hdr, nexthdr));
			fill_ipv6(skb, offset + offsetof(struct ipv6hdr, saddr), layer->ip_src, &hash_src);
			fill_ipv6(skb, offset + offsetof(struct ipv6hdr, daddr), layer->ip_dst, &hash_dst);
			break;
		default:
			return;
	}

	__u8 verlen = load_byte(skb, offset);
	offset += (verlen & 0xF) << 2;

	len -= (verlen & 0xF) << 2;

	switch (transproto) {
		case IPPROTO_GRE:
			// TODO
			break;
		case IPPROTO_SCTP:
			// TODO
		case IPPROTO_UDP:
		case IPPROTO_TCP:
			fill_transport(skb, transproto, offset, flow, len, tm);
			break;
		case IPPROTO_ICMP:
			fill_icmpv4(skb, offset, flow);
			break;
		case IPPROTO_ICMPV6:
			fill_icmpv6(skb, offset, flow);
			break;
	}

	layer->_hash = FNV_BASIS ^ hash_src ^ hash_dst ^ netproto ^ transproto;

	flow->layers_info |= NETWORK_LAYER_INFO;
}

static inline __u16 fill_vlan(struct __sk_buff *skb, int offset, struct flow *flow)
{
	struct link_layer *layer = &flow->link_layer;

	__u16 tci = load_half(skb, offset + offsetof(struct vlan, tci));
	__u16 protocol = load_half(skb, offset + offsetof(struct vlan, ethertype));
	__u16 vlanID = tci & 0x0fff;

	__u64 hash_vlan = 0;
	update_hash_half(&hash_vlan, vlanID);

	layer->_hash ^= hash_vlan;
	layer->id = (layer->id << 12) | vlanID;

    add_layer(flow, DOT1Q_LAYER);

	return protocol;
}

static inline void fill_vlans(struct __sk_buff *skb, __u16 *protocol, int *offset, struct flow *flow) {
	if (*protocol == ETH_P_8021Q) {
		#pragma unroll
		for(int i=0;i<MAX_VLAN_LAYERS;i++) {
			*protocol = fill_vlan(skb, *offset, flow);
			*offset += 4;
			if (*protocol != ETH_P_8021Q) {
				break;
			}
		}
	}

	struct link_layer *layer = &flow->link_layer;
	if (skb->vlan_present) {
		__u16 vlanID = skb->vlan_tci & 0x0fff;
		layer->_hash ^= vlanID;
		layer->id = (layer->id << 12) | vlanID;

        add_layer(flow, DOT1Q_LAYER);
	}
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

static inline void fill_link(struct __sk_buff *skb, int offset, struct flow *flow)
{
	struct link_layer *layer = &flow->link_layer;

	fill_haddr(skb, offset + offsetof(struct ethhdr, h_source), layer->mac_src);
	fill_haddr(skb, offset + offsetof(struct ethhdr, h_dest), layer->mac_dst);

	update_hash_half(&layer->_hash_src, layer->mac_src[0] << 8 | layer->mac_src[1]);
	update_hash_half(&layer->_hash_src, layer->mac_src[2] << 8 | layer->mac_src[3]);
	update_hash_half(&layer->_hash_src, layer->mac_src[4] << 8 | layer->mac_src[5]);

	__u64 hash_dst = 0;
	update_hash_half(&hash_dst, layer->mac_dst[0] << 8 | layer->mac_dst[1]);
	update_hash_half(&hash_dst, layer->mac_dst[2] << 8 | layer->mac_dst[3]);
	update_hash_half(&hash_dst, layer->mac_dst[4] << 8 | layer->mac_dst[5]);

	layer->_hash = FNV_BASIS ^ layer->_hash_src ^ hash_dst;

    add_layer(flow, ETH_LAYER);
	flow->layers_info |= LINK_LAYER_INFO; 
}

static inline void update_metrics(struct __sk_buff *skb, struct flow *flow, __u64 tm, int ab)
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

static inline void fill_flow(struct __sk_buff *skb, struct flow *flow, __u64 tm)
{
	fill_link(skb, 0, flow);

	__u16 protocol = load_half(skb, offsetof(struct ethhdr, h_proto));
	int offset = ETH_HLEN;

	fill_vlans(skb, &protocol, &offset, flow);

	switch (protocol) {
	case ETH_P_ARP:
		update_hash_half(&flow->link_layer._hash, protocol);
		flow->layers_info |= ARP_LAYER;
		break;
	case ETH_P_IP:
	case ETH_P_IPV6:
		fill_network(skb, (__u16)protocol, offset, flow, tm);
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
	__u64 *sns = bpf_map_lookup_element(&u64_config_values, &key);
	if (sns != NULL && *sns == 0) {
		bpf_map_update_element(&u64_config_values, &key, &tm, BPF_ANY);
	}

	struct flow flow = {}, *prev;
	fill_flow(skb, &flow,tm);

	prev = bpf_map_lookup_element(&flow_table, &flow.key);
	if (prev) {
		update_metrics(skb, prev, tm,
					   flow.link_layer._hash_src == prev->link_layer._hash_src);
		__sync_fetch_and_add(&prev->last, tm - prev->last);

		if (prev->layers_info & flow.layers_info & TRANSPORT_LAYER_INFO > 0) {
			if (prev->transport_layer.port_src == flow.transport_layer.port_src) {
				if (prev->transport_layer.ab_syn == 0 && flow.transport_layer.ab_syn != 0) {
					__sync_fetch_and_add(&prev->transport_layer.ab_syn, flow.transport_layer.ab_syn);
				}
				if (prev->transport_layer.ab_fin == 0 && flow.transport_layer.ab_fin != 0) {
					__sync_fetch_and_add(&prev->transport_layer.ab_fin, flow.transport_layer.ab_fin);
				}
				if (prev->transport_layer.ab_rst == 0 && flow.transport_layer.ab_rst != 0) {
					__sync_fetch_and_add(&prev->transport_layer.ab_rst, flow.transport_layer.ab_rst);
				}
			}
			else {
				if (prev->transport_layer.ba_syn == 0 && flow.transport_layer.ab_syn != 0) {
					__sync_fetch_and_add(&prev->transport_layer.ba_syn, flow.transport_layer.ab_syn);
				}
				if (prev->transport_layer.ba_fin == 0 && flow.transport_layer.ab_fin != 0) {
					__sync_fetch_and_add(&prev->transport_layer.ba_fin, flow.transport_layer.ab_fin);
				}
				if (prev->transport_layer.ba_rst == 0 && flow.transport_layer.ab_rst != 0) {
					__sync_fetch_and_add(&prev->transport_layer.ba_rst, flow.transport_layer.ab_rst);
				}
			}
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