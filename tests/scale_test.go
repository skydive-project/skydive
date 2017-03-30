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
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/skydive-project/skydive/api"
	gclient "github.com/skydive-project/skydive/cmd/client"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/flow"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/tests/helper"
	"github.com/skydive-project/skydive/topology/graph"
)

func TestHA(t *testing.T) {
	gopath := os.Getenv("GOPATH")
	scale := gopath + "/src/github.com/skydive-project/skydive/scripts/scale.sh"

	setupCmds := []helper.Cmd{
		{fmt.Sprintf("%s start 2 2 2", scale), true},
		{"sleep 30", false},
	}

	tearDownCmds := []helper.Cmd{
		{fmt.Sprintf("%s stop 2 3 2", scale), false},
	}
	helper.ExecCmds(t, setupCmds...)
	defer helper.ExecCmds(t, tearDownCmds...)

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

	checkHostNodes := func(nodeExpected int) {
		t.Logf("Check for host node: %d", nodeExpected)
		retry = func() error {
			if nodes, err = gh.GetNodes(`g.V().Has("Type", "host")`); err != nil {
				return err
			}

			if len(nodes) != nodeExpected {
				return fmt.Errorf("Should return 2 host nodes got : %v", nodes)
			}

			return nil
		}
		if err = common.Retry(retry, 10, 5*time.Second); err != nil {
			helper.ExecCmds(t, tearDownCmds...)
			t.Fatalf(err.Error())
		}
	}

	checkFlows := func(flowExpected int) {
		t.Logf("Check for flows: %d", flowExpected)
		retry = func() error {
			if flows, err = gh.GetFlows("G.Flows().Has('LayersPath', 'Ethernet/IPv4/ICMPv4/Payload')"); err != nil {
				return err
			}

			// two capture 2 flows
			if len(flows) != flowExpected {
				return fmt.Errorf("Should get %d ICMPv4 flow got : %v", flowExpected, flows)
			}

			return nil
		}
		if err = common.Retry(retry, 10, time.Second); err != nil {
			helper.ExecCmds(t, tearDownCmds...)
			t.Fatalf(err.Error())
		}

		// check in the storage
		retry = func() error {
			if flows, err = gh.GetFlows("G.At('-1s', 300).Flows().Has('LayersPath', 'Ethernet/IPv4/ICMPv4/Payload')"); err != nil {
				return err
			}

			if len(flows) != flowExpected {
				return fmt.Errorf("Should get %d ICMPv4 flow got : %v", flowExpected, flows)
			}

			return nil
		}
		if err = common.Retry(retry, 20, time.Second); err != nil {
			helper.ExecCmds(t, tearDownCmds...)
			t.Fatalf(err.Error())
		}
	}

	// test if we have our 2 hosts
	checkHostNodes(2)

	// start a capture
	capture := api.NewCapture("g.V().Has('Type', 'netns', 'Name', 'vm1').Out().Has('Name', 'eth0')", "")
	if err = client.Create("capture", capture); err != nil {
		t.Fatal(err)
	}

	checkCaptures := func(captureExpected int) {
		t.Logf("Check for captures: %d", captureExpected)
		retry = func() error {
			if nodes, err = gh.GetNodes(`g.V().HasKey("Capture/ID")`); err != nil {
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

	// generate some packet, do not check because connectivity is not ensured
	for i := 0; i != 30; i++ {
		setupCmds = []helper.Cmd{
			{fmt.Sprintf("%s ping agent-1-vm1 agent-2-vm1 -c 1", scale), false},
		}
		helper.ExecCmds(t, setupCmds...)
	}

	// 2 flows expected as we have two captures
	checkFlows(2)

	// increase the agent number
	setupCmds = []helper.Cmd{
		{fmt.Sprintf("%s start 2 3 2", scale), false},
	}
	helper.ExecCmds(t, setupCmds...)

	// test if we have now 3 hosts
	checkHostNodes(3)

	// check that we have 2 captures, one per vm1
	checkCaptures(3)

	// destroy the second analyzer
	setupCmds = []helper.Cmd{
		{fmt.Sprintf("%s stop-analyzer 2", scale), false},
		{"sleep 5", false},
	}
	helper.ExecCmds(t, setupCmds...)

	// switch back to the first analyzer
	os.Setenv("SKYDIVE_ANALYZERS", "localhost:8082")

	// test if the remaing analyzer have a correct graph
	checkHostNodes(3)

	// generate more icmp traffic
	for i := 0; i != 30; i++ {
		setupCmds = []helper.Cmd{
			{fmt.Sprintf("%s ping agent-3-vm1 agent-1-vm1 -c 1", scale), false},
		}
		helper.ExecCmds(t, setupCmds...)
	}

	// 4 expected because the gremlin expression matches all the eth0
	checkFlows(4)

	// delete an agent
	setupCmds = []helper.Cmd{
		{fmt.Sprintf("%s stop-agent 1", scale), false},
	}
	helper.ExecCmds(t, setupCmds...)

	// test if we have now 2 hosts
	checkHostNodes(2)
}
