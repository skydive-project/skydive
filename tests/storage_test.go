// +build storage

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
	"testing"
	"time"

	"github.com/skydive-project/skydive/api"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/tests/helper"
	"github.com/skydive-project/skydive/topology/graph"
)

const confStorage = `---
agent:
  listen: :58081
  analyzers: localhost:{{.AnalyzerPort}}
  topology:
    probes:
      - netlink
      - netns
      - ovsdb
  flow:
    probes:
      - ovssflow

cache:
  expire: 300
  cleanup: 30

sflow:
  port_min: 55000
  port_max: 55005

analyzer:
  listen: :{{.AnalyzerPort}}
  flowtable_expire: 600
  flowtable_update: 20
  flowtable_agent_ratio: 0.5
  storage: {{.Storage}}

etcd:
  embedded: {{.EmbeddedEtcd}}
  port: 2374
  data_dir: /tmp/skydive-test-etcd
  servers:
    - {{.EtcdServer}}

logging:
  default: {{.LogLevel}}

storage:
  elasticsearch:
    host: 127.0.0.1:9200
    bulk_maxdocs: 1

  orientdb:
    addr: http://127.0.0.1:2480
    database: TestSkydive
    username: root
    password: {{.OrientDBRootPassword}}

graph:
  backend: {{.GraphBackend}}
`

func TestFlowStorage(t *testing.T) {
	aa := helper.NewAgentAnalyzerWithConfig(t, confStorage, nil)
	aa.Start()
	defer aa.Stop()

	client, err := api.NewCrudClientFromConfig(&http.AuthenticationOpts{})
	if err != nil {
		t.Fatal(err.Error())
	}

	capture := api.NewCapture("G.V().Has('Name', 'br-sflow', 'Type', 'ovsbridge')", "")
	if err = client.Create("capture", capture); err != nil {
		t.Fatal(err.Error())
	}
	defer client.Delete("capture", capture.ID())

	time.Sleep(3 * time.Second)
	now := time.Now().UTC()

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

		{"ip netns exec sflow-vm1 ping -c 15 -s 1024 -I sflow-intf1 169.254.33.34", false},
	}

	tearDownCmds := []helper.Cmd{
		{"ip netns del sflow-vm1", true},
		{"ip netns del sflow-vm2", true},
		{"ovs-vsctl del-br br-sflow", true},
	}

	helper.ExecCmds(t, setupCmds...)
	defer helper.ExecCmds(t, tearDownCmds...)

	time.Sleep(time.Second)
	aa.Flush()

	// Wait for the flows to be indexed in elasticsearch
	time.Sleep(5 * time.Second)

	gh := helper.NewGremlinQueryHelper(&http.AuthenticationOpts{})

	node := gh.GetNodeFromGremlinReply(t, `g.V().Has("Name", "br-sflow", "Type", "ovsbridge")`)

	filters := flow.NewFilterForNodes([]*graph.Node{node})
	flowSearchQuery := flow.FlowSearchQuery{Filter: filters}
	flowset, err := aa.Analyzer.Storage.SearchFlows(flowSearchQuery)
	if err != nil {
		t.Fatalf("Failed to query flows: %s", err.Error())
	}

	if len(flowset.Flows) == 0 {
		t.Fatalf("SearchFlows should return at least one flow, got %d", len(flowset.Flows))
	}

	queryFlowMetrics(t, now.Add(9*time.Second).Unix(), 15)
}
