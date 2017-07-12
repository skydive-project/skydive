/*
 * Copyright (C) 2015 Red Hat, Inc.
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
	"io/ioutil"
	"net/http"
	"os/exec"
	"testing"
	"time"

	"github.com/skydive-project/skydive/agent"
	"github.com/skydive-project/skydive/analyzer"
	"github.com/skydive-project/skydive/api"
	gclient "github.com/skydive-project/skydive/cmd/client"
	"github.com/skydive-project/skydive/common"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/tests/helper"
)

const (
	Replay = iota
	OneShot
)

const testConfig = `---
ws_pong_timeout: 10

analyzers:
  - 127.0.0.1:8082

analyzer:
  listen: 0.0.0.0:8082
  storage:
    backend: {{.Storage}}
  analyzer_username: admin
  analyzer_password: password

agent:
  listen: 8081
  topology:
    probes:
      - netlink
      - netns
      - ovsdb
      - docker
    netlink:
      metrics_update: 5

  flow:
    probes:
      - ovssflow
      - gopacket
      - pcapsocket
  metadata:
    info: This is compute node

flow:
  expire: 600
  update: 10

ovs:
  ovsdb: unix:///var/run/openvswitch/db.sock

storage:
  elasticsearch:
    host: 127.0.0.1:9200
  orientdb:
    addr: http://127.0.0.1:2480
    database: Skydive
    username: root
    password: {{.OrientDBRootPassword}}

graph:
  backend: {{.GraphBackend}}

logging:
  level: DEBUG

auth:
  type: noauth

etcd:
  data_dir: /tmp/skydive-etcd
  embedded: {{.EmbeddedEtcd}}
  servers:
    - {{.EtcdServer}}
`

type TestContext struct {
	gh          *gclient.GremlinQueryHelper
	client      *shttp.CrudClient
	captures    []*api.Capture
	time        time.Time
	setupTime   time.Time
	startTime   time.Time
	successTime time.Time
	data        map[string]interface{}
}

type TestCapture struct {
	gremlin string
	kind    string
	bpf     string
}

type Test struct {
	setupCmds        []helper.Cmd
	setupFunction    func(c *TestContext) error
	settleFunction   func(c *TestContext) error
	tearDownCmds     []helper.Cmd
	tearDownFunction func(c *TestContext) error
	captures         []TestCapture
	retries          int
	mode             int
	check            func(c *TestContext) error
}

func (c *TestContext) getWholeGraph(t *testing.T) string {
	var g interface{}

	gremlin := "G"
	if !c.time.IsZero() {
		gremlin += fmt.Sprintf(".Context(%d)", common.UnixMillis(c.time))
	}

	switch helper.GraphOutputFormat {
	case "ascii":
		header := make(http.Header)
		header.Set("Accept", "vnd.graphviz")
		resp, err := c.gh.Request(gremlin, header)
		if err != nil {
			t.Fatal(err.Error())
		}

		b, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()

		cmd := exec.Command("graph-easy", "--as_ascii")
		stdin, err := cmd.StdinPipe()
		if err != nil {
			t.Fatal(err.Error())
		}

		if _, err = stdin.Write(b); err != nil {
			t.Fatal(err.Error())
		}
		stdin.Write([]byte("\n"))
		stdin.Close()

		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatal(err.Error())
		}

		return "\n" + string(output)

	default:
		if err := c.gh.Query(gremlin, &g); err != nil {
			t.Error(err.Error())
		}

		b, err := json.Marshal(&g)
		if err != nil {
			t.Fatal(err.Error())
		}

		return string(b)
	}
}

func (c *TestContext) getAllFlows(t *testing.T) string {
	gremlin := "G"
	if !c.time.IsZero() {
		gremlin += fmt.Sprintf(".Context(%d)", common.UnixMillis(c.time))
	}
	gremlin += ".V().Flows()"

	flows, err := c.gh.GetFlows(gremlin)
	if err != nil {
		t.Error(err.Error())
		return ""
	}

	return helper.FlowsToString(flows)
}

func RunTest(t *testing.T, test *Test) {
	client, err := api.NewCrudClientFromConfig(&shttp.AuthenticationOpts{})
	if err != nil {
		t.Fatalf("Failed to create client: %s", err.Error())
	}

	var captures []*api.Capture
	defer func() {
		for _, capture := range captures {
			client.Delete("capture", capture.ID())
		}
	}()

	for _, tc := range test.captures {
		capture := api.NewCapture(tc.gremlin, tc.bpf)
		capture.Type = tc.kind
		if err = client.Create("capture", capture); err != nil {
			t.Fatal(err)
		}
		captures = append(captures, capture)
	}

	helper.ExecCmds(t, test.setupCmds...)

	context := &TestContext{
		gh:       gclient.NewGremlinQueryHelper(&shttp.AuthenticationOpts{}),
		client:   client,
		captures: captures,
		data:     make(map[string]interface{}),
	}

	err = common.Retry(func() error {
		for _, capture := range captures {
			nodes, err := context.gh.GetNodes(capture.GremlinQuery)
			if err != nil {
				return err
			}

			if len(nodes) == 0 {
				return fmt.Errorf("No node matching capture %s, graph: %s", capture.GremlinQuery, context.getWholeGraph(t))
			}

			for _, node := range nodes {
				tp, err := node.GetFieldString("Type")
				if err != nil || !common.IsCaptureAllowed(tp) {
					continue
				}

				captureID, err := node.GetFieldString("Capture.ID")
				if err != nil {
					return fmt.Errorf("Node %+v matched the capture but capture is not enabled, graph: %s", node, context.getWholeGraph(t))
				}

				if captureID != capture.ID() {
					return fmt.Errorf("Node %s matches multiple captures, graph: %s", node.ID, context.getWholeGraph(t))
				}
			}
		}

		return nil
	}, 15, time.Second)

	if err != nil {
		g := context.getWholeGraph(t)
		helper.ExecCmds(t, test.tearDownCmds...)
		t.Fatalf("Failed to setup captures: %s, graph: %s", err.Error(), g)
	}

	retries := test.retries
	if retries <= 0 {
		retries = 30
	}

	if test.settleFunction != nil {
		err = common.Retry(func() error {
			return test.settleFunction(context)
		}, retries, time.Second)

		if err != nil {
			g := context.getWholeGraph(t)
			f := context.getAllFlows(t)
			helper.ExecCmds(t, test.tearDownCmds...)
			t.Errorf("Test failed to settle: %s, graph: %s, flows: %s", err.Error(), g, f)
			return
		}
	}

	context.setupTime = time.Now()

	if test.setupFunction != nil {
		if err = test.setupFunction(context); err != nil {
			g := context.getWholeGraph(t)
			f := context.getAllFlows(t)
			helper.ExecCmds(t, test.tearDownCmds...)
			t.Fatalf("Failed to setup test: %s, graph: %s, flows: %s", err.Error(), g, f)
		}
	}

	context.startTime = time.Now()

	err = common.Retry(func() error {
		if err = test.check(context); err != nil {
			return err
		}
		context.successTime = time.Now()
		if context.time.IsZero() {
			context.time = context.successTime
		}
		return nil
	}, retries, time.Second)

	if err != nil {
		g := context.getWholeGraph(t)
		f := context.getAllFlows(t)
		helper.ExecCmds(t, test.tearDownCmds...)
		t.Errorf("Test failed: %s, graph: %s, flows: %s", err.Error(), g, f)
		return
	}

	if test.tearDownFunction != nil {
		if err = test.tearDownFunction(context); err != nil {
			helper.ExecCmds(t, test.tearDownCmds...)
			t.Fatalf("Fail to tear test down: %s", err.Error())
		}
	}

	helper.ExecCmds(t, test.tearDownCmds...)

	if test.mode == Replay {
		t.Logf("Replaying test with time %s (Unix: %d), startTime %s (Unix: %d)", context.time, context.time.Unix(), context.startTime, context.startTime.Unix())
		err = common.Retry(func() error {
			return test.check(context)
		}, retries, time.Second)

		if err != nil {
			t.Errorf("Failed to replay test: %s, graph: %s, flows: %s", err.Error(), context.getWholeGraph(t), context.getAllFlows(t))
		}
	}
}

func pingRequest(t *testing.T, context *TestContext, packet *api.PacketParamsReq) error {
	return context.client.Create("injectpacket", packet)
}

func ping(t *testing.T, context *TestContext, ipVersion int, src string, dst string, count int64, id int64) error {
	packet := &api.PacketParamsReq{
		Src:      src,
		Dst:      dst,
		Type:     fmt.Sprintf("icmp%d", ipVersion),
		Count:    count,
		ID:       id,
		Interval: 1000,
	}

	return pingRequest(t, context, packet)
}

func init() {
	http.DefaultTransport.(*http.Transport).MaxIdleConnsPerHost = 100

	if helper.Standalone {
		if err := helper.InitConfig(testConfig); err != nil {
			panic(fmt.Sprintf("Failed to initialize config: %s", err.Error()))
		}

		if err := logging.InitLogging(); err != nil {
			panic(fmt.Sprintf("Failed to initialize logging system: %s", err.Error()))
		}

		server := analyzer.NewServerFromConfig()
		server.Start()

		agent := agent.NewAgent()
		agent.Start()

		// TODO: check for storage status instead of sleeping
		time.Sleep(3 * time.Second)
	}
}
