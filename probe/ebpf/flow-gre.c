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

#include <linux/stddef.h>
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

#define MAX_GRE_ROUTING_INFO 4

MAP(flow_nostack){
	.type = BPF_MAP_TYPE_PERCPU_ARRAY,
	.key_size = sizeof(__u32),
	.value_size = sizeof(struct flow),
	.max_entries = 1,
};

MAP(jmp_map){
	.type = BPF_MAP_TYPE_PROG_ARRAY,
	.key_size = sizeof(__u32),
	.value_size = sizeof(__u32),
	.max_entries = JMP_TABLE_SIZE,
};

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

MAP(l2_table){
	.type = BPF_MAP_TYPE_ARRAY,
	.key_size = sizeof(__u32),
	.value_size = sizeof(struct l2),
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

static inline void add_layer_l2(struct l2 *l2, __u8 layer)
{
	if (l2->layers_path & (LAYERS_PATH_MASK << ((LAYERS_PATH_LEN - 1) * LAYERS_PATH_SHIFT)))
	{
		return;
	}
	l2->layers_path = (l2->layers_path << LAYERS_PATH_SHIFT) | layer;
}

static inline __u16 fill_gre(struct __sk_buff *skb, size_t *offset, struct flow *flow)
{
	__u8 config = load_byte(skb, *offset);
	__u8 cfg_checksum = config & 0x80;
	__u8 cfg_routing = config & 0x40;
	__u8 cfg_key = config & 0x20;
	__u8 cfg_seq = config & 0x10;
	__u8 flags = load_byte(skb, (*offset) + 1);
	__u8 cfg_ack = flags & 0x80;

	__u16 protocol = load_half(skb, (*offset) + 2);
	(*offset) += 4;
	if ((cfg_checksum > 0) || (cfg_routing > 0))
	{
		(*offset) += 4;
	}
	if (cfg_key > 0)
	{
		(*offset) += 4;
	}
	if (cfg_seq > 0)
	{
		(*offset) += 4;
	}
	if (cfg_routing > 0)
	{
#pragma unroll
		for (int i = 0; i < MAX_GRE_ROUTING_INFO; i++)
		{
			__u16 addr_family = load_half(skb, *offset);
			__u8 len_SRE = load_byte(skb, (*offset) + 3);

			*offset += (4 + len_SRE);
			if (addr_family == 0 && len_SRE == 0)
			{
				break;
			}
		}
	}
	if (cfg_ack > 0)
	{
		(*offset) += 4;
	}
	return protocol;
}

static inline void fill_network(struct __sk_buff *skb, __u16 netproto, size_t offset, int len, struct flow *flow)
{
	struct network_layer *layer = &flow->network_layer;
	__u8 transproto = 0;

	__u64 hash_src = 0;
	__u64 hash_dst = 0;

	__u64 ordered_src = 0;
	__u64 ordered_dst = 0;

	layer->_hash = FNV_BASIS;

	flow->key = flow->link_layer._hash;
	flow->key = rotl(flow->key, 16);

#define TUNNEL
#include "flow_network.c"

	layer->_hash_src = hash_src;
	if (ordered_src < ordered_dst)
	{
		layer->_hash = FNV_BASIS ^ rotl(hash_src, 32) ^ hash_dst ^ netproto ^ transproto;
	}
	else
	{
		layer->_hash = FNV_BASIS ^ rotl(hash_dst, 32) ^ hash_src ^ netproto ^ transproto;
	}

	flow->key ^= flow->network_layer_outer._hash;
	flow->key = rotl(flow->key, 16);
	flow->key ^= flow->network_layer._hash;
	flow->key = rotl(flow->key, 16);
	flow->key ^= flow->transport_layer._hash;
	flow->key = rotl(flow->key, 16);
	flow->key ^= flow->icmp_layer._hash;

	flow->layers_info |= NETWORK_LAYER_INFO;
}

static inline __u16 fill_vlan(struct __sk_buff *skb, struct l2 *l2)
{
	struct link_layer *layer = &l2->link_layer;

	__u16 tci = load_half(skb, l2->next_layer_offset + offsetof(struct vlan, tci));
	__u16 protocol = load_half(skb, l2->next_layer_offset + offsetof(struct vlan, ethertype));
	__u16 vlanID = tci & 0x0fff;

	__u64 hash_vlan = 0;
	update_hash_half(&hash_vlan, vlanID);

	layer->_hash ^= hash_vlan;
	layer->id = (layer->id << 12) | vlanID;
	add_layer_l2(l2, DOT1Q_LAYER);

	return protocol;
}

static inline void fill_vlans(struct __sk_buff *skb, __u16 *protocol, struct l2 *l2)
{
	if (*protocol == ETH_P_8021Q)
	{
#pragma unroll
		for (int i = 0; i < MAX_VLAN_LAYERS; i++)
		{
			*protocol = fill_vlan(skb, l2);
			l2->next_layer_offset += 4;
			if (*protocol != ETH_P_8021Q)
			{
				break;
			}
		}
	}

	struct link_layer *layer = &l2->link_layer;
	if (skb->vlan_present)
	{
		__u16 vlanID = skb->vlan_tci & 0x0fff;
		layer->_hash ^= vlanID;
		layer->id = (layer->id << 12) | vlanID;
		add_layer_l2(l2, DOT1Q_LAYER);
	}
}

static inline __u16 fill_link(struct __sk_buff *skb, size_t offset, struct l2 *l2)
{
	struct link_layer *layer = &l2->link_layer;

	fill_haddr(skb, offset + offsetof(struct ethhdr, h_source), layer->mac_src);
	fill_haddr(skb, offset + offsetof(struct ethhdr, h_dest), layer->mac_dst);

	update_hash_half(&layer->_hash_src, layer->mac_src[0] << 8 | layer->mac_src[1]);
	update_hash_half(&layer->_hash_src, layer->mac_src[2] << 8 | layer->mac_src[3]);
	update_hash_half(&layer->_hash_src, layer->mac_src[4] << 8 | layer->mac_src[5]);

	__u64 hash_dst = 0;
	update_hash_half(&hash_dst, layer->mac_dst[0] << 8 | layer->mac_dst[1]);
	update_hash_half(&hash_dst, layer->mac_dst[2] << 8 | layer->mac_dst[3]);
	update_hash_half(&hash_dst, layer->mac_dst[4] << 8 | layer->mac_dst[5]);

	__u16 ethertype = load_half(skb, offsetof(struct ethhdr, h_proto));
	layer->_hash = FNV_BASIS ^ layer->_hash_src ^ hash_dst;
	add_layer_l2(l2, ETH_LAYER);

	return ethertype;
}

static inline __u16 fill_l2(struct __sk_buff *skb, struct l2 *l2)
{
	__u16 protocol = bpf_ntohs((__u16)skb->protocol);

	if (((protocol == ETH_P_IP) || (protocol == ETH_P_IPV6)) && (skb->len >= 14))
	{
		__u16 eth_proto = load_half(skb, 12);
		if ((eth_proto == ETH_P_IP) || (eth_proto == ETH_P_IPV6))
		{
			protocol = ETH_P_ALL;
		}
	}

	if ((protocol != ETH_P_IP) && (protocol != ETH_P_IPV6))
	{
		protocol = fill_link(skb, 0, l2);
		l2->next_layer_offset = ETH_HLEN;

		fill_vlans(skb, &protocol, l2);

		if (protocol == ETH_P_ARP)
		{
			add_layer_l2(l2, ARP_LAYER);
		}
	}

	update_hash_half(&l2->link_layer._hash, protocol);
	l2->key = l2->link_layer._hash;
	return protocol;
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

	__u32 k32 = 0;
	struct l2 *l2 = bpf_map_lookup_element(&l2_table, &k32);
	if (l2 == NULL)
	{
		bpf_printk("=== l2 not found bpf_flow_table ===\n");
		return 0;
	}
	memset(l2, 0, sizeof(struct l2));
	l2->last = tm;

	l2->ethertype = fill_l2(skb, l2);

	bpf_tail_call(skb, &jmp_map, JMP_NETWORK_LAYER);
	bpf_printk("=== no tail call l2 key %x %x ====\n", l2->key, l2->link_layer.id);
	return 0;
}

SOCKET(network_layer)
int network_layer(struct __sk_buff *skb)
{
	__u32 k32 = 0;
	struct l2 *l2 = bpf_map_lookup_element(&l2_table, &k32);
	if (l2 == NULL)
	{
		bpf_printk("=== l2 not found network_layer ===\n");
		return 0;
	}

	struct flow *new, *flow;
	new = bpf_map_lookup_element(&flow_nostack, &k32);
	if (new == NULL)
	{
		bpf_printk("=== no free struct flow\n");
		return 0;
	}
	memset(new, 0, sizeof(struct flow));

	new->key = l2->key;
	new->layers_path = l2->layers_path;
	new->link_layer = l2->link_layer;
	new->last = l2->last;

	if (l2->next_layer_offset != 0)
	{
		new->layers_info |= LINK_LAYER_INFO;
	}

	/* L3/L4/... */
	fill_network(skb, l2->ethertype, l2->next_layer_offset, skb->len - l2->next_layer_offset, new);

	__u32 key = FLOW_PAGE;
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
	flow = bpf_map_lookup_element(flowtable, &new->key);

	if (flow == NULL)
	{
		/* New flow */
		new->start = new->last;
		update_metrics(skb, new, 1);

		if (bpf_map_update_element(flowtable, &new->key, new, BPF_ANY) == -1)
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
		return 0;
	}

	/* Update flow */
	flow->last = l2->last;
	update_metrics(skb, flow, is_ab_packet(new, flow));

	if (flow->layers_info & TRANSPORT_LAYER_INFO)
	{
#define update_transport_flags(ab, ba)                                         \
	do                                                                         \
	{                                                                          \
		if ((flow->transport_layer.ab == 0) && (new->transport_layer.ba != 0)) \
		{                                                                      \
			flow->transport_layer.ab = new->transport_layer.ba;                \
		}                                                                      \
	} while (0)

		if (flow->transport_layer.port_src == new->transport_layer.port_src)
		{
			update_transport_flags(ab_syn, ab_syn);
			update_transport_flags(ab_fin, ab_fin);
			update_transport_flags(ab_rst, ab_rst);
		}
		else
		{
			update_transport_flags(ba_syn, ab_syn);
			update_transport_flags(ba_fin, ab_fin);
			update_transport_flags(ba_rst, ab_rst);
		}
#undef update_transport_flags
	}

	return 0;
}

char _license[] LICENSE = "GPL";
