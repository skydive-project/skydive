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
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/skydive-project/skydive/api"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/tests/helper"
	"github.com/skydive-project/skydive/topology"
)

const confAgentAnalyzer = `---
agent:
  listen: :58081
  X509_cert: {{.AgentX509_cert}}
  X509_key: {{.AgentX509_key}}
  analyzers: localhost:{{.AnalyzerPort}}
  topology:
    probes:
      - netlink
      - netns
      - ovsdb
  flow:
    probes:
      - ovssflow
      - gopacket

cache:
  expire: 300
  cleanup: 30

sflow:
  port_min: 55000
  port_max: 55005

analyzer:
  listen: :{{.AnalyzerPort}}
  X509_cert: {{.AnalyzerX509_cert}}
  X509_key: {{.AnalyzerX509_key}}
  flowtable_expire: 600
  flowtable_update: 20
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

const confAgentAnalyzerIPv6 = `---
agent:
  listen: "[::1]:58081"
  X509_cert: {{.AgentX509_cert}}
  X509_key: {{.AgentX509_key}}
  analyzers: "[::1]:{{.AnalyzerPort}}"
  topology:
    probes:
      - netlink
      - netns
      - ovsdb
  flow:
    probes:
      - ovssflow
      - gopacket

cache:
  expire: 300
  cleanup: 30

sflow:
  port_min: 55000
  port_max: 55005

analyzer:
  listen: "[::1]:{{.AnalyzerPort}}"
  X509_cert: {{.AnalyzerX509_cert}}
  X509_key: {{.AnalyzerX509_key}}
  flowtable_expire: 600
  flowtable_update: 20
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

func (s *TestStorage) SearchFlows(fsq flow.FlowSearchQuery) (*flow.FlowSet, error) {
	return nil, nil
}

func (s *TestStorage) SearchMetrics(ffsq flow.FlowSearchQuery, metricFilter *flow.Filter) (map[string][]*flow.FlowMetric, error) {
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

func TestSFlowProbeNode(t *testing.T) {
	ts := NewTestStorage()

	aa := helper.NewAgentAnalyzerWithConfig(t, confAgentAnalyzer, ts)
	aa.Start()
	defer aa.Stop()

	client, err := api.NewCrudClientFromConfig(&http.AuthenticationOpts{})
	if err != nil {
		t.Fatal(err.Error())
	}

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

	gh := helper.NewGremlinQueryHelper(&http.AuthenticationOpts{})

	node := gh.GetNodeFromGremlinReply(t, `g.V().Has("Name", "br-sflow", "Type", "ovsbridge")`)

	ok := false
	for _, f := range ts.GetFlows() {
		if f.NodeTID == node.Metadata()["TID"].(string) && f.LayersPath == "Ethernet/ARP/Payload" {
			ok = true
			break
		}
	}

	if !ok {
		t.Error("Unable to find a flow with the expected NodeTID")
	}

	client.Delete("capture", capture.ID())
}

func TestSFlowNodeTIDOvsInternalNetNS(t *testing.T) {
	ts := NewTestStorage()

	aa := helper.NewAgentAnalyzerWithConfig(t, confAgentAnalyzer, ts)
	aa.Start()
	defer aa.Stop()

	client, err := api.NewCrudClientFromConfig(&http.AuthenticationOpts{})
	if err != nil {
		t.Fatal(err.Error())
	}

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

	gh := helper.NewGremlinQueryHelper(&http.AuthenticationOpts{})

	node := gh.GetNodeFromGremlinReply(t, `g.V().Has("Name", "br-sflow", "Type", "ovsbridge")`)

	ok := false
	for _, f := range ts.GetFlows() {
		if f.NodeTID == node.Metadata()["TID"].(string) && f.LayersPath == "Ethernet/ARP/Payload" {
			ok = true
			break
		}
	}

	if !ok {
		t.Error("Unable to find a flow with the expected NodeTID")
	}

	client.Delete("capture", capture.ID())
}

func TestSFlowTwoNodeTID(t *testing.T) {
	ts := NewTestStorage()

	aa := helper.NewAgentAnalyzerWithConfig(t, confAgentAnalyzer, ts)
	aa.Start()
	defer func() {
		aa.Stop()
	}()

	client, err := api.NewCrudClientFromConfig(&http.AuthenticationOpts{})
	if err != nil {
		t.Fatal(err.Error())
	}

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
		t.Errorf("Should have 2 flow entries one per NodeTID got: %d", len(flows))
	}

	gh := helper.NewGremlinQueryHelper(&http.AuthenticationOpts{})

	node1 := gh.GetNodeFromGremlinReply(t, `g.V().Has("Name", "br-sflow1", "Type", "ovsbridge")`)
	node2 := gh.GetNodeFromGremlinReply(t, `g.V().Has("Name", "br-sflow2", "Type", "ovsbridge")`)

	if flows[0].NodeTID != node1.Metadata()["TID"].(string) &&
		flows[0].NodeTID != node2.Metadata()["TID"].(string) {
		t.Errorf("Bad NodeTID for the first flow: %s", flows[0].NodeTID)
	}

	if flows[1].NodeTID != node1.Metadata()["TID"].(string) &&
		flows[1].NodeTID != node2.Metadata()["TID"].(string) {
		t.Errorf("Bad NodeTID for the second flow: %s", flows[1].NodeTID)
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

	client, err := api.NewCrudClientFromConfig(&http.AuthenticationOpts{})
	if err != nil {
		t.Fatal(err.Error())
	}

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

	gh := helper.NewGremlinQueryHelper(&http.AuthenticationOpts{})

	node := gh.GetNodeFromGremlinReply(t, `g.V().Has("Name", "br-pcap", "Type", "bridge")`)

	ok := false
	flows := ts.GetFlows()
	for _, f := range flows {
		if f.NodeTID == node.Metadata()["TID"].(string) {
			ok = true
			break
		}
	}

	if !ok {
		t.Errorf("Unable to find a flow with the expected NodeTID: %v\n%v", flows, aa.Agent.Graph.String())
	}

	client.Delete("capture", capture.ID())
}

func TestPCAPProbeTLS(t *testing.T) {
	ts := NewTestStorage()

	params := []helper.HelperParams{make(helper.HelperParams)}
	cert, key := helper.GenerateFakeX509Certificate("server")
	params[0]["AnalyzerX509_cert"] = cert
	params[0]["AnalyzerX509_key"] = key
	cert, key = helper.GenerateFakeX509Certificate("client")
	params[0]["AgentX509_cert"] = cert
	params[0]["AgentX509_key"] = key
	aa := helper.NewAgentAnalyzerWithConfig(t, confAgentAnalyzer, ts, params...)
	aa.Start()
	defer aa.Stop()

	client, err := api.NewCrudClientFromConfig(&http.AuthenticationOpts{})
	if err != nil {
		t.Fatal(err.Error())
	}

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

	gh := helper.NewGremlinQueryHelper(&http.AuthenticationOpts{})

	node := gh.GetNodeFromGremlinReply(t, `g.V().Has("Name", "br-pcap", "Type", "bridge")`)

	ok := false
	flows := ts.GetFlows()
	for _, f := range flows {
		if f.NodeTID == node.Metadata()["TID"].(string) {
			ok = true
			break
		}
	}

	if !ok {
		t.Errorf("Unable to find a flow with the expected NodeTID: %v\n%v", flows, aa.Agent.Graph.String())
	}

	client.Delete("capture", capture.ID())
}

func TestSFlowSrcDstPath(t *testing.T) {
	ts := NewTestStorage()

	aa := helper.NewAgentAnalyzerWithConfig(t, confAgentAnalyzer, ts)
	aa.Start()
	defer aa.Stop()

	client, err := api.NewCrudClientFromConfig(&http.AuthenticationOpts{})
	if err != nil {
		t.Fatal(err.Error())
	}

	capture := api.NewCapture("G.V().Has('Name', 'br-sflow', 'Type', 'ovsbridge')", "")
	if err := client.Create("capture", capture); err != nil {
		t.Fatal(err.Error())
	}
	defer client.Delete("capture", capture.ID())

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

	gh := helper.NewGremlinQueryHelper(&http.AuthenticationOpts{})

	node1 := gh.GetNodeFromGremlinReply(t, `g.V().Has("Name", "sflow-intf1", "Type", "internal")`)
	node2 := gh.GetNodeFromGremlinReply(t, `g.V().Has("Name", "sflow-intf2", "Type", "internal")`)

	if node1 == nil {
		t.Errorf("Could not find sflow-intf1 internal interface: %s", aa.Analyzer.GraphServer.Graph.String())
		return
	}

	if node2 == nil {
		t.Errorf("Could not find sflow-intf2 internal interface: %s", aa.Analyzer.GraphServer.Graph.String())
		return
	}

	ok := false
	for _, f := range ts.GetFlows() {
		// we can have both way depending on which packet has been seen first
		if (f.ANodeTID == node1.Metadata()["TID"].(string) && f.BNodeTID == node2.Metadata()["TID"].(string)) ||
			(f.ANodeTID == node2.Metadata()["TID"].(string) && f.BNodeTID == node1.Metadata()["TID"].(string)) {
			ok = true
			break
		}
	}

	if !ok {
		t.Errorf("Unable to find flows with the expected path: %v\n %s", ts.GetFlows(), aa.Agent.Graph.String())
	}
}

func TestFlowQuery(t *testing.T) {
	delay := 500 * time.Second
	al := flow.NewTableAllocator(delay, delay)

	f := func(flows []*flow.Flow) {}

	ft1 := al.Alloc(f)

	flow.GenerateTestFlows(t, ft1, 1, "probe-tid1")
	flows1 := flow.GenerateTestFlows(t, ft1, 2, "probe-tid2")

	ft2 := al.Alloc(f)
	flows2 := flow.GenerateTestFlows(t, ft2, 3, "probe-tid2")

	ft1.Start()
	ft2.Start()

	time.Sleep(time.Second)

	obj, _ := proto.Marshal(&flow.FlowSearchQuery{
		Filter: &flow.Filter{
			BoolFilter: &flow.BoolFilter{
				Op: flow.BoolFilterOp_OR,
				Filters: []*flow.Filter{
					&flow.Filter{
						TermStringFilter: &flow.TermStringFilter{Key: "NodeTID", Value: "probe-tid2"},
					},
					&flow.Filter{
						TermStringFilter: &flow.TermStringFilter{Key: "ANodeTID", Value: "probe-tid2"},
					},
					&flow.Filter{
						TermStringFilter: &flow.TermStringFilter{Key: "BNodeTID", Value: "probe-tid2"},
					},
				},
			},
		},
	})

	query := &flow.TableQuery{
		Type: "FlowSearchQuery",
		Obj:  obj,
	}
	reply := al.QueryTable(query)

	ft1.Stop()
	ft2.Stop()

	flowset := flow.NewFlowSet()
	for _, r := range reply.Obj {
		var fsr flow.FlowSearchReply
		if err := proto.Unmarshal(r, &fsr); err != nil {
			t.Fatal(err.Error())
		}

		flowset.Merge(fsr.FlowSet, flow.MergeContext{})
	}

	if len(flowset.Flows) != len(flows1)+len(flows2) {
		t.Fatalf("FlowQuery should return at least one flow")
	}

	for _, flow := range flowset.Flows {
		if flow.NodeTID != "probe-tid2" {
			t.Fatalf("FlowQuery should only return flows with probe-tid2, got: %s", flow)
		}
	}
}

func TestTableServer(t *testing.T) {
	ts := NewTestStorage()

	aa := helper.NewAgentAnalyzerWithConfig(t, confAgentAnalyzer, ts)
	fclient := flow.NewTableClient(aa.Analyzer.WSServer)

	aa.Start()
	defer aa.Stop()

	client, err := api.NewCrudClientFromConfig(&http.AuthenticationOpts{})
	if err != nil {
		t.Fatal(err.Error())
	}

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

	gh := helper.NewGremlinQueryHelper(&http.AuthenticationOpts{})

	node := gh.GetNodeFromGremlinReply(t, `g.V().Has("Name", "br-sflow", "Type", "ovsbridge")`)

	hnmap := make(topology.HostNodeTIDMap)
	hnmap[node.Host()] = append(hnmap[node.Host()], node.Metadata()["TID"].(string))

	fsq := flow.FlowSearchQuery{}
	flowset, err := fclient.LookupFlowsByNodes(hnmap, fsq)
	if err != nil {
		t.Fatal(err.Error())
	}

	aa.Flush()

	if len(ts.GetFlows()) != len(flowset.Flows) {
		t.Fatalf("Should return the same number of flows than in the database, got: %v", flowset)
	}

	for _, f := range flowset.Flows {
		if f.NodeTID != node.Metadata()["TID"].(string) {
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

	client, err := api.NewCrudClientFromConfig(&http.AuthenticationOpts{})
	if err != nil {
		t.Fatal(err.Error())
	}

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

	gh := helper.NewGremlinQueryHelper(&http.AuthenticationOpts{})

	node := gh.GetNodeFromGremlinReply(t, `g.V().Has("Name", "br-sflow", "Type", "ovsbridge")`)

	var count int64
	gh.GremlinQuery(t, `G.V().Has("Name", "br-sflow", "Type", "ovsbridge").Count()`, &count)
	if count != 1 {
		t.Fatalf("Should return 1, got: %d", count)
	}

	tid := node.Metadata()["TID"].(string)

	flows := gh.GetFlowsFromGremlinReply(t, `g.V().Has("Name", "br-sflow", "Type", "ovsbridge").Flows().Has("NodeTID", "`+tid+`")`)
	if len(flows) == 0 {
		t.Fatalf("Should return at least 1 flow, got: %v", flows)
	}

	flowsOpt := gh.GetFlowsFromGremlinReply(t, `g.V().Has("Name", "br-sflow", "Type", "ovsbridge").Flows().Has("NodeTID", "`+tid+`")`)
	if len(flowsOpt) != len(flows) {
		t.Fatalf("Should return the same number of flows that without optimisation, got: %v", flowsOpt)
	}

	nodes := gh.GetNodesFromGremlinReply(t, `g.V().Has("Name", "br-sflow", "Type", "ovsbridge").Flows().Has("NodeTID", "`+tid+`").Out()`)
	if len(nodes) != 0 {
		t.Fatalf("Should return no destination node, got %d", len(nodes))
	}

	nodes = gh.GetNodesFromGremlinReply(t, `g.V().Has("Name", "br-sflow", "Type", "ovsbridge").Flows().Has("NodeTID", "`+tid+`").Both().Dedup()`)
	if len(nodes) != 1 {
		t.Fatalf("Should return one node, got %d", len(nodes))
	}

	nodes = gh.GetNodesFromGremlinReply(t, `g.V().Has("Name", "br-sflow", "Type", "ovsbridge").Flows().Has("NodeTID", "`+tid+`").In().Dedup()`)
	if len(nodes) != 1 {
		t.Fatalf("Should return one source node, got %d", len(nodes))
	}

	gh.GremlinQuery(t, `g.V().Has("Name", "br-sflow", "Type", "ovsbridge").Flows().Has("NodeTID", "`+tid+`").Count()`, &count)
	if int(count) != len(flows) {
		t.Fatalf("Gremlin count doesn't correspond to the number of flows, got: %v, expected: %v", len(flows), count)
	}

	aa.Flush()

	if len(ts.GetFlows()) != int(count) {
		t.Fatalf("Should get the same number of flows in the database than previously in the agent, got: %v", ts.GetFlows())
	}

	client.Delete("capture", capture.ID())
}

func getFlowEndpoint(t *testing.T, gremlin string, protocol flow.FlowProtocol) *flow.FlowMetric {
	gh := helper.NewGremlinQueryHelper(&http.AuthenticationOpts{})

	flows := gh.GetFlowsFromGremlinReply(t, gremlin)
	if len(flows) != 1 {
		return nil
	}
	return flows[0].Metric
}

func queryFlowMetrics(t *testing.T, timeContext int64, pings int64) {
	graphGremlin := "g"

	gh := helper.NewGremlinQueryHelper(&http.AuthenticationOpts{})

	if timeContext != -1 {
		graphGremlin += fmt.Sprintf(".Context(%d)", timeContext)
	}

	ovsGremlin := graphGremlin + `.V().Has("Name", "br-sflow", "Type", "ovsbridge")`
	ovsBridge := gh.GetNodeFromGremlinReply(t, ovsGremlin)
	gremlin := ovsGremlin + `.Flows().Has("LayersPath", Regex(".*ICMPv4.*"))`

	icmp := gh.GetFlowsFromGremlinReply(t, gremlin)
	switch len(icmp) {
	case 0:
		t.Error("Should return one icmp flow, got none")
		return
	case 1:
	default:
		t.Errorf("Should return only one icmp flow, got: %v", icmp)
	}
	if icmp[0].LayersPath != "Ethernet/IPv4/ICMPv4/Payload" {
		t.Errorf("Wrong layer path, should be 'Ethernet/IPv4/ICMPv4/Payload', got %s", icmp[0].LayersPath)
	}

	ethernet := icmp[0].Metric
	if ethernet.BAPackets != pings {
		t.Errorf("Number of packets is wrong, got: %v", ethernet.BAPackets)
	}

	if ethernet.ABBytes < pings*1066 || ethernet.BABytes < pings*1066 {
		t.Errorf("Number of bytes is wrong, got: %v", ethernet.BABytes)
	}

	flows := gh.GetFlowsFromGremlinReply(t, fmt.Sprintf(`%s.Flows().Has("LayersPath", "Ethernet/IPv4/ICMPv4/Payload", "Metric.ABPackets", %d)`, ovsGremlin, pings))
	if len(flows) != 1 || flows[0].Metric.BAPackets != pings {
		t.Errorf("Number of packets is wrong, got %d, flows: %v", len(flows), flows)
	}

	ipv4 := icmp[0].Metric // FIXME double check protocol Network = IPv4/v6
	if flows[0].Metric.ABBytes < ipv4.ABBytes {
		t.Errorf("Layers bytes error, got: %v", icmp)
	}

	pingLen := icmp[0].Metric.ABBytes
	endpoint := getFlowEndpoint(t, gremlin+fmt.Sprintf(`.Has("Metric.ABBytes", Gt(%d))`, pingLen-1), flow.FlowProtocol_ETHERNET)
	if endpoint == nil || endpoint.ABBytes < pingLen {
		t.Errorf("Number of bytes is wrong, got: %v", endpoint)
	}

	endpoint = getFlowEndpoint(t, gremlin+fmt.Sprintf(`.Has("Metric.ABBytes", Gt(%d))`, pingLen), flow.FlowProtocol_ETHERNET)
	if endpoint != nil {
		t.Errorf("Wrong number of flow, should have none, got : %v", endpoint)
	}

	endpoint = getFlowEndpoint(t, gremlin+fmt.Sprintf(`.Has("Metric.ABBytes", Gte(%d))`, pingLen), flow.FlowProtocol_ETHERNET)
	if endpoint == nil || endpoint.ABBytes < pingLen {
		t.Errorf("Number of bytes is wrong, got: %v", endpoint)
	}

	endpoint = getFlowEndpoint(t, gremlin+fmt.Sprintf(`.Has("Metric.ABBytes", Gte(%d))`, pingLen+1), flow.FlowProtocol_ETHERNET)
	if endpoint != nil {
		t.Errorf("Wrong number of flow, should have none, got : %v", endpoint)
	}

	endpoint = getFlowEndpoint(t, gremlin+fmt.Sprintf(`.Has("Metric.ABBytes", Lt(%d))`, pingLen+1), flow.FlowProtocol_ETHERNET)
	if endpoint == nil || endpoint.ABBytes > pingLen {
		t.Errorf("Number of bytes is wrong, got: %v", endpoint)
	}

	endpoint = getFlowEndpoint(t, gremlin+fmt.Sprintf(`.Has("Metric.ABBytes", Lt(%d))`, pingLen), flow.FlowProtocol_ETHERNET)
	if endpoint != nil {
		t.Errorf("Wrong number of flow, should have none, got : %v", endpoint)
	}

	endpoint = getFlowEndpoint(t, gremlin+fmt.Sprintf(`.Has("Metric.ABBytes", Lte(%d))`, pingLen), flow.FlowProtocol_ETHERNET)
	if endpoint == nil || endpoint.ABBytes > pingLen {
		t.Errorf("Number of bytes is wrong, got: %v", endpoint)
	}

	endpoint = getFlowEndpoint(t, gremlin+fmt.Sprintf(`.Has("Metric.ABBytes", Lte(%d))`, pingLen-1), flow.FlowProtocol_ETHERNET)
	if endpoint != nil {
		t.Errorf("Wrong number of flow, should have none, got : %v", endpoint)
	}

	endpoint = getFlowEndpoint(t, gremlin+fmt.Sprintf(`.Has("Metric.ABBytes", Inside(%d, %d))`, pingLen-1, pingLen+1), flow.FlowProtocol_ETHERNET)
	if endpoint == nil || endpoint.ABBytes <= pingLen-1 || endpoint.ABBytes >= pingLen+1 {
		t.Errorf("Number of bytes is wrong, got: %v", endpoint)
	}

	endpoint = getFlowEndpoint(t, gremlin+fmt.Sprintf(`.Has("Metric.ABBytes", Inside(%d, %d))`, pingLen, pingLen+1), flow.FlowProtocol_ETHERNET)
	if endpoint != nil {
		t.Errorf("Wrong number of flow, should have none, got : %v", endpoint)
	}

	endpoint = getFlowEndpoint(t, gremlin+fmt.Sprintf(`.Has("Metric.ABBytes", Between(%d, %d))`, pingLen, pingLen+1), flow.FlowProtocol_ETHERNET)
	if endpoint == nil || endpoint.ABBytes <= pingLen-1 || endpoint.ABBytes >= pingLen+1 {
		t.Errorf("Number of bytes is wrong, got: %v", endpoint)
	}

	endpoint = getFlowEndpoint(t, gremlin+fmt.Sprintf(`.Has("Metric.ABBytes", Between(%d, %d))`, pingLen, pingLen), flow.FlowProtocol_ETHERNET)
	if endpoint != nil {
		t.Errorf("Wrong number of flow, should have none, got : %v", endpoint)
	}

	nodes := gh.GetNodesFromGremlinReply(t, gremlin+".Hops()")
	if len(nodes) != 1 {
		t.Errorf("Hops should return one node, got %d", len(nodes))
	} else if nodes[0].ID != ovsBridge.ID {
		t.Errorf("Hops should return the node %v, got %v\n", ovsBridge, nodes[0])
	}
}

func TestFlowMetrics(t *testing.T) {
	ts := NewTestStorage()

	aa := helper.NewAgentAnalyzerWithConfig(t, confAgentAnalyzer, ts)
	aa.Start()
	defer aa.Stop()

	client, err := api.NewCrudClientFromConfig(&http.AuthenticationOpts{})
	if err != nil {
		t.Fatal(err.Error())
	}

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
		{"sleep 2", false},

		{"ip netns exec sflow-vm1 ping -c 1 -s 1024 -I sflow-intf1 169.254.33.34", false},
	}

	tearDownCmds := []helper.Cmd{
		{"ip netns del sflow-vm1", true},
		{"ip netns del sflow-vm2", true},
		{"ovs-vsctl del-br br-sflow", true},
	}

	helper.ExecCmds(t, setupCmds...)
	defer helper.ExecCmds(t, tearDownCmds...)

	time.Sleep(1 * time.Second)

	queryFlowMetrics(t, -1, 1)

	client.Delete("capture", capture.ID())
}

func TestFlowMetricsSum(t *testing.T) {
	ts := NewTestStorage()

	aa := helper.NewAgentAnalyzerWithConfig(t, confAgentAnalyzer, ts)
	aa.Start()
	defer aa.Stop()

	client, err := api.NewCrudClientFromConfig(&http.AuthenticationOpts{})
	if err != nil {
		t.Fatal(err.Error())
	}

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
		{"sleep 2", false},

		{"ip netns exec sflow-vm1 ping -c 1 -s 1024 -I sflow-intf1 169.254.33.34", false},
	}

	tearDownCmds := []helper.Cmd{
		{"ip netns del sflow-vm1", true},
		{"ip netns del sflow-vm2", true},
		{"ovs-vsctl del-br br-sflow", true},
	}

	helper.ExecCmds(t, setupCmds...)
	defer helper.ExecCmds(t, tearDownCmds...)

	// since the agent update ticker is about 10 sec according to the configuration
	// wait 11 sec to have the first update and the MetricRange filled
	time.Sleep(11 * time.Second)

	gh := helper.NewGremlinQueryHelper(&http.AuthenticationOpts{})

	gremlin := `g.V().Has("Name", "br-sflow", "Type", "ovsbridge").Flows().Has("LayersPath", "Ethernet/IPv4/ICMPv4/Payload").Dedup().Metrics().Sum()`

	// this check needs to be close to the beginning of the test since it's a time
	// based test and it will fail if we wait one more update tick
	metric := gh.GetFlowMetricFromGremlinReply(t, gremlin)
	if metric.ABPackets != 1 || metric.BAPackets != 1 || metric.ABBytes < 1066 || metric.BABytes < 1066 {
		t.Errorf("Wrong metric returned, got : %v", metric)
	}

	client.Delete("capture", capture.ID())
}

func TestFlowHops(t *testing.T) {
	ts := NewTestStorage()

	aa := helper.NewAgentAnalyzerWithConfig(t, confAgentAnalyzer, ts)
	aa.Start()
	defer aa.Stop()

	client, err := api.NewCrudClientFromConfig(&http.AuthenticationOpts{})
	if err != nil {
		t.Fatal(err.Error())
	}

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
		{"sleep 2", false},

		{"ip netns exec sflow-vm1 ping -c 1 -s 1024 -I sflow-intf1 169.254.33.34", false},
	}

	tearDownCmds := []helper.Cmd{
		{"ip netns del sflow-vm1", true},
		{"ip netns del sflow-vm2", true},
		{"ovs-vsctl del-br br-sflow", true},
	}

	helper.ExecCmds(t, setupCmds...)
	defer helper.ExecCmds(t, tearDownCmds...)

	// since the agent update ticker is about 10 sec according to the configuration
	// wait 11 sec to have the first update and the MetricRange filled
	time.Sleep(11 * time.Second)

	gh := helper.NewGremlinQueryHelper(&http.AuthenticationOpts{})

	gremlin := `g.Flows().Has("LayersPath", "Ethernet/IPv4/ICMPv4/Payload")`
	flows := gh.GetFlowsFromGremlinReply(t, gremlin)
	if len(flows) != 1 {
		t.Fatal("We should receive only one ICMPv4 flow")
	}
	gremlin = fmt.Sprintf(`g.Flows().Has("TrackingID", "%s").Nodes()`, flows[0].TrackingID)
	tnodes := gh.GetNodesFromGremlinReply(t, gremlin)
	if len(tnodes) != 3 {
		t.Fatal("We should have 3 nodes NodeTID,A,B")
	}
	gremlin = `g.Flows().Has("LayersPath", "Ethernet/IPv4/ICMPv4/Payload").Hops()`
	nodes := gh.GetNodesFromGremlinReply(t, gremlin)
	if len(nodes) != 1 {
		t.Fatal("We should have 1 node NodeTID")
	}

	found := false
	m := nodes[0].Metadata()
	for _, n := range tnodes {
		if n.MatchMetadata(m) == true {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("We should found the Hops nodes in the TrackingID nodes")
	}

	client.Delete("capture", capture.ID())
}

func TestIPv6FlowHopsIPv6(t *testing.T) {
	if !common.IPv6Supported() {
		t.Skipf("Platform doesn't support IPv6")
	}

	ts := NewTestStorage()

	aa := helper.NewAgentAnalyzerWithConfig(t, confAgentAnalyzerIPv6, ts)
	aa.Start()
	defer aa.Stop()

	client, err := api.NewCrudClientFromConfig(&http.AuthenticationOpts{})
	if err != nil {
		t.Fatal(err.Error())
	}

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
		{"ip netns exec sflow-vm1 ip address add fd49:37c8:5229::1/48 dev sflow-intf1", true},
		{"ip netns exec sflow-vm1 ip link set sflow-intf1 up", true},

		{"ovs-vsctl add-port br-sflow sflow-intf2 -- set interface sflow-intf2 type=internal", true},
		{"ip netns add sflow-vm2", true},
		{"ip link set sflow-intf2 netns sflow-vm2", true},
		{"ip netns exec sflow-vm2 ip address add fd49:37c8:5229::2/48 dev sflow-intf2", true},
		{"ip netns exec sflow-vm2 ip link set sflow-intf2 up", true},

		// wait to have everything ready, sflow, interfaces
		{"sleep 2", false},

		{"ip netns exec sflow-vm1 ping6 -c 5 -s 1024 -I sflow-intf1 fd49:37c8:5229::2", true},
	}

	tearDownCmds := []helper.Cmd{
		{"ip netns del sflow-vm1", true},
		{"ip netns del sflow-vm2", true},
		{"ovs-vsctl del-br br-sflow", true},
	}

	helper.ExecCmds(t, setupCmds...)
	defer helper.ExecCmds(t, tearDownCmds...)

	// since the agent update ticker is about 10 sec according to the configuration
	// wait 11 sec to have the first update and the MetricRange filled
	time.Sleep(11 * time.Second)

	gh := helper.NewGremlinQueryHelper(&http.AuthenticationOpts{})

	gremlin := `g.Flows().Has("LayersPath", "Ethernet/IPv6/ICMPv6/Payload")`
	/* filterIPv6AddrAnd() as we received multicast/broadcast from fresh registered interfances announcement */
	allFlows := gh.GetFlowsFromGremlinReply(t, gremlin)
	flows := helper.FilterIPv6AddrAnd(allFlows, "fd49:37c8:5229::1", "fd49:37c8:5229::2")
	if len(flows) != 1 {
		t.Fatal("We should receive only one ICMPv6 flow")
	}
	gremlin = fmt.Sprintf(`g.Flows().Has("TrackingID", "%s").Nodes()`, flows[0].TrackingID)
	tnodes := gh.GetNodesFromGremlinReply(t, gremlin)
	if len(tnodes) != 3 {
		t.Fatal("We should have 3 nodes NodeTID,A,B")
	}
	/* Dedup() here for same reason than above ^^^ */
	gremlin = `g.Flows().Has("LayersPath", "Ethernet/IPv6/ICMPv6/Payload").Hops().Dedup()`
	nodes := gh.GetNodesFromGremlinReply(t, gremlin)
	if len(nodes) != 1 {
		t.Fatal("We should have 1 node NodeTID")
	}

	found := false
	m := nodes[0].Metadata()
	for _, n := range tnodes {
		if n.MatchMetadata(m) == true {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("We should found the Hops nodes in the TrackingID nodes")
	}

	client.Delete("capture", capture.ID())
}

func TestFlowGRETunnel(t *testing.T) {
	testFlowTunnel(t, "gre")
}

func TestFlowVxlanTunnel(t *testing.T) {
	testFlowTunnel(t, "vxlan")
}

// tunnelType is gre or vxlan
func testFlowTunnel(t *testing.T, tunnelType string) {
	ts := NewTestStorage()

	aa := helper.NewAgentAnalyzerWithConfig(t, confAgentAnalyzer, ts)
	aa.Start()
	defer aa.Stop()

	client, err := api.NewCrudClientFromConfig(&http.AuthenticationOpts{})
	if err != nil {
		t.Fatal(err.Error())
	}

	capture1 := api.NewCapture("G.V().Has('Name', 'tunnel-vm1').Out().Has('Name', 'tunnel')", "")
	if err := client.Create("capture", capture1); err != nil {
		t.Fatal(err.Error())
	}

	capture2 := api.NewCapture("G.V().Has('Name', 'tunnel-vm2-eth0')", "")
	if err := client.Create("capture", capture2); err != nil {
		t.Fatal(err.Error())
	}
	time.Sleep(1 * time.Second)

	setupCmds := []helper.Cmd{
		{"sudo ovs-vsctl add-br br-tunnel", true},

		{"sudo ip netns add tunnel-vm1", true},
		{"sudo ip link add tunnel-vm1-eth0 type veth peer name eth0 netns tunnel-vm1", true},
		{"sudo ip link set tunnel-vm1-eth0 up", true},

		{"sudo ip netns exec tunnel-vm1 ip link set eth0 up", true},
		{"sudo ip netns exec tunnel-vm1 ip address add 172.16.0.1/24 dev eth0", true},

		{"sudo ip netns add tunnel-vm2", true},
		{"sudo ip link add tunnel-vm2-eth0 type veth peer name eth0 netns tunnel-vm2", true},
		{"sudo ip link set tunnel-vm2-eth0 up", true},
		{"sudo ip netns exec tunnel-vm2 ip link set eth0 up", true},
		{"sudo ip netns exec tunnel-vm2 ip address add 172.16.0.2/24 dev eth0", true},

		{"sudo ovs-vsctl add-port br-tunnel tunnel-vm1-eth0", true},
		{"sudo ovs-vsctl add-port br-tunnel tunnel-vm2-eth0", true}}

	tunnelAdd := ""
	if tunnelType == "gre" {
		tunnelAdd = "sudo ip netns exec tunnel-vm1 ip tunnel add tunnel mode gre remote 172.16.0.2 local 172.16.0.1 ttl 255"
	} else {
		tunnelAdd = "sudo ip netns exec tunnel-vm1 ip link add tunnel type vxlan id 10 remote 172.16.0.2 local 172.16.0.1 ttl 255 dev eth0 dstport 4789"
	}
	setupCmds = append(setupCmds, []helper.Cmd{
		{tunnelAdd, true},
		{"sudo ip netns exec tunnel-vm1 ip link set tunnel up", true},
		{"sudo ip netns exec tunnel-vm1 ip link add name dummy0 type dummy", true},
		{"sudo ip netns exec tunnel-vm1 ip link set dummy0 up", true},
		{"sudo ip netns exec tunnel-vm1 ip a add 192.168.0.1/32 dev dummy0", true},
		{"sudo ip netns exec tunnel-vm1 ip r add 192.168.0.0/24 dev tunnel", true}}...)

	if tunnelType == "gre" {
		tunnelAdd = "sudo ip netns exec tunnel-vm2 ip tunnel add tunnel mode gre remote 172.16.0.1 local 172.16.0.2 ttl 255"
	} else {
		tunnelAdd = "sudo ip netns exec tunnel-vm2 ip link add tunnel type vxlan id 10 remote 172.16.0.1 local 172.16.0.2 ttl 255 dev eth0 dstport 4789"
	}
	setupCmds = append(setupCmds, []helper.Cmd{
		{tunnelAdd, true},
		{"sudo ip netns exec tunnel-vm2 ip link set tunnel up", true},
		{"sudo ip netns exec tunnel-vm2 ip link add name dummy0 type dummy", true},
		{"sudo ip netns exec tunnel-vm2 ip link set dummy0 up", true},
		{"sudo ip netns exec tunnel-vm2 ip a add 192.168.0.2/32 dev dummy0", true},
		{"sudo ip netns exec tunnel-vm2 ip r add 192.168.0.0/24 dev tunnel", true},
		{"sleep 10", false},
		{"sudo ip netns exec tunnel-vm1 ping -c 5 -I 192.168.0.1 192.168.0.2", false}}...)

	tearDownCmds := []helper.Cmd{
		{"ip netns del tunnel-vm1", true},
		{"ip netns del tunnel-vm2", true},
		{"ovs-vsctl del-br br-tunnel", true},
	}

	helper.ExecCmds(t, setupCmds...)
	defer helper.ExecCmds(t, tearDownCmds...)

	gh := helper.NewGremlinQueryHelper(&http.AuthenticationOpts{})

	flowsInnerTunnel := gh.GetFlowsFromGremlinReply(t, `G.V().Has('Name', 'tunnel-vm1').Out().Has('Name', 'tunnel').Flows()`)
	flowsBridge := gh.GetFlowsFromGremlinReply(t, `G.V().Has('Name', 'tunnel-vm2-eth0').Flows()`)

	var TrackID string
	for _, flow := range flowsInnerTunnel {
		// A vxlan innerpacket contains Ethernet while gre
		// innerpacket does not
		if strings.Contains(flow.LayersPath, "IPv4/ICMPv4/Payload") {
			if TrackID != "" {
				t.Errorf("We should only found one ICMPv4 flow in the tunnel %v", flowsInnerTunnel)
			}
			TrackID = flow.TrackingID
		}
	}

	success := false
	for _, f := range flowsBridge {
		if TrackID == f.TrackingID && strings.Contains(f.LayersPath, "ICMPv4/Payload") && f.Network != nil && f.Network.Protocol == flow.FlowProtocol_IPV4 {
			success = true
			break
		}
	}

	if !success {
		t.Errorf("TrackingID not found in %s tunnel: leaving the interface(%v) == seen in the tunnel(%v)", tunnelType, flowsInnerTunnel, flowsBridge)
	}

	client.Delete("capture", capture1.ID())
	client.Delete("capture", capture2.ID())
}

func TestFlowGRETunnelPCAPLegacy(t *testing.T) {
	ts := NewTestStorage()

	aa := helper.NewAgentAnalyzerWithConfig(t, confAgentAnalyzer, ts)
	aa.Start()
	defer aa.Stop()

	client, err := api.NewCrudClientFromConfig(&http.AuthenticationOpts{})
	if err != nil {
		t.Fatal(err.Error())
	}

	capture1 := api.NewCapture("G.V().Has('Name', 'gre-vm1').Out().Has('Name', 'gre')", "")
	if err := client.Create("capture", capture1); err != nil {
		t.Fatal(err.Error())
	}
	capture1.Type = "pcap"

	capture2 := api.NewCapture("G.V().Has('Name', 'gre-vm2-eth0')", "")
	if err := client.Create("capture", capture2); err != nil {
		t.Fatal(err.Error())
	}
	capture2.Type = "pcap"

	time.Sleep(1 * time.Second)
	setupCmds := []helper.Cmd{
		{"sudo ovs-vsctl add-br br-gre", true},

		{"sudo ip netns add gre-vm1", true},
		{"sudo ip link add gre-vm1-eth0 type veth peer name eth0 netns gre-vm1", true},
		{"sudo ip link set gre-vm1-eth0 up", true},

		{"sudo ip netns exec gre-vm1 ip link set eth0 up", true},
		{"sudo ip netns exec gre-vm1 ip address add 172.16.0.1/24 dev eth0", true},

		{"sudo ip netns add gre-vm2", true},
		{"sudo ip link add gre-vm2-eth0 type veth peer name eth0 netns gre-vm2", true},
		{"sudo ip link set gre-vm2-eth0 up", true},
		{"sudo ip netns exec gre-vm2 ip link set eth0 up", true},
		{"sudo ip netns exec gre-vm2 ip address add 172.16.0.2/24 dev eth0", true},

		{"sudo ovs-vsctl add-port br-gre gre-vm1-eth0", true},
		{"sudo ovs-vsctl add-port br-gre gre-vm2-eth0", true},

		{"sudo ip netns exec gre-vm1 ip tunnel add gre mode gre remote 172.16.0.2 local 172.16.0.1 ttl 255", true},
		{"sudo ip netns exec gre-vm1 ip l set gre up", true},
		{"sudo ip netns exec gre-vm1 ip link add name dummy0 type dummy", true},
		{"sudo ip netns exec gre-vm1 ip l set dummy0 up", true},
		{"sudo ip netns exec gre-vm1 ip a add 192.168.0.1/32 dev dummy0", true},
		{"sudo ip netns exec gre-vm1 ip r add 192.168.0.0/24 dev gre", true},

		{"sudo ip netns exec gre-vm2 ip tunnel add gre mode gre remote 172.16.0.1 local 172.16.0.2 ttl 255", true},
		{"sudo ip netns exec gre-vm2 ip l set gre up", true},
		{"sudo ip netns exec gre-vm2 ip link add name dummy0 type dummy", true},
		{"sudo ip netns exec gre-vm2 ip l set dummy0 up", true},
		{"sudo ip netns exec gre-vm2 ip a add 192.168.0.2/32 dev dummy0", true},
		{"sudo ip netns exec gre-vm2 ip r add 192.168.0.0/24 dev gre", true},

		{"sleep 10", false},

		{"sudo ip netns exec gre-vm1 ping -c 10 -I 192.168.0.1 192.168.0.2", false},
	}

	tearDownCmds := []helper.Cmd{
		{"ip netns del gre-vm1", true},
		{"ip netns del gre-vm2", true},
		{"ovs-vsctl del-br br-gre", true},
	}

	helper.ExecCmds(t, setupCmds...)
	defer helper.ExecCmds(t, tearDownCmds...)

	gh := helper.NewGremlinQueryHelper(&http.AuthenticationOpts{})

	flowsInnerTunnel := gh.GetFlowsFromGremlinReply(t, `G.V().Has('Name', 'gre-vm1').Out().Has('Name', 'gre').Flows()`)
	flowsBridge := gh.GetFlowsFromGremlinReply(t, `G.V().Has('Name', 'gre-vm2-eth0').Flows()`)

	var TrackID string
	for _, flow := range flowsInnerTunnel {
		if flow.LayersPath == "IPv4/ICMPv4/Payload" {
			if TrackID != "" {
				t.Errorf("We should only found one ICMPv4 flow in the tunnel %v", flowsInnerTunnel)
			}
			TrackID = flow.TrackingID
		}
	}

	success := false
	for _, f := range flowsBridge {
		if TrackID == f.TrackingID && strings.Contains(f.LayersPath, "ICMPv4/Payload") && f.Network != nil && f.Network.Protocol == flow.FlowProtocol_IPV4 {
			success = true
			break
		}
	}

	if !success {
		t.Errorf("TrackingID not found in GRE tunnel: %v == %v", flowsInnerTunnel, flowsBridge)
	}

	client.Delete("capture", capture1.ID())
	client.Delete("capture", capture2.ID())
}

func TestIPv6FlowGRETunnelIPv6(t *testing.T) {
	t.Skip("Fedora seems didn't support ip6gre tunnel for the moment")

	if !common.IPv6Supported() {
		t.Skipf("Platform doesn't support IPv6")
	}

	ts := NewTestStorage()

	aa := helper.NewAgentAnalyzerWithConfig(t, confAgentAnalyzerIPv6, ts)
	aa.Start()
	defer aa.Stop()

	client, err := api.NewCrudClientFromConfig(&http.AuthenticationOpts{})
	if err != nil {
		t.Fatal(err.Error())
	}

	capture1 := api.NewCapture("G.V().Has('Name', 'gre-vm1').Out().Has('Name', 'gre')", "")
	if err := client.Create("capture", capture1); err != nil {
		t.Fatal(err.Error())
	}

	capture2 := api.NewCapture("G.V().Has('Name', 'gre-vm2-eth0')", "")
	if err := client.Create("capture", capture2); err != nil {
		t.Fatal(err.Error())
	}

	time.Sleep(1 * time.Second)
	setupCmds := []helper.Cmd{
		{"sudo ovs-vsctl add-br br-gre", true},

		{"sudo ip netns add gre-vm1", true},
		{"sudo ip link add gre-vm1-eth0 type veth peer name eth0 netns gre-vm1", true},
		{"sudo ip link set gre-vm1-eth0 up", true},

		{"sudo ip netns exec gre-vm1 ip link set eth0 up", true},
		{"sudo ip netns exec gre-vm1 ip address add fd49:37c8:5229::1/48 dev eth0", true},

		{"sudo ip netns add gre-vm2", true},
		{"sudo ip link add gre-vm2-eth0 type veth peer name eth0 netns gre-vm2", true},
		{"sudo ip link set gre-vm2-eth0 up", true},
		{"sudo ip netns exec gre-vm2 ip link set eth0 up", true},
		{"sudo ip netns exec gre-vm2 ip address add fd49:37c8:5229::2/48 dev eth0", true},

		{"sudo ovs-vsctl add-port br-gre gre-vm1-eth0", true},
		{"sudo ovs-vsctl add-port br-gre gre-vm2-eth0", true},

		{"sudo ip netns exec gre-vm1 ip tunnel add gre mode ip6gre remote fd49:37c8:5229::2/48 local fd49:37c8:5229::1/48 ttl 255", true},
		{"sudo ip netns exec gre-vm1 ip l set gre up", true},
		{"sudo ip netns exec gre-vm1 ip link add name dummy0 type dummy", true},
		{"sudo ip netns exec gre-vm1 ip l set dummy0 up", true},
		{"sudo ip netns exec gre-vm1 ip a add fdfe:38b:489c::1/128 dev dummy0", true},
		{"sudo ip netns exec gre-vm1 ip r add fdfe:38b:489c::/48 dev gre", true},

		{"sudo ip netns exec gre-vm2 ip tunnel add gre mode ip6gre remote fd49:37c8:5229::1/48 local fd49:37c8:5229::2/48 ttl 255", true},
		{"sudo ip netns exec gre-vm2 ip l set gre up", true},
		{"sudo ip netns exec gre-vm2 ip link add name dummy0 type dummy", true},
		{"sudo ip netns exec gre-vm2 ip l set dummy0 up", true},
		{"sudo ip netns exec gre-vm2 ip a add fdfe:38b:489c::2/128 dev dummy0", true},
		{"sudo ip netns exec gre-vm2 ip r add fdfe:38b:489c::/48 dev gre", true},

		{"sudo ip netns exec gre-vm1 ping6 -c 5 -I fdfe:38b:489c::1 fdfe:38b:489c::2", false},
	}

	tearDownCmds := []helper.Cmd{
		{"ip netns del gre-vm1", true},
		{"ip netns del gre-vm2", true},
		{"ovs-vsctl del-br br-gre", true},
	}

	helper.ExecCmds(t, setupCmds...)
	defer helper.ExecCmds(t, tearDownCmds...)

	gh := helper.NewGremlinQueryHelper(&http.AuthenticationOpts{})

	flowsInnerTunnel := gh.GetFlowsFromGremlinReply(t, `G.V().Has('Name', 'gre-vm1').Out().Has('Name', 'gre').Flows()`)
	flowsBridge := gh.GetFlowsFromGremlinReply(t, `G.V().Has('Name', 'gre-vm2-eth0').Flows()`)

	var TrackID string
	for _, flow := range flowsInnerTunnel {
		if flow.LayersPath == "IPv6/ICMPv6/Payload" {
			if TrackID != "" {
				t.Errorf("We should only found one ICMPv6 flow in the tunnel %v", flowsInnerTunnel)
			}
			TrackID = flow.TrackingID
		}
	}

	success := false
	for _, f := range flowsBridge {
		if TrackID == f.TrackingID && strings.HasSuffix(f.LayersPath, "ICMPv6/Payload") && f.Network != nil && f.Network.Protocol == flow.FlowProtocol_IPV6 {
			success = true
			break
		}
	}

	if !success {
		t.Errorf("TrackingID not found in GRE tunnel: %v == %v", flowsInnerTunnel, flowsBridge)
	}

	client.Delete("capture", capture1.ID())
	client.Delete("capture", capture2.ID())
}
