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

#include "defs.h"
#include "flow.h"
#include "common.h"

MAP(u64_config_values){
	.type = BPF_MAP_TYPE_ARRAY,
	.key_size = sizeof(__u32),
	.value_size = sizeof(__u64),
	.max_entries = 2,
};

MAP(stats_map){
	.type = BPF_MAP_TYPE_HASH,
	.key_size = sizeof(__u32),
	.value_size = sizeof(__u64),
	.max_entries = 1,
};

MAP(flow_table_p1){
	.type = BPF_MAP_TYPE_HASH,
	.key_size = sizeof(__u64),
	.value_size = sizeof(struct flow),
	.max_entries = 500000,
};

MAP(flow_table_p2){
	.type = BPF_MAP_TYPE_HASH,
	.key_size = sizeof(__u64),
	.value_size = sizeof(struct flow),
	.max_entries = 500000,
};

static inline void fill_network(struct __sk_buff *skb, __u16 netproto, int offset,
								struct flow *flow, __u64 tm)
{
	struct network_layer *layer = &flow->network_layer;

	int len = skb->len - sizeof(struct ethhdr);
	int frag = 0;

	__u8 transproto = 0;

	__u64 hash_src = 0;
	__u64 hash_dst = 0;

	__u64 ordered_src = 0;
	__u64 ordered_dst = 0;

#define IP_SRC (__u64) layer->ip_src
#define IP_DST (__u64) layer->ip_dst

	layer->protocol = netproto;
	switch (netproto)
	{
	case ETH_P_IP:
		frag = load_half(skb, offset + offsetof(struct iphdr, frag_off)) & (IP_MF | IP_OFFSET);
		if (frag)
		{
			// TODO report fragment
			return;
		}

		transproto = load_byte(skb, offset + offsetof(struct iphdr, protocol));
		
		fill_ipv4(skb, offset + offsetof(struct iphdr, saddr), layer->ip_src, &hash_src);
		fill_ipv4(skb, offset + offsetof(struct iphdr, daddr), layer->ip_dst, &hash_dst);

		ordered_src = IP_SRC[12] << 24 | IP_SRC[13] << 16 | IP_SRC[14] << 8 | IP_SRC[15];
		ordered_dst = IP_DST[12] << 24 | IP_DST[13] << 16 | IP_DST[14] << 8 | IP_DST[15];
		break;
	case ETH_P_IPV6:
		transproto = load_byte(skb, offset + offsetof(struct ipv6hdr, nexthdr));
		fill_ipv6(skb, offset + offsetof(struct ipv6hdr, saddr), layer->ip_src, &hash_src);
		fill_ipv6(skb, offset + offsetof(struct ipv6hdr, daddr), layer->ip_dst, &hash_dst);

#ifdef FIX_STACK_512LIMIT
		ordered_src = (IP_SRC[0] << 56 | IP_SRC[1] << 48 | IP_SRC[2] << 40 | IP_SRC[3] << 32 |
					   IP_SRC[4] << 24 | IP_SRC[5] << 16 | IP_SRC[6] << 8 | IP_SRC[7]) ^
					  (IP_SRC[8] << 56 | IP_SRC[9] << 48 | IP_SRC[10] << 40 | IP_SRC[11] << 32 |
					   IP_SRC[12] << 24 | IP_SRC[13] << 16 | IP_SRC[14] << 8 | IP_SRC[15]);
		ordered_dst = (IP_DST[0] << 56 | IP_DST[1] << 48 | IP_DST[2] << 40 | IP_DST[3] << 32 |
					   IP_DST[4] << 24 | IP_DST[5] << 16 | Ip_DST[6] << 8 | IP_DST[7]) ^
					  (IP_DST[8] << 56 | IP_DST[9] << 48 | IP_DST[10] << 40 | IP_DST[11] << 32 |
					   IP_DST[12] << 24 | IP_DST[13] << 16 | IP_DST[14] << 8 | IP_DST[15]);
#endif
		break;
	default:
		return;
	}

	__u8 verlen = load_byte(skb, offset);
	offset += (verlen & 0xF) << 2;
	len -= (verlen & 0xF) << 2;

	switch (transproto)
	{
	case IPPROTO_GRE:
		// TODO
		break;
	case IPPROTO_SCTP:
		// TODO
	case IPPROTO_UDP:
	case IPPROTO_TCP:
		fill_transport(skb, transproto, offset, len, flow, ordered_src < ordered_dst, ordered_src == ordered_dst);
		break;
	case IPPROTO_ICMP:
		fill_icmpv4(skb, offset, flow);
		break;
	case IPPROTO_ICMPV6:
		fill_icmpv6(skb, offset, flow);
		break;
	}

	layer->_hash_src = hash_src;
	if (ordered_src < ordered_dst)
	{
		layer->_hash = FNV_BASIS ^ rotl(hash_src, 32) ^ hash_dst ^ netproto ^ transproto;
	}
	else
	{
		layer->_hash = FNV_BASIS ^ rotl(hash_dst, 32) ^ hash_src ^ netproto ^ transproto;
	}
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

static inline void fill_vlans(struct __sk_buff *skb, __u16 *protocol, int *offset, struct flow *flow)
{
	if (*protocol == ETH_P_8021Q)
	{
#pragma unroll
		for (int i = 0; i < MAX_VLAN_LAYERS; i++)
		{
			*protocol = fill_vlan(skb, *offset, flow);
			*offset += 4;
			if (*protocol != ETH_P_8021Q)
			{
				break;
			}
		}
	}

	struct link_layer *layer = &flow->link_layer;
	if (skb->vlan_present)
	{
		__u16 vlanID = skb->vlan_tci & 0x0fff;
		layer->_hash ^= vlanID;
		layer->id = (layer->id << 12) | vlanID;

		add_layer(flow, DOT1Q_LAYER);
	}
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

static inline void fill_flow(struct __sk_buff *skb, struct flow *flow, __u64 tm)
{
	fill_link(skb, 0, flow);

	__u16 protocol = load_half(skb, offsetof(struct ethhdr, h_proto));
	int offset = ETH_HLEN;

	fill_vlans(skb, &protocol, &offset, flow);

	switch (protocol)
	{
	case ETH_P_ARP:
		update_hash_half(&flow->link_layer._hash, protocol);
		flow->layers_info |= ARP_LAYER;
		break;
	case ETH_P_IP:
	case ETH_P_IPV6:
		fill_network(skb, (__u16)protocol, offset, flow, tm);
		break;
	}

	flow->key = flow->link_layer._hash;
	flow->key = rotl(flow->key, 16);
	flow->key ^= flow->network_layer._hash;
	flow->key = rotl(flow->key, 16);
	flow->key ^= flow->transport_layer._hash;
	flow->key = rotl(flow->key, 16);
	flow->key ^= flow->icmp_layer._hash;
}

SOCKET(flow_table)
int bpf_flow_table(struct __sk_buff *skb)
{
	__u64 tm = bpf_ktime_get_ns();

	__u32 key = START_TIME_NS;
	__u64 *sns = bpf_map_lookup_element(&u64_config_values, &key);
	if (sns != NULL && *sns == 0)
	{
		bpf_map_update_element(&u64_config_values, &key, &tm, BPF_ANY);
	}

	struct flow flow = { .last = tm }, *prev;
	fill_flow(skb, &flow, tm);

	key = FLOW_PAGE;
	__u64 *page = bpf_map_lookup_element(&u64_config_values, &key);
	__u64 flow_page = 0;
	if (page != NULL)
	{
		flow_page = *page;
	}

	struct bpf_map_def *flowtable = &flow_table_p1;
	if (flow_page == 1)
	{
		flowtable = &flow_table_p2;
	}

	prev = bpf_map_lookup_element(flowtable, &flow.key);
	if (prev)
	{
		update_metrics(skb, prev, is_ab_packet(&flow, prev));

		__sync_fetch_and_add(&prev->last, tm - prev->last);

		if (prev->layers_info & flow.layers_info & TRANSPORT_LAYER_INFO > 0)
		{
			if (prev->transport_layer.port_src == flow.transport_layer.port_src)
			{
				if (prev->transport_layer.ab_syn == 0 && flow.transport_layer.ab_syn != 0)
				{
					__sync_fetch_and_add(&prev->transport_layer.ab_syn, flow.transport_layer.ab_syn);
				}
				if (prev->transport_layer.ab_fin == 0 && flow.transport_layer.ab_fin != 0)
				{
					__sync_fetch_and_add(&prev->transport_layer.ab_fin, flow.transport_layer.ab_fin);
				}
				if (prev->transport_layer.ab_rst == 0 && flow.transport_layer.ab_rst != 0)
				{
					__sync_fetch_and_add(&prev->transport_layer.ab_rst, flow.transport_layer.ab_rst);
				}
			}
			else
			{
				if (prev->transport_layer.ba_syn == 0 && flow.transport_layer.ab_syn != 0)
				{
					__sync_fetch_and_add(&prev->transport_layer.ba_syn, flow.transport_layer.ab_syn);
				}
				if (prev->transport_layer.ba_fin == 0 && flow.transport_layer.ab_fin != 0)
				{
					__sync_fetch_and_add(&prev->transport_layer.ba_fin, flow.transport_layer.ab_fin);
				}
				if (prev->transport_layer.ba_rst == 0 && flow.transport_layer.ab_rst != 0)
				{
					__sync_fetch_and_add(&prev->transport_layer.ba_rst, flow.transport_layer.ab_rst);
				}
			}
		}
	}
	else
	{
		update_metrics(skb, &flow, 1);

		__sync_fetch_and_add(&flow.start, tm);

		if (bpf_map_update_element(flowtable, &flow.key, &flow, BPF_ANY) == -1)
		{
			__u32 stats_key = 0;
			__u64 stats_update_val = 1;
			__u64 *stats_val = bpf_map_lookup_element(&stats_map, &stats_key);
			if (stats_val == NULL)
			{
				bpf_map_update_element(&stats_map, &stats_key, &stats_update_val, BPF_ANY);
			}
			else
			{
				__sync_fetch_and_add(stats_val, stats_update_val);
			}
		}
	}

	return 0;
}
char _license[] LICENSE = "GPL";
