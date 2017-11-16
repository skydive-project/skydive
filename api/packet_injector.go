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
	"errors"
	"math/rand"
	"net/http"
	"strings"

	"github.com/abbot/go-http-auth"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/packet_injector"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/graph"
	"github.com/skydive-project/skydive/validator"
)

const (
	min = 1024
	max = 65535
)

// PacketInjectorAPI exposes the packet injector API
type PacketInjectorAPI struct {
	PIClient *packet_injector.PacketInjectorClient
	Graph    *graph.Graph
}

// PacketParamsReq packet injector API parameters
type PacketParamsReq struct {
	Src        string
	Dst        string
	SrcIP      string
	DstIP      string
	SrcMAC     string
	DstMAC     string
	SrcPort    int64
	DstPort    int64
	Type       string
	Payload    string
	TrackingID string
	ID         int64
	Count      int64
	Interval   int64
}

func (pi *PacketInjectorAPI) normalizeIP(ip, ipFamily string) string {
	if strings.Contains(ip, "/") {
		return ip
	}
	if ipFamily == "IPV4" {
		return ip + "/32"
	}
	return ip + "/64"
}

func (pi *PacketInjectorAPI) requestToParams(ppr *PacketParamsReq) (string, *packet_injector.PacketParams, error) {
	pi.Graph.RLock()
	defer pi.Graph.RUnlock()

	srcNode := pi.getNode(ppr.Src)
	dstNode := pi.getNode(ppr.Dst)

	if srcNode == nil {
		return "", nil, errors.New("Not able to find a source node")
	}

	ipField := "IPV4"
	if ppr.Type == "icmp6" || ppr.Type == "tcp6" || ppr.Type == "udp6" {
		ipField = "IPV6"
	}

	if ppr.SrcIP == "" {
		ips, _ := srcNode.GetFieldStringList(ipField)
		if len(ips) == 0 {
			return "", nil, errors.New("No source IP in node and user input")
		}
		ppr.SrcIP = ips[0]
	} else {
		ppr.SrcIP = pi.normalizeIP(ppr.SrcIP, ipField)
	}

	if ppr.DstIP == "" {
		if dstNode != nil {
			ips, _ := dstNode.GetFieldStringList(ipField)
			if len(ips) == 0 {
				return "", nil, errors.New("No dest IP in node and user input")
			}
			ppr.DstIP = ips[0]
		} else {
			return "", nil, errors.New("Not able to find a dest node and dest IP also empty")
		}
	} else {
		ppr.DstIP = pi.normalizeIP(ppr.DstIP, ipField)
	}

	if ppr.SrcMAC == "" {
		if srcNode != nil {
			mac, _ := srcNode.GetFieldString("MAC")
			if mac == "" {
				return "", nil, errors.New("No source MAC in node and user input")
			}
			ppr.SrcMAC = mac
		} else {
			return "", nil, errors.New("Not able to find a source node and source MAC also empty")
		}
	}

	if ppr.DstMAC == "" {
		if dstNode != nil {
			mac, _ := dstNode.GetFieldString("MAC")
			if mac == "" {
				return "", nil, errors.New("No dest MAC in node and user input")
			}
			ppr.DstMAC = mac
		} else {
			return "", nil, errors.New("Not able to find a dest node and dest MAC also empty")
		}
	}

	if ppr.Type == "tcp4" || ppr.Type == "tcp6" {
		if ppr.SrcPort == 0 {
			ppr.SrcPort = rand.Int63n(max-min) + min
		}
		if ppr.DstPort == 0 {
			ppr.DstPort = rand.Int63n(max-min) + min
		}
	}

	pp := &packet_injector.PacketParams{
		SrcNodeID: srcNode.ID,
		SrcIP:     ppr.SrcIP,
		SrcMAC:    ppr.SrcMAC,
		SrcPort:   ppr.SrcPort,
		DstIP:     ppr.DstIP,
		DstMAC:    ppr.DstMAC,
		DstPort:   ppr.DstPort,
		Type:      ppr.Type,
		Payload:   ppr.Payload,
		Count:     ppr.Count,
		Interval:  ppr.Interval,
		ID:        ppr.ID,
	}

	if errs := validator.Validate(pp); errs != nil {
		return "", nil, errors.New("All the parms not set properly")
	}

	return srcNode.Host(), pp, nil
}

func (pi *PacketInjectorAPI) injectPacket(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
	decoder := json.NewDecoder(r.Body)
	var ppr PacketParamsReq
	if err := decoder.Decode(&ppr); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	defer r.Body.Close()

	host, pp, err := pi.requestToParams(&ppr)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	trackingID, err := pi.PIClient.InjectPacket(host, pp)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	ppr.TrackingID = trackingID
	if err := json.NewEncoder(w).Encode(ppr); err != nil {
		panic(err)
	}
}

func (pi *PacketInjectorAPI) getNode(gremlinQuery string) *graph.Node {
	res, err := topology.ExecuteGremlinQuery(pi.Graph, gremlinQuery)
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

func (pi *PacketInjectorAPI) registerEndpoints(r *shttp.Server) {
	routes := []shttp.Route{
		{
			Name:        "InjectPacket",
			Method:      "POST",
			Path:        "/api/injectpacket",
			HandlerFunc: pi.injectPacket,
		},
	}

	r.RegisterRoutes(routes)
}

// RegisterPacketInjectorAPI registers a new packet injector resource in the API
func RegisterPacketInjectorAPI(pic *packet_injector.PacketInjectorClient, g *graph.Graph, r *shttp.Server) {
	pia := &PacketInjectorAPI{
		PIClient: pic,
		Graph:    g,
	}

	pia.registerEndpoints(r)
}
