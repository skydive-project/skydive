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
	"fmt"
	"math/rand"
	"net"
	"testing"
	"time"

	"encoding/json"

	v "github.com/gima/govalid/v1"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

type ProtocolType int

const (
	ETH ProtocolType = 1 + iota
	IPv4
	IPv6
	TCP
	UDP
)

/* protos must contain a UDP or TCP layer on top of IPv4 */
func forgePacket(t *testing.T, seed int64, protos ...ProtocolType) *gopacket.Packet {
	rnd := rand.New(rand.NewSource(seed))

	rawBytes := []byte{10, 20, 30}
	var protoStack []gopacket.SerializableLayer

	for i, proto := range protos {
		switch proto {
		case ETH:
			ethernetLayer := &layers.Ethernet{
				SrcMAC:       net.HardwareAddr{0x00, 0x0F, 0xAA, 0xFA, 0xAA, byte(rnd.Intn(0x100))},
				DstMAC:       net.HardwareAddr{0x00, 0x0D, 0xBD, 0xBD, byte(rnd.Intn(0x100)), 0xBD},
				EthernetType: layers.EthernetTypeIPv4,
			}
			protoStack = append(protoStack, ethernetLayer)
		case IPv4:
			ipv4Layer := &layers.IPv4{
				SrcIP: net.IP{127, 0, 0, byte(rnd.Intn(0x100))},
				DstIP: net.IP{byte(rnd.Intn(0x100)), 8, 8, 8},
			}
			switch protos[i+1] {
			case TCP:
				ipv4Layer.Protocol = layers.IPProtocolTCP
			case UDP:
				ipv4Layer.Protocol = layers.IPProtocolUDP
			}
			protoStack = append(protoStack, ipv4Layer)
		case TCP:
			tcpLayer := &layers.TCP{
				SrcPort: layers.TCPPort(byte(rnd.Intn(0x10000))),
				DstPort: layers.TCPPort(byte(rnd.Intn(0x10000))),
			}
			protoStack = append(protoStack, tcpLayer)
		case UDP:
			udpLayer := &layers.UDP{
				SrcPort: layers.UDPPort(byte(rnd.Intn(0x10000))),
				DstPort: layers.UDPPort(byte(rnd.Intn(0x10000))),
			}
			protoStack = append(protoStack, udpLayer)
		default:
			t.Log("forgePacket : Unsupported protocol ", proto)
		}
	}
	protoStack = append(protoStack, gopacket.Payload(rawBytes))

	buffer := gopacket.NewSerializeBuffer()
	options := gopacket.SerializeOptions{FixLengths: true}
	err := gopacket.SerializeLayers(buffer, options, protoStack...)

	if err != nil {
		t.Fail()
	}

	gpacket := gopacket.NewPacket(buffer.Bytes(), layers.LayerTypeEthernet, gopacket.Default)
	return &gpacket
}

func generateFlows(t *testing.T, ft *FlowTable) []*Flow {
	flows := []*Flow{}
	for i := 0; i < 10; i++ {
		var packet *gopacket.Packet
		if i < 5 {
			packet = forgePacket(t, int64(i), ETH, IPv4, TCP)
		} else {
			packet = forgePacket(t, int64(i), ETH, IPv4, UDP)
		}
		flow, new := ft.GetFlow(string(i))
		if !new {
			t.Fail()
		}
		err := flow.fillFromGoPacket(packet)
		if err != nil {
			t.Error("fillFromGoPacket : " + err.Error())
		}
		flows = append(flows, flow)
	}
	return flows
}

func checkFlowTable(t *testing.T, ft *FlowTable) {
	for uuid, f := range ft.table {
		if uuid != f.UUID {
			t.Error("FlowTable Collision ", uuid, f.UUID)
		}
	}
}

func NewFlowTableSimple(t *testing.T) *FlowTable {
	ft := NewFlowTable()
	var flows []*Flow
	flow := &Flow{}
	flow.UUID = "1234"
	flows = append(flows, flow)
	ft.Update(flows)
	checkFlowTable(t, ft)
	ft.Update(flows)
	checkFlowTable(t, ft)
	if "1 flows" != ft.String() {
		t.Error("We should got only 1 flow")
	}
	flow = &Flow{}
	flow.UUID = "4567"
	flows = append(flows, flow)
	ft.Update(flows)
	checkFlowTable(t, ft)
	ft.Update(flows)
	checkFlowTable(t, ft)
	if "2 flows" != ft.String() {
		t.Error("We should got only 2 flows")
	}
	return ft
}
func NewFlowTableComplex(t *testing.T) *FlowTable {
	ft := NewFlowTable()
	flows := generateFlows(t, ft)
	ft = ft.NewFlowTableFromFlows(flows)
	checkFlowTable(t, ft)
	return ft
}

func TestNewFlowTable(t *testing.T) {
	ft := NewFlowTable()
	if ft == nil {
		t.Error("new FlowTable return null")
	}
}
func TestFlowTable_String(t *testing.T) {
	ft := NewFlowTable()
	if "0 flows" != ft.String() {
		t.Error("FlowTable too big")
	}
	ft = NewFlowTableSimple(t)
	if "2 flows" != ft.String() {
		t.Error("FlowTable too big")
	}
}
func TestFlowTable_Update(t *testing.T) {
	ft := NewFlowTableSimple(t)
	/* simulate a collision */
	f := &Flow{}
	ft.table["789"] = f
	ft.table["789"].UUID = "78910"
	f = &Flow{}
	f.UUID = "789"
	ft.Update([]*Flow{f})

	ft2 := NewFlowTableComplex(t)
	if "10 flows" != ft2.String() {
		t.Error("We should got only 10 flows")
	}
	ft2.Update([]*Flow{f})
	if "11 flows" != ft2.String() {
		t.Error("We should got only 11 flows")
	}
}

type MyTestFlowCounter struct {
	NbFlow int
}

func (fo *MyTestFlowCounter) countFlowsCallback(flows []*Flow) {
	fo.NbFlow += len(flows)
}

func TestFlowTable_expire(t *testing.T) {
	const MaxInt64 = int64(^uint64(0) >> 1)
	ft := NewFlowTableComplex(t)

	fc := MyTestFlowCounter{}
	beforeNbFlow := fc.NbFlow
	ft.expire(fc.countFlowsCallback, 0)
	afterNbFlow := fc.NbFlow
	if beforeNbFlow != 0 || afterNbFlow != 0 {
		t.Error("we should not expire a flow")
	}

	fc = MyTestFlowCounter{}
	beforeNbFlow = fc.NbFlow
	ft.expire(fc.countFlowsCallback, MaxInt64)
	afterNbFlow = fc.NbFlow
	if beforeNbFlow != 0 || afterNbFlow != 10 {
		t.Error("we should expire all flows")
	}
}

func TestFlowTable_updated(t *testing.T) {
	const MaxInt64 = int64(^uint64(0) >> 1)
	ft := NewFlowTableComplex(t)

	fc := MyTestFlowCounter{}
	beforeNbFlow := fc.NbFlow
	ft.updated(fc.countFlowsCallback, 0)
	afterNbFlow := fc.NbFlow
	if beforeNbFlow != 0 || afterNbFlow != 10 {
		t.Error("all flows should be updated")
	}

	fc = MyTestFlowCounter{}
	beforeNbFlow = fc.NbFlow
	ft.updated(fc.countFlowsCallback, MaxInt64)
	afterNbFlow = fc.NbFlow
	if beforeNbFlow != 0 || afterNbFlow != 0 {
		t.Error("no flows should be updated")
	}
}

func TestFlowTable_AsyncExpire(t *testing.T) {
	t.Skip()
}

func TestFlowTable_AsyncUpdated(t *testing.T) {
	t.Skip()
}

func TestFlowTable_IsExist(t *testing.T) {
	ft := NewFlowTableSimple(t)
	flow := &Flow{}
	flow.UUID = "1234"
	if ft.IsExist(flow) == false {
		t.Fail()
	}
	flow.UUID = "12345"
	if ft.IsExist(flow) == true {
		t.Fail()
	}
}

func TestFlowTable_GetFlow(t *testing.T) {
	ft := NewFlowTableComplex(t)
	flows := generateFlows(t, ft)
	if len(flows) != 10 {
		t.Error("missing some flows ", len(flows))
	}
	forgePacket(t, int64(1234), ETH, IPv4, TCP)
	_, new := ft.GetFlow("abcd")
	if !new {
		t.Error("Collision in the FlowTable, should be new")
	}
	forgePacket(t, int64(1234), ETH, IPv4, TCP)
	_, new = ft.GetFlow("abcd")
	if new {
		t.Error("Collision in the FlowTable, should be an update")
	}
	forgePacket(t, int64(1234), ETH, IPv4, TCP)
	_, new = ft.GetFlow("abcde")
	if !new {
		t.Error("Collision in the FlowTable, should be a new flow")
	}
}

func TestFlowTable_JSONFlowConversationEthernetPath(t *testing.T) {
	ft := NewFlowTableComplex(t)
	statStr := ft.JSONFlowConversationEthernetPath(FlowEndpointType_ETHERNET)
	if statStr == `{"nodes":[],"links":[]}` {
		t.Error("stat should not be empty")
	}

	var decoded interface{}
	if err := json.Unmarshal([]byte(statStr), &decoded); err != nil {
		t.Error("JSON parsing failed:", err)
	}

	schema := v.Object(
		v.ObjKV("nodes", v.Array(v.ArrEach(v.Object(
			v.ObjKV("name", v.String(v.StrMin(1))),
			v.ObjKV("group", v.Number(v.NumMin(0), v.NumMax(20))),
		)))),
		v.ObjKV("links", v.Array(v.ArrEach(v.Object(
			v.ObjKV("source", v.Number(v.NumMin(0), v.NumMax(20))),
			v.ObjKV("target", v.Number(v.NumMin(0), v.NumMax(20))),
			v.ObjKV("value", v.Number(v.NumMin(0), v.NumMax(9999))),
		)))),
	)

	if path, err := schema.Validate(decoded); err != nil {
		t.Errorf("Failed (%s). Path: %s", err, path)
	}
}

func test_JSONFlowDiscovery(t *testing.T, DiscoType DiscoType) {
	ft := NewFlowTableComplex(t)
	disco := ft.JSONFlowDiscovery(DiscoType)

	if disco == `{"name":"root","children":[]}` {
		t.Error("disco should not be empty")
	}

	var decoded interface{}
	if err := json.Unmarshal([]byte(disco), &decoded); err != nil {
		t.Error("JSON parsing failed:", err)
	}

	schema := v.Object(
		v.ObjKV("name", v.String(v.StrMin(1))), // root
		v.ObjKV("children", v.Array(v.ArrEach(v.Object(
			v.ObjKV("name", v.String(v.StrMin(1))), // Ethernet
			v.ObjKV("children", v.Array(v.ArrEach(v.Object(
				v.ObjKV("name", v.String(v.StrMin(1))), // IPv4
				v.ObjKV("children", v.Array(v.ArrEach(v.Object(
					v.ObjKV("name", v.String(v.StrMin(1))), // TCP or UDP
					v.ObjKV("children", v.Array(v.ArrEach(v.Object(
						v.ObjKV("name", v.String(v.StrMin(1))), // Payload
						v.ObjKV("size", v.Number(v.NumMin(0), v.NumMax(9999))),
					)))),
				)))),
			)))),
		)))),
	)

	if path, err := schema.Validate(decoded); err != nil {
		t.Errorf("Failed (%s). Path: %s", err, path)
	}
}

func TestFlowTable_JSONFlowDiscovery(t *testing.T) {
	test_JSONFlowDiscovery(t, BYTES)
	t.Log("JSONFlowDiscovery BYTES : ok")
	test_JSONFlowDiscovery(t, PACKETS)
	t.Log("JSONFlowDiscovery PACKETS : ok")
}

func TestFlowTable_NewFlowTableFromFlows(t *testing.T) {
	ft := NewFlowTableComplex(t)
	var flows []*Flow
	for _, f := range ft.table {
		flow := *f
		flows = append(flows, &flow)
	}
	ft2 := ft.NewFlowTableFromFlows(flows)
	if len(ft.table) != len(ft2.table) {
		t.Error("NewFlowTable(copy) are not the same size")
	}
	flows = flows[:0]
	for _, f := range ft.table {
		flows = append(flows, f)
	}
	ft3 := ft.NewFlowTableFromFlows(flows)
	if len(ft.table) != len(ft3.table) {
		t.Error("NewFlowTable(ref) are not the same size")
	}
}

func TestFlowTable_FilterLast(t *testing.T) {
	ft := NewFlowTableComplex(t)
	/* hack to put the FlowTable 1 second older */
	for _, f := range ft.table {
		fs := f.GetStatistics()
		fs.Start -= int64(1)
		fs.Last -= int64(1)
	}
	flows := ft.FilterLast(10 * time.Minute)
	if len(flows) != 10 {
		t.Error("FilterLast should return more/less flows", len(flows), 10)
	}
	flows = ft.FilterLast(0 * time.Minute)
	if len(flows) != 0 {
		t.Error("FilterLast should return less flows", len(flows), 0)
	}
}

func TestFlowTable_SelectLayer(t *testing.T) {
	ft := NewFlowTableComplex(t)

	var macs []string
	flows := ft.SelectLayer(FlowEndpointType_ETHERNET, macs)
	if len(ft.table) <= len(flows) && len(flows) != 0 {
		t.Errorf("SelectLayer should select none flows %d %d", len(ft.table), len(flows))
	}

	for mac := 0; mac < 0xff; mac++ {
		macs = append(macs, fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", 0x00, 0x0F, 0xAA, 0xFA, 0xAA, mac))
	}
	flows = ft.SelectLayer(FlowEndpointType_ETHERNET, macs)
	if len(ft.table) != len(flows) {
		t.Errorf("SelectLayer should select all flows %d %d", len(ft.table), len(flows))
	}
}
