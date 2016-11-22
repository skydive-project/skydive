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

package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/abbot/go-http-auth"
	"github.com/gorilla/mux"

	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/flow/storage"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
)

type FlowApi struct {
	Service   string
	FlowTable *flow.Table
	Storage   storage.Storage
}

func (f *FlowApi) flowSearch(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
	andFilter := &flow.BoolFilter{Op: flow.BoolFilterOp_AND}
	for k, v := range r.URL.Query() {
		andFilter.Filters = append(andFilter.Filters,
			&flow.Filter{
				TermStringFilter: &flow.TermStringFilter{Key: k, Value: v[0]},
			},
		)
	}
	filter := &flow.Filter{BoolFilter: andFilter}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	if f.Storage == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	flows, err := f.Storage.SearchFlows(flow.FlowSearchQuery{Filter: filter})
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(flows); err != nil {
		panic(err)
	}
}

func (f *FlowApi) serveDataIndex(w http.ResponseWriter, r *auth.AuthenticatedRequest, message string) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(message))
}

func (f *FlowApi) jsonFlowConversation(layer string) string {
	//	{"nodes":[{"name":"Myriel","group":1}, ... ],"links":[{"source":1,"target":0,"value":1},...]}
	nodes := []string{}
	links := []string{}

	pathMap := make(map[string]int)
	layerMap := make(map[string]int)

	for _, f := range f.FlowTable.GetFlows(nil).Flows {
		var layerFlow *flow.FlowLayer
		if layer == "ethernet" && f.Link != nil && f.Link.Protocol == flow.FlowProtocol_ETHERNET {
			layerFlow = f.Link
		} else if layer == "ipv4" && f.Network != nil && f.Network.Protocol == flow.FlowProtocol_IPV4 {
			layerFlow = f.Network
		} else if layer == "ipv6" && f.Network != nil && f.Network.Protocol == flow.FlowProtocol_IPV6 {
			layerFlow = f.Network
		} else if layer == "tcp" && f.Transport != nil && f.Transport.Protocol == flow.FlowProtocol_TCPPORT {
			layerFlow = f.Transport
		} else if layer == "udp" && f.Transport != nil && f.Transport.Protocol == flow.FlowProtocol_UDPPORT {
			layerFlow = f.Transport
		} else if layer == "sctp" && f.Transport != nil && f.Transport.Protocol == flow.FlowProtocol_SCTPPORT {
			layerFlow = f.Transport
		}

		if layerFlow == nil {
			continue
		}

		if _, found := pathMap[f.LayersPath]; found {
			pathMap[f.LayersPath] = len(pathMap)
		}

		AB := layerFlow.A
		BA := layerFlow.B

		if _, found := layerMap[AB]; !found {
			layerMap[AB] = len(layerMap)
			nodes = append(nodes, fmt.Sprintf(`{"name":"%s","group":%d}`, AB, pathMap[f.LayersPath]))
		}
		if _, found := layerMap[BA]; !found {
			layerMap[BA] = len(layerMap)
			nodes = append(nodes, fmt.Sprintf(`{"name":"%s","group":%d}`, BA, pathMap[f.LayersPath]))
		}

		link := fmt.Sprintf(`{"source":%d,"target":%d,"value":%d}`, layerMap[AB], layerMap[BA], f.Metric.ABBytes+f.Metric.BABytes)
		links = append(links, link)
	}

	return fmt.Sprintf(`{"nodes":[%s], "links":[%s]}`, strings.Join(nodes, ","), strings.Join(links, ","))
}

func (f *FlowApi) conversationLayer(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
	vars := mux.Vars(&r.Request)
	f.serveDataIndex(w, r, f.jsonFlowConversation(vars["layer"]))
}

type discoType int

const (
	bytes discoType = 1 + iota
	packets
)

type discoNode struct {
	name     string
	size     int64
	children map[string]*discoNode
}

func (d *discoNode) marshalJSON() ([]byte, error) {
	str := "{"
	str += fmt.Sprintf(`"name":"%s",`, d.name)
	if d.size > 0 {
		str += fmt.Sprintf(`"size": %d,`, d.size)
	}
	str += fmt.Sprintf(`"children": [`)
	idx := 0
	for _, child := range d.children {
		bytes, err := child.marshalJSON()
		if err != nil {
			return []byte(str), err
		}
		str += string(bytes)
		if idx != len(d.children)-1 {
			str += ","
		}
		idx++
	}
	str += "]"
	str += "}"
	return []byte(str), nil
}

func newDiscoNode() *discoNode {
	return &discoNode{
		children: make(map[string]*discoNode),
	}
}

type flowMetricStat struct {
	Bytes   int64
	Packets int64
}

func (f *FlowApi) jsonFlowDiscovery(DiscoType discoType) string {
	// {"name":"root","children":[{"name":"Ethernet","children":[{"name":"IPv4","children":
	//		[{"name":"UDP","children":[{"name":"Payload","size":360,"children":[]}]},
	//     {"name":"TCP","children":[{"name":"Payload","size":240,"children":[]}]}]}]}]}

	pathMap := make(map[string]flowMetricStat)

	for _, f := range f.FlowTable.GetFlows(nil).Flows {
		eth := f.Metric
		if eth == nil || f.Link.Protocol != flow.FlowProtocol_ETHERNET {
			continue
		}

		p, _ := pathMap[f.LayersPath]
		p.Bytes += eth.ABBytes
		p.Bytes += eth.BABytes
		p.Packets += eth.ABPackets
		p.Packets += eth.BAPackets
		pathMap[f.LayersPath] = p
	}

	root := newDiscoNode()
	root.name = "root"
	for path, stat := range pathMap {
		node := root
		layers := strings.Split(path, "/")
		for i, layer := range layers {
			l, found := node.children[layer]
			if !found {
				node.children[layer] = newDiscoNode()
				l = node.children[layer]
				l.name = layer
			}
			if len(layers)-1 == i {
				switch DiscoType {
				case bytes:
					l.size = stat.Bytes
				case packets:
					l.size = stat.Packets
				}
			}
			node = l
		}
	}

	bytes, err := root.marshalJSON()
	if err != nil {
		logging.GetLogger().Fatal(err)
	}
	return string(bytes)
}

func (f *FlowApi) discoveryType(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
	vars := mux.Vars(&r.Request)
	discoType := vars["type"]
	dtype := bytes
	switch discoType {
	case "bytes":
		dtype = bytes
	case "packets":
		dtype = packets
	}
	f.serveDataIndex(w, r, f.jsonFlowDiscovery(dtype))
}
func (f *FlowApi) registerEndpoints(r *shttp.Server) {
	routes := []shttp.Route{
		{
			"FlowSearch",
			"GET",
			"/api/flow/search",
			f.flowSearch,
		},
		{
			"ConversationLayer",
			"GET",
			"/api/flow/conversation/{layer}",
			f.conversationLayer,
		},
		{
			"Discovery",
			"GET",
			"/api/flow/discovery/{type}",
			f.discoveryType,
		},
	}

	r.RegisterRoutes(routes)
}

func RegisterFlowApi(s string, f *flow.Table, st storage.Storage, r *shttp.Server) {
	fa := &FlowApi{
		Service:   s,
		FlowTable: f,
		Storage:   st,
	}

	fa.registerEndpoints(r)
}
