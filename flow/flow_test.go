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
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"

	v "github.com/gima/govalid/v1"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/skydive-project/skydive/filters"
)

func TestFlowReflection(t *testing.T) {
	f := &Flow{}
	if strings.Contains(fmt.Sprintf("%v", f), "PANIC=") == true {
		t.Fail()
	}
}

func TestFlowMetric(t *testing.T) {
	flows := flowsFromPCAP(t, "pcaptraces/simple-tcpv4.pcap", layers.LinkTypeEthernet, nil)
	if len(flows) != 1 {
		t.Error("A single packet must generate 1 flow")
	}

	m := flows[0].Metric
	if m.Start == 0 || m.Last == 0 {
		t.Error("Start/Last empty")
	}

	e := &FlowMetric{
		ABPackets: 5,
		ABBytes:   344,
		BAPackets: 3,
		BABytes:   206,
		Start:     m.Start,
		Last:      m.Last,
	}

	if !reflect.DeepEqual(m, e) {
		t.Errorf("Expected metric %v not found, got: %v", e, m)
	}

	if flows[0].LastUpdateMetric != nil {
		t.Error("Shouldn't get LastUpdateMetric, as the flow table update didn't call")
	}
}

func TestFlowSimpleIPv4(t *testing.T) {
	flows := flowsFromPCAP(t, "pcaptraces/simple-tcpv4.pcap", layers.LinkTypeEthernet, nil)
	if len(flows) != 1 {
		t.Error("A single packet must generate 1 flow")
	}
	if flows[0].LayersPath != "Ethernet/IPv4/TCP" {
		t.Errorf("Flow LayersPath must be Ethernet/IPv4/TCP got : %s", flows[0].LayersPath)
	}
	if flows[0].RTT != 33000 {
		t.Errorf("Flow RTT must be 33000 got : %v", flows[0].RTT)
	}
}

func TestFlowSimpleIPv6(t *testing.T) {
	flows := flowsFromPCAP(t, "pcaptraces/simple-tcpv6.pcap", layers.LinkTypeEthernet, nil)
	if len(flows) != 1 {
		t.Error("A single packet must generate 1 flow")
	}
	if flows[0].LayersPath != "Ethernet/IPv6/TCP" {
		t.Errorf("Flow LayersPath must be Ethernet/IPv6/TCP got : %s", flows[0].LayersPath)
	}
	if flows[0].RTT != 28000 {
		t.Errorf("Flow RTT must be 28000 got : %v", flows[0].RTT)
	}
}

func TestBPFFilter(t *testing.T) {
	bpf, err := NewBPF(layers.LinkTypeEthernet, DefaultCaptureLength, "port 53 or port 80")
	if err != nil {
		t.Error(err.Error())
	}

	flows := flowsFromPCAP(t, "pcaptraces/eth-ip4-arp-dns-req-http-google.pcap", layers.LinkTypeEthernet, bpf)
	if len(flows) != 4 {
		t.Errorf("A single packet must generate 1 flow got : %v", flows)
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
		Start:            1111,
		Last:             222,
		LastUpdateMetric: &FlowMetric{},
		Metric: &FlowMetric{
			ABBytes:   33,
			ABPackets: 34,
			BABytes:   44,
			BAPackets: 55,
			Start:     1111111,
			Last:      2222222,
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
		v.ObjKV("Start", v.Number()),
		v.ObjKV("Last", v.Number()),
		v.ObjKV("Link", v.Object(
			v.ObjKV("Protocol", v.String()),
			v.ObjKV("A", v.String()),
			v.ObjKV("B", v.String()),
		)),
		v.ObjKV("LastUpdateMetric", v.Object(
			v.ObjKV("ABPackets", v.Number()),
			v.ObjKV("ABBytes", v.Number()),
			v.ObjKV("BAPackets", v.Number()),
			v.ObjKV("BABytes", v.Number()),
			v.ObjKV("Start", v.Number()),
			v.ObjKV("Last", v.Number()),
		)),
		v.ObjKV("Metric", v.Object(
			v.ObjKV("ABPackets", v.Number()),
			v.ObjKV("ABBytes", v.Number()),
			v.ObjKV("BAPackets", v.Number()),
			v.ObjKV("BABytes", v.Number()),
			v.ObjKV("Start", v.Number()),
			v.ObjKV("Last", v.Number()),
		),
		))

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

	return expected.Protocol == tested.Protocol && expected.A == tested.A && expected.B == tested.B && expected.ID == tested.ID
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
	if expected.LastUpdateMetric != nil && !compareFlowMetric(expected.LastUpdateMetric,
		tested.LastUpdateMetric) {
		return false
	}

	return true
}

func fillTableFromPCAP(t *testing.T, table *Table, filename string, linkType layers.LinkType, bpf *BPF) {
	handleRead, err := pcap.OpenOffline(filename)
	if err != nil {
		t.Fatal("PCAP OpenOffline error (handle to read packet): ", err)
	}
	defer handleRead.Close()

	var pcapPacketNB int
	for {
		data, ci, err := handleRead.ReadPacketData()
		if err != nil && err != io.EOF {
			t.Fatal("PCAP OpenOffline error (handle to read packet): ", err)
		} else if err == io.EOF {
			break
		} else {
			p := gopacket.NewPacket(data, linkType, gopacket.Default)
			p.Metadata().CaptureInfo = ci
			if p.ErrorLayer() != nil {
				t.Fatalf("Failed to decode packet with layer path '%s': %s", LayerPathFromGoPacket(&p), p.ErrorLayer().Error())
			}
			pcapPacketNB++
			if strings.Contains(LayerPathFromGoPacket(&p), "DecodeFailure") {
				t.Fatalf("GoPacket decode this pcap packet %d as DecodeFailure :\n%s", pcapPacketNB, p.Dump())
			}
			ps := PacketSeqFromGoPacket(&p, 0, bpf)
			if ps == nil {
				t.Fatal("Failed to get PacketSeq: ", err)
			}
			for level, p := range ps.Packets {
				if strings.Contains(LayerPathFromGoPacket((&p).gopacket), "DecodeFailure") {
					t.Fatalf("GoPacket decode this pcap packet %d level %d as DecodeFailure :\n%s", pcapPacketNB, level+1, (*(&p).gopacket).Dump())
				}
			}
			table.processPacketSeq(ps)
		}
	}
}

func getFlowChain(t *testing.T, table *Table, uuid string) []*Flow {
	// lookup for the parent
	searchQuery := &filters.SearchQuery{
		Filter: filters.NewTermStringFilter("UUID", uuid),
	}

	flows := table.getFlows(searchQuery).GetFlows()
	if len(flows) != 1 {
		t.Errorf("Should return only one flow got : %+v", flows)
	}
	fl := flows[0]

	flowChain := []*Flow{}
Chain:
	for {
		flowChain = append(flowChain, fl)
		searchQuery.Filter = filters.NewTermStringFilter("ParentUUID", fl.UUID)
		children := table.getFlows(searchQuery).GetFlows()
		switch len(children) {
		case 0:
			break Chain
		case 1:
			fl = children[0]
		default:
			t.Errorf("Should return only one flow got : %+v", children)
		}
	}

	return flowChain
}

func validateAllParentChains(t *testing.T, table *Table) {
	searchQuery := &filters.SearchQuery{
		Filter: filters.NewTermStringFilter("ParentUUID", ""),
	}

	flowChained := []*Flow{}

	flows := table.getFlows(searchQuery).GetFlows()
	for _, f := range flows {
		fls := getFlowChain(t, table, f.UUID)
		flowChained = append(flowChained, fls...)
	}

	// we should have touch all the flow
	flows = table.getFlows(&filters.SearchQuery{}).GetFlows()
	if len(flows) != len(flowChained) {
		t.Errorf("Flow parent chain is incorrect : %+v", flows)
	}
}

func flowsFromPCAP(t *testing.T, filename string, linkType layers.LinkType, bpf *BPF) []*Flow {
	table := NewTable(nil, nil, NewEnhancerPipeline(), "", TableOpts{TCPMetric: true})
	fillTableFromPCAP(t, table, filename, linkType, bpf)
	validateAllParentChains(t, table)

	return table.getFlows(&filters.SearchQuery{}).Flows
}

func validatePCAP(t *testing.T, filename string, linkType layers.LinkType, bpf *BPF, expected []*Flow) {
	flows := flowsFromPCAP(t, filename, linkType, bpf)
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

	validatePCAP(t, "pcaptraces/eth-ip4-arp-dns-req-http-google.pcap", layers.LinkTypeEthernet, nil, expected)
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
			LayersPath:  "IPv4/ICMPv4",
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
	validatePCAP(t, "pcaptraces/contrail-udp-mpls-eth-and-ipv4.pcap", layers.LinkTypeEthernet, nil, expected)
}

func TestPCAPL3TrackingID(t *testing.T) {
	var l3TrackingID string

	flows := flowsFromPCAP(t, "pcaptraces/ping-with-without-ethernet.pcap", layers.LinkTypeEthernet, nil)
	for _, flow := range flows {
		if flow.Application == "ICMPv4" {
			if l3TrackingID == "" {
				l3TrackingID = flow.L3TrackingID
			} else {
				if l3TrackingID != flow.L3TrackingID {
					t.Errorf("L3TrackingID are not equal: %s != %s\n", l3TrackingID, flow.L3TrackingID)
				}
			}
		}
	}
}

// This trace contains two packets sniffed by one capture, on one
// interface. There are 4 pings running in parallel (10 echo, 10 reply) each
//
// Ethernet/VLAN/VLAN/VLAN/VLAN/IPv4/ICMPv4
// Ethernet/VLAN/VLAN/VLAN/IPv4/ICMPv4
// Ethernet/VLAN/VLAN/IPv4/ICMPv4
// Ethernet/VLAN/IPv4/ICMPv4
//
func TestVlansQinQ(t *testing.T) {
	expected := []*Flow{
		{
			LayersPath:  "Ethernet/Dot1Q/IPv4/ICMPv4",
			Application: "ICMPv4",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "92:b6:d9:98:93:bb",
				B:        "f2:74:63:a0:e3:7f",
				ID:       8,
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "172.16.0.2",
				B:        "172.16.0.1",
			},
			Metric: &FlowMetric{
				ABPackets: 10,
				ABBytes:   1020,
				BAPackets: 8,
				BABytes:   816,
			},
		},
		{
			LayersPath:  "Ethernet/Dot1Q/Dot1Q/IPv4/ICMPv4",
			Application: "ICMPv4",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "92:b6:d9:98:93:bb",
				B:        "f2:74:63:a0:e3:7f",
				ID:       40968,
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "172.16.10.2",
				B:        "172.16.10.1",
			},
			Metric: &FlowMetric{
				ABPackets: 10,
				ABBytes:   1060,
				BAPackets: 10,
				BABytes:   1060,
			},
		},
		{
			LayersPath:  "Ethernet/Dot1Q/Dot1Q/Dot1Q/IPv4/ICMPv4",
			Application: "ICMPv4",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "92:b6:d9:98:93:bb",
				B:        "f2:74:63:a0:e3:7f",
				ID:       335585288,
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "172.16.20.2",
				B:        "172.16.20.1",
			},
			Metric: &FlowMetric{
				ABPackets: 10,
				ABBytes:   1100,
				BAPackets: 9,
				BABytes:   990,
			},
		},
		{
			LayersPath:  "Ethernet/Dot1Q/Dot1Q/Dot1Q/Dot1Q/IPv4/ICMPv4",
			Application: "ICMPv4",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "92:b6:d9:98:93:bb",
				B:        "f2:74:63:a0:e3:7f",
				ID:       2061919887368,
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "172.16.30.2",
				B:        "172.16.30.1",
			},
			Metric: &FlowMetric{
				ABPackets: 10,
				ABBytes:   1140,
				BAPackets: 8,
				BABytes:   912,
			},
		},
	}

	validatePCAP(t, "pcaptraces/icmpv4-4vlanQinQ-id-8-10-20-30.pcap", layers.LinkTypeEthernet, nil, expected)
}

func TestGREEthernet(t *testing.T) {
	expected := []*Flow{
		{
			LayersPath:  "Ethernet/IPv4/GRE",
			Application: "GRE",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "00:0f:fe:dd:22:42",
				B:        "00:1b:d5:ff:54:d9",
				ID:       0,
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "72.205.54.70",
				B:        "86.106.164.150",
				ID:       0,
			},
			Metric: &FlowMetric{
				ABPackets: 5,
				ABBytes:   810,
				BAPackets: 5,
				BABytes:   810,
			},
		},
		{
			LayersPath:  "IPv4/GRE",
			Application: "GRE",
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "10.10.11.2",
				B:        "10.10.13.2",
				ID:       0,
			},
			Metric: &FlowMetric{
				ABPackets: 5,
				ABBytes:   620,
				BAPackets: 5,
				BABytes:   620,
			},
		},
		{
			LayersPath:  "IPv4/ICMPv4",
			Application: "ICMPv4",
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "10.10.25.1",
				B:        "192.168.1.2",
				ID:       0,
			},
			Metric: &FlowMetric{
				ABPackets: 5,
				ABBytes:   500,
				BAPackets: 5,
				BABytes:   500,
			},
		},
	}

	validatePCAP(t, "pcaptraces/gre-gre-icmpv4.pcap", layers.LinkTypeEthernet, nil, expected)
}

func TestGREMPLS(t *testing.T) {
	expected := []*Flow{
		{
			LayersPath:  "Ethernet/IPv4/GRE/MPLS",
			Application: "MPLS",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "1e:1a:51:f4:45:46",
				B:        "ce:6e:35:76:f0:ab",
				ID:       0,
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "172.16.0.1",
				B:        "172.16.0.2",
				ID:       0,
			},
			Metric: &FlowMetric{
				ABPackets: 2,
				ABBytes:   252,
				BAPackets: 0,
				BABytes:   0,
			},
		},
		{
			LayersPath:  "IPv4/ICMPv4",
			Application: "ICMPv4",
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "172.16.0.1",
				B:        "192.168.0.2",
				ID:       0,
			},
			Metric: &FlowMetric{
				ABPackets: 2,
				ABBytes:   168,
				BAPackets: 0,
				BABytes:   0,
			},
		},
	}

	validatePCAP(t, "pcaptraces/gre-mpls-icmpv4.pcap", layers.LinkTypeEthernet, nil, expected)
}

func TestFlowSimpleSynFin(t *testing.T) {
	flows := flowsFromPCAP(t, "pcaptraces/simple-tcpv4.pcap", layers.LinkTypeEthernet, nil)
	// In test pcap SYNs happen at 2017-03-21 10:58:23.768977 +0200 IST
	synTimestamp := int64(1490086703768)
	// In test pcap FINs happen at 2017-03-21 10:58:27.507679 +0200 IST
	finTimestamp := int64(1490086707507)
	synTTL := uint32(64)

	if len(flows) != 1 {
		t.Error("A single packet must generate 1 flow")
	}
	if flows[0].TCPFlowMetric == nil {
		t.Errorf("Flow SYN/FIN is disabled")
		return
	}
	if flows[0].TCPFlowMetric.ABSynStart != synTimestamp {
		t.Errorf("In the flow AB-SYN must start at: %d, received at %d", synTimestamp, flows[0].TCPFlowMetric.ABSynStart)
	}
	if flows[0].TCPFlowMetric.BASynStart != synTimestamp {
		t.Errorf("In the flow BA-SYN must start at: %d, received at %d", synTimestamp, flows[0].TCPFlowMetric.BASynStart)
	}
	if flows[0].TCPFlowMetric.ABSynTTL != synTTL {
		t.Errorf("In flow AB-SYN TTL is: %d, supposed to be: %d", flows[0].TCPFlowMetric.ABSynTTL, synTTL)
	}
	if flows[0].TCPFlowMetric.BASynTTL != synTTL {
		t.Errorf("In flow BA-SYN TTL is: %d, supposed to be: %d", flows[0].TCPFlowMetric.BASynTTL, synTTL)
	}
	if flows[0].TCPFlowMetric.ABFinStart != finTimestamp {
		t.Errorf("In the flow AB-FIN must start at: %d, received at %d", finTimestamp, flows[0].TCPFlowMetric.ABFinStart)
	}
	if flows[0].TCPFlowMetric.BAFinStart != finTimestamp {
		t.Errorf("In the flow BA-FIN must start at: %d, received at %d", finTimestamp, flows[0].TCPFlowMetric.BAFinStart)
	}
}

func TestGetFieldsXXX(t *testing.T) {
	f := &Flow{}

	fields := f.GetFields()
	for _, i := range fields {
		k := i.(string)
		if strings.HasPrefix(k, "XXX_") {
			t.Error("XXX_ private field exposed")
		}
	}
}

func TestGetFieldInterface(t *testing.T) {
	f := &Flow{}

	field, err := f.GetFieldInterface("Metric")
	if err != nil {
		t.Error(err)
	}

	if field == nil {
		t.Error("Should return a Metric struct")
	}
}
