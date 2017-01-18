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
	"io"
	"reflect"
	"testing"

	v "github.com/gima/govalid/v1"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

func TestFlowSimple(t *testing.T) {
	table := NewTable(nil, nil)
	packet := forgeTestPacket(t, 64, false, IPv4, TCP)
	flowPackets := FlowPacketsFromGoPacket(packet, 0)
	table.FlowPacketsToFlow(flowPackets)

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
	flowPackets := FlowPacketsFromGoPacket(packet, 0)
	table.FlowPacketsToFlow(flowPackets)

	flows := table.GetFlows(nil).GetFlows()
	if len(flows) != 1 {
		t.Error("A single packet must generate 1 flow")
	}
	if flows[0].LayersPath != "IPv6/TCP/Payload" {
		t.Error("Flow LayersPath must be IPv6/TCP/Payload")
	}
}

func sortFlowByRelationship(flows []*Flow) []*Flow {
	res := make([]*Flow, len(flows))

	var parentUUID string
	for i := range flows {
		for _, flow := range flows {
			if flow.ParentUUID == parentUUID {
				res[i] = flow
				parentUUID = flow.UUID
				break
			}
		}
	}
	return res
}

func TestFlowParentUUID(t *testing.T) {
	table := NewTable(nil, nil)
	packet := forgeTestPacket(t, 64, false, ETH, IPv4, GRE, IPv4, UDP)
	flowPackets := FlowPacketsFromGoPacket(packet, 0)
	table.FlowPacketsToFlow(flowPackets)

	flows := table.GetFlows(nil).GetFlows()
	if len(flows) != 2 {
		t.Error("An encapsulated encaspsulated packet must generate 2 flows")
	}

	flows = sortFlowByRelationship(flows)

	if flows[1].ParentUUID == "" || flows[1].ParentUUID != flows[0].UUID {
		t.Errorf("Encapsulated flow must have ParentUUID == %s", flows[0].UUID)
	}
	if flows[0].LayersPath != "Ethernet/IPv4/GRE" || flows[1].LayersPath != "IPv4/UDP/Payload" {
		t.Errorf("Flows LayersPath must be Ethernet/IPv4/GRE | IPv4/UDP/Payload")
	}
}

func TestFlowEncaspulation(t *testing.T) {
	table := NewTable(nil, nil)
	packet := forgeTestPacket(t, 64, false, ETH, IPv4, GRE, IPv4, GRE, IPv4, TCP)
	flowPackets := FlowPacketsFromGoPacket(packet, 0)
	table.FlowPacketsToFlow(flowPackets)

	flows := table.GetFlows(nil).GetFlows()
	if len(flows) != 3 {
		t.Error("An encapsulated encaspsulated packet must generate 3 flows")
	}

	flows = sortFlowByRelationship(flows)

	if flows[0].LayersPath != "Ethernet/IPv4/GRE" || flows[1].LayersPath != "IPv4/GRE" || flows[2].LayersPath != "IPv4/TCP/Payload" {
		t.Errorf("Flows LayersPath must be Ethernet/IPv4/GRE | IPv4/GRE | IPv4/TCP/Payload")
	}
}

func TestFlowEncaspulationMplsUdp(t *testing.T) {
	layers.RegisterUDPPortLayerType(layers.UDPPort(444), layers.LayerTypeMPLS)
	table := NewTable(nil, nil)
	packet := forgeTestPacket(t, 64, false, ETH, IPv4, UDP_MPLS, MPLS, IPv4, TCP)
	flowPackets := FlowPacketsFromGoPacket(packet, 0)
	table.FlowPacketsToFlow(flowPackets)

	flows := table.GetFlows(nil).GetFlows()
	if len(flows) != 2 {
		t.Error("An MPLSoUDP packet must generate 2 flows")
	}

	flows = sortFlowByRelationship(flows)

	if flows[0].LayersPath != "Ethernet/IPv4/UDP/MPLS" || flows[1].LayersPath != "IPv4/TCP/Payload" {
		t.Errorf("Flows LayersPath must be Ethernet/IPv4/UDP/MPLS | IPv4/TCP/Payload")
	}
}

func TestFlowEncaspulationMplsGRE(t *testing.T) {
	table := NewTable(nil, nil)
	packet := forgeTestPacket(t, 64, false, ETH, IPv4, GRE, MPLS, IPv4, TCP)
	flowPackets := FlowPacketsFromGoPacket(packet, 0)
	table.FlowPacketsToFlow(flowPackets)

	flows := table.GetFlows(nil).GetFlows()
	if len(flows) != 2 {
		t.Error("An MPLSoGRE packet must generate 2 flows")
	}

	flows = sortFlowByRelationship(flows)

	if flows[0].LayersPath != "Ethernet/IPv4/GRE/MPLS" || flows[1].LayersPath != "IPv4/TCP/Payload" {
		t.Errorf("Flows LayersPath must be Ethernet/IPv4/GRE/MPLS | IPv4/TCP/Payload")
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
		NodeTID:  "probe-tid",
		ANodeTID: "anode-tid",
		BNodeTID: "bnode-tid",
	}

	j, err := json.Marshal(f)
	if err != nil {
		t.Error(err.Error())
	}

	schema := v.Object(
		v.ObjKV("UUID", v.String()),
		v.ObjKV("LayersPath", v.String()),
		v.ObjKV("NodeTID", v.String()),
		v.ObjKV("ANodeTID", v.String()),
		v.ObjKV("BNodeTID", v.String()),
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

func compareFlowLayer(expected, tested *FlowLayer) bool {
	if tested == nil {
		return false
	}

	return expected.Protocol == tested.Protocol && expected.A == tested.A && expected.B == tested.B
}

func compareFlowMetric(expected, tested *FlowMetric) bool {
	if tested == nil {
		return false
	}

	return expected.ABBytes == tested.ABBytes && expected.ABPackets == tested.ABPackets &&
		expected.BABytes == tested.BABytes && expected.BAPackets == tested.BAPackets
}

func compareFlow(expected, tested *Flow) bool {
	if expected.LayersPath != "" && expected.LayersPath != tested.LayersPath {
		return false
	}
	if expected.Application != "" && expected.Application != tested.Application {
		return false
	}
	if expected.TrackingID != "" && expected.TrackingID != tested.TrackingID {
		return false
	}
	if expected.ParentUUID != "" && expected.ParentUUID != tested.ParentUUID {
		return false
	}
	if expected.Link != nil && !compareFlowLayer(expected.Link, tested.Link) {
		return false
	}
	if expected.Network != nil && !compareFlowLayer(expected.Network, tested.Network) {
		return false
	}
	if expected.Transport != nil && !compareFlowLayer(expected.Transport, tested.Transport) {
		return false
	}
	if expected.Metric != nil && !compareFlowMetric(expected.Metric, tested.Metric) {
		return false
	}

	return true
}

func validatePCAP(t *testing.T, filename string, linkType layers.LinkType, expected []*Flow) {
	handleRead, err := pcap.OpenOffline(filename)
	if err != nil {
		t.Fatal("PCAP OpenOffline error (handle to read packet): ", err)
	}
	defer handleRead.Close()

	table := NewTable(nil, nil)

	for {
		data, _, err := handleRead.ReadPacketData()
		if err != nil && err != io.EOF {
			t.Fatal("PCAP OpenOffline error (handle to read packet): ", err)
		} else if err == io.EOF {
			break
		} else {
			p := gopacket.NewPacket(data, linkType, gopacket.Default)
			if p.ErrorLayer() != nil {
				t.Fatalf("Failed to decode packet with layer path '%s': %s\n", layerPathFromGoPacket(&p), p.ErrorLayer().Error())
			}

			fp := FlowPacketsFromGoPacket(&p, 0)
			if fp == nil {
				t.Fatal("Failed to get FlowPackets: ", err)
			}
			table.FlowPacketsToFlow(fp)
		}
	}

	flows := table.GetFlows(&FlowSearchQuery{}).Flows
	for _, e := range expected {
		found := false
		for _, f := range flows {
			if compareFlow(e, f) {
				found = true
			}
		}
		if !found {
			je, _ := json.MarshalIndent(e, "", "\t")
			f, _ := json.MarshalIndent(flows, "", "\t")
			t.Errorf("Flows mismatch, \nexpected %s\ngot  %s\n", string(je), string(f))
		}
	}
}

func TestPCAP1(t *testing.T) {
	expected := []*Flow{
		{
			LayersPath:  "Ethernet/ARP",
			Application: "ARP",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "fa:16:3e:29:e0:82",
				B:        "ff:ff:ff:ff:ff:ff",
			},
			Metric: &FlowMetric{
				ABPackets: 1,
				ABBytes:   42,
				BAPackets: 1,
				BABytes:   42,
			},
		},
		{
			LayersPath:  "Ethernet/IPv4/UDP/DNS",
			Application: "DNS",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "fa:16:3e:29:e0:82",
				B:        "fa:16:3e:96:06:e8",
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "192.168.0.5",
				B:        "8.8.8.8",
			},
			Transport: &FlowLayer{
				Protocol: FlowProtocol_UDPPORT,
				A:        "37686",
				B:        "53",
			},
			Metric: &FlowMetric{
				ABPackets: 2,
				ABBytes:   148,
				BAPackets: 2,
				BABytes:   256,
			},
		},
		{
			LayersPath:  "Ethernet/IPv4/TCP",
			Application: "TCP",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "fa:16:3e:29:e0:82",
				B:        "fa:16:3e:96:06:e8",
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "192.168.0.5",
				B:        "173.194.40.147",
			},
			Transport: &FlowLayer{
				Protocol: FlowProtocol_TCPPORT,
				A:        "47838",
				B:        "80",
			},
			Metric: &FlowMetric{
				ABPackets: 6,
				ABBytes:   516,
				BAPackets: 4,
				BABytes:   760,
			},
		},
		{
			LayersPath:  "Ethernet/IPv4/UDP/DNS",
			Application: "DNS",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "fa:16:3e:29:e0:82",
				B:        "fa:16:3e:96:06:e8",
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "192.168.0.5",
				B:        "8.8.8.8",
			},
			Transport: &FlowLayer{
				Protocol: FlowProtocol_UDPPORT,
				A:        "33553",
				B:        "53",
			},
			Metric: &FlowMetric{
				ABPackets: 2,
				ABBytes:   146,
				BAPackets: 2,
				BABytes:   190,
			},
		},
		{
			LayersPath:  "Ethernet/IPv4/TCP",
			Application: "TCP",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "fa:16:3e:29:e0:82",
				B:        "fa:16:3e:96:06:e8",
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "192.168.0.5",
				B:        "216.58.211.67",
			},
			Transport: &FlowLayer{
				Protocol: FlowProtocol_TCPPORT,
				A:        "54785",
				B:        "80",
			},
			Metric: &FlowMetric{
				ABPackets: 20,
				ABBytes:   1475,
				BAPackets: 18,
				BABytes:   21080,
			},
		},
	}

	validatePCAP(t, "pcaptraces/eth-ip4-arp-dns-req-http-google.pcap", layers.LinkTypeEthernet, expected)
}

func TestEmptyParentUUIDExported(t *testing.T) {
	flow := &Flow{}

	m, err := json.Marshal(&flow)
	if err != nil {
		t.Fatal(err.Error())
	}

	var i map[string]interface{}
	if err = json.Unmarshal(m, &i); err != nil {
		t.Fatal(err.Error())
	}

	if _, ok := i["ParentUUID"]; !ok {
		t.Fatal("ParentUUID field should always be exported")
	}
}

// This trace contains two packets sniffed by one capture, on one
// interface. One packet layerpath is
// Ethernet/IPv4/UDP/MPLS/IPv4/ICMPv4 while the other one is
// Ethernet/IPv4/UDP/MPLS/Ethernet/IPv4/TCP
//
// Contrail can remove the ethernet header of the packet generated by
// a VM when it pushes it into the tunnel. In particular, if source
// and destination IPs  don't belong to the same network, Contrail
// remove the ethernet header. So, the MPLS payload can be ethernet or
// IP.
func TestPCAPMplsContrail(t *testing.T) {
	expected := []*Flow{
		{
			LayersPath:  "IPv4/ICMPv4/Payload",
			Application: "ICMPv4",
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "10.65.65.5",
				B:        "10.35.35.3",
			},
			Metric: &FlowMetric{
				ABPackets: 1,
				ABBytes:   104,
			},
		},
		{
			LayersPath:  "Ethernet/IPv4/UDP/MPLS",
			Application: "MPLS",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "d4:ae:52:9e:5d:2f",
				B:        "90:b1:1c:0f:bb:b8",
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "10.11.0.56",
				B:        "10.11.0.55",
			},
			Transport: &FlowLayer{
				Protocol: FlowProtocol_UDPPORT,
				A:        "53580",
				B:        "51234",
			},
			Metric: &FlowMetric{
				ABPackets: 1,
				ABBytes:   120,
			},
		},
		{
			LayersPath:  "Ethernet/IPv4/TCP",
			Application: "TCP",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "02:47:10:72:2d:5b",
				B:        "02:70:7f:82:12:ab",
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "10.65.65.4",
				B:        "10.65.65.5",
			},
			Transport: &FlowLayer{
				Protocol: FlowProtocol_TCPPORT,
				A:        "52477",
				B:        "80",
			},
			Metric: &FlowMetric{
				ABPackets: 1,
				ABBytes:   74,
			},
		},
		{
			LayersPath:  "Ethernet/IPv4/UDP/MPLS",
			Application: "MPLS",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "90:b1:1c:0f:bb:b8",
				B:        "d4:ae:52:9e:5d:2f",
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "10.11.0.55",
				B:        "10.11.0.56",
			},
			Transport: &FlowLayer{
				Protocol: FlowProtocol_UDPPORT,
				A:        "51822",
				B:        "51234",
			},
			Metric: &FlowMetric{
				ABPackets: 1,
				ABBytes:   150,
			},
		},
	}

	layers.RegisterUDPPortLayerType(layers.UDPPort(51234), layers.LayerTypeMPLS)
	validatePCAP(t, "pcaptraces/contrail-udp-mpls-eth-and-ipv4.pcap", layers.LinkTypeEthernet, expected)
}
