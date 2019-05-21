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

	layer->protocol = netproto;
	switch (netproto) {
		case ETH_P_IP:
		{
			add_layer(flow, IP4_LAYER);
			__u8 verlen = load_byte(skb, offset);
			__be16 frag = load_half(skb, offset + offsetof(struct iphdr, frag_off));
			if (frag  & (IP_MF | IP_OFFSET)) {
				// TODO report fragment
				return;
			}

			transproto = load_byte(skb, offset + offsetof(struct iphdr, protocol));
			fill_ipv4(skb, offset + offsetof(struct iphdr, saddr), layer->ip_src, &hash_src);
			fill_ipv4(skb, offset + offsetof(struct iphdr, daddr), layer->ip_dst, &hash_dst);

			offset += (verlen & 0xF) << 2;
			len -= (verlen & 0xF) << 2;
		}
			break;
		case ETH_P_IPV6:
			add_layer(flow, IP6_LAYER);
			transproto = load_byte(skb, offset + offsetof(struct ipv6hdr, nexthdr));
			fill_ipv6(skb, offset + offsetof(struct ipv6hdr, saddr), layer->ip_src, &hash_src);
			fill_ipv6(skb, offset + offsetof(struct ipv6hdr, daddr), layer->ip_dst, &hash_dst);

			// TODO(nplanel) skip optional headers
			offset += sizeof(struct ipv6hdr);
			len -= sizeof(struct ipv6hdr);
			break;
		default:
			return;
	}

	switch (transproto) {
		case IPPROTO_SCTP:
		case IPPROTO_UDP:
		case IPPROTO_TCP:
			fill_transport(skb, transproto, offset, len, flow);
			break;
		case IPPROTO_ICMP:
			fill_icmpv4(skb, offset, flow);
			break;
		case IPPROTO_ICMPV6:
			fill_icmpv6(skb, offset, flow);
			break;
#ifdef TUNNEL
#undef TUNNEL
	case IPPROTO_GRE:
	{
		layer->_hash_src = hash_src;
		layer->_hash ^= hash_src ^ hash_dst ^ netproto ^ transproto;
		flow->network_layer_outer = flow->network_layer;

		layer->_hash = FNV_BASIS;
		flow->key_outer = flow->key;
		flow->key_outer ^= flow->network_layer._hash ^ IPPROTO_GRE;

		add_layer(flow, GRE_LAYER);
		netproto = fill_gre(skb, &offset, flow);
		{
#include "flow_network.c"
		}
		break;
	}
#endif
	}
