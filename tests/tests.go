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
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os/exec"
	"testing"
	"time"

	"github.com/skydive-project/skydive/agent"
	"github.com/skydive-project/skydive/analyzer"
	gclient "github.com/skydive-project/skydive/api/client"
	"github.com/skydive-project/skydive/api/types"
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
http:
  ws:
    pong_timeout: 10

analyzers:
  - 127.0.0.1:8082

analyzer:
  listen: 0.0.0.0:8082
  storage:
    backend: {{.Storage}}
  analyzer_username: admin
  analyzer_password: password
  topology:
    probes: {{block "list" .}}{{"\n"}}{{range .AnalyzerProbes}}{{println "    -" .}}{{end}}{{end}}

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

  metadata:
    info: This is compute node
    mydict:
      value: 123
      onearray:
      - name: first
        value: 1
      - name: last
        value: 10

flow:
  expire: 600
  update: 10

ovs:
  ovsdb: unix:///var/run/openvswitch/db.sock
  oflow:
    enable: true

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
	gh        *gclient.GremlinQueryHelper
	client    *shttp.CrudClient
	captures  []*types.Capture
	setupTime time.Time
	data      map[string]interface{}
}

type TestCapture struct {
	gremlin    string
	kind       string
	bpf        string
	rawPackets int
}

type CheckFunction func(c *CheckContext) error

type CheckContext struct {
	*TestContext
	startTime   time.Time
	successTime time.Time
	time        time.Time
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
	checks           []CheckFunction
	checkContexts    []*CheckContext
}

func (c *TestContext) getWholeGraph(t *testing.T, at time.Time) string {
	gremlin := "G"
	if !at.IsZero() {
		gremlin += fmt.Sprintf(".Context(%d)", common.UnixMillis(at))
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
		data, err := c.gh.QueryRaw(gremlin)
		if err != nil {
			t.Fatal(err.Error())
		}

		return string(data)
	}
}

func (c *TestContext) getAllFlows(t *testing.T, at time.Time) string {
	gremlin := "G"
	if !at.IsZero() {
		gremlin += fmt.Sprintf(".Context(%d)", common.UnixMillis(at))
	}
	gremlin += ".V().Flows().Sort()"

	flows, err := c.gh.GetFlows(gremlin)
	if err != nil {
		t.Error(err.Error())
		return ""
	}

	return helper.FlowsToString(flows)
}

func (c *TestContext) getSystemState(t *testing.T) {
	stateCmds := []helper.Cmd{
		{"ip addr", false},
		{"ip netns list", false},
		{"ovs-vsctl show", false},
		{"brctl show", false},
	}
	helper.ExecCmds(t, stateCmds...)
}

func RunTest(t *testing.T, test *Test) {
	client, err := gclient.NewCrudClientFromConfig(&shttp.AuthenticationOpts{})
	if err != nil {
		t.Fatalf("Failed to create client: %s", err.Error())
	}

	var captures []*types.Capture
	defer func() {
		for _, capture := range captures {
			client.Delete("capture", capture.ID())
		}
	}()

	for _, tc := range test.captures {
		capture := types.NewCapture(tc.gremlin, tc.bpf)
		capture.Type = tc.kind
		capture.RawPacketLimit = tc.rawPackets
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
				return fmt.Errorf("No node matching capture %s, graph: %s", capture.GremlinQuery, context.getWholeGraph(t, time.Now()))
			}

			for _, node := range nodes {
				tp, err := node.GetFieldString("Type")
				if err != nil || !common.IsCaptureAllowed(tp) {
					continue
				}

				captureID, err := node.GetFieldString("Capture.ID")
				if err != nil {
					return fmt.Errorf("Node %+v matched the capture but capture is not enabled, graph: %s", node, context.getWholeGraph(t, time.Now()))
				}

				if captureID != capture.ID() {
					return fmt.Errorf("Node %s matches multiple captures, graph: %s", node.ID, context.getWholeGraph(t, time.Now()))
				}
			}
		}

		return nil
	}, 15, time.Second)

	if err != nil {
		g := context.getWholeGraph(t, time.Now())
		helper.ExecCmds(t, test.tearDownCmds...)
		context.getSystemState(t)
		t.Fatalf("Failed to setup captures: %s, graph: %s", err.Error(), g)
	}

	retries := test.retries
	if retries <= 0 {
		retries = 30
	}

	settleTime := time.Now()

	if test.settleFunction != nil {
		err = common.Retry(func() error {
			return test.settleFunction(context)
		}, retries, time.Second)

		if err != nil {
			g := context.getWholeGraph(t, settleTime)
			f := context.getAllFlows(t, settleTime)
			helper.ExecCmds(t, test.tearDownCmds...)
			context.getSystemState(t)
			t.Errorf("Test failed to settle: %s, graph: %s, flows: %s", err.Error(), g, f)
			return
		}
	}

	context.setupTime = time.Now()

	if test.setupFunction != nil {
		if err = test.setupFunction(context); err != nil {
			g := context.getWholeGraph(t, context.setupTime)
			f := context.getAllFlows(t, context.setupTime)
			helper.ExecCmds(t, test.tearDownCmds...)
			context.getSystemState(t)
			t.Fatalf("Failed to setup test: %s, graph: %s, flows: %s", err.Error(), g, f)
		}
	}

	test.checkContexts = make([]*CheckContext, len(test.checks))

	for i, check := range test.checks {
		checkContext := &CheckContext{
			TestContext: context,
			startTime:   time.Now(),
		}
		test.checkContexts[i] = checkContext

		err = common.Retry(func() error {
			if err = check(checkContext); err != nil {
				return err
			}
			checkContext.successTime = time.Now()
			if checkContext.time.IsZero() {
				checkContext.time = checkContext.successTime
			}
			return nil
		}, retries, time.Second)

		if err != nil {
			g := checkContext.getWholeGraph(t, checkContext.startTime)
			f := checkContext.getAllFlows(t, checkContext.startTime)
			helper.ExecCmds(t, test.tearDownCmds...)
			context.getSystemState(t)
			t.Errorf("Test failed: %s, graph: %s, flows: %s", err.Error(), g, f)
			return
		}
	}

	if test.tearDownFunction != nil {
		if err = test.tearDownFunction(context); err != nil {
			helper.ExecCmds(t, test.tearDownCmds...)
			context.getSystemState(t)
			t.Fatalf("Fail to tear test down: %s", err.Error())
		}
	}

	helper.ExecCmds(t, test.tearDownCmds...)

	if test.mode == Replay {
		for i, check := range test.checks {
			checkContext := test.checkContexts[i]
			t.Logf("Replaying test with time %s (Unix: %d), startTime %s (Unix: %d)", checkContext.time, checkContext.time.Unix(), checkContext.startTime, checkContext.startTime.Unix())
			err = common.Retry(func() error {
				return check(checkContext)
			}, retries, time.Second)

			if err != nil {
				t.Errorf("Failed to replay test: %s, graph: %s, flows: %s", err.Error(), checkContext.getWholeGraph(t, checkContext.time), checkContext.getAllFlows(t, checkContext.time))
			}
		}
	}
}

func pingRequest(t *testing.T, context *TestContext, packet *types.PacketParamsReq) error {
	return context.client.Create("injectpacket", packet)
}

func ping(t *testing.T, context *TestContext, ipVersion int, src string, dst string, count int64, id int64) error {
	packet := &types.PacketParamsReq{
		Src:      src,
		Dst:      dst,
		Type:     fmt.Sprintf("icmp%d", ipVersion),
		Count:    count,
		ICMPID:   id,
		Interval: 1000,
	}

	return pingRequest(t, context, packet)
}

func getFirstAvailableIPv4Addr() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	for _, iface := range ifaces {
		//neglect interfaces which are down
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		//neglect loopback interface
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			return "", err
		}
		for _, addr := range addrs {
			var ip net.IP
			switch t := addr.(type) {
			case *net.IPNet:
				ip = t.IP
			case *net.IPAddr:
				ip = t.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			if ip = ip.To4(); ip != nil {
				return ip.String(), nil
			}
		}
	}
	return "", fmt.Errorf("No IP found")
}

// delay is just a wrapper to introduce a delay after a function call
// ex: delay(5*time.Second, sh.ClickOn)
func delay(delay time.Duration, err ...error) error {
	if len(err) > 0 && err[0] != nil {
		return err[0]
	}
	time.Sleep(delay)
	return nil
}

func delaySec(sec int, err ...error) error {
	return delay(time.Duration(sec)*time.Second, err...)
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

		server, err := analyzer.NewServerFromConfig()
		if err != nil {
			panic(err)
		}
		server.Start()

		agent, err := agent.NewAgent()
		if err != nil {
			panic(err)
		}

		agent.Start()

		// TODO: check for storage status instead of sleeping
		time.Sleep(3 * time.Second)
	}
}
