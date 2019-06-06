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

/* tail_call jump table */
enum {
	JMP_NETWORK_LAYER = 0,
	JMP_TABLE_SIZE,
};

enum {
	START_TIME_NS    = 0,
	FIRST_LAYER
};

enum {
	LINK_LAYER_INFO       = (1<<0),
	NETWORK_LAYER_INFO    = (1<<1),
	TRANSPORT_LAYER_INFO  = (1<<2),
	ICMP_LAYER_INFO       = (1<<3),
};


#define LAYERS_PATH_SHIFT 6
#define LAYERS_PATH_MASK 0x3fULL
#define LAYERS_PATH_LEN 10

#define ETH_LAYER   1
#define ARP_LAYER   2
#define DOT1Q_LAYER 3
#define IP4_LAYER   4
#define IP6_LAYER   5
#define ICMP4_LAYER 6
#define ICMP6_LAYER 7
#define UDP_LAYER   8
#define TCP_LAYER   9
#define SCTP_LAYER 10
#define GRE_LAYER  11
#define MAX_LAYERS (GRE_LAYER+1)

#if MAX_LAYERS >= (1<<LAYERS_PATH_SHIFT)
#error "Too much declared layers"
#endif

struct link_layer {
	__u8   protocol;	// currently only supporting ethernet
	__u8   mac_src[ETH_ALEN];
	__u8   mac_dst[ETH_ALEN];

	__u64 id;
	__u64 _hash;
	__u64 _hash_src;
};

struct network_layer {
	__u8   ip_src[16];
	__u8   ip_dst[16];
	__u16  protocol;

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

	__u64  ab_syn;
	__u64  ab_fin;
	__u64  ab_rst;
	__u64  ba_syn;
	__u64  ba_fin;
	__u64  ba_rst;

	__u64  _hash;
};

struct flow_metrics {
	__u64 ab_packets;
	__u64 ab_bytes;
	__u64 ba_packets;
	__u64 ba_bytes;
};

struct l2 {
	__u64                  key;
	__u64                  layers_path;

	__u32                  next_layer_offset;
	struct link_layer      link_layer;
	__u16                  ethertype;
	__u64                  last;
};

struct flow {
	__u64                  key;
	__u64                  key_outer;

	// define layers set in the flow
	__u64                  layers_path;

	// layers information
	__u8                   layers_info;
	struct link_layer      link_layer;
	struct network_layer   network_layer_outer;
	struct network_layer   network_layer;
	struct transport_layer transport_layer;
	struct icmp_layer      icmp_layer;

	struct flow_metrics    metrics;

	__u64                  start;
	__u64                  last;

	__u64                  _flags;
};

#endif
