// +build scale

/*
 * Copyright (C) 2017 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy ofthe License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specificlanguage governing permissions and
 * limitations under the License.
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

	"github.com/davecgh/go-spew/spew"
	"github.com/skydive-project/skydive/analyzer"
	gclient "github.com/skydive-project/skydive/api/client"
	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/flow"
	g "github.com/skydive-project/skydive/gremlin"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/websocket"
)

func getAnalyzerStatus(client *shttp.CrudClient) (status analyzer.Status, err error) {
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

func checkAgents(client *shttp.CrudClient, agentsExpected int) error {
	status, err := getAnalyzerStatus(client)
	if err != nil {
		return err
	}

	if count := len(status.Agents); count != agentsExpected {
		return fmt.Errorf("Expected %d agent(s), got %d", agentsExpected, count)
	}

	return nil
}

func checkHostNodes(client *shttp.CrudClient, gh *gclient.GremlinQueryHelper, nodeExpected int) error {
	retry := func() error {
		nodes, err := gh.GetNodes(g.G.V().Has("Type", "host"))
		if err != nil {
			return err
		}

		if len(nodes) != nodeExpected {
			return fmt.Errorf("Should return %d host nodes got : %v", nodeExpected, spew.Sdump(nodes))
		}

		if err := checkAgents(client, nodeExpected); err != nil {
			return err
		}

		return nil
	}
	return common.Retry(retry, 10, 5*time.Second)
}

func checkPeers(client *shttp.CrudClient, peersExpected int, state websocket.ConnState) error {
	status, err := getAnalyzerStatus(client)
	if err != nil {
		return err
	}

	count := 0
	for _, peer := range status.Peers.Incomers {
		if *peer.State == state {
			count++
		}
	}

	for _, peer := range status.Peers.Outgoers {
		if *peer.State == state {
			count++
		}
	}

	if count != peersExpected {
		return fmt.Errorf("Expected %d peers, got %d, status: %+v", peersExpected, count, status)
	}

	return nil
}

const (
	checkLive = iota + 1
	checkHisto
	checkBoth
)

func _checkICMPv4Flows(gh *gclient.GremlinQueryHelper, nodeSel g.QueryString, flowExpected int, cmp func(seen, exp int) bool, live bool) error {
	node, err := gh.GetNode(nodeSel)
	if err != nil {
		return errors.New("Node node found: agent-1")
	}
	tid, _ := node.GetFieldString("TID")

	prefix := g.G
	if !live {
		prefix = prefix.At("-0s", 300)
	}
	gremlin := prefix.Flows().Has("LayersPath", "Ethernet/IPv4/ICMPv4", "NodeTID", tid).Sort()

	retry := func() error {
		flows, err := gh.GetFlows(gremlin)
		if err != nil {
			return fmt.Errorf("%s: %s", gremlin, err)
		}

		if !cmp(len(flows), flowExpected) {
			return fmt.Errorf("Should get %d ICMPv4 flow with prefix(%s) got %s", flowExpected, prefix, flowsToString(flows))
		}

		return nil
	}
	return common.Retry(retry, 40, time.Second)
}

func checkICMPv4Flows(gh *gclient.GremlinQueryHelper, nodeSel g.QueryString, flowExpected int, cmp func(seen, exp int) bool, mode int) error {
	if mode == checkBoth || mode == checkLive {
		if err := _checkICMPv4Flows(gh, nodeSel, flowExpected, cmp, true); err != nil {
			return err
		}
	}
	if mode == checkBoth || mode == checkHisto {
		if err := _checkICMPv4Flows(gh, nodeSel, flowExpected, cmp, false); err != nil {
			return err
		}
	}

	return nil
}

func checkIPerfFlows(gh *gclient.GremlinQueryHelper, flowExpected int) error {
	retry := func() error {
		flows, err := gh.GetFlows(g.G.Flows().Has("LayersPath", "Ethernet/IPv4/TCP").Has("Transport.B", 5001).Sort())
		if err != nil {
			return err
		}

		// two capture 2 flows
		if len(flows) != flowExpected {
			var flowsTCP []*flow.Flow
			if flowsTCP, err = gh.GetFlows(g.G.Flows().Has("LayersPath", "Ethernet/IPv4/TCP").Sort()); err != nil {
				return err
			}
			return fmt.Errorf("Should get %d iperf(tcp/5001) flows, got %s", flowExpected, flowsToString(flowsTCP))
		}

		return nil
	}
	if err := common.Retry(retry, 20, time.Second); err != nil {
		return err
	}

	// check in the storage
	retry = func() error {
		flows, err := gh.GetFlows(g.G.At("-1s", 300).Flows().Has("LayersPath", "Ethernet/IPv4/TCP").Has("Transport.B", 5001).Sort())
		if err != nil {
			return err
		}

		if len(flows) != flowExpected {
			var flowsTCP []*flow.Flow
			if flowsTCP, err = gh.GetFlows(g.G.At("-1s", 300).Flows().Has("LayersPath", "Ethernet/IPv4/TCP").Sort()); err != nil {
				return err
			}
			return fmt.Errorf("Should get %d iperf(tcp/5001) flow from datastore got %s", flowExpected, flowsToString(flowsTCP))
		}

		maps, err := gh.GetSockets(g.G.At("-1s", 300).Flows().Has("LayersPath", "Ethernet/IPv4/TCP").Sockets())
		if err != nil {
			return err
		}

		if len(maps) != len(flows) {
			return fmt.Errorf("Should get as many sockets as flows in datastore, %d != %d", len(maps), len(flows))
		}

		for _, sockets := range maps {
			for _, socket := range sockets {
				if socket == nil || socket.ProcessInfo.Process != "/usr/bin/iperf" {
					return fmt.Errorf("Should get iperf exe as socket info %v", socket)
				}
				if socket.Name != "iperf" || socket.ProcessInfo.Name != "iperf" {
					return fmt.Errorf("Should get iperf thread name %v", socket)
				}
			}
		}
		return nil
	}
	if err := common.Retry(retry, 40, time.Second); err != nil {
		return err
	}

	return nil
}

func checkCaptures(gh *gclient.GremlinQueryHelper, captureExpected int) error {
	retry := func() error {
		nodes, err := gh.GetNodes(g.G.V().Has("Capture.State", "active"))
		if err != nil {
			return err
		}

		if len(nodes) != captureExpected {
			return fmt.Errorf("Should return %d capture got : %s", captureExpected, spew.Sdump(nodes))
		}

		return nil
	}

	return common.Retry(retry, 20, time.Second)
}

func waitForFirstFlows(gh *gclient.GremlinQueryHelper, expected int) error {
	retry := func() error {
		flows, err := gh.GetFlows(g.G.Flows().Has("LayersPath", "Ethernet/IPv4/ICMPv4").Sort())
		if err != nil {
			return err
		}

		if len(flows) != expected {
			return fmt.Errorf("Should get at least one flow, got %s", spew.Sdump(flows))
		}
		return nil
	}
	return common.Retry(retry, 10, time.Second)
}

func genICMPv4(t *testing.T, scale, src string, dst string, count int) error {
	// generate some packet and wait for seeing them, to be sure that the capture is started
	var seen int
	pingFnc := func() error {
		setupCmds := []Cmd{
			{fmt.Sprintf("%s ping %s %s -c 1", scale, src, dst), false},
		}
		if err := execCmds(t, setupCmds...); err == nil {
			seen++
			if seen == count {
				return nil
			}
		}
		return errors.New("Quota not reached yet")
	}
	return common.Retry(pingFnc, 2*count, time.Second)
}

func TestScaleHA(t *testing.T) {
	gopath := os.Getenv("GOPATH")
	scale := gopath + "/src/github.com/skydive-project/skydive/scripts/scale.sh"

	setupCmds := []Cmd{
		{fmt.Sprintf("%s start 2 2 2", scale), true},
		{"sleep 30", false},
	}
	execCmds(t, setupCmds...)

	tearDownCmds := []Cmd{
		{fmt.Sprintf("%s stop 2 4 2", scale), false},
	}
	defer execCmds(t, tearDownCmds...)

	// Load Agent-1 as default config for our client
	config.InitConfig("file", []string{"/tmp/skydive-scale/agent-1.yml"})
	authOptions := &shttp.AuthenticationOpts{Username: "admin", Password: "password"}

	client, err := gclient.NewCrudClientFromConfig(authOptions)
	if err != nil {
		t.Fatalf("Failed to create client: %s", err)
	}

	// switch to the other analyzer
	os.Setenv("SKYDIVE_ANALYZERS", "localhost:8084")

	gh := gclient.NewGremlinQueryHelper(authOptions)

	// expected 2 for because of 1 incoming and 1 outgoer
	if err = common.Retry(func() error { return checkPeers(client, 2, common.RunningState) }, 5, time.Second); err != nil {
		execCmds(t, tearDownCmds...)
		t.Fatal(err)
	}

	// test if we have our 2 hosts
	if err = checkHostNodes(client, gh, 2); err != nil {
		execCmds(t, tearDownCmds...)
		t.Fatal(err)
	}

	// start a capture
	capture := types.NewCapture(g.G.V().Has("Type", "netns", "Name", "vm1").Out().Has("Name", "eth0").String(), "")
	capture.Type = "pcap"
	if err = client.Create("capture", capture); err != nil {
		execCmds(t, tearDownCmds...)
		t.Fatal(err)
	}

	// check that we have 2 captures, one per vm1
	if err = checkCaptures(gh, 2); err != nil {
		execCmds(t, tearDownCmds...)
		t.Fatal(err)
	}

	// generate some icmpv4
	if err = genICMPv4(t, scale, "agent-1-vm1", "agent-2-vm1", 30); err != nil {
		execCmds(t, tearDownCmds...)
		t.Fatal(err)
	}

	// 30 flows
	node1 := g.G.V().Has("Name", "agent-1").Out().Has("Type", "netns", "Name", "vm1").Out().Has("Name", "eth0")
	if err = checkICMPv4Flows(gh, node1, 30, func(seen, exp int) bool { return seen == exp }, checkBoth); err != nil {
		execCmds(t, tearDownCmds...)
		t.Fatal(err)
	}
	node2 := g.G.V().Has("Name", "agent-2").Out().Has("Type", "netns", "Name", "vm1").Out().Has("Name", "eth0")
	if err = checkICMPv4Flows(gh, node2, 30, func(seen, exp int) bool { return seen == exp }, checkBoth); err != nil {
		execCmds(t, tearDownCmds...)
		t.Fatal(err)
	}

	// increase the agent number
	setupCmds = []Cmd{
		{fmt.Sprintf("%s start 2 4 2", scale), false},
	}
	execCmds(t, setupCmds...)

	// test if we have now 4 hosts
	if err = checkHostNodes(client, gh, 4); err != nil {
		execCmds(t, tearDownCmds...)
		t.Fatal(err)
	}

	// check that we have 4 captures, one per vm1
	if err = checkCaptures(gh, 4); err != nil {
		execCmds(t, tearDownCmds...)
		t.Fatal(err)
	}

	// kill the last agent
	setupCmds = []Cmd{
		{fmt.Sprintf("%s stop-agent 4", scale), false},
	}
	execCmds(t, setupCmds...)

	// test if we have now 3 hosts
	if err = checkHostNodes(client, gh, 3); err != nil {
		execCmds(t, tearDownCmds...)
		t.Fatal(err)
	}

	// switch back to the first analyzer
	os.Setenv("SKYDIVE_ANALYZERS", "localhost:8082")
	client, err = gclient.NewCrudClientFromConfig(authOptions)
	if err != nil {
		t.Fatalf("Failed to create client: %s", err)
	}

	// test if we have still 3 hosts
	if err = checkHostNodes(client, gh, 3); err != nil {
		execCmds(t, tearDownCmds...)
		t.Fatal(err)
	}

	// destroy the second analyzer
	setupCmds = []Cmd{
		{fmt.Sprintf("%s stop-analyzer 2", scale), false},
		{"sleep 5", false},
	}
	execCmds(t, setupCmds...)

	if err = checkPeers(client, 0, common.RunningState); err != nil {
		execCmds(t, tearDownCmds...)
		t.Fatal(err)
	}

	// test if the remaining analyzer have a correct graph
	if err = checkHostNodes(client, gh, 3); err != nil {
		execCmds(t, tearDownCmds...)
		t.Fatal(err)
	}

	// generate more icmp traffic
	if err = genICMPv4(t, scale, "agent-3-vm1", "agent-1-vm1", 30); err != nil {
		execCmds(t, tearDownCmds...)
		t.Fatal(err)
	}

	node3 := g.G.V().Has("Name", "agent-3").Out().Has("Type", "netns", "Name", "vm1").Out().Has("Name", "eth0")
	if err = checkICMPv4Flows(gh, node3, 30, func(seen, exp int) bool { return seen == exp }, checkBoth); err != nil {
		execCmds(t, tearDownCmds...)
		t.Fatal(err)
	}

	// iperf test  10 sec, 1Mbits/s
	setupCmds = []Cmd{
		{fmt.Sprintf("%s iperf agent-3-vm1 agent-1-vm1", scale), false},
	}
	execCmds(t, setupCmds...)
	if err = checkIPerfFlows(gh, 2); err != nil {
		execCmds(t, tearDownCmds...)
		t.Fatal(err)
	}

	// delete the capture to check that all captures will be delete at the agent side
	client.Delete("capture", capture.ID())
	if err = checkCaptures(gh, 0); err != nil {
		execCmds(t, tearDownCmds...)
		t.Fatal(err)
	}

	// restore the second analyzer
	setupCmds = []Cmd{
		{fmt.Sprintf("%s start 2 3 2", scale), false},
		{"sleep 5", false},
	}
	execCmds(t, setupCmds...)

	if err = common.Retry(func() error {
		return checkPeers(client, 2, common.RunningState)
	}, 15, time.Second); err != nil {
		execCmds(t, tearDownCmds...)
		t.Fatal(err)
	}

	// delete an agent
	setupCmds = []Cmd{
		{fmt.Sprintf("%s stop-agent 1", scale), false},
	}
	execCmds(t, setupCmds...)

	// test if we have now 2 hosts
	if err = checkHostNodes(client, gh, 2); err != nil {
		execCmds(t, tearDownCmds...)
		t.Fatal(err)
	}

	// restart the agent 1 to check that flows are still forwarded to analyzer
	setupCmds = []Cmd{
		{fmt.Sprintf("%s start 2 3 2", scale), false},
		{"sleep 5", false},
	}
	execCmds(t, setupCmds...)

	// test if we have now 2 hosts
	if err = checkHostNodes(client, gh, 3); err != nil {
		execCmds(t, tearDownCmds...)
		t.Fatal(err)
	}

	// restart a capture on all eth0
	capture = types.NewCapture(g.G.V().Has("Type", "netns", "Name", "vm1").Out().Has("Name", "eth0").String(), "")
	capture.Type = "pcap"
	if err = client.Create("capture", capture); err != nil {
		t.Fatal(err)
	}

	// check that we have 3 captures, one per vm1
	if err = checkCaptures(gh, 3); err != nil {
		execCmds(t, tearDownCmds...)
		t.Fatal(err)
	}

	if err = genICMPv4(t, scale, "agent-1-vm1", "agent-2-vm1", 30); err != nil {
		execCmds(t, tearDownCmds...)
		t.Fatal(err)
	}

	// check that we have 30 flow in live as the oldest has been deleted by agent stop
	if err = checkICMPv4Flows(gh, node1, 30, func(seen, exp int) bool { return seen == exp }, checkLive); err != nil {
		execCmds(t, tearDownCmds...)
		t.Fatal(err)
	}

	// check that we have > 30 flow in histo the ones before stop and the ones just generated
	if err = checkICMPv4Flows(gh, node1, 40, func(seen, exp int) bool { return seen >= exp }, checkHisto); err != nil {
		execCmds(t, tearDownCmds...)
		t.Fatal(err)
	}
}
