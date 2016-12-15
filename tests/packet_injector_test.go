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
	"github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/tests/helper"
)

func TestPacketInjector(t *testing.T) {

	aa := helper.NewAgentAnalyzerWithConfig(t, confAgentAnalyzer, nil)
	aa.Start()
	defer aa.Stop()

	client, err := api.NewCrudClientFromConfig(&http.AuthenticationOpts{})
	if err != nil {
		t.Fatal(err.Error())
	}

	setupCmds := []helper.Cmd{
		{"ovs-vsctl add-br br-int", true},
		{"ip netns add vm1", true},
		{"ip link add vm1-eth0 type veth peer name eth-src netns vm1", true},
		{"ip link set vm1-eth0 up", true},
		{"ip netns exec vm1 ip link set eth-src up", true},
		{"ip netns exec vm1 ip address add 169.254.33.33/24 dev eth-src", true},
		{"ovs-vsctl add-port br-int vm1-eth0", true},
		{"ip netns add vm2", true},
		{"ip link add vm2-eth0 type veth peer name eth-dst netns vm2", true},
		{"ip link set vm2-eth0 up", true},
		{"ip netns exec vm2 ip link set eth-dst up", true},
		{"ip netns exec vm2 ip address add 169.254.33.34/24 dev eth-dst", true},
		{"ovs-vsctl add-port br-int vm2-eth0", true},
	}

	tearDownCmds := []helper.Cmd{
		{"ovs-vsctl del-br br-int", true},
		{"ip link del vm1-eth0", true},
		{"ip netns del vm1", true},
		{"ip link del vm2-eth0", true},
		{"ip netns del vm2", true},
	}

	helper.ExecCmds(t, setupCmds...)
	defer helper.ExecCmds(t, tearDownCmds...)

	captureQuery := "G.V().Has('Name', 'eth-src').ShortestPathTo(Metadata('Name', 'eth-dst'), Metadata('RelationType', 'layer2'))"
	capture := api.NewCapture(captureQuery, "")
	if err := client.Create("capture", capture); err != nil {
		t.Fatal(err.Error())
	}
	defer client.Delete("capture", capture.ID())
	time.Sleep(1 * time.Second)

	packet := &api.PacketParamsReq{}
	packet.Src = "G.V().Has('Name', 'eth-src')"
	packet.Dst = "G.V().Has('Name', 'eth-dst')"
	packet.Type = "icmp"
	packet.Count = 10

	if err := client.Create("injectpacket", &packet); err != nil {
		t.Fatal(err.Error())
	}

	time.Sleep(11 * time.Second)

	gh := helper.NewGremlinQueryHelper(&http.AuthenticationOpts{})
	flowQuery := "G.Flows().Has('Network.A', '169.254.33.33').Has('Network.B', '169.254.33.34').Dedup()"
	flows := gh.GetFlowsFromGremlinReply(t, flowQuery)
	if len(flows) != 1 {
		t.Fatalf("we should recived only one flow, got %+v", flows)
	}
}
