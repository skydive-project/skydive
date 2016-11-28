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

package flow

import (
	"encoding/json"
	"reflect"
	"testing"

	v "github.com/gima/govalid/v1"
	"github.com/google/gopacket/layers"
)

func TestFlowSimple(t *testing.T) {
	table := NewTable(nil, nil)
	packet := forgeTestPacket(t, 64, false, IPv4, TCP)
	FlowsFromGoPacket(table, packet, 0, nil)
	flows := table.GetFlows(nil).GetFlows()
	if len(flows) != 1 {
		t.Error("A single packet must generate 1 flow")
	}
	if flows[0].LayersPath != "IPv4/TCP/Payload" {
		t.Error("Flow LayersPath must be IPv4/TCP/Payload")
	}
}

func TestFlowSimpleIPv6(t *testing.T) {
	table := NewTable(nil, nil)
	packet := forgeTestPacket(t, 64, false, IPv6, TCP)
	FlowsFromGoPacket(table, packet, 0, nil)
	flows := table.GetFlows(nil).GetFlows()
	if len(flows) != 1 {
		t.Error("A single packet must generate 1 flow")
	}
	if flows[0].LayersPath != "IPv6/TCP/Payload" {
		t.Error("Flow LayersPath must be IPv6/TCP/Payload")
	}
}

func TestFlowParentUUID(t *testing.T) {
	table := NewTable(nil, nil)
	packet := forgeTestPacket(t, 64, false, ETH, IPv4, GRE, IPv4, UDP)
	flows := FlowsFromGoPacket(table, packet, 0, nil)
	flowsTable := table.GetFlows(nil).GetFlows()
	if len(flowsTable) != 2 {
		t.Error("An encapsulated encaspsulated packet must generate 2 flows")
	}
	if flows[1].ParentUUID != flows[0].UUID {
		t.Errorf("Encapsulated flow must have ParentUUID == %s", flows[0].UUID)
	}
	if flows[0].LayersPath != "Ethernet/IPv4/GRE" || flows[1].LayersPath != "IPv4/UDP/Payload" {
		t.Errorf("Flows LayersPath must be Ethernet/IPv4/GRE | IPv4/UDP/Payload")
	}
}

func TestFlowEncaspulation(t *testing.T) {
	table := NewTable(nil, nil)
	packet := forgeTestPacket(t, 64, false, ETH, IPv4, GRE, IPv4, GRE, IPv4, TCP)
	flows := FlowsFromGoPacket(table, packet, 0, nil)
	flowsTable := table.GetFlows(nil).GetFlows()
	if len(flowsTable) != 3 {
		t.Error("An encapsulated encaspsulated packet must generate 3 flows")
	}
	if flows[0].LayersPath != "Ethernet/IPv4/GRE" || flows[1].LayersPath != "IPv4/GRE" || flows[2].LayersPath != "IPv4/TCP/Payload" {
		t.Errorf("Flows LayersPath must be Ethernet/IPv4/GRE | IPv4/GRE | IPv4/TCP/Payload")
	}
}

func TestFlowEncaspulationMplsUdp(t *testing.T) {
	layers.RegisterUDPPortLayerType(layers.UDPPort(444), layers.LayerTypeMPLS)
	table := NewTable(nil, nil)
	packet := forgeTestPacket(t, 64, false, ETH, IPv4, UDP_MPLS, MPLS, IPv4, TCP)
	flows := FlowsFromGoPacket(table, packet, 0, nil)
	flowsTable := table.GetFlows(nil).GetFlows()
	if len(flowsTable) != 2 {
		t.Error("An MPLSoUDP packet must generate 2 flows")
	}
	if flows[0].LayersPath != "Ethernet/IPv4/UDP/MPLS" || flows[1].LayersPath != "IPv4/TCP/Payload" {
		t.Errorf("Flows LayersPath must be Ethernet/IPv4/UDP/MPLS | IPv4/TCP/Payload")
	}
}

func TestFlowJSON(t *testing.T) {
	f := Flow{
		UUID:       "uuid-1",
		LayersPath: "layerpath-1",
		Link: &FlowLayer{
			Protocol: FlowProtocol_ETHERNET,
			A:        "value-1",
			B:        "value-2",
		},
		Metric: &FlowMetric{
			Start: 1111,
			Last:  222,

			ABBytes:   33,
			ABPackets: 34,

			BABytes:   44,
			BAPackets: 55,
		},
		NodeUUID:  "probepath-1",
		ANodeUUID: "srcgraphpath-1",
		BNodeUUID: "dstgraphpath-1",
	}

	j, err := json.Marshal(f)
	if err != nil {
		t.Error(err.Error())
	}

	schema := v.Object(
		v.ObjKV("UUID", v.String()),
		v.ObjKV("LayersPath", v.String()),
		v.ObjKV("NodeUUID", v.String()),
		v.ObjKV("ANodeUUID", v.String()),
		v.ObjKV("BNodeUUID", v.String()),
		v.ObjKV("Link", v.Object(
			v.ObjKV("Protocol", v.String()),
			v.ObjKV("A", v.String()),
			v.ObjKV("B", v.String()),
		)),
		v.ObjKV("Metric", v.Object(
			v.ObjKV("Start", v.Number()),
			v.ObjKV("Last", v.Number()),
			v.ObjKV("ABPackets", v.Number()),
			v.ObjKV("ABBytes", v.Number()),
			v.ObjKV("BAPackets", v.Number()),
			v.ObjKV("BABytes", v.Number()),
		)),
	)

	var data interface{}
	if err := json.Unmarshal(j, &data); err != nil {
		t.Fatal("JSON parsing failed. Err =", err)
	}

	if path, err := schema.Validate(data); err != nil {
		t.Fatalf("Validation failed at %s. Error (%s)", path, err)
	}

	var e Flow
	if err := json.Unmarshal(j, &e); err != nil {
		t.Fatal("JSON parsing failed. Err =", err)
	}

	if !reflect.DeepEqual(f, e) {
		t.Fatal("Unmarshalled flow not equal to the original")
	}
}
