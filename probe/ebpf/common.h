/*
 * Copyright (C) 2019 Red Hat, Inc.
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

#ifndef __COMMON_H
#define __COMMON_H

// Fowler/Noll/Vo hash
#define FNV_BASIS ((__u64)14695981039346656037U)
#define FNV_PRIME ((__u64)1099511628211U)

#define IP_MF 0x2000
#define IP_OFFSET 0x1FFF

#define MAX_VLAN_LAYERS 5

struct vlan
{
        __u16 tci;
        __u16 ethertype;
};

struct sctp
{
        __be16 src;
        __be16 dst;
        __be32 tag;
        __be32 checksum;
};

#ifndef offsetof
#define offsetof(TYPE, MEMBER) __builtin_offsetof(TYPE, MEMBER)
#endif
#ifndef size_t
#define size_t long
#endif
#ifndef NULL
#define NULL ((void *)0)
#endif

static inline __u64 rotl(__u64 value, unsigned int shift)
{
        return (value << shift) | (value >> (64 - shift));
}

#define __update_hash(key, data)       \
        do                             \
        {                              \
                *key ^= (__u64)(data); \
                *key *= FNV_PRIME;     \
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

static inline void add_layer(struct flow *flow, __u8 layer)
{
        if (flow->layers_path & (LAYERS_PATH_MASK << ((LAYERS_PATH_LEN - 1) * LAYERS_PATH_SHIFT)))
        {
                return;
        }
        flow->layers_path = (flow->layers_path << LAYERS_PATH_SHIFT) | layer;
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

static inline void fill_transport(struct __sk_buff *skb, __u8 protocol, size_t offset, int len,
                                  struct flow *flow, __u8 swap, __u8 netequal)
{
        struct transport_layer *layer = &flow->transport_layer;

        layer->protocol = protocol;
        layer->port_src = load_half(skb, offset);
        layer->port_dst = load_half(skb, offset + sizeof(__be16));

        __u64 hash_src = 0;
        update_hash_half(&hash_src, layer->port_src);
        __u64 hash_dst = 0;
        update_hash_half(&hash_dst, layer->port_dst);
        if (netequal)
        {
                swap = layer->port_src > layer->port_dst;
        }

        switch (protocol)
        {
        case IPPROTO_SCTP:
                add_layer(flow, SCTP_LAYER);
                break;
        case IPPROTO_UDP:
                add_layer(flow, UDP_LAYER);
                break;
        case IPPROTO_TCP:
        {
                __u64 tm = flow->last;
                __u8 flags = load_byte(skb, offset + 13);
                add_layer(flow, TCP_LAYER);
                layer->ab_rst = (flags & 0x04) ? tm : 0;
                layer->ab_syn = (flags & 0x02) ? tm : 0;
                layer->ab_fin = (flags & 0x01) ? tm : 0;
                break;
        }
        }

        if (swap)
        {
                layer->_hash = FNV_BASIS ^ rotl(hash_dst, 16) ^ hash_src;
        }
        else
        {
                layer->_hash = FNV_BASIS ^ rotl(hash_src, 16) ^ hash_dst;
        }

        flow->layers_info |= TRANSPORT_LAYER_INFO;
}

static inline void fill_icmpv4(struct __sk_buff *skb, int offset, struct flow *flow)
{
        struct icmp_layer *layer = &flow->icmp_layer;

        layer->kind = load_byte(skb, offset + offsetof(struct icmphdr, type));
        layer->code = load_byte(skb, offset + offsetof(struct icmphdr, code));

        __u64 hash = 0;
        update_hash_byte(&hash, layer->code);

        switch (layer->kind)
        {
        case ICMP_ECHO:
        case ICMP_ECHOREPLY:
                update_hash_byte(&hash, ICMP_ECHO | ICMP_ECHOREPLY);

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

        switch (layer->kind)
        {
        case ICMPV6_ECHO_REQUEST:
        case ICMPV6_ECHO_REPLY:
                update_hash_byte(&hash, ICMPV6_ECHO_REQUEST | ICMPV6_ECHO_REPLY);

                layer->id = load_half(skb, offset + offsetof(struct icmp6hdr, icmp6_dataun.u_echo.identifier));

                update_hash_byte(&hash, layer->id);
                break;
        }

        layer->_hash = FNV_BASIS ^ hash;

        add_layer(flow, ICMP6_LAYER);
        flow->layers_info |= ICMP_LAYER_INFO;
}

static inline void update_metrics(struct __sk_buff *skb, struct flow *flow, int ab)
{
        if (ab)
        {
                __sync_fetch_and_add(&flow->metrics.ab_packets, 1);
                __sync_fetch_and_add(&flow->metrics.ab_bytes, skb->len);
        }
        else
        {
                __sync_fetch_and_add(&flow->metrics.ba_packets, 1);
                __sync_fetch_and_add(&flow->metrics.ba_bytes, skb->len);
        }
}

static inline int is_ab_packet(struct flow *flow, struct flow *prev)
{
        int cmp = flow->link_layer._hash_src == prev->link_layer._hash_src;
        if (memcmp(flow->link_layer.mac_src, flow->link_layer.mac_dst, ETH_ALEN) == 0)
        {
                cmp = flow->network_layer._hash_src == prev->network_layer._hash_src;
                if (memcmp(flow->network_layer.ip_src, flow->network_layer.ip_dst, 16) == 0)
                {
                        cmp = flow->transport_layer.port_src > flow->transport_layer.port_dst;
                }
        }
        return cmp;
}

#endif
