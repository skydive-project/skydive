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

package client

import (
	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/http"

	"github.com/spf13/cobra"
)

var injectOpts = struct {
	SrcNode    string `flag:"src,source node gremlin expression (mandatory)"`
	DstNode    string `flag:"dst,destination node gremlin expression"`
	SrcIP      string `flag:"srcIP,source node IP"`
	DstIP      string `flag:"dstIP,destination node IP"`
	SrcMAC     string `flag:"srcMAC,source node MAC"`
	DstMAC     string `flag:"dstMAC,destination node MAC"`
	SrcPort    int    `flag:"srcPort,source port for TCP packet"`
	DstPort    int    `flag:"dstPort,destination port for TCP packet"`
	PacketType string `flag:"type,packet type: icmp4, icmp6, tcp4, tcp6, udp4 and udp6"`
	Payload    string `flag:"payload,payload"`
	ICMPID     int    `flag:"id,ICMP identification"`
	Increment  bool   `flag:"increment,increment ICMP id for each packet"`
	Count      int    `flag:"count,number of packets to be generated"`
	Interval   int    `flag:"interval,wait interval milliseconds between sending each packet"`
}{
	PacketType: "icmp4",
	Count:      1,
	Interval:   1000,
}

var packetInjectorCmd = crudRootCommand{
	Resource: "injectpacket",
	Command:  "inject-packet",
	Name:     "packet injection",

	Get: &getHandler{
		Run: func(client *http.CrudClient, id string) (types.Resource, error) {
			var inject types.PacketInjection
			err := client.Get("injectpacket", id, &inject)
			return &inject, err
		},
	},

	List: &listHandler{
		Run: func(client *http.CrudClient) (map[string]types.Resource, error) {
			var injections map[string]types.PacketInjection
			if err := client.List("injectpacket", &injections); err != nil {
				return nil, err
			}
			resources := make(map[string]types.Resource, len(injections))
			for _, inject := range injections {
				resources[inject.ID()] = &inject
			}
			return resources, nil
		},
	},

	Create: &createHandler{
		Flags: func(crud *crudRootCommand, c *cobra.Command) {
			crud.setFlags(c, &injectOpts)
		},

		Run: func(client *http.CrudClient) (types.Resource, error) {
			return &types.PacketInjection{
				Src:       injectOpts.SrcNode,
				Dst:       injectOpts.DstNode,
				SrcIP:     injectOpts.SrcIP,
				SrcMAC:    injectOpts.SrcMAC,
				SrcPort:   int64(injectOpts.SrcPort),
				DstIP:     injectOpts.DstIP,
				DstMAC:    injectOpts.DstMAC,
				DstPort:   int64(injectOpts.DstPort),
				Type:      injectOpts.PacketType,
				Payload:   injectOpts.Payload,
				ICMPID:    int64(injectOpts.ICMPID),
				Count:     int64(injectOpts.Count),
				Interval:  int64(injectOpts.Interval),
				Increment: injectOpts.Increment,
			}, nil
		},
	},
}
