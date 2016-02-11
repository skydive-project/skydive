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
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/redhat-cip/skydive/flow"
	"github.com/redhat-cip/skydive/storage"
	"github.com/redhat-cip/skydive/tests/helper"
)

const confAgentAnalyzer = `
[agent]
listen = 58081
flowtable_expire = 1
analyzers = localhost:58082

[cache]
expire = 300
cleanup = 30

[sflow]
listen = 55000

[ovs]
ovsdb = 6400

[analyzer]
listen = 58082
flowtable_expire = 10
`

type flowStat struct {
	Checked   bool
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
		filename: "eth-ip4-arp-dns-req-http-google.pcap",
		flowStat: []flowStat{
			{false, "Ethernet/ARP/Payload", 1, 44, 1, 44},
			{false, "Ethernet/IPv4/UDP/DNS", 2, 152, 2, 260},
			{false, "Ethernet/IPv4/TCP", 4, 392, 3, 700},
			{false, "Ethernet/IPv4/UDP/DNS", 2, 152, 2, 196},
			{false, "Ethernet/IPv4/TCP", 4, 280, 2, 144},
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

func (s *TestStorage) StoreFlows(flows []*flow.Flow) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	for _, f := range flows {
		s.flows[f.UUID] = f
	}

	return nil
}

func (s *TestStorage) SearchFlows(filters storage.Filters) ([]*flow.Flow, error) {
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
	eth := f.GetStatistics().Endpoints[flow.FlowEndpointType_ETHERNET.Value()]

	for _, fi := range trace.flowStat {
		if fi.Path == f.LayersPath {
			if (fi.ABPackets == eth.AB.Packets) && (fi.ABBytes == eth.AB.Bytes) && (fi.BAPackets == eth.BA.Packets) && (fi.BABytes == eth.BA.Bytes) {
				fi.Checked = true
				return true
			}
		}
	}

	return false
}

func pcapTraceValidate(t *testing.T, flows []*flow.Flow, trace *flowsTraceInfo) {
	if len(trace.flowStat) != len(flows) {
		t.Errorf("NB Flows mismatch : %d %d", len(trace.flowStat), len(flows))
	}
	for _, f := range flows {
		r := pcapTraceCheckFlow(t, f, trace)
		if r == false {
			eth := f.GetStatistics().Endpoints[flow.FlowEndpointType_ETHERNET.Value()]
			t.Logf("%s %s %d %d %d %d\n", f.UUID, f.LayersPath, eth.AB.Packets, eth.AB.Bytes, eth.BA.Packets, eth.BA.Bytes)
			t.Error("Flow not found")
		}
	}
}

func TestSFlowWithPCAP(t *testing.T) {
	ts := NewTestStorage()

	agent, analyzer := helper.StartAgentAndAnalyzerWithConfig(t, confAgentAnalyzer, ts)
	defer agent.Stop()
	defer analyzer.Stop()

	time.Sleep(1 * time.Second)
	for _, trace := range flowsTraces {
		fulltrace, _ := filepath.Abs("pcaptraces" + string(filepath.Separator) + trace.filename)
		helper.ReplayTraceHelper(t, fulltrace, "localhost:55000")

		/* FIXME (nplanel) remove this Sleep when agent.FlushFlowTable() exist */
		time.Sleep(2 * time.Second)
		pcapTraceValidate(t, ts.GetFlows(), &trace)
	}
}

func TestSFlowProbPath(t *testing.T) {
	hostname, err := os.Hostname()
	if err != nil {
		t.Fatal(err.Error())
	}

	ts := NewTestStorage()

	agent, analyzer := helper.StartAgentAndAnalyzerWithConfig(t, confAgentAnalyzer, ts)
	defer agent.Stop()
	defer analyzer.Stop()

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

	ok := false
	for _, f := range ts.GetFlows() {
		if f.ProbeGraphPath == hostname+"/br-sflow" && f.LayersPath == "Ethernet/ARP/Payload" {
			ok = true
			break
		}
	}

	if !ok {
		t.Error("Unable to find a flow with the expected probePath")
	}
}
