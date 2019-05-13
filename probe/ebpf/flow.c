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

#include "bpf.h"
#include "flow.h"

#ifndef offsetof
#define offsetof(TYPE, MEMBER) __builtin_offsetof (TYPE, MEMBER)
#endif
#ifndef size_t
#define size_t long
#endif
#ifndef NULL
#define NULL ((void*)0)
#endif


// Fowler/Noll/Vo hash
#define FNV_BASIS (14695981039346656037ULL)
#define FNV_PRIME (1099511628211ULL)

#define IP_MF     0x2000
#define IP_OFFSET	0x1FFF

#define MAX_VLAN_LAYERS 5
struct vlan {
	__be16		tci;
	__be16		ethertype;
};

struct sctp {
        __be16 src;
        __be16 dst;
        __be32 tag;
        __be32 checksum;
};

#define MAX_GRE_ROUTING_INFO 4

MAP(flow_nostack) {
	.type = BPF_MAP_TYPE_PERCPU_ARRAY,
	.key_size = sizeof(__u32),
	.value_size = sizeof(struct flow),
	.max_entries = 1,
};

MAP(jmp_map) {
	.type = BPF_MAP_TYPE_PROG_ARRAY,
	.key_size = sizeof(__u32),
	.value_size = sizeof(__u32),
	.max_entries = JMP_TABLE_SIZE,
};

MAP(u64_config_values) {
	.type = BPF_MAP_TYPE_ARRAY,
	.key_size = sizeof(__u32),
	.value_size = sizeof(__u64),
	.max_entries = 1,
};

MAP(l2_table) {
	.type = BPF_MAP_TYPE_ARRAY,
	.key_size = sizeof(__u32),
	.value_size = sizeof(struct l2),
	.max_entries = 1,
};

MAP(flow_table) {
	.type = BPF_MAP_TYPE_HASH,
	.key_size = sizeof(__u64),
	.value_size = sizeof(struct flow),
	.max_entries = 500000,
};

#define __update_hash(key, data) do { \
	*key ^= (__u64)(data);	      \
	*key *= FNV_PRIME;            \
} while (0)

static inline void update_hash_byte(__u64 *key, __u8 byte)
{
	__update_hash(key, byte);
}

//FIXME(nplanel) bad code generated (llvm) (stack overflow)
#if 0
static inline void update_hash_half(__u64 *key, __u16 half)
{
	__update_hash(key, half >> 8);
	__update_hash(key, half & 0xff);
}

static inline void update_hash_word(__u64 *key, __u32 word)
{
	__update_hash(key, word >> 24);
	__update_hash(key, (word >> 16) & 0xff);
	__update_hash(key, (word >> 8) & 0xff);
	__update_hash(key, word & 0xff);
}
#else
static inline void update_hash_half(__u64 *key, __u16 half)
{
	__update_hash(key, half);
}

static inline void update_hash_word(__u64 *key, __u32 word)
{
	__update_hash(key, word);
}
#endif

static inline void add_layer_l2(struct l2 *l2, __u8 layer) {
	if (l2->layers_path & (LAYERS_PATH_MASK << ((LAYERS_PATH_LEN-1)*LAYERS_PATH_SHIFT))) {
		return;
	}
	l2->layers_path = (l2->layers_path << LAYERS_PATH_SHIFT) | layer;
}

static inline void add_layer(struct flow *flow, __u8 layer) {
	if (flow->layers_path & (LAYERS_PATH_MASK << ((LAYERS_PATH_LEN-1)*LAYERS_PATH_SHIFT))) {
		return;
	}
	flow->layers_path = (flow->layers_path << LAYERS_PATH_SHIFT) | layer;
}

static inline void fill_payload(struct __sk_buff *skb, size_t offset, struct flow *flow, int len)
{
//	bpf_skb_load_bytes(skb, offset, flow->payload, sizeof(flow->payload));
}

static inline __u16 fill_gre(struct __sk_buff *skb, size_t *offset, struct flow *flow)
{
	__u8 config = load_byte(skb, *offset);
	__u8 cfg_checksum = config & 0x80;
	__u8 cfg_routing = config & 0x40;
	__u8 cfg_key = config & 0x20;
	__u8 cfg_seq = config & 0x10;
	__u8 flags = load_byte(skb, (*offset)+1);
	__u8 cfg_ack = flags & 0x80;

	__u16 protocol = load_half(skb, (*offset)+2);
	(*offset) += 4;
	if ((cfg_checksum > 0) || (cfg_routing > 0)) {
		(*offset) += 4;
	}
	if (cfg_key > 0) {
		(*offset) += 4;
	}
	if (cfg_seq > 0) {
		(*offset) += 4;
	}
	if (cfg_routing > 0) {
#pragma unroll
		for(int i = 0;i < MAX_GRE_ROUTING_INFO; i++) {
			__u16 addr_family = load_half(skb, *offset);
			__u8 len_SRE = load_byte(skb, (*offset)+3);

			*offset += (4 + len_SRE);
			if (addr_family == 0 && len_SRE == 0) {
				break;
			}
		}
	}
	if (cfg_ack > 0) {
		(*offset) += 4;
	}
	return protocol;
}

static inline void fill_transport(struct __sk_buff *skb, __u8 protocol, size_t offset, int len,
	struct flow *flow)
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
		case IPPROTO_SCTP:
			add_layer(flow, SCTP_LAYER);
			offset += sizeof(struct sctp);
			len -= sizeof(struct sctp);
			fill_payload(skb, offset, flow, len);
			break;
		case IPPROTO_UDP:
			add_layer(flow, UDP_LAYER);
			offset += sizeof(struct udphdr);
			len -= sizeof(struct udphdr);
			fill_payload(skb, offset, flow, len);
			break;
		case IPPROTO_TCP:
		{
			__u64 tm = flow->last;
			__u8 flags = load_byte(skb, offset + 13);
			add_layer(flow, TCP_LAYER);
			layer->ab_rst = (flags & 0x04) ? tm : 0;
			layer->ab_syn = (flags & 0x02) ? tm : 0;
			layer->ab_fin = (flags & 0x01) ? tm : 0;
			offset += sizeof(struct tcphdr);
			len -= sizeof(struct tcphdr);
			fill_payload(skb, offset, flow, len);
			break;
		}
	}

	layer->_hash = FNV_BASIS ^ hash_src ^ hash_dst;

	flow->layers_info |= TRANSPORT_LAYER_INFO;
}

static inline void fill_icmpv4(struct __sk_buff *skb, size_t offset, struct flow *flow)
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

static inline void fill_icmpv6(struct __sk_buff *skb, size_t offset, struct flow *flow)
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

static inline void fill_word(__u32 src, __u8 *dst, size_t offset)
{
	dst[offset] = (src >> 24) & 0xff;
	dst[offset + 1] = (src >> 16) & 0xff;
	dst[offset + 2] = (src >> 8) & 0xff;
	dst[offset + 3] = src & 0xff;
}

static inline void fill_ipv4(struct __sk_buff *skb, size_t offset, __u8 *dst, __u64 *hash)
{
	__u32 w = load_word(skb, offset);
	fill_word(w, dst, 12);
	update_hash_word(hash, w);
}

static inline void fill_ipv6(struct __sk_buff *skb, size_t offset, __u8 *dst, __u64 *hash)
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

static inline void fill_network(struct __sk_buff *skb, __u16 netproto, size_t offset, int len, struct flow *flow)
{
	struct network_layer *layer = &flow->network_layer;
	__u8 transproto = 0;

	__u64 hash_src = 0;
	__u64 hash_dst = 0;

	layer->_hash = FNV_BASIS;

#define TUNNEL
#include "flow_network.c"

	layer->_hash_src = hash_src;
	layer->_hash ^= hash_src ^ hash_dst ^ netproto ^ transproto;

	flow->key ^= flow->network_layer_outer._hash ^ flow->network_layer._hash ^
		flow->transport_layer._hash ^ flow->icmp_layer._hash;

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

static inline void fill_vlans(struct __sk_buff *skb, __u16 *protocol, struct l2 *l2) {
	if (*protocol == ETH_P_8021Q) {
		#pragma unroll
		for(int i=0;i<MAX_VLAN_LAYERS;i++) {
			*protocol = fill_vlan(skb, l2);
			l2->next_layer_offset += 4;
			if (*protocol != ETH_P_8021Q) {
				break;
			}
		}
	}

	struct link_layer *layer = &l2->link_layer;
	if (skb->vlan_present) {
		__u16 vlanID = skb->vlan_tci & 0x0fff;
		layer->_hash ^= vlanID;
		layer->id = (layer->id << 12) | vlanID;
		add_layer_l2(l2, DOT1Q_LAYER);
	}
}

static inline void fill_haddr(struct __sk_buff *skb, size_t offset,
	unsigned char *mac)
{
	mac[0] = load_byte(skb, offset);
	mac[1] = load_byte(skb, offset + 1);
	mac[2] = load_byte(skb, offset + 2);
	mac[3] = load_byte(skb, offset + 3);
	mac[4] = load_byte(skb, offset + 4);
	mac[5] = load_byte(skb, offset + 5);
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

static inline void update_metrics(struct __sk_buff *skb, struct flow *flow, int ab)
{
	if (ab) {
		__sync_fetch_and_add(&flow->metrics.ab_packets, 1);
		__sync_fetch_and_add(&flow->metrics.ab_bytes, skb->len);
	} else {
		__sync_fetch_and_add(&flow->metrics.ba_packets, 1);
		__sync_fetch_and_add(&flow->metrics.ba_bytes, skb->len);
	}
}

static inline __u16 fill_l2(struct __sk_buff *skb, struct l2 *l2)
{
	__u16 protocol = bpf_ntohs((__u16)skb->protocol);

	if (((protocol == ETH_P_IP) || (protocol == ETH_P_IPV6)) && (skb->len >= 14)) {
		__u16 eth_proto = load_half(skb, 12);
		if ((eth_proto == ETH_P_IP) || (eth_proto == ETH_P_IPV6)) {
			protocol = ETH_P_ALL;
		}
	}

	if ((protocol != ETH_P_IP) && (protocol != ETH_P_IPV6)) {
		protocol = fill_link(skb, 0, l2);
		l2->next_layer_offset = ETH_HLEN;

		fill_vlans(skb, &protocol, l2);

		if (protocol == ETH_P_ARP) {
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
	if (sns != NULL && *sns == 0) {
		bpf_map_update_element(&u64_config_values, &key, &tm, BPF_ANY);
	}

	__u32 k32 = 0;
	struct l2 *l2 = bpf_map_lookup_element(&l2_table, &k32);
	if (l2 == NULL) {
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
	if (l2 == NULL) {
		bpf_printk("=== l2 not found network_layer ===\n");
		return 0;
	}

	struct flow *new, *flow;
	new = bpf_map_lookup_element(&flow_nostack, &k32);
	if (new == NULL) {
		bpf_printk("=== no free struct flow\n");
		return 0;
	}
	memset(new, 0, sizeof(struct flow));
	
	new->key = l2->key;
	new->layers_path = l2->layers_path;
	new->link_layer = l2->link_layer;
	new->last = l2->last;

	if (l2->next_layer_offset != 0) {
		new->layers_info |= LINK_LAYER_INFO;
	}

	/* L3/L4/... */
	fill_network(skb, l2->ethertype, l2->next_layer_offset, skb->len - l2->next_layer_offset, new);

	flow = bpf_map_lookup_element(&flow_table, &new->key);
	if (flow == NULL) {
		/* New flow */
		new->start = new->last;
		update_metrics(skb, new, 1);
		bpf_map_update_element(&flow_table, &new->key, new, BPF_ANY);

		return 0;
	}

	/* Update flow */
	int ab;
	int first_layer_ip = (l2->next_layer_offset == 0);
	if (first_layer_ip == 0) {
		ab = new->link_layer._hash_src == flow->link_layer._hash_src;
	} else {
		ab = new->network_layer._hash_src == flow->network_layer._hash_src;
	}
	update_metrics(skb, flow, ab);
	if (flow->layers_info & TRANSPORT_LAYER_INFO) {
#define update_transport_flags(ab, ba)					\
		do {							\
			if ((flow->transport_layer.ab == 0) && (new->transport_layer.ba != 0)) { \
				flow->transport_layer.ab = new->transport_layer.ba; \
			}						\
		} while (0)

		if (flow->transport_layer.port_src == new->transport_layer.port_src) {
			update_transport_flags(ab_syn, ab_syn);
			update_transport_flags(ab_fin, ab_fin);
			update_transport_flags(ab_rst, ab_rst);
		} else {
			update_transport_flags(ba_syn, ab_syn);
			update_transport_flags(ba_fin, ab_fin);
			update_transport_flags(ba_rst, ab_rst);
		}
#undef update_transport_flags
	}

	return 0;
}

char _license[] LICENSE = "GPL";

