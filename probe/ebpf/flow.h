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

#ifndef __TABLE_H
#define __TABLE_H

#include <linux/if_ether.h>

enum {
	START_TIME_NS    = 0,

	LINK_LAYER       = 1,
	NETWORK_LAYER    = 2,
	TRANSPORT_LAYER  = 4,
	ICMP_LAYER       = 8,

	PAYLOAD_LENGTH   = 30
};

struct link_layer {
	__u8   protocol;	// currenlty only supporting ethernet
	__u8   mac_src[ETH_ALEN];
	__u8   mac_dst[ETH_ALEN];

	__u64 _hash;
	__u64 _hash_src;
};

struct network_layer {
	__u16  protocol;
	__u8   ip_src[16];
	__u8   ip_dst[16];

	__u64  _hash;
	__u64  _hash_src;
};

struct icmp_layer {
	__u8   kind;
	__u32  code;
	__u32  id;

	__u64  _hash;
};

struct transport_layer {
	__u8   protocol;
	__be16 port_src;
	__be16 port_dst;

	__u64  _hash;
};

struct flow_metrics {
	__u64 ab_packets;
	__u64 ab_bytes;
	__u64 ba_packets;
	__u64 ba_bytes;
};

struct flow {
	__u64                  key;

	// define layers set in the flow
	__u8                   layers;

	// ifIndex of the interface seing the packet
	__u32                  ifindex;

	// layers information
	struct link_layer      link_layer;
	struct network_layer   network_layer;
	struct transport_layer transport_layer;
	struct icmp_layer      icmp_layer;

	struct flow_metrics    metrics;

	__u64                  start;
	__u64                  last;

	__u8                   payload[PAYLOAD_LENGTH];

	__u64                  _flags;
};

#endif
