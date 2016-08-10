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

package tests

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/skydive-project/skydive/api"
	cmd "github.com/skydive-project/skydive/cmd/client"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/tests/helper"
	"github.com/skydive-project/skydive/tools"
	"github.com/skydive-project/skydive/topology/graph"
)

const confAgentAnalyzer = `---
agent:
  listen: 58081
  analyzers: localhost:{{.AnalyzerPort}}
  topology:
    probes:
      - netlink
      - netns
      - ovsdb
  flow:
    probes:
      - ovssflow
      - pcap

cache:
  expire: 300
  cleanup: 30

sflow:
  port_min: 55000
  port_max: 55005

analyzer:
  listen: {{.AnalyzerPort}}
  flowtable_expire: 600
  flowtable_update: 10
  flowtable_agent_ratio: 0.5

etcd:
  embedded: {{.EmbeddedEtcd}}
  port: 2374
  data_dir: /tmp/skydive-test-etcd
  servers:
    - {{.EtcdServer}}

logging:
  default: {{.LogLevel}}
`

type flowStat struct {
	Path      string
	ABPackets uint64
	ABBytes   uint64
	BAPackets uint64
	BABytes   uint64
}
type flowsTraceInfo struct {
	filename string
	flowStat []flowStat
}

var flowsTraces = [...]flowsTraceInfo{
	{
		filename: "pcaptraces/eth-ip4-arp-dns-req-http-google.pcap",
		flowStat: []flowStat{
			{"Ethernet/ARP/Payload", 1, 42, 1, 42},
			{"Ethernet/IPv4/UDP/DNS", 2, 148, 2, 256},
			{"Ethernet/IPv4/TCP", 4, 384, 3, 694},
			{"Ethernet/IPv4/UDP/DNS", 2, 146, 2, 190},
			{"Ethernet/IPv4/TCP", 4, 272, 2, 140},
		},
	},
}

type TestStorage struct {
	lock  sync.RWMutex
	flows map[string]*flow.Flow
}

func NewTestStorage() *TestStorage {
	return &TestStorage{flows: make(map[string]*flow.Flow)}
}

func (s *TestStorage) Start() {
}

func (s *TestStorage) Stop() {
}

func (s *TestStorage) StoreFlows(flows []*flow.Flow) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	for _, f := range flows {
		s.flows[f.UUID] = f
	}

	return nil
}

func (s *TestStorage) SearchFlows(filter flow.Filter) ([]*flow.Flow, error) {
	return nil, nil
}

func (s *TestStorage) GetFlows() []*flow.Flow {
	s.lock.Lock()
	defer s.lock.Unlock()

	flows := make([]*flow.Flow, len(s.flows))

	i := 0
	for _, f := range s.flows {
		flows[i] = f
		i++
	}

	return flows
}

func pcapTraceCheckFlow(t *testing.T, f *flow.Flow, trace *flowsTraceInfo) bool {
	eth := f.GetStatistics().GetEndpointsType(flow.FlowEndpointType_ETHERNET)
	if eth == nil {
		t.Fail()
	}

	for _, fi := range trace.flowStat {
		if fi.Path == f.LayersPath {
			if (fi.ABPackets == eth.AB.Packets) && (fi.ABBytes == eth.AB.Bytes) && (fi.BAPackets == eth.BA.Packets) && (fi.BABytes == eth.BA.Bytes) {
				return true
			}
		}
	}

	return false
}

func pcapTraceValidate(t *testing.T, flows []*flow.Flow, trace *flowsTraceInfo) {
	found := 0
	for _, f := range flows {
		if pcapTraceCheckFlow(t, f, trace) {
			found++
		}
	}

	if found != len(trace.flowStat) {
		t.Errorf("Flow expected not found %v != %v", trace.flowStat, flows)
	}
}

func gremlinQuery(t *testing.T, query string, values interface{}) {
	body, err := cmd.SendGremlinQuery(&http.AuthenticationOpts{}, query)
	if err != nil {
		t.Fatalf("Error while executing query %s: %s", query, err.Error())
	}

	err = json.NewDecoder(body).Decode(values)
	if err != nil {
		t.Fatal(err.Error())
	}
}

func getNodesFromGremlinReply(t *testing.T, query string) []graph.Node {
	var values []interface{}
	gremlinQuery(t, query, &values)
	nodes := make([]graph.Node, len(values))
	for i, node := range values {
		if err := nodes[i].Decode(node); err != nil {
			t.Fatal(err.Error())
		}
	}
	return nodes
}

func getNodeFromGremlinReply(t *testing.T, query string) *graph.Node {
	nodes := getNodesFromGremlinReply(t, query)
	if len(nodes) > 0 {
		return &nodes[0]
	}
	return nil
}

func getFlowsFromGremlinReply(t *testing.T, query string) (flows []*flow.Flow) {
	gremlinQuery(t, query, &flows)
	return
}

func getFlowSetBandwidthFromGremlinReply(t *testing.T, query string) flow.FlowSetBandwidth {
	body, err := cmd.SendGremlinQuery(&http.AuthenticationOpts{}, query)
	if err != nil {
		t.Fatalf("%s: %s", query, err.Error())
	}

	var bw []flow.FlowSetBandwidth
	err = json.NewDecoder(body).Decode(&bw)
	if err != nil {
		t.Fatal(err.Error())
	}

	return bw[0]
}

func TestSFlowWithPCAP(t *testing.T) {
	ts := NewTestStorage()

	aa := helper.NewAgentAnalyzerWithConfig(t, confAgentAnalyzer, ts)
	aa.Start()
	defer aa.Stop()

	client := api.NewCrudClientFromConfig(&http.AuthenticationOpts{})
	capture := api.NewCapture("G.V().Has('Name', 'br-sflow', 'Type', 'ovsbridge')", "")
	if err := client.Create("capture", capture); err != nil {
		t.Fatal(err.Error())
	}

	time.Sleep(1 * time.Second)
	setupCmds := []helper.Cmd{
		{"ovs-vsctl add-br br-sflow", true},
	}

	tearDownCmds := []helper.Cmd{
		{"ovs-vsctl del-br br-sflow", true},
	}

	helper.ExecCmds(t, setupCmds...)
	defer helper.ExecCmds(t, tearDownCmds...)

	time.Sleep(5 * time.Second)

	// FIX(safchain): need to be reworked as there is no more static sflow agent
	// running at a specific port and agent couldn't speak sflow at all
	for _, trace := range flowsTraces {
		fulltrace, _ := filepath.Abs(trace.filename)
		err := tools.PCAP2SFlowReplay("localhost", 55000, fulltrace, 1000, 5)
		if err != nil {
			t.Fatalf("Error during the replay: %s", err.Error())
		}

		aa.Flush()
		pcapTraceValidate(t, ts.GetFlows(), &trace)
	}

	client.Delete("capture", capture.ID())
}

func TestSFlowProbeNode(t *testing.T) {
	ts := NewTestStorage()

	aa := helper.NewAgentAnalyzerWithConfig(t, confAgentAnalyzer, ts)
	aa.Start()
	defer aa.Stop()

	client := api.NewCrudClientFromConfig(&http.AuthenticationOpts{})
	capture := api.NewCapture("G.V().Has('Name', 'br-sflow', 'Type', 'ovsbridge')", "")
	if err := client.Create("capture", capture); err != nil {
		t.Fatal(err.Error())
	}

	time.Sleep(1 * time.Second)
	setupCmds := []helper.Cmd{
		{"ovs-vsctl add-br br-sflow", true},
		{"ovs-vsctl add-port br-sflow sflow-intf1 -- set interface sflow-intf1 type=internal", true},
		{"ip address add 169.254.33.33/24 dev sflow-intf1", true},
		{"ip link set sflow-intf1 up", true},
		{"ping -c 15 -I sflow-intf1 169.254.33.34", false},
	}

	tearDownCmds := []helper.Cmd{
		{"ovs-vsctl del-br br-sflow", true},
	}

	helper.ExecCmds(t, setupCmds...)
	defer helper.ExecCmds(t, tearDownCmds...)

	aa.Flush()

	node := getNodeFromGremlinReply(t, `g.V().Has("Name", "br-sflow", "Type", "ovsbridge")`)

	ok := false
	for _, f := range ts.GetFlows() {
		if f.ProbeNodeUUID == string(node.ID) && f.LayersPath == "Ethernet/ARP/Payload" {
			ok = true
			break
		}
	}

	if !ok {
		t.Error("Unable to find a flow with the expected ProbeNodeUUID")
	}

	client.Delete("capture", capture.ID())
}

func TestSFlowProbeNodeUUIDOvsInternalNetNS(t *testing.T) {
	ts := NewTestStorage()

	aa := helper.NewAgentAnalyzerWithConfig(t, confAgentAnalyzer, ts)
	aa.Start()
	defer aa.Stop()

	client := api.NewCrudClientFromConfig(&http.AuthenticationOpts{})
	capture := api.NewCapture("G.V().Has('Name', 'br-sflow', 'Type', 'ovsbridge')", "")
	if err := client.Create("capture", capture); err != nil {
		t.Fatal(err.Error())
	}

	time.Sleep(1 * time.Second)
	setupCmds := []helper.Cmd{
		{"ovs-vsctl add-br br-sflow", true},
		{"ovs-vsctl add-port br-sflow sflow-intf1 -- set interface sflow-intf1 type=internal", true},
		{"ip netns add sflow-vm1", true},
		{"ip link set sflow-intf1 netns sflow-vm1", true},
		{"ip netns exec sflow-vm1 ip address add 169.254.33.33/24 dev sflow-intf1", true},
		{"ip netns exec sflow-vm1 ip link set sflow-intf1 up", true},
		{"ip netns exec sflow-vm1 ping -c 15 -I sflow-intf1 169.254.33.34", false},
	}

	tearDownCmds := []helper.Cmd{
		{"ip netns del sflow-vm1", true},
		{"ovs-vsctl del-br br-sflow", true},
	}

	helper.ExecCmds(t, setupCmds...)
	defer helper.ExecCmds(t, tearDownCmds...)

	aa.Flush()

	node := getNodeFromGremlinReply(t, `g.V().Has("Name", "br-sflow", "Type", "ovsbridge")`)

	ok := false
	for _, f := range ts.GetFlows() {
		if f.ProbeNodeUUID == string(node.ID) && f.LayersPath == "Ethernet/ARP/Payload" {
			ok = true
			break
		}
	}

	if !ok {
		t.Error("Unable to find a flow with the expected ProbeNodeUUID")
	}

	client.Delete("capture", capture.ID())
}

func TestSFlowTwoProbeNodeUUID(t *testing.T) {
	ts := NewTestStorage()

	aa := helper.NewAgentAnalyzerWithConfig(t, confAgentAnalyzer, ts)
	aa.Start()
	defer aa.Stop()

	client := api.NewCrudClientFromConfig(&http.AuthenticationOpts{})
	capture1 := api.NewCapture("G.V().Has('Name', 'br-sflow1', 'Type', 'ovsbridge')", "")
	if err := client.Create("capture", capture1); err != nil {
		t.Fatal(err.Error())
	}
	capture2 := api.NewCapture("G.V().Has('Name', 'br-sflow2', 'Type', 'ovsbridge')", "")
	if err := client.Create("capture", capture2); err != nil {
		t.Fatal(err.Error())
	}

	time.Sleep(5 * time.Second)
	setupCmds := []helper.Cmd{
		{"ovs-vsctl add-br br-sflow1", true},
		{"ovs-vsctl add-port br-sflow1 sflow-intf1 -- set interface sflow-intf1 type=internal", true},
		{"ip netns add sflow-vm1", true},
		{"ip link set sflow-intf1 netns sflow-vm1", true},
		{"ip netns exec sflow-vm1 ip address add 169.254.33.33/24 dev sflow-intf1", true},
		{"ip netns exec sflow-vm1 ip link set sflow-intf1 up", true},

		{"ovs-vsctl add-br br-sflow2", true},
		{"ovs-vsctl add-port br-sflow2 sflow-intf2 -- set interface sflow-intf2 type=internal", true},
		{"ip netns add sflow-vm2", true},
		{"ip link set sflow-intf2 netns sflow-vm2", true},
		{"ip netns exec sflow-vm2 ip address add 169.254.33.34/24 dev sflow-intf2", true},
		{"ip netns exec sflow-vm2 ip link set sflow-intf2 up", true},

		// interfaces used to link br-sflow1 and br-sflow2 without a patch
		{"ovs-vsctl add-port br-sflow1 sflow-link1 -- set interface sflow-link1 type=internal", true},
		{"ip link set sflow-link1 up", true},
		{"ovs-vsctl add-port br-sflow2 sflow-link2 -- set interface sflow-link2 type=internal", true},
		{"ip link set sflow-link2 up", true},

		{"brctl addbr br-link", true},
		{"ip link set br-link up", true},
		{"brctl addif br-link sflow-link1", true},
		{"brctl addif br-link sflow-link2", true},

		{"ip netns exec sflow-vm2 ping -c 15 -I sflow-intf2 169.254.33.33", false},
	}

	tearDownCmds := []helper.Cmd{
		{"ip netns del sflow-vm1", true},
		{"ip netns del sflow-vm2", true},
		{"ovs-vsctl del-br br-sflow1", true},
		{"ovs-vsctl del-br br-sflow2", true},
		{"sleep 1", true},
		{"ip link set br-link down", true},
		{"brctl delbr br-link", true},
	}

	helper.ExecCmds(t, setupCmds...)
	defer helper.ExecCmds(t, tearDownCmds...)

	aa.Flush()

	flows := []*flow.Flow{}
	for _, f := range ts.GetFlows() {
		if f.LayersPath == "Ethernet/IPv4/ICMPv4/Payload" {
			flows = append(flows, f)
		}
	}

	if len(flows) != 2 {
		t.Errorf("Should have 2 flow entries one per ProbeNodeUUID got: %d", len(flows))
	}

	node1 := getNodeFromGremlinReply(t, `g.V().Has("Name", "br-sflow1", "Type", "ovsbridge")`)
	node2 := getNodeFromGremlinReply(t, `g.V().Has("Name", "br-sflow2", "Type", "ovsbridge")`)

	if flows[0].ProbeNodeUUID != string(node1.ID) &&
		flows[0].ProbeNodeUUID != string(node2.ID) {
		t.Errorf("Bad ProbeNodeUUID for the first flow: %s", flows[0].ProbeNodeUUID)
	}

	if flows[1].ProbeNodeUUID != string(node1.ID) &&
		flows[1].ProbeNodeUUID != string(node2.ID) {
		t.Errorf("Bad ProbeNodeUUID for the second flow: %s", flows[1].ProbeNodeUUID)
	}

	if flows[0].TrackingID != flows[1].TrackingID {
		t.Errorf("Both flows should have the same TrackingID: %v", flows)
	}

	if flows[0].UUID == flows[1].UUID {
		t.Errorf("Both flows should have different UUID: %v", flows)
	}

	client.Delete("capture", capture1.ID())
	client.Delete("capture", capture2.ID())
}

func TestPCAPProbe(t *testing.T) {
	ts := NewTestStorage()

	aa := helper.NewAgentAnalyzerWithConfig(t, confAgentAnalyzer, ts)
	aa.Start()
	defer aa.Stop()

	client := api.NewCrudClientFromConfig(&http.AuthenticationOpts{})
	capture := api.NewCapture("G.V().Has('Name', 'br-pcap', 'Type', 'bridge')", "")
	if err := client.Create("capture", capture); err != nil {
		t.Fatal(err.Error())
	}
	time.Sleep(1 * time.Second)

	setupCmds := []helper.Cmd{
		{"brctl addbr br-pcap", true},
		{"ip link set br-pcap up", true},
		{"ip netns add vm1", true},
		{"ip link add name vm1-eth0 type veth peer name eth0 netns vm1", true},
		{"ip link set vm1-eth0 up", true},
		{"ip netns exec vm1 ip link set eth0 up", true},
		{"ip netns exec vm1 ip address add 169.254.66.66/24 dev eth0", true},
		{"brctl addif br-pcap vm1-eth0", true},

		{"ip netns add vm2", true},
		{"ip link add name vm2-eth0 type veth peer name eth0 netns vm2", true},
		{"ip link set vm2-eth0 up", true},
		{"ip netns exec vm2 ip link set eth0 up", true},
		{"ip netns exec vm2 ip address add 169.254.66.67/24 dev eth0", true},
		{"brctl addif br-pcap vm2-eth0", true},

		{"ip netns exec vm1 ping -c 15 169.254.66.67", false},
	}

	tearDownCmds := []helper.Cmd{
		{"ip link set br-pcap down", true},
		{"brctl delbr br-pcap", true},
		{"ip link del vm1-eth0", true},
		{"ip link del vm2-eth0", true},
		{"ip netns del vm1", true},
		{"ip netns del vm2", true},
	}

	helper.ExecCmds(t, setupCmds...)
	defer helper.ExecCmds(t, tearDownCmds...)

	aa.Flush()

	node := getNodeFromGremlinReply(t, `g.V().Has("Name", "br-pcap", "Type", "bridge")`)

	ok := false
	flows := ts.GetFlows()
	for _, f := range flows {
		if f.ProbeNodeUUID == string(node.ID) {
			ok = true
			break
		}
	}

	if !ok {
		t.Errorf("Unable to find a flow with the expected ProbeNodeUUID: %v\n%v", flows, aa.Agent.Graph.String())
	}

	client.Delete("capture", capture.ID())
}

func TestSFlowSrcDstPath(t *testing.T) {
	ts := NewTestStorage()

	aa := helper.NewAgentAnalyzerWithConfig(t, confAgentAnalyzer, ts)
	aa.Start()
	defer aa.Stop()

	client := api.NewCrudClientFromConfig(&http.AuthenticationOpts{})
	capture := api.NewCapture("G.V().Has('Name', 'br-sflow', 'Type', 'ovsbridge')", "")
	if err := client.Create("capture", capture); err != nil {
		t.Fatal(err.Error())
	}

	time.Sleep(1 * time.Second)
	setupCmds := []helper.Cmd{
		{"ovs-vsctl add-br br-sflow", true},

		{"ovs-vsctl add-port br-sflow sflow-intf1 -- set interface sflow-intf1 type=internal", true},
		{"ip netns add sflow-vm1", true},
		{"ip link set sflow-intf1 netns sflow-vm1", true},
		{"ip netns exec sflow-vm1 ip address add 169.254.33.33/24 dev sflow-intf1", true},
		{"ip netns exec sflow-vm1 ip link set sflow-intf1 up", true},

		{"ovs-vsctl add-port br-sflow sflow-intf2 -- set interface sflow-intf2 type=internal", true},
		{"ip netns add sflow-vm2", true},
		{"ip link set sflow-intf2 netns sflow-vm2", true},
		{"ip netns exec sflow-vm2 ip address add 169.254.33.34/24 dev sflow-intf2", true},
		{"ip netns exec sflow-vm2 ip link set sflow-intf2 up", true},

		{"ip netns exec sflow-vm1 ping -c 25 -I sflow-intf1 169.254.33.34", false},
	}

	tearDownCmds := []helper.Cmd{
		{"ip netns del sflow-vm1", true},
		{"ip netns del sflow-vm2", true},
		{"ovs-vsctl del-br br-sflow", true},
	}

	helper.ExecCmds(t, setupCmds...)
	defer helper.ExecCmds(t, tearDownCmds...)

	aa.Flush()

	node1 := getNodeFromGremlinReply(t, `g.V().Has("Name", "sflow-intf1", "Type", "internal")`)
	node2 := getNodeFromGremlinReply(t, `g.V().Has("Name", "sflow-intf2", "Type", "internal")`)

	ok := false
	for _, f := range ts.GetFlows() {
		// we can have both way depending on which packet has been seen first
		if (f.IfSrcNodeUUID == string(node1.ID) && f.IfDstNodeUUID == string(node2.ID)) ||
			(f.IfSrcNodeUUID == string(node2.ID) && f.IfDstNodeUUID == string(node1.ID)) {
			ok = true
			break
		}
	}

	if !ok {
		t.Errorf("Unable to find flows with the expected path: %v\n %s", ts.GetFlows(), aa.Agent.Graph.String())
	}

	client.Delete("capture", capture.ID())
}

func TestFlowQuery(t *testing.T) {
	al := flow.NewTableAllocator(500, 500, 500, 500)

	f := func(flows []*flow.Flow) {}

	ft1 := al.Alloc(f)

	flow.GenerateTestFlows(t, ft1, 1, "probe1")
	flows1 := flow.GenerateTestFlows(t, ft1, 2, "probe2")

	ft2 := al.Alloc(f)
	flows2 := flow.GenerateTestFlows(t, ft2, 3, "probe2")

	go ft1.Start()
	go ft2.Start()

	time.Sleep(time.Second)

	query := &flow.TableQuery{
		Obj: &flow.FlowSearchQuery{
			flow.BoolFilter{
				Op: flow.OR,
				Filters: []flow.Filter{
					flow.TermFilter{Key: "ProbeNodeUUID", Value: "probe2"},
					flow.TermFilter{Key: "IfSrcNodeUUID", Value: "probe2"},
					flow.TermFilter{Key: "IfDstNodeUUID", Value: "probe2"},
				},
			},
		},
	}
	reply := al.QueryTable(query)

	ft1.Stop()
	ft2.Stop()

	fsr := []flow.FlowSearchReply{}
	err := json.Unmarshal(reply.Obj, &fsr)
	if err != nil {
		t.Fatal(err.Error())
	}

	flowset := flow.NewFlowSet()
	for _, reply := range fsr {
		flowset.Merge(reply.FlowSet)
	}

	if len(flowset.Flows) != len(flows1)+len(flows2) {
		t.Fatalf("FlowQuery should return at least one flow")
	}

	for _, flow := range flowset.Flows {
		if flow.ProbeNodeUUID != "probe2" {
			t.Fatalf("FlowQuery should only return flows with probe2, got: %s", flow)
		}
	}
}

func TestTableServer(t *testing.T) {
	ts := NewTestStorage()

	aa := helper.NewAgentAnalyzerWithConfig(t, confAgentAnalyzer, ts)
	aa.Start()
	defer aa.Stop()

	client := api.NewCrudClientFromConfig(&http.AuthenticationOpts{})
	capture := api.NewCapture("G.V().Has('Name', 'br-sflow', 'Type', 'ovsbridge')", "")
	if err := client.Create("capture", capture); err != nil {
		t.Fatal(err.Error())
	}

	time.Sleep(1 * time.Second)
	setupCmds := []helper.Cmd{
		{"ovs-vsctl add-br br-sflow", true},
		{"ovs-vsctl add-port br-sflow sflow-intf1 -- set interface sflow-intf1 type=internal", true},
		{"ip address add 169.254.33.33/24 dev sflow-intf1", true},
		{"ip link set sflow-intf1 up", true},
		{"ping -c 15 -I sflow-intf1 169.254.33.34", false},
	}

	tearDownCmds := []helper.Cmd{
		{"ovs-vsctl del-br br-sflow", true},
	}

	helper.ExecCmds(t, setupCmds...)
	defer helper.ExecCmds(t, tearDownCmds...)

	aa.Flush()

	node := getNodeFromGremlinReply(t, `g.V().Has("Name", "br-sflow", "Type", "ovsbridge")`)

	hnmap := make(flow.HostNodeIDMap)
	hnmap[node.Host()] = append(hnmap[node.Host()], string(node.ID))

	fclient := flow.NewTableClient(aa.Analyzer.WSServer)
	flowset, err := fclient.LookupFlowsByNodes(hnmap, nil)
	if err != nil {
		t.Fatal(err.Error())
	}

	if len(ts.GetFlows()) != len(flowset.Flows) {
		t.Fatalf("Should return the same number of flows than in the database, got: %v", flowset)
	}

	for _, f := range flowset.Flows {
		if f.ProbeNodeUUID != string(node.ID) {
			t.Fatalf("Returned a non expected flow: %v", f)
		}
	}

	client.Delete("capture", capture.ID())
}

func TestFlowGremlin(t *testing.T) {
	ts := NewTestStorage()

	aa := helper.NewAgentAnalyzerWithConfig(t, confAgentAnalyzer, ts)
	aa.Start()
	defer aa.Stop()

	client := api.NewCrudClientFromConfig(&http.AuthenticationOpts{})
	capture := api.NewCapture("G.V().Has('Name', 'br-sflow', 'Type', 'ovsbridge')", "")
	if err := client.Create("capture", capture); err != nil {
		t.Fatal(err.Error())
	}

	time.Sleep(1 * time.Second)
	setupCmds := []helper.Cmd{
		{"ovs-vsctl add-br br-sflow", true},
		{"ovs-vsctl add-port br-sflow sflow-intf1 -- set interface sflow-intf1 type=internal", true},
		{"ip address add 169.254.33.33/24 dev sflow-intf1", true},
		{"ip link set sflow-intf1 up", true},
		{"ping -c 15 -I sflow-intf1 169.254.33.34", false},
	}

	tearDownCmds := []helper.Cmd{
		{"ovs-vsctl del-br br-sflow", true},
	}

	helper.ExecCmds(t, setupCmds...)
	defer helper.ExecCmds(t, tearDownCmds...)

	aa.Flush()

	node := getNodeFromGremlinReply(t, `g.V().Has("Name", "br-sflow", "Type", "ovsbridge")`)

	var count int64
	gremlinQuery(t, `G.V().Has("Name", "br-sflow", "Type", "ovsbridge").Count()`, &count)
	if count != 1 {
		t.Fatalf("Should return 1, got: %d", count)
	}

	flows := getFlowsFromGremlinReply(t, `g.V().Has("Name", "br-sflow", "Type", "ovsbridge").Flows().Has("ProbeNodeUUID", "`+string(node.ID)+`")`)
	if len(ts.GetFlows()) != len(flows) {
		t.Fatalf("Should return the same number of flows than in the database, got: %v, expected: %v", len(flows), len(ts.GetFlows()))
	}

	flows = getFlowsFromGremlinReply(t, `g.V().Has("Name", "br-sflow", "Type", "ovsbridge").Flows("ProbeNodeUUID", "`+string(node.ID)+`")`)
	if len(ts.GetFlows()) != len(flows) {
		t.Fatalf("Should return the same number of flows than in the database, got: %v, expected: %v", len(flows), len(ts.GetFlows()))
	}

	nodes := getNodesFromGremlinReply(t, `g.V().Has("Name", "br-sflow", "Type", "ovsbridge").Flows("ProbeNodeUUID", "`+string(node.ID)+`").Out()`)
	if len(nodes) != 0 {
		t.Fatalf("Should return no destination node, got %d", len(nodes))
	}

	nodes = getNodesFromGremlinReply(t, `g.V().Has("Name", "br-sflow", "Type", "ovsbridge").Flows("ProbeNodeUUID", "`+string(node.ID)+`").Both().Dedup()`)
	if len(nodes) != 1 {
		t.Fatalf("Should return one node, got %d", len(nodes))
	}

	nodes = getNodesFromGremlinReply(t, `g.V().Has("Name", "br-sflow", "Type", "ovsbridge").Flows("ProbeNodeUUID", "`+string(node.ID)+`").In().Dedup()`)
	if len(nodes) != 1 {
		t.Fatalf("Should return one source node, got %d", len(nodes))
	}

	gremlinQuery(t, `g.V().Has("Name", "br-sflow", "Type", "ovsbridge").Flows("ProbeNodeUUID", "`+string(node.ID)+`").Count()`, &count)
	if int(count) != len(flows) {
		t.Fatalf("Should return the same number of flows than in the database, got: %v, expected: %v", len(flows), len(ts.GetFlows()))
	}

	client.Delete("capture", capture.ID())
}

func getFlowEndpoint(t *testing.T, gremlin string, endpointType flow.FlowEndpointType) *flow.FlowEndpointsStatistics {
	flows := getFlowsFromGremlinReply(t, gremlin)
	if len(flows) != 1 {
		return nil
	}
	return flows[0].GetStatistics().GetEndpointsType(endpointType)
}

func TestFlowMetrics(t *testing.T) {
	ts := NewTestStorage()

	aa := helper.NewAgentAnalyzerWithConfig(t, confAgentAnalyzer, ts)
	aa.Start()
	defer aa.Stop()

	client := api.NewCrudClientFromConfig(&http.AuthenticationOpts{})
	capture := api.NewCapture("G.V().Has('Name', 'br-sflow', 'Type', 'ovsbridge')", "")
	if err := client.Create("capture", capture); err != nil {
		t.Fatal(err.Error())
	}

	time.Sleep(1 * time.Second)
	setupCmds := []helper.Cmd{
		{"ovs-vsctl add-br br-sflow", true},

		{"ovs-vsctl add-port br-sflow sflow-intf1 -- set interface sflow-intf1 type=internal", true},
		{"ip netns add sflow-vm1", true},
		{"ip link set sflow-intf1 netns sflow-vm1", true},
		{"ip netns exec sflow-vm1 ip address add 169.254.33.33/24 dev sflow-intf1", true},
		{"ip netns exec sflow-vm1 ip link set sflow-intf1 up", true},

		{"ovs-vsctl add-port br-sflow sflow-intf2 -- set interface sflow-intf2 type=internal", true},
		{"ip netns add sflow-vm2", true},
		{"ip link set sflow-intf2 netns sflow-vm2", true},
		{"ip netns exec sflow-vm2 ip address add 169.254.33.34/24 dev sflow-intf2", true},
		{"ip netns exec sflow-vm2 ip link set sflow-intf2 up", true},

		// wait to have everything ready, sflow, interfaces
		{"sleep 1", false},

		{"ip netns exec sflow-vm1 ping -c 1 -s 1024 -I sflow-intf1 169.254.33.34", false},
	}

	tearDownCmds := []helper.Cmd{
		{"ip netns del sflow-vm1", true},
		{"ip netns del sflow-vm2", true},
		{"ovs-vsctl del-br br-sflow", true},
	}

	helper.ExecCmds(t, setupCmds...)
	defer helper.ExecCmds(t, tearDownCmds...)

	time.Sleep(2 * time.Second)
	aa.Flush()

	icmp := getFlowsFromGremlinReply(t, `g.V().Has("Name", "br-sflow", "Type", "ovsbridge").Flows().Has("LayersPath", "Ethernet/IPv4/ICMPv4/Payload")`)
	if len(icmp) != 1 {
		t.Errorf("Should return only one icmp flow, got: %v", icmp)
	}
	if icmp[0].LayersPath != "Ethernet/IPv4/ICMPv4/Payload" {
		t.Errorf("Wrong layer path, should be 'Ethernet/IPv4/ICMPv4/Payload', got %s", icmp[0].LayersPath)
	}

	pingLen := icmp[0].GetStatistics().GetEndpointsType(flow.FlowEndpointType_ETHERNET).GetAB().Bytes

	ethernet := icmp[0].GetStatistics().GetEndpointsType(flow.FlowEndpointType_ETHERNET)
	if ethernet.GetBA().Packets != 1 {
		t.Errorf("Number of packets is wrong, got: %v", icmp)
	}

	if ethernet.GetAB().Bytes < 1066 || ethernet.GetBA().Bytes < 1066 {
		t.Errorf("Number of bytes is wrong, got: %v", icmp)
	}

	flows := getFlowsFromGremlinReply(t, `g.V().Has("Name", "br-sflow", "Type", "ovsbridge").Flows("LayersPath", "Ethernet/IPv4/ICMPv4/Payload", "Statistics.Endpoints.ETHERNET.AB.Packets", 1)`)
	if len(flows) != 1 || flows[0].GetStatistics().GetEndpointsType(flow.FlowEndpointType_ETHERNET).GetBA().Packets != 1 {
		t.Errorf("Number of packets is wrong, got: %v", flows)
	}

	ipv4 := icmp[0].GetStatistics().GetEndpointsType(flow.FlowEndpointType_IPV4)
	if flows[0].GetStatistics().GetEndpointsType(flow.FlowEndpointType_ETHERNET).GetAB().Bytes < ipv4.GetAB().Bytes {
		t.Errorf("Layers bytes error, got: %v", icmp)
	}

	gremlin := `g.V().Has("Name", "br-sflow", "Type", "ovsbridge").Flows("LayersPath", "Ethernet/IPv4/ICMPv4/Payload")`
	endpoint := getFlowEndpoint(t, gremlin+fmt.Sprintf(`.Has("Statistics.Endpoints.ETHERNET.AB.Bytes", Gt(%d))`, pingLen-1), flow.FlowEndpointType_ETHERNET)
	if endpoint == nil || endpoint.GetAB().Bytes < pingLen {
		t.Errorf("Number of bytes is wrong, got: %v", endpoint)
	}

	endpoint = getFlowEndpoint(t, gremlin+fmt.Sprintf(`.Has("Statistics.Endpoints.ETHERNET.AB.Bytes", Gt(%d))`, pingLen), flow.FlowEndpointType_ETHERNET)
	if endpoint != nil {
		t.Errorf("Wrong number of flow, should have none, got : %v", endpoint)
	}

	endpoint = getFlowEndpoint(t, gremlin+fmt.Sprintf(`.Has("Statistics.Endpoints.ETHERNET.AB.Bytes", Gte(%d))`, pingLen), flow.FlowEndpointType_ETHERNET)
	if endpoint == nil || endpoint.GetAB().Bytes < pingLen {
		t.Errorf("Number of bytes is wrong, got: %v", endpoint)
	}

	endpoint = getFlowEndpoint(t, gremlin+fmt.Sprintf(`.Has("Statistics.Endpoints.ETHERNET.AB.Bytes", Gte(%d))`, pingLen+1), flow.FlowEndpointType_ETHERNET)
	if endpoint != nil {
		t.Errorf("Wrong number of flow, should have none, got : %v", endpoint)
	}

	endpoint = getFlowEndpoint(t, gremlin+fmt.Sprintf(`.Has("Statistics.Endpoints.ETHERNET.AB.Bytes", Lt(%d))`, pingLen+1), flow.FlowEndpointType_ETHERNET)
	if endpoint == nil || endpoint.GetAB().Bytes > pingLen {
		t.Errorf("Number of bytes is wrong, got: %v", endpoint)
	}

	endpoint = getFlowEndpoint(t, gremlin+fmt.Sprintf(`.Has("Statistics.Endpoints.ETHERNET.AB.Bytes", Lt(%d))`, pingLen), flow.FlowEndpointType_ETHERNET)
	if endpoint != nil {
		t.Errorf("Wrong number of flow, should have none, got : %v", endpoint)
	}

	endpoint = getFlowEndpoint(t, gremlin+fmt.Sprintf(`.Has("Statistics.Endpoints.ETHERNET.AB.Bytes", Lte(%d))`, pingLen), flow.FlowEndpointType_ETHERNET)
	if endpoint == nil || endpoint.GetAB().Bytes > pingLen {
		t.Errorf("Number of bytes is wrong, got: %v", endpoint)
	}

	endpoint = getFlowEndpoint(t, gremlin+fmt.Sprintf(`.Has("Statistics.Endpoints.ETHERNET.AB.Bytes", Lte(%d))`, pingLen-1), flow.FlowEndpointType_ETHERNET)
	if endpoint != nil {
		t.Errorf("Wrong number of flow, should have none, got : %v", endpoint)
	}

	endpoint = getFlowEndpoint(t, gremlin+fmt.Sprintf(`.Has("Statistics.Endpoints.ETHERNET.AB.Bytes", Inside(%d, %d))`, pingLen-1, pingLen+1), flow.FlowEndpointType_ETHERNET)
	if endpoint == nil || endpoint.GetAB().Bytes <= pingLen-1 || endpoint.GetAB().Bytes >= pingLen+1 {
		t.Errorf("Number of bytes is wrong, got: %v", endpoint)
	}

	endpoint = getFlowEndpoint(t, gremlin+fmt.Sprintf(`.Has("Statistics.Endpoints.ETHERNET.AB.Bytes", Inside(%d, %d))`, pingLen, pingLen+1), flow.FlowEndpointType_ETHERNET)
	if endpoint != nil {
		t.Errorf("Wrong number of flow, should have none, got : %v", endpoint)
	}

	endpoint = getFlowEndpoint(t, gremlin+fmt.Sprintf(`.Has("Statistics.Endpoints.ETHERNET.AB.Bytes", Between(%d, %d))`, pingLen, pingLen+1), flow.FlowEndpointType_ETHERNET)
	if endpoint == nil || endpoint.GetAB().Bytes <= pingLen-1 || endpoint.GetAB().Bytes >= pingLen+1 {
		t.Errorf("Number of bytes is wrong, got: %v", endpoint)
	}

	endpoint = getFlowEndpoint(t, gremlin+fmt.Sprintf(`.Has("Statistics.Endpoints.ETHERNET.AB.Bytes", Between(%d, %d))`, pingLen, pingLen), flow.FlowEndpointType_ETHERNET)
	if endpoint != nil {
		t.Errorf("Wrong number of flow, should have none, got : %v", endpoint)
	}

	bw := getFlowSetBandwidthFromGremlinReply(t, gremlin+".Dedup().Bandwidth()")
	if bw.NBFlow != 1 || bw.ABpackets != 1 || bw.BApackets != 1 || bw.ABbytes < 1066 || bw.BAbytes < 1066 {
		t.Errorf("Wrong bandwidth returned, got : %v", bw)
	}

	client.Delete("capture", capture.ID())
}
