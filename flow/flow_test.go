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
	"strconv"
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

func TestFlowTruncatedMetric(t *testing.T) {
	flows := flowsFromPCAP(t, "pcaptraces/icmpv4-truncated.pcap", layers.LinkTypeEthernet, nil)
	if len(flows) != 1 {
		t.Error("A single packet must generate 1 flow")
	}

	m := flows[0].Metric
	if m.Start == 0 || m.Last == 0 {
		t.Error("Start/Last empty")
	}

	e := &FlowMetric{
		ABPackets: 1,
		ABBytes:   1066,
		BAPackets: 1,
		BABytes:   1066,
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

func TestFlowIPv4DefragDisabled(t *testing.T) {
	opt := TableOpts{ExtraTCPMetric: true, ReassembleTCP: true, IPDefrag: false}
	flows := flowsFromPCAP(t, "pcaptraces/ipv4-fragments.pcap", layers.LinkTypeEthernet, nil, opt)
	if len(flows) != 2 {
		t.Error("A fragmented packets must generate 2 flow", len(flows))
	}
	if flows[0].LayersPath != "Ethernet/IPv4/Fragment" {
		if flows[1].LayersPath != "Ethernet/IPv4/Fragment" {
			t.Errorf("Flow LayersPath must be Ethernet/IPv4/Fragment got : %s %s", flows[0].LayersPath, flows[1].LayersPath)
		}
	}
}

func TestFlowIPv4DefragEnabled(t *testing.T) {
	flows := flowsFromPCAP(t, "pcaptraces/ipv4-fragments.pcap", layers.LinkTypeEthernet, nil)
	if len(flows) != 1 {
		t.Error("A fragmented packets must generate 1 flow", len(flows))
	}
	if flows[0].LayersPath != "Ethernet/IPv4/Fragment/ICMPv4" {
		t.Errorf("Flow LayersPath must be Ethernet/IPv4/Fragment/ICMPv4 got : %s", flows[0].LayersPath)
	}
	if flows[0].IPMetric.FragmentErrors != 0 || flows[0].IPMetric.Fragments != 1 {
		t.Errorf("Flow IPMetric fragmentErrors %d fragments %d", flows[0].IPMetric.FragmentErrors, flows[0].IPMetric.Fragments)
	}
}

func TestFlowTCPSegments(t *testing.T) {
	flows := flowsFromPCAP(t, "pcaptraces/eth-ipv4-tcp-http-ooo.pcap", layers.LinkTypeEthernet, nil)
	if len(flows) != 1 {
		t.Error("A out of order tcp packets must generate 1 flow", len(flows))
	}
	if flows[0].LayersPath != "Ethernet/IPv4/TCP" {
		t.Errorf("Flow LayersPath must be Ethernet/IPv4/TCP got : %s", flows[0].LayersPath)
	}
	m := flows[0].TCPMetric
	if !(m.ABSegmentOutOfOrder == 0 && m.BASegmentOutOfOrder == 3) {
		t.Errorf("Flow SegmentOutOfOrder do not match, got : %d %d", m.ABSegmentOutOfOrder, m.BASegmentOutOfOrder)
	}
}

func TestBPFFilter(t *testing.T) {
	bpf, err := NewBPF(layers.LinkTypeEthernet, DefaultCaptureLength, "port 53 or port 80")
	if err != nil {
		t.Error(err)
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
		ICMP: &ICMPLayer{
			Type: ICMPType_ECHO,
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
		NodeTID: "probe-tid",
	}

	j, err := json.Marshal(f)
	if err != nil {
		t.Error(err)
	}

	schema := v.Object(
		v.ObjKV("UUID", v.String()),
		v.ObjKV("LayersPath", v.String()),
		v.ObjKV("NodeTID", v.String()),
		v.ObjKV("Start", v.Number()),
		v.ObjKV("Last", v.Number()),
		v.ObjKV("Link", v.Object(
			v.ObjKV("Protocol", v.String()),
			v.ObjKV("A", v.String()),
			v.ObjKV("B", v.String()),
		)),
		v.ObjKV("ICMP", v.Object(
			v.ObjKV("Type", v.String()),
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

func compareTransportLayer(expected, tested *TransportLayer) bool {
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
	if expected.Transport != nil && !compareTransportLayer(expected.Transport, tested.Transport) {
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

	for {
		data, ci, err := handleRead.ReadPacketData()
		if err != nil && err != io.EOF {
			t.Fatal("PCAP OpenOffline error (handle to read packet): ", err)
		} else if err == io.EOF {
			break
		} else {
			p := gopacket.NewPacket(data, linkType, gopacket.Default)
			p.Metadata().CaptureInfo = ci

			ps := PacketSeqFromGoPacket(p, 0, bpf, table.IPDefragger())
			table.processPacketSeq(ps)
		}
	}
}

func getFlowChain(t *testing.T, table *Table, uuid string, flowChain map[string]*Flow) {
	// lookup for the parent
	searchQuery := &filters.SearchQuery{
		Filter: filters.NewTermStringFilter("UUID", uuid),
	}

	flows := table.getFlows(searchQuery).GetFlows()
	if len(flows) != 1 {
		t.Errorf("Should return only one flow got : %+v", flows)
	}
	fl := flows[0]

	flowChain[fl.UUID] = fl

	if fl.ParentUUID != "" {
		getFlowChain(t, table, fl.ParentUUID, flowChain)
	}
}

func validateAllParentChains(t *testing.T, table *Table) {
	searchQuery := &filters.SearchQuery{
		Filter: filters.NewNotNullFilter("ParentUUID"),
	}

	flowChain := make(map[string]*Flow, 0)

	flows := table.getFlows(searchQuery).GetFlows()
	for _, fl := range flows {
		flowChain[fl.UUID] = fl

		if fl.ParentUUID != "" {
			getFlowChain(t, table, fl.UUID, flowChain)
		}
	}

	// we should have touch all the flow
	flows = table.getFlows(&filters.SearchQuery{}).GetFlows()
	if len(flows) != len(flowChain) {
		t.Errorf("Flow parent chain is incorrect : %+v", flows)
	}
}

func flowsFromPCAP(t *testing.T, filename string, linkType layers.LinkType, bpf *BPF, opts ...TableOpts) []*Flow {
	opt := TableOpts{ExtraTCPMetric: true, ReassembleTCP: true, IPDefrag: true}
	if len(opts) > 0 {
		opt = opts[0]
	}

	table := NewTable(nil, nil, "", opt)
	fillTableFromPCAP(t, table, filename, linkType, bpf)
	validateAllParentChains(t, table)

	return table.getFlows(&filters.SearchQuery{}).Flows
}

func validatePCAP(t *testing.T, filename string, linkType layers.LinkType, bpf *BPF, expected []*Flow, opts ...TableOpts) {
	flows := flowsFromPCAP(t, filename, linkType, bpf, opts...)
	for _, e := range expected {
		found := false
		for _, f := range flows {
			if compareFlow(e, f) {
				found = true
			}

			// check timestamp > 0
			if f.GetStart() < 0 || f.GetLast() < 0 {
				f, _ := json.MarshalIndent(flows, "", "\t")
				t.Errorf("Wrong timestamps: %s", string(f))
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
				BAPackets: 0,
				BABytes:   0,
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
			Transport: &TransportLayer{
				Protocol: FlowProtocol_UDP,
				A:        37686,
				B:        53,
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
			Transport: &TransportLayer{
				Protocol: FlowProtocol_TCP,
				A:        47838,
				B:        80,
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
			Transport: &TransportLayer{
				Protocol: FlowProtocol_UDP,
				A:        33553,
				B:        53,
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
			Transport: &TransportLayer{
				Protocol: FlowProtocol_TCP,
				A:        54785,
				B:        80,
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
		t.Fatal(err)
	}

	var i map[string]interface{}
	if err = json.Unmarshal(m, &i); err != nil {
		t.Fatal(err)
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
			Transport: &TransportLayer{
				Protocol: FlowProtocol_UDP,
				A:        53580,
				B:        51234,
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
			Transport: &TransportLayer{
				Protocol: FlowProtocol_TCP,
				A:        52477,
				B:        80,
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
			Transport: &TransportLayer{
				Protocol: FlowProtocol_UDP,
				A:        51822,
				B:        51234,
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

func benchmarkPacketParsing(b *testing.B, filename string, linkType layers.LinkType) {
	handleRead, err := pcap.OpenOffline(filename)
	if err != nil {
		b.Fatal("PCAP OpenOffline error (handle to read packet): ", err)
	}
	defer handleRead.Close()

	data, ci, err := handleRead.ReadPacketData()
	if err != nil {
		b.Fatal("PCAP OpenOffline error (handle to read packet): ", err)
	}
	p := gopacket.NewPacket(data, linkType, gopacket.Default)
	p.Metadata().CaptureInfo = ci

	for n := 0; n != b.N; n++ {
		ps := PacketSeqFromGoPacket(p, 0, nil, nil)
		if ps == nil {
			b.Fatal("Failed to get PacketSeq: ", err)
		}
		for _, packet := range ps.Packets {
			NewFlowFromGoPacket(packet.GoPacket, "", UUIDs{}, Opts{})
		}
	}
}

func getgopacket(b *testing.B, handleRead *pcap.Handle, linkType layers.LinkType) (gopacket.Packet, error) {
	data, ci, err := handleRead.ReadPacketData()
	if err != nil {
		if err == io.EOF {
			return nil, err
		}
		b.Fatal("PCAP OpenOffline error (handle to read packet): ", err)
	}
	p := gopacket.NewPacket(data, linkType, gopacket.Default)
	p.Metadata().CaptureInfo = ci
	return p, nil
}

func benchPacketList(b *testing.B, filename string, linkType layers.LinkType, callback func(b *testing.B, table *Table, packetsList []gopacket.Packet)) (packets int, table *Table) {
	bulkNbPackets := 100000

	handleRead, err := pcap.OpenOffline(filename)
	if err != nil {
		b.Fatal("PCAP OpenOffline error (handle to read packet): ", err)
	}
	ft := NewTable(nil, nil, "", TableOpts{})
	packets = 0

	for {
		var packetsList []gopacket.Packet
		ended := false
		for {
			p, err := getgopacket(b, handleRead, linkType)
			if err != nil {
				ended = true
				break
			}
			packetsList = append(packetsList, p)

			packets++
			if (packets % bulkNbPackets) == 0 {
				break
			}
		}

		b.StartTimer()
		callback(b, ft, packetsList)
		b.StopTimer()

		if ended {
			break
		}
	}
	handleRead.Close()

	return packets, ft
}

// Bench creation of a new flow for each packets
func benchmarkPacketsParsing(b *testing.B, filename string, linkType layers.LinkType) {
	b.ResetTimer()
	for n := 0; n != b.N; n++ {
		benchPacketList(b, filename, linkType,
			func(b *testing.B, table *Table, packetsList []gopacket.Packet) {
				for _, p := range packetsList {
					ps := PacketSeqFromGoPacket(p, 0, nil, nil)
					if ps == nil {
						b.Fatal("Failed to get PacketSeq")
					}
					for _, packet := range ps.Packets {
						NewFlowFromGoPacket(packet.GoPacket, "", UUIDs{}, Opts{})
					}
				}
			})
	}
}

// Bench creation of flow and connection tracking, via FlowTable
func benchmarkPacketsFlowTable(b *testing.B, filename string, linkType layers.LinkType) {
	b.ResetTimer()
	b.StopTimer()
	b.ReportAllocs()

	var ft *Table
	packets := 0
	for n := 0; n != b.N; n++ {
		packets, ft = benchPacketList(b, filename, linkType,
			func(b *testing.B, table *Table, packetsList []gopacket.Packet) {
				for _, p := range packetsList {
					ps := PacketSeqFromGoPacket(p, 0, nil, nil)
					if ps == nil {
						b.Fatal("Failed to get PacketSeq")
					}
					table.processPacketSeq(ps)
				}
			})
	}

	b.Logf("packets %d flows %d", packets, len(ft.table))
	b.Logf("packets per flows %d", packets/len(ft.table))
	fset := ft.getFlows(&filters.SearchQuery{
		Filter: filters.NewTermStringFilter("Network.Protocol", "IPV4"),
	})
	nbFlows := len(fset.Flows)
	b.Logf("IPv4 flows %d", nbFlows)
	fset = ft.getFlows(&filters.SearchQuery{
		Filter: filters.NewTermStringFilter("Network.Protocol", "IPV6"),
	})
	b.Logf("IPv6 flows %d", len(fset.Flows))

	if packets != 4679031 {
		b.Fail()
	}
	if len(ft.table) != 9088 || nbFlows != 9088 {
		b.Fail()
	}
}

func BenchmarkPacketParsing1(b *testing.B) {
	benchmarkPacketParsing(b, "pcaptraces/gre-gre-icmpv4.pcap", layers.LinkTypeEthernet)
}

func BenchmarkPacketParsing2(b *testing.B) {
	benchmarkPacketParsing(b, "pcaptraces/simple-tcpv4.pcap", layers.LinkTypeEthernet)
}

func BenchmarkPacketsParsing(b *testing.B) {
	benchmarkPacketsParsing(b, "pcaptraces/201801011400.small.pcap", layers.LinkTypeEthernet)
}

func BenchmarkPacketsFlowTable(b *testing.B) {
	benchmarkPacketsFlowTable(b, "pcaptraces/201801011400.small.pcap", layers.LinkTypeEthernet)
}

// Bench creation of flow and connection tracking, via FlowTable
func BenchmarkQueryFlowTable(b *testing.B) {
	t := NewTable(nil, nil, "")

	for i := 0; i != 10; i++ {
		f := &Flow{
			ICMP: &ICMPLayer{
				ID: uint32(i),
			},
		}
		t.table[strconv.Itoa(i)] = f
	}

	query := &filters.SearchQuery{
		Filter: filters.NewTermInt64Filter("ICMP.ID", 4444),
	}

	for n := 0; n != b.N; n++ {
		t.getFlows(query)
	}
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
	if flows[0].TCPMetric == nil {
		t.Errorf("Flow SYN/FIN is disabled")
		return
	}
	if flows[0].TCPMetric.ABSynStart != synTimestamp {
		t.Errorf("In the flow AB-SYN must start at: %d, received at %d", synTimestamp, flows[0].TCPMetric.ABSynStart)
	}
	if flows[0].TCPMetric.BASynStart != synTimestamp {
		t.Errorf("In the flow BA-SYN must start at: %d, received at %d", synTimestamp, flows[0].TCPMetric.BASynStart)
	}
	if flows[0].TCPMetric.ABSynTTL != synTTL {
		t.Errorf("In flow AB-SYN TTL is: %d, supposed to be: %d", flows[0].TCPMetric.ABSynTTL, synTTL)
	}
	if flows[0].TCPMetric.BASynTTL != synTTL {
		t.Errorf("In flow BA-SYN TTL is: %d, supposed to be: %d", flows[0].TCPMetric.BASynTTL, synTTL)
	}
	if flows[0].TCPMetric.ABFinStart != finTimestamp {
		t.Errorf("In the flow AB-FIN must start at: %d, received at %d", finTimestamp, flows[0].TCPMetric.ABFinStart)
	}
	if flows[0].TCPMetric.BAFinStart != finTimestamp {
		t.Errorf("In the flow BA-FIN must start at: %d, received at %d", finTimestamp, flows[0].TCPMetric.BAFinStart)
	}
}

func TestGetFieldsXXX(t *testing.T) {
	f := &Flow{}

	fields := f.GetFields()
	for _, k := range fields {
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

func TestVxlanIcmpv4Truncated(t *testing.T) {
	expected := []*Flow{
		{
			LayersPath:  "Ethernet/IPv4/UDP/VXLAN",
			Application: "VXLAN",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "fa:36:71:46:76:19",
				B:        "3a:07:fe:34:45:8e",
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "172.16.0.1",
				B:        "172.16.0.2",
				ID:       10,
			},
			Transport: &TransportLayer{
				Protocol: FlowProtocol_UDP,
				A:        55091,
				B:        4789,
			},
			Metric: &FlowMetric{
				ABPackets: 1,
				ABBytes:   1116,
				BAPackets: 1,
				BABytes:   1116,
			},
		},
		{
			LayersPath:  "Ethernet/IPv4/ICMPv4",
			Application: "ICMPv4",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "f2:98:72:99:56:08",
				B:        "26:09:b6:98:f9:64",
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "192.168.0.1",
				B:        "192.168.0.2",
			},
			ICMP: &ICMPLayer{
				Type: ICMPType_ECHO,
				Code: 0,
				ID:   10222,
			},
			Metric: &FlowMetric{
				ABPackets: 1,
				ABBytes:   1066,
				BAPackets: 1,
				BABytes:   1066,
			},
		},
	}

	validatePCAP(t, "pcaptraces/vxlan-icmpv4-truncated.pcap", layers.LinkTypeEthernet, nil, expected)
}

func TestNTPCorrupted(t *testing.T) {
	expected := []*Flow{
		{
			LayersPath:  "Ethernet/Dot1Q/IPv4/UDP",
			Application: "UDP",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "00:1c:0f:5c:a2:83",
				B:        "00:1c:0f:09:00:10",
				ID:       202,
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "89.46.101.31",
				B:        "196.95.70.83",
			},
			Transport: &TransportLayer{
				Protocol: FlowProtocol_UDP,
				A:        40820,
				B:        123,
			},
			Metric: &FlowMetric{
				ABPackets: 1,
				ABBytes:   50,
				BAPackets: 0,
				BABytes:   0,
			},
			TrackingID:   "35888c9feebbb826",
			L3TrackingID: "1a38b07877d9a8bd",
		},
	}

	validatePCAP(t, "pcaptraces/ntp-corrupted.pcap", layers.LinkTypeEthernet, nil, expected)
}

func TestLinkType12(t *testing.T) {
	expected := []*Flow{
		{
			LayersPath:  "IPv4/TCP",
			Application: "TCP",
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "10.171.169.243",
				B:        "192.168.255.1",
				ID:       0,
			},
			Transport: &TransportLayer{
				Protocol: FlowProtocol_TCP,
				A:        10250,
				B:        41252,
				ID:       0,
			},
		},
	}

	validatePCAP(t, "pcaptraces/link-type-12.pcap", layers.LinkTypeRaw, nil, expected)
}

func TestVxlanSrcPort(t *testing.T) {
	expected := []*Flow{
		{
			LayersPath:  "Ethernet/IPv4/UDP/VXLAN",
			Application: "VXLAN",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "fa:36:71:46:76:19",
				B:        "3a:07:fe:34:45:8e",
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "172.16.0.1",
				B:        "172.16.0.2",
				ID:       10,
			},
			Transport: &TransportLayer{
				Protocol: FlowProtocol_UDP,
				A:        51031,
				B:        4789,
			},
			Metric: &FlowMetric{
				ABPackets: 2,
				ABBytes:   208,
				BAPackets: 0,
				BABytes:   0,
			},
		},
		{
			LayersPath:  "Ethernet/IPv4/TCP",
			Application: "TCP",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "f2:98:72:99:56:08",
				B:        "26:09:b6:98:f9:64",
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "192.168.0.1",
				B:        "192.168.0.2",
			},
			Transport: &TransportLayer{
				Protocol: FlowProtocol_TCP,
				A:        1468,
				B:        8080,
			},
			Metric: &FlowMetric{
				ABPackets: 1,
				ABBytes:   54,
				BAPackets: 0,
				BABytes:   0,
			},
		},
		{
			LayersPath:  "Ethernet/IPv4/TCP",
			Application: "TCP",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "f2:98:72:99:56:08",
				B:        "26:09:b6:98:f9:64",
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "192.168.0.1",
				B:        "192.168.0.2",
			},
			Transport: &TransportLayer{
				Protocol: FlowProtocol_TCP,
				A:        1890,
				B:        8080,
			},
			Metric: &FlowMetric{
				ABPackets: 1,
				ABBytes:   54,
				BAPackets: 0,
				BABytes:   0,
			},
		},
	}

	validatePCAP(t, "pcaptraces/vxlan-src-port.pcap", layers.LinkTypeEthernet, nil, expected)
}

func TestGeneve(t *testing.T) {
	expected := []*Flow{
		{
			LayersPath:  "Ethernet/IPv4/UDP/Geneve",
			Application: "Geneve",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "00:1b:21:3c:ab:64",
				B:        "00:1b:21:3c:ac:30",
				ID:       0,
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "20.0.0.1",
				B:        "20.0.0.2",
				ID:       10,
			},
			Transport: &TransportLayer{
				Protocol: FlowProtocol_UDP,
				A:        12618,
				B:        6081,
				ID:       0,
			},
			Metric: &FlowMetric{
				ABPackets: 19,
				ABBytes:   5027,
				BAPackets: 0,
				BABytes:   0,
			},
		},
		{
			LayersPath:  "Ethernet/IPv4/UDP/Geneve",
			Application: "Geneve",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "00:1b:21:3c:ac:30",
				B:        "00:1b:21:3c:ab:64",
				ID:       0,
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "20.0.0.2",
				B:        "20.0.0.1",
				ID:       11,
			},
			Transport: &TransportLayer{
				Protocol: FlowProtocol_UDP,
				A:        50525,
				B:        6081,
				ID:       0,
			},
			Metric: &FlowMetric{
				ABPackets: 20,
				ABBytes:   4253,
				BAPackets: 0,
				BABytes:   0,
			},
		},
		{
			LayersPath:  "Ethernet/IPv4/ICMPv4",
			Application: "ICMPv4",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "fe:71:d8:83:72:4f",
				B:        "b6:9e:d2:49:51:48",
				ID:       0,
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "30.0.0.2",
				B:        "30.0.0.1",
				ID:       0,
			},
			ICMP: &ICMPLayer{
				Type: ICMPType_ECHO,
				Code: 0,
				ID:   10578,
			},
			Metric: &FlowMetric{
				ABPackets: 3,
				ABBytes:   294,
				BAPackets: 0,
				BABytes:   0,
			},
		},
		{
			LayersPath:  "Ethernet/IPv4/ICMPv4",
			Application: "ICMPv4",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "b6:9e:d2:49:51:48",
				B:        "fe:71:d8:83:72:4f",
				ID:       0,
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "30.0.0.1",
				B:        "30.0.0.2",
				ID:       0,
			},
			ICMP: &ICMPLayer{
				Type: ICMPType_ECHO,
				Code: 0,
				ID:   10578,
			},
			Metric: &FlowMetric{
				ABPackets: 3,
				ABBytes:   294,
				BAPackets: 0,
				BABytes:   0,
			},
		},
		{
			LayersPath:  "Ethernet/IPv4/TCP",
			Application: "TCP",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "fe:71:d8:83:72:4f",
				B:        "b6:9e:d2:49:51:48",
				ID:       0,
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "30.0.0.2",
				B:        "30.0.0.1",
				ID:       0,
			},
			Transport: &TransportLayer{
				Protocol: FlowProtocol_TCP,
				A:        51225,
				B:        22,
				ID:       0,
			},
			Metric: &FlowMetric{
				ABPackets: 17,
				ABBytes:   2959,
				BAPackets: 0,
				BABytes:   0,
			},
		},
	}

	validatePCAP(t, "pcaptraces/geneve.pcap", layers.LinkTypeEthernet, nil, expected)
}

func TestLayerKeyMode(t *testing.T) {
	expected := []*Flow{
		{
			UUID:        "4238ac6e55e4fe84",
			LayersPath:  "Ethernet/IPv4/ICMPv4",
			Application: "ICMPv4",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "f2:45:25:5b:3a:bc",
				B:        "2e:b9:2d:76:24:07",
				ID:       0,
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "192.168.0.2",
				B:        "192.168.0.1",
				ID:       0,
			},
			ICMP: &ICMPLayer{
				Type: ICMPType_ECHO,
				Code: 0,
				ID:   23604,
			},
			Metric: &FlowMetric{
				ABPackets: 2,
				ABBytes:   196,
				BAPackets: 2,
				BABytes:   196,
			},
			TrackingID:   "fccc60686022c1d7",
			L3TrackingID: "8d393414982d64e4",
		},
	}

	validatePCAP(t, "pcaptraces/layer-key-mode.pcap", layers.LinkTypeEthernet, nil, expected, TableOpts{LayerKeyMode: L3PreferedKeyMode})

	expected = []*Flow{
		{
			UUID:        "4238ac6e55e4fe84",
			LayersPath:  "Ethernet/IPv4/ICMPv4",
			Application: "ICMPv4",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "f2:45:25:5b:3a:ac",
				B:        "2e:b9:2d:76:24:07",
				ID:       0,
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "192.168.0.2",
				B:        "192.168.0.1",
				ID:       0,
			},
			ICMP: &ICMPLayer{
				Type: ICMPType_ECHO,
				Code: 0,
				ID:   23604,
			},
			Metric: &FlowMetric{
				ABPackets: 1,
				ABBytes:   98,
				BAPackets: 1,
				BABytes:   98,
			},
			TrackingID:   "dceb357acb653343",
			L3TrackingID: "8d393414982d64e4",
		},
		{
			UUID:        "4238ac6e55e4fe84",
			LayersPath:  "Ethernet/IPv4/ICMPv4",
			Application: "ICMPv4",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "f2:45:25:5b:3a:bc",
				B:        "2e:b9:2d:76:24:07",
				ID:       0,
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "192.168.0.2",
				B:        "192.168.0.1",
				ID:       0,
			},
			ICMP: &ICMPLayer{
				Type: ICMPType_ECHO,
				Code: 0,
				ID:   23604,
			},
			Metric: &FlowMetric{
				ABPackets: 1,
				ABBytes:   98,
				BAPackets: 1,
				BABytes:   98,
			},
			TrackingID:   "5a98493143d10285",
			L3TrackingID: "8d393414982d64e4",
		},
	}

	validatePCAP(t, "pcaptraces/layer-key-mode.pcap", layers.LinkTypeEthernet, nil, expected, TableOpts{LayerKeyMode: L2KeyMode})
}
