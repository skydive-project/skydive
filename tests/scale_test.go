// +build scale

/*
 * Copyright (C) 2017 Red Hat, Inc.
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
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/skydive-project/skydive/analyzer"
	"github.com/skydive-project/skydive/api"
	gclient "github.com/skydive-project/skydive/cmd/client"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/flow"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/tests/helper"
	"github.com/skydive-project/skydive/topology/graph"
)

func TestScaleHA(t *testing.T) {
	gopath := os.Getenv("GOPATH")
	scale := gopath + "/src/github.com/skydive-project/skydive/scripts/scale.sh"

	setupCmds := []helper.Cmd{
		{fmt.Sprintf("%s start 2 2 2", scale), true},
		{"sleep 30", false},
	}
	helper.ExecCmds(t, setupCmds...)

	tearDownCmds := []helper.Cmd{
		{fmt.Sprintf("%s stop 2 4 2", scale), false},
	}
	defer helper.ExecCmds(t, tearDownCmds...)

	// Load Agent-1 as default config for our client
	config.InitConfig("file", []string{"/tmp/skydive-scale/agent-1.yml"})
	authOptions := &shttp.AuthenticationOpts{}

	client, err := api.NewCrudClientFromConfig(authOptions)
	if err != nil {
		t.Fatalf("Failed to create client: %s", err.Error())
	}

	// switch to the other analyzer
	os.Setenv("SKYDIVE_ANALYZERS", "localhost:8084")

	gh := gclient.NewGremlinQueryHelper(authOptions)

	var retry func() error
	var nodes []*graph.Node
	var flows []*flow.Flow

	getAnalyzerStatus := func() (status analyzer.AnalyzerStatus, err error) {
		resp, err := client.Request("GET", "status", nil, nil)
		if err != nil {
			return status, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			data, _ := ioutil.ReadAll(resp.Body)
			return status, fmt.Errorf("Failed to get status, %s: %s", resp.Status, data)
		}

		if err := common.JSONDecode(resp.Body, &status); err != nil {
			return status, err
		}

		return
	}

	checkAgents := func(agentsExpected int) error {
		status, err := getAnalyzerStatus()
		if err != nil {
			return err
		}

		if count := len(status.Agents); count != agentsExpected {
			return fmt.Errorf("Expected %d agent(s), got %d", agentsExpected, count)
		}

		return nil
	}

	checkHostNodes := func(nodeExpected int) {
		t.Logf("Check for host node: %d", nodeExpected)
		retry = func() error {
			if nodes, err = gh.GetNodes(`g.V().Has("Type", "host")`); err != nil {
				return err
			}

			if len(nodes) != nodeExpected {
				return fmt.Errorf("Should return %d host nodes got : %v", nodeExpected, nodes)
			}

			if err := checkAgents(nodeExpected); err != nil {
				return err
			}

			return nil
		}
		if err = common.Retry(retry, 10, 5*time.Second); err != nil {
			helper.ExecCmds(t, tearDownCmds...)
			t.Fatalf(err.Error())
		}
	}

	checkPeers := func(peersExpected int, state shttp.WSConnState) error {
		status, err := getAnalyzerStatus()
		if err != nil {
			return err
		}

		count := 0
		for _, peer := range status.Peers {
			if *peer.State == state {
				count++
			}
		}

		if count != peersExpected {
			return fmt.Errorf("Expected %d peers, got %d", peersExpected, count)
		}

		return nil
	}

	checkICMPv4FlowsLive := func(flowExpected int, cmp func(seen, exp int) bool) {
		t.Logf("Check for live flows: %d", flowExpected)
		retry = func() error {
			if flows, err = gh.GetFlows("G.Flows().Has('LayersPath', 'Ethernet/IPv4/ICMPv4')"); err != nil {
				return err
			}

			// two capture 2 flows
			if !cmp(len(flows), flowExpected) {
				return fmt.Errorf("Should get %d ICMPv4 flow got %d : %v", flowExpected, len(flows), flows)
			}

			return nil
		}
		if err = common.Retry(retry, 10, time.Second); err != nil {
			helper.ExecCmds(t, tearDownCmds...)
			t.Fatalf(err.Error())
		}
	}

	checkICMPv4FlowsReplay := func(flowExpected int, cmp func(seen, exp int) bool) {
		t.Logf("Check for replay flows: %d", flowExpected)
		retry = func() error {
			if flows, err = gh.GetFlows("G.At('-1s', 300).Flows().Has('LayersPath', 'Ethernet/IPv4/ICMPv4')"); err != nil {
				return err
			}

			if !cmp(len(flows), flowExpected) {
				return fmt.Errorf("Should get %d ICMPv4 flow from datastore got %d : %v", flowExpected, len(flows), flows)
			}

			return nil
		}
		if err = common.Retry(retry, 40, time.Second); err != nil {
			helper.ExecCmds(t, tearDownCmds...)
			t.Fatalf(err.Error())
		}
	}

	checkICMPv4Flows := func(flowExpected int, cmp func(seen, exp int) bool) {
		checkICMPv4FlowsLive(flowExpected, cmp)
		checkICMPv4FlowsReplay(flowExpected, cmp)
	}

	checkIPerfFlows := func(flowExpected int) {
		t.Logf("Check for flows: %d", flowExpected)
		retry = func() error {
			if flows, err = gh.GetFlows("G.Flows().Has('LayersPath', 'Ethernet/IPv4/TCP').Has('Transport.B', '5001')"); err != nil {
				return err
			}

			// two capture 2 flows
			if len(flows) != flowExpected {
				var flowsTCP []*flow.Flow
				if flowsTCP, err = gh.GetFlows("G.Flows().Has('LayersPath', 'Ethernet/IPv4/TCP')"); err != nil {
					return err
				}
				return fmt.Errorf("Should get %d iperf(tcp/5001) flow got %d : %v", flowExpected, len(flows), flowsTCP)
			}

			for _, f := range flows {
				if f.SocketA == nil || f.SocketA.Process != "/usr/bin/iperf" || f.SocketB == nil || f.SocketB.Process != "/usr/bin/iperf" {
					return fmt.Errorf("Should get iperf exe as socket info %v", flows)
				}
				if f.SocketA.Name != "iperf" || f.SocketB.Name != "iperf" {
					return fmt.Errorf("Should get iperf thread name %v", flows)
				}
			}
			return nil
		}
		if err = common.Retry(retry, 10, time.Second); err != nil {
			helper.ExecCmds(t, tearDownCmds...)
			t.Fatalf(err.Error())
		}

		// check in the storage
		retry = func() error {
			if flows, err = gh.GetFlows("G.At('-1s', 300).Flows().Has('LayersPath', 'Ethernet/IPv4/TCP').Has('Transport.B', '5001')"); err != nil {
				return err
			}

			if len(flows) != flowExpected {
				var flowsTCP []*flow.Flow
				if flowsTCP, err = gh.GetFlows("G.At('-1s', 300).Flows().Has('LayersPath', 'Ethernet/IPv4/TCP')"); err != nil {
					return err
				}
				return fmt.Errorf("Should get %d iperf(tcp/5001) flow from datastore got %d : %#+v", flowExpected, len(flows), flowsTCP)
			}

			for _, f := range flows {
				if f.SocketA == nil || f.SocketA.Process != "/usr/bin/iperf" || f.SocketB == nil || f.SocketB.Process != "/usr/bin/iperf" {
					return fmt.Errorf("Should get iperf exe as socket info %v", flows)
				}
				if f.SocketA.Name != "iperf" || f.SocketB.Name != "iperf" {
					return fmt.Errorf("Should get iperf thread name %v", flows)
				}
			}
			return nil
		}
		if err = common.Retry(retry, 40, time.Second); err != nil {
			helper.ExecCmds(t, tearDownCmds...)
			t.Fatalf(err.Error())
		}
	}

	if err := checkPeers(1, common.RunningState); err != nil {
		t.Fatal(err)
	}

	// test if we have our 2 hosts
	checkHostNodes(2)

	// before creating the capture check that ovs if ready and the connectivity is ok
	common.Retry(func() error {
		pings := []helper.Cmd{
			{fmt.Sprintf("%s ping agent-1-vm1 agent-2-vm1 -c 1", scale), false},
		}
		return helper.ExecCmds(t, pings...)
	}, 30, time.Second)

	// start a capture
	capture := api.NewCapture("g.V().Has('Type', 'netns', 'Name', 'vm1').Out().Has('Name', 'eth0')", "")
	capture.SocketInfo = true
	capture.Type = "pcap"
	if err = client.Create("capture", capture); err != nil {
		t.Fatal(err)
	}

	checkCaptures := func(captureExpected int) {
		t.Logf("Check for captures: %d", captureExpected)
		retry = func() error {
			if nodes, err = gh.GetNodes(`g.V().HasKey("Capture.ID")`); err != nil {
				return err
			}

			if len(nodes) != captureExpected {
				return fmt.Errorf("Should return %d capture got : %v", captureExpected, nodes)
			}

			return nil
		}

		if err = common.Retry(retry, 10, time.Second); err != nil {
			helper.ExecCmds(t, tearDownCmds...)
			t.Fatalf(err.Error())
		}
	}

	// check that we have 2 captures, one per vm1
	checkCaptures(2)

	// generate some packet and wait for seeing them, to be sure that the capture is started
	waitForFirstFlows := func() error {
		t.Logf("Wait for first flows...")

		retry = func() error {
			if flows, err = gh.GetFlows("G.Flows().Has('LayersPath', 'Ethernet/IPv4/ICMPv4')"); err != nil {
				return err
			}

			if len(flows) != 2 {
				return errors.New("Should get at least one flow")
			}
			return nil
		}
		if err = common.Retry(retry, 10, time.Second); err != nil {
			return err
		}
		return nil
	}

	firstFlowsFnc := func() error {
		setupCmds = []helper.Cmd{
			{fmt.Sprintf("%s ping agent-1-vm1 agent-2-vm1 -c 1", scale), false},
		}
		helper.ExecCmds(t, setupCmds...)

		return waitForFirstFlows()
	}
	if err = common.Retry(firstFlowsFnc, 10, time.Second); err != nil {
		helper.ExecCmds(t, tearDownCmds...)
		t.Fatalf(err.Error())
	}

	// generate some packet, do not check because connectivity is not ensured
	for i := 0; i != 30; i++ {
		setupCmds = []helper.Cmd{
			{fmt.Sprintf("%s ping agent-1-vm1 agent-2-vm1 -c 1", scale), false},
		}
		helper.ExecCmds(t, setupCmds...)
	}

	// 60 flows expected as we have two captures
	checkICMPv4Flows(60+2, func(seen, exp int) bool { return seen == exp })

	// increase the agent number
	setupCmds = []helper.Cmd{
		{fmt.Sprintf("%s start 2 4 2", scale), false},
	}
	helper.ExecCmds(t, setupCmds...)

	// test if we have now 4 hosts
	checkHostNodes(4)

	// check that we have 4 captures, one per vm1
	checkCaptures(4)

	// kill the last agent
	setupCmds = []helper.Cmd{
		{fmt.Sprintf("%s stop-agent 4", scale), false},
	}
	helper.ExecCmds(t, setupCmds...)

	// test if we have now 3 hosts
	checkHostNodes(3)

	// switch back to the first analyzer
	os.Setenv("SKYDIVE_ANALYZERS", "localhost:8082")

	// test if we have now 3 hosts
	checkHostNodes(3)

	// destroy the second analyzer
	setupCmds = []helper.Cmd{
		{fmt.Sprintf("%s stop-analyzer 2", scale), false},
		{"sleep 5", false},
	}
	helper.ExecCmds(t, setupCmds...)

	if err := checkPeers(1, common.StoppedState); err != nil {
		t.Fatal(err)
	}

	// test if the remaining analyzer have a correct graph
	checkHostNodes(3)

	// generate more icmp traffic
	for i := 0; i != 30; i++ {
		setupCmds = []helper.Cmd{
			{fmt.Sprintf("%s ping agent-3-vm1 agent-1-vm1 -c 1", scale), false},
		}
		helper.ExecCmds(t, setupCmds...)
	}

	// 4*30 expected because the gremlin expression matches all the eth0
	checkICMPv4Flows(120+2, func(seen, exp int) bool { return seen == exp })

	// iperf test  10 sec, 1Mbits/s
	setupCmds = []helper.Cmd{
		{fmt.Sprintf("%s iperf agent-3-vm1 agent-1-vm1", scale), false},
	}
	helper.ExecCmds(t, setupCmds...)
	checkIPerfFlows(2)

	// delete the capture to check that all captures will be delete at the agent side
	client.Delete("capture", capture.ID())
	checkCaptures(0)

	// restore the second analyzer
	setupCmds = []helper.Cmd{
		{fmt.Sprintf("%s start 2 3 2", scale), false},
		{"sleep 5", false},
	}
	helper.ExecCmds(t, setupCmds...)

	if err = common.Retry(func() error {
		return checkPeers(1, common.RunningState)
	}, 15, time.Second); err != nil {
		t.Fatal(err)
	}

	// delete an agent
	setupCmds = []helper.Cmd{
		{fmt.Sprintf("%s stop-agent 1", scale), false},
	}
	helper.ExecCmds(t, setupCmds...)

	// test if we have now 2 hosts
	checkHostNodes(2)

	// restart the agent 1 to check that flows are still forwarded to analyzer
	setupCmds = []helper.Cmd{
		{fmt.Sprintf("%s start 2 3 2", scale), false},
		{"sleep 5", false},
	}
	helper.ExecCmds(t, setupCmds...)

	// test if we have now 2 hosts
	checkHostNodes(3)

	// restart a capture on all eth0
	capture = api.NewCapture("g.V().Has('Type', 'netns', 'Name', 'vm1').Out().Has('Name', 'eth0')", "")
	capture.SocketInfo = true
	capture.Type = "pcap"
	if err = client.Create("capture", capture); err != nil {
		t.Fatal(err)
	}

	// check that we have 3 captures, one per vm1
	checkCaptures(3)

	// generate some packet, do not check because connectivity is not ensured
	for i := 0; i != 30; i++ {
		setupCmds = []helper.Cmd{
			{fmt.Sprintf("%s ping agent-1-vm1 agent-2-vm1 -c 1", scale), false},
		}
		helper.ExecCmds(t, setupCmds...)
	}

	// we generate a bit more flow just check that we have more than before
	checkICMPv4FlowsReplay(130+2, func(seen, exp int) bool { return seen >= exp })
}
