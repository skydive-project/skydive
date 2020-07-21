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

package packetinjector

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net"

	apiServer "github.com/skydive-project/skydive/api/server"
	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/graffiti/api/rest"
	etcd "github.com/skydive-project/skydive/graffiti/etcd/client"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/graffiti/ondemand/client"
	ws "github.com/skydive-project/skydive/graffiti/websocket"
	ge "github.com/skydive-project/skydive/gremlin/traversal"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/validator"
)

const (
	min = 1024
	max = 65535
)

// Reply describes the reply to a packet injection request
type Reply struct {
	TrackingID string
	Error      string
}

// Client describes a packet injector client
type Client struct {
	etcd.MasterElection
	pool      ws.StructSpeakerPool
	watcher   rest.StoppableWatcher
	graph     *graph.Graph
	piHandler *apiServer.PacketInjectorAPI
}

type onDemandPacketInjectionHandler struct {
	graph *graph.Graph
}

func (h *onDemandPacketInjectionHandler) getNode(gremlinQuery string) *graph.Node {
	values := h.applyGremlinExpr(gremlinQuery)
	for _, value := range values {
		switch value.(type) {
		case *graph.Node:
			return value.(*graph.Node)
		default:
			return nil
		}
	}
	return nil
}

func (h *onDemandPacketInjectionHandler) createRequest(nodeID graph.Identifier, pi *types.PacketInjection) (string, *PacketInjectionRequest, error) {
	h.graph.RLock()
	defer h.graph.RUnlock()

	srcNode := h.graph.GetNode(graph.Identifier(nodeID))
	if srcNode == nil {
		return "", nil, errors.New("Not able to find a source node")
	}

	var dstNode *graph.Node
	if pi.Dst != "" {
		dstNode = h.getNode(pi.Dst)
	}

	srcMAC, _ := net.ParseMAC(pi.SrcMAC)
	dstMAC, _ := net.ParseMAC(pi.DstMAC)

	pir := &PacketInjectionRequest{
		BasicResource: rest.BasicResource{
			UUID: pi.UUID,
		},
		SrcIP:            net.ParseIP(pi.SrcIP),
		SrcMAC:           srcMAC,
		SrcPort:          pi.SrcPort,
		DstIP:            net.ParseIP(pi.DstIP),
		DstMAC:           dstMAC,
		DstPort:          pi.DstPort,
		Type:             pi.Type,
		Payload:          pi.Payload,
		Pcap:             pi.Pcap,
		Count:            pi.Count,
		Interval:         pi.Interval,
		ICMPID:           pi.ICMPID,
		Mode:             pi.Mode,
		IncrementPayload: pi.IncrementPayload,
		TTL:              pi.TTL,
	}

	if len(pir.Mode) == 0 {
		pir.Mode = types.PIModeUniqPerNode
	}

	if len(pir.Pcap) == 0 {
		ipField := "IPV4"
		if pir.Type == "icmp6" || pir.Type == "tcp6" || pir.Type == "udp6" {
			ipField = "IPV6"
		}

		if pir.SrcIP == nil {
			ips, _ := srcNode.GetFieldStringList("Neutron." + ipField)
			if len(ips) == 0 {
				ips, _ = srcNode.GetFieldStringList(ipField)
				if len(ips) == 0 {
					return "", nil, errors.New("No source IP in node and user input")
				}
			}
			pir.SrcIP, _, _ = net.ParseCIDR(ips[0])
		}

		if pir.DstIP == nil {
			if dstNode != nil {
				ips, _ := dstNode.GetFieldStringList("Neutron." + ipField)
				if len(ips) == 0 {
					ips, _ = dstNode.GetFieldStringList(ipField)
					if len(ips) == 0 {
						return "", nil, errors.New("No dest IP in node and user input")
					}
				}
				pir.DstIP, _, _ = net.ParseCIDR(ips[0])
			} else {
				return "", nil, errors.New("Not able to find a dest node and dest IP also empty")
			}
		}

		var err error
		if pir.SrcMAC == nil {
			if srcNode != nil {
				mac, _ := srcNode.GetFieldString("ExtID.attached-mac")
				if mac == "" {
					mac, _ = srcNode.GetFieldString("MAC")
					if mac == "" {
						return "", nil, errors.New("No source MAC in node and user input")
					}
				}
				if pir.SrcMAC, err = net.ParseMAC(mac); err != nil {
					return "", nil, err
				}
			} else {
				return "", nil, errors.New("Not able to find a source node and source MAC also empty")
			}
		}

		if pir.DstMAC == nil {
			var dstMAC string
			if nextHop, err := topology.GetNextHop(srcNode, pir.DstIP); err != nil || nextHop.MAC == "" {
				if dstNode != nil {
					if dstMAC, _ = dstNode.GetFieldString("ExtID.attached-mac"); dstMAC == "" {
						dstMAC, _ = dstNode.GetFieldString("MAC")
					}
				}
			} else {
				dstMAC = nextHop.MAC
			}

			if pir.DstMAC, err = net.ParseMAC(dstMAC); err != nil {
				return "", nil, errors.New("Failed to resolve destination MAC address")
			}
		}

		if pir.Type == "tcp4" || pir.Type == "tcp6" {
			if pir.SrcPort == 0 {
				pir.SrcPort = uint16(rand.Int63n(max-min) + min)
			}
			if pir.DstPort == 0 {
				pir.DstPort = uint16(rand.Int63n(max-min) + min)
			}
		}
	}

	if errs := validator.Validate("packetinjectionrequest", pir); errs != nil {
		return "", nil, fmt.Errorf("All the params were not set properly: %s", errs)
	}

	return srcNode.Host, pir, nil
}

func (h *onDemandPacketInjectionHandler) DecodeMessage(msg json.RawMessage) (rest.Resource, error) {
	var pi PacketInjectionRequest
	if err := json.Unmarshal(msg, &pi); err != nil {
		return nil, fmt.Errorf("Unable to decode packet injection: %s", err)
	}
	return &pi, nil
}

func (h *onDemandPacketInjectionHandler) EncodeMessage(nodeID graph.Identifier, resource rest.Resource) (json.RawMessage, error) {
	_, request, err := h.createRequest(nodeID, resource.(*types.PacketInjection))
	if err != nil {
		return nil, err
	}

	bytes, err := json.Marshal(request)
	return json.RawMessage(bytes), err
}

func (h *onDemandPacketInjectionHandler) CheckState(node *graph.Node, resource rest.Resource) bool {
	injection := resource.(*types.PacketInjection)
	if injections, err := node.GetField("PacketInjections"); err == nil {
		for _, i := range *injections.(*Injections) {
			if i.ID == injection.UUID && i.State == "active" {
				return true
			}
		}
	}
	return false
}

func (h *onDemandPacketInjectionHandler) ResourceName() string {
	return "PacketInjection"
}

func (h *onDemandPacketInjectionHandler) GetNodeResources(resource rest.Resource) []client.OnDemandNodeResource {
	var nrs []client.OnDemandNodeResource

	pi := resource.(*types.PacketInjection)

	query := pi.Src
	query += fmt.Sprintf(".Dedup('TID').Has('PacketInjections.ID', NEE('%s'))", resource.GetID())

	if nodes := h.applyGremlinExpr(query); len(nodes) > 0 {
		id := pi.ICMPID
		srcPort := pi.SrcPort

		if id == 0 {
			id++
		}

		if srcPort == 0 {
			srcPort++
		}

		addNrs := func(n *graph.Node) {
			r := *pi
			r.ICMPID = id
			r.SrcPort = srcPort

			nrs = append(nrs, client.OnDemandNodeResource{Node: n, Resource: &r})

			if r.Mode == types.PIModeUniqPerNode {
				switch r.Type {
				case types.PITypeICMP4, types.PITypeICMP6:
					id++
				default:
					srcPort++
				}
			}
		}

		for _, i := range nodes {
			switch i.(type) {
			case *graph.Node:
				addNrs(i.(*graph.Node))
			case []*graph.Node:
				// case of shortestpath that returns a list of nodes
				for _, node := range i.([]*graph.Node) {
					addNrs(node)
				}
			}
		}
	}

	return nrs
}

func (h *onDemandPacketInjectionHandler) applyGremlinExpr(query string) []interface{} {
	res, err := ge.TopologyGremlinQuery(h.graph, query)
	if err != nil {
		logging.GetLogger().Errorf("Gremlin %s error: %s", query, err)
		return nil
	}
	return res.Values()
}

// NewOnDemandInjectionClient creates a new ondemand client based on API, graph and websocket
func NewOnDemandInjectionClient(g *graph.Graph, ch rest.WatchableHandler, agentPool ws.StructSpeakerPool, subscriberPool ws.StructSpeakerPool, etcdClient *etcd.Client) *client.OnDemandClient {
	return client.NewOnDemandClient(g, ch, agentPool, subscriberPool, etcdClient, &onDemandPacketInjectionHandler{graph: g})
}
