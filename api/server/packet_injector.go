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

package server

import (
	"errors"
	"fmt"

	"github.com/skydive-project/skydive/api/types"
	ge "github.com/skydive-project/skydive/gremlin/traversal"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/topology/graph"
)

type packetInjectorResourceHandler struct {
	ResourceHandler
}

// PacketInjectorAPI exposes the packet injector API
type PacketInjectorAPI struct {
	BasicAPIHandler
	Graph      *graph.Graph
	TrackingID chan string
}

func (pirh *packetInjectorResourceHandler) Name() string {
	return "injectpacket"
}

func (pirh *packetInjectorResourceHandler) New() types.Resource {
	return &types.PacketInjection{}
}

// Create allocates a new packet injection
func (pi *PacketInjectorAPI) Create(r types.Resource) error {
	ppr := r.(*types.PacketInjection)

	if err := pi.validateRequest(ppr); err != nil {
		return err
	}
	e := pi.BasicAPIHandler.Create(ppr)
	ppr.TrackingID = <-pi.TrackingID
	return e
}

func (pi *PacketInjectorAPI) validateRequest(ppr *types.PacketInjection) error {
	pi.Graph.RLock()
	defer pi.Graph.RUnlock()

	srcNode := pi.getNode(ppr.Src)
	dstNode := pi.getNode(ppr.Dst)

	if srcNode == nil {
		return fmt.Errorf("Not able to find a source node for '%s'", ppr.Src)
	}

	ipField := "IPV4"
	if ppr.Type == "icmp6" || ppr.Type == "tcp6" || ppr.Type == "udp6" {
		ipField = "IPV6"
	}

	if len(ppr.Pcap) == 0 {
		ips, _ := srcNode.GetFieldStringList(ipField)
		if len(ips) == 0 && ppr.SrcIP == "" {
			return errors.New("No source IP in node")
		}
		if dstNode == nil && ppr.DstIP == "" {
			return errors.New("No destination node and IP")
		}
		if ppr.DstIP == "" {
			ips, _ := dstNode.GetFieldStringList(ipField)
			if len(ips) == 0 {
				return errors.New("No destination IP in node")
			}
		}

		mac, _ := srcNode.GetFieldString("MAC")
		if mac == "" && ppr.SrcMAC == "" {
			return errors.New("No source MAC in node")
		}
		if dstNode == nil && ppr.DstMAC == "" {
			return errors.New("No destination node and MAC")
		}
		if ppr.DstMAC == "" {
			mac, _ := dstNode.GetFieldString("MAC")
			if mac == "" {
				return errors.New("No destination MAC in node")
			}
		}
	}
	return nil
}

func (pi *PacketInjectorAPI) getNode(gremlinQuery string) *graph.Node {
	res, err := ge.TopologyGremlinQuery(pi.Graph, gremlinQuery)
	if err != nil {
		return nil
	}

	for _, value := range res.Values() {
		switch value.(type) {
		case *graph.Node:
			return value.(*graph.Node)
		default:
			return nil
		}
	}
	return nil
}

// RegisterPacketInjectorAPI registers a new packet injector resource in the API
func RegisterPacketInjectorAPI(g *graph.Graph, apiServer *Server, authBackend shttp.AuthenticationBackend) (*PacketInjectorAPI, error) {
	pia := &PacketInjectorAPI{
		BasicAPIHandler: BasicAPIHandler{
			ResourceHandler: &packetInjectorResourceHandler{},
			EtcdKeyAPI:      apiServer.EtcdKeyAPI,
		},
		Graph:      g,
		TrackingID: make(chan string),
	}
	if err := apiServer.RegisterAPIHandler(pia, authBackend); err != nil {
		return nil, err
	}

	return pia, nil
}
