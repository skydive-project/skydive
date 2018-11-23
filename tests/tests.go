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
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/skydive-project/skydive/agent"
	"github.com/skydive-project/skydive/analyzer"
	gclient "github.com/skydive-project/skydive/api/client"
	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/flow"
	g "github.com/skydive-project/skydive/gremlin"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
)

const (
	// OneShot is used for tests that do not support history.
	OneShot = iota
	// Replay should be used for tests that can be replayed:
	// checks are executed in live. In case of success, the timestamps are recorded.
	// Checks are then replayed against the history with the recorded timestamps.
	Replay
)

const testConfig = `---
http:
  ws:
    pong_timeout: 10

analyzers:
  - 127.0.0.1:{{.AnalyzerPort}}

analyzer:
  listen: {{.AnalyzerAddr}}:{{.AnalyzerPort}}
  flow:
    backend: {{.FlowBackend}}
  analyzer_username: admin
  analyzer_password: password
  topology:
    backend: {{.TopologyBackend}}
    probes: {{block "list" .}}{{"\n"}}{{range .AnalyzerProbes}}{{println "    -" .}}{{end}}{{end}}
  startup:
    capture_gremlin: "g.V().Has('Name','startup-vm2')"

agent:
  listen: {{.AgentAddr}}:{{.AgentPort}}
  topology:
    probes:
      - netlink
      - netns
      - ovsdb
      - docker
      - lxd
      - opencontrail
      - lldp
      - sriov
    netlink:
      metrics_update: 5
    lldp:
      interfaces:
      - lldp0

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
  oflow:
    enable: true

storage:
  orientdb:
    addr: http://127.0.0.1:2480
    database: Skydive
    username: root
    password: {{.OrientDBRootPassword}}

logging:
  level: DEBUG

etcd:
  data_dir: /tmp/skydive-etcd
  embedded: {{.EmbeddedEtcd}}
  servers:
    - {{.EtcdServer}}
`

// Cmd describes a command line to execute and whether it result code should be checked
type Cmd struct {
	Cmd   string
	Check bool
}

type helperParams map[string]interface{}

// TestContext holds the context (client, captures, injections, timestamp, ...) of a test
type TestContext struct {
	gh         *gclient.GremlinQueryHelper
	client     *shttp.CrudClient
	captures   []*types.Capture
	injections []*types.PacketInjection
	setupTime  time.Time
	data       map[string]interface{}
}

// TestCapture describes a capture to be created in tests
type TestCapture struct {
	gremlin    g.QueryString
	kind       string
	bpf        string
	rawPackets int
	port       int
}

// TestInjection describes a packet injection to be created in tests
type TestInjection struct {
	intf      g.QueryString
	from      g.QueryString
	fromMAC   string
	fromIP    string
	to        g.QueryString
	toMAC     string
	toIP      string
	ipv6      bool
	count     int64
	id        int64
	increment bool
	payload   string
	pcap      string
}

// CheckFunction describes a function that actually does a check and returns an error if needed
type CheckFunction func(c *CheckContext) error

// CheckContext describes the context (gremlin prefix, timestamps... )of one check of a test
type CheckContext struct {
	*TestContext
	gremlin     g.QueryString
	startTime   time.Time
	successTime time.Time
	time        time.Time
}

// Test describes a test. It contains:
// - the list of commands and functions to be executed at startup
// - the list of commands and functions to be executed at cleanup
// - the list of captures to be created
// - the list of packet injections to be created
// - the number of retries
// - a list of checks
type Test struct {
	setupCmds        []Cmd
	setupFunction    func(c *TestContext) error
	settleFunction   func(c *TestContext) error
	tearDownCmds     []Cmd
	tearDownFunction func(c *TestContext) error
	captures         []TestCapture
	injections       []TestInjection
	preCleanup       bool
	retries          int
	mode             int
	checks           []CheckFunction
	checkContexts    []*CheckContext
}

var (
	agentTestsOnly    bool
	analyzerListen    string
	analyzerProbes    string
	etcdServer        string
	flowBackend       string
	graphOutputFormat string
	noOFTests         bool
	standalone        bool
	topologyBackend   string
)

func initConfig(conf string, params ...helperParams) error {
	f, err := ioutil.TempFile("", "skydive_agent")
	if err != nil {
		return err
	}

	if len(params) == 0 {
		params = []helperParams{make(helperParams)}
	}

	sa, err := common.ServiceAddressFromString(analyzerListen)
	if err != nil {
		return err
	}
	params[0]["AnalyzerAddr"] = sa.Addr
	params[0]["AnalyzerPort"] = sa.Port
	params[0]["AgentAddr"] = sa.Addr
	params[0]["AgentPort"] = sa.Port - 1

	if testing.Verbose() {
		params[0]["LogLevel"] = "DEBUG"
	} else {
		params[0]["LogLevel"] = "INFO"
	}
	if etcdServer != "" {
		params[0]["EmbeddedEtcd"] = "false"
		params[0]["EtcdServer"] = etcdServer
	} else {
		params[0]["EmbeddedEtcd"] = "true"
		params[0]["EtcdServer"] = "http://localhost:12379"
	}
	if flowBackend != "" {
		params[0]["FlowBackend"] = flowBackend
	}
	if flowBackend == "orientdb" || topologyBackend == "orientdb" {
		orientDBPassword := os.Getenv("ORIENTDB_ROOT_PASSWORD")
		if orientDBPassword == "" {
			orientDBPassword = "root"
		}
		params[0]["OrientDBRootPassword"] = orientDBPassword
	}
	if topologyBackend != "" {
		params[0]["TopologyBackend"] = topologyBackend
	}
	if analyzerProbes != "" {
		params[0]["AnalyzerProbes"] = strings.Split(analyzerProbes, ",")
	}

	tmpl, err := template.New("config").Parse(conf)
	if err != nil {
		return err
	}
	buff := bytes.NewBufferString("")
	tmpl.Execute(buff, params[0])

	f.Write(buff.Bytes())
	f.Close()

	fmt.Printf("Config: %s\n", string(buff.Bytes()))

	return config.InitConfig("file", []string{f.Name()})
}

func execCmds(t *testing.T, cmds ...Cmd) (e error) {
	for _, cmd := range cmds {
		args := strings.Split(cmd.Cmd, " ")
		command := exec.Command(args[0], args[1:]...)
		logging.GetLogger().Debugf("Executing command %+v", args)
		stdouterr, err := command.CombinedOutput()
		if stdouterr != nil {
			logging.GetLogger().Debugf("Command returned %s", string(stdouterr))
		}
		if err != nil {
			if cmd.Check {
				t.Fatal("cmd : ("+cmd.Cmd+") returned ", err.Error(), string(stdouterr))
			}
			e = err
		}
	}
	return
}

func flowsToString(flows []*flow.Flow) string {
	s := fmt.Sprintf("%d flows:\n", len(flows))
	b, _ := json.MarshalIndent(flows, "", "\t")
	s += string(b) + "\n"
	return s
}

func (c *TestContext) getWholeGraph(t *testing.T, at time.Time) string {
	gremlin := g.G
	if !at.IsZero() {
		gremlin = gremlin.Context(at)
	}

	switch graphOutputFormat {
	case "ascii":
		header := make(http.Header)
		header.Set("Accept", "vnd.graphviz")

		resp, err := c.gh.Request(gremlin, header)
		if err != nil {
			t.Error(err)
			return ""
		}

		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
			t.Error(err)
			return ""
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Error(string(b))
			return ""
		}

		cmd := exec.Command("graph-easy", "--as_ascii")
		stdin, err := cmd.StdinPipe()
		if err != nil {
			t.Error(err)
			return ""
		}

		if _, err = stdin.Write(b); err != nil {
			t.Error(err)
			return ""
		}
		stdin.Write([]byte("\n"))
		stdin.Close()

		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Error(err)
			return ""
		}

		return "\n" + string(output)

	default:
		data, err := c.gh.Query(gremlin)
		if err != nil {
			t.Error(err)
			return ""
		}

		return string(data)
	}
}

func (c *TestContext) getAllFlows(t *testing.T, at time.Time) string {
	gremlin := g.G
	if !at.IsZero() {
		gremlin = gremlin.Context(at)
	}
	gremlin = gremlin.V().Flows().Sort()

	flows, err := c.gh.GetFlows(gremlin)
	if err != nil {
		t.Error(err)
		return ""
	}

	return flowsToString(flows)
}

func (c *TestContext) getSystemState(t *testing.T) {
	stateCmds := []Cmd{
		{"ip addr", false},
		{"ip netns list", false},
		{"ovs-vsctl show", false},
		{"brctl show", false},
	}
	execCmds(t, stateCmds...)
}

func (c *TestContext) postmortem(t *testing.T, test *Test, timestamp time.Time) {
	g := c.getWholeGraph(t, timestamp)
	f := c.getAllFlows(t, timestamp)
	execCmds(t, test.tearDownCmds...)
	c.getSystemState(t)
	t.Logf("Graph: %s", g)
	t.Logf("Flows: %s", f)
}

// RunTest executes a test. It executes the following steps:
// - create all the captures
// - execute all the setup commands
// - checks the created captures are active
// - checks the topology has settled
// - execute all the setup functions
// - create all the packet injections
// - run all the tests in live mode
// - execute the cleanup functions and commands
// - replay the tests against the history with the recorded timestamps
func RunTest(t *testing.T, test *Test) {
	client, err := gclient.NewCrudClientFromConfig(&shttp.AuthenticationOpts{})
	if err != nil {
		t.Fatalf("Failed to create client: %s", err)
	}

	t.Log("Removing existing captures")
	var captures []*types.Capture
	defer func() {
		for _, capture := range captures {
			client.Delete("capture", capture.ID())
		}
	}()

	t.Log("Creating captures")
	for _, tc := range test.captures {
		capture := types.NewCapture(tc.gremlin.String(), tc.bpf)
		capture.Type = tc.kind
		capture.RawPacketLimit = tc.rawPackets
		capture.Port = tc.port
		if err = client.Create("capture", capture); err != nil {
			t.Fatal(err)
		}
		captures = append(captures, capture)
	}

	t.Log("Executing setup commands")
	if test.preCleanup {
		execCmds(t, test.tearDownCmds...)
	}
	execCmds(t, test.setupCmds...)

	context := &TestContext{
		gh:       gclient.NewGremlinQueryHelper(&shttp.AuthenticationOpts{}),
		client:   client,
		captures: captures,
		data:     make(map[string]interface{}),
	}

	t.Log("Checking captures are correctly set up")
	err = common.Retry(func() error {
		for _, capture := range captures {
			nodes, err := context.gh.GetNodes(capture.GremlinQuery)
			if err != nil {
				return err
			}

			if len(nodes) == 0 {
				return fmt.Errorf("No node matching capture %s, graph: %s", capture.GremlinQuery, context.getWholeGraph(t, time.Time{}))
			}

			for _, node := range nodes {
				tp, err := node.GetFieldString("Type")
				if err != nil || !common.IsCaptureAllowed(tp) {
					continue
				}

				captureID, err := node.GetFieldString("Capture.ID")
				if err != nil {
					return fmt.Errorf("Node %+v matched the capture but capture is not enabled, graph: %s", node, context.getWholeGraph(t, time.Time{}))
				}
				if captureID != capture.ID() {
					return fmt.Errorf("Node %s matches multiple captures, graph: %s", node.ID, context.getWholeGraph(t, time.Time{}))
				}

				captureState, err := node.GetFieldString("Capture.State")
				if err != nil {
					return fmt.Errorf("Node %+v matched the capture but capture state is not set, graph: %s", node, context.getWholeGraph(t, time.Time{}))
				}
				if captureState != "active" {
					return fmt.Errorf("Capture %s is not active, graph: %s", capture.ID(), context.getWholeGraph(t, time.Time{}))
				}
			}
		}

		return nil
	}, 15, time.Second)

	if err != nil {
		context.postmortem(t, test, time.Now())
		t.Fatalf("Failed to setup captures: %s", err)
	}

	retries := test.retries
	if retries <= 0 {
		retries = 30
	}

	settleTime := time.Now()

	t.Log("Executing settle function")
	if test.settleFunction != nil {
		err = common.Retry(func() error {
			return test.settleFunction(context)
		}, retries, time.Second)

		if err != nil {
			t.Errorf("Test failed to settle: %s", err)
			context.postmortem(t, test, settleTime)
			return
		}
	}

	context.setupTime = time.Now()

	t.Log("Executing setup function")
	if test.preCleanup && test.tearDownFunction != nil {
		test.tearDownFunction(context)
	}
	if test.setupFunction != nil {
		if err = test.setupFunction(context); err != nil {
			context.postmortem(t, test, time.Time{})
			t.Fatalf("Failed to setup test: %s", err)
		}
	}

	// Wait for the interfaces to be ready for packet injection
	err = common.Retry(func() error {
		isReady := func(gremlin g.QueryString, ipv6 bool) error {
			gremlin = gremlin.Has("LinkFlags", "UP")
			if ipv6 {
				gremlin = gremlin.HasKey("IPV6")
			} else {
				gremlin = gremlin.HasKey("IPV4")
			}

			nodes, err := context.gh.GetNodes(gremlin)
			if err != nil {
				return fmt.Errorf("Gremlin request error `%s`: %s", gremlin, err)
			}

			if len(nodes) == 0 {
				return fmt.Errorf("No node matching injection %s, graph: %s", gremlin, context.getWholeGraph(t, time.Now()))
			}

			return nil
		}

		for _, injection := range test.injections {
			if err := isReady(injection.from, injection.ipv6); err != nil {
				return err
			}

			if injection.to != "" {
				if err := isReady(injection.to, injection.ipv6); err != nil {
					return err
				}
			}
		}

		return nil
	}, 15, time.Second)

	if err != nil {
		context.postmortem(t, test, time.Time{})
		t.Fatalf("Failed to setup test: %s", err)
	}

	for _, injection := range test.injections {
		ipVersion := 4
		if injection.ipv6 {
			ipVersion = 6
		}

		if injection.toIP != "" && injection.toMAC == "" {
			injection.toMAC = "00:11:22:33:44:55"
		}

		var src, srcIP, srcMAC string
		if injection.intf != "" {
			srcNode, err := context.gh.GetNode(injection.from)
			if err != nil {
				continue
			}

			src = injection.intf.String()
			srcMAC, _ = srcNode.GetFieldString("MAC")
			if addresses, _ := srcNode.GetFieldStringList(fmt.Sprintf("IPV%d", ipVersion)); len(addresses) > 0 {
				srcIP = strings.Split(addresses[0], "/")[0]
			}
		} else {
			src = injection.from.String()
		}

		if injection.count == 0 {
			injection.count = 1
		}

		var pcap []byte
		if injection.pcap != "" {
			pcapFile, err := os.Open(injection.pcap)
			if err != nil {
				t.Fatal(err)
			}

			pcap, err = ioutil.ReadAll(pcapFile)
			if err != nil {
				t.Fatal(err)
			}
		}
		packet := &types.PacketInjection{
			Src:       src,
			SrcMAC:    srcMAC,
			SrcIP:     srcIP,
			Dst:       injection.to.String(),
			DstMAC:    injection.toMAC,
			DstIP:     injection.toIP,
			Type:      fmt.Sprintf("icmp%d", ipVersion),
			Count:     injection.count,
			ICMPID:    injection.id,
			Increment: injection.increment,
			Payload:   injection.payload,
			Interval:  1000,
			Pcap:      pcap,
		}

		if err := pingRequest(t, context, packet); err != nil {
			t.Errorf("Packet injection failed: %s", err)
			context.postmortem(t, test, time.Time{})
			return
		}

		context.injections = append(context.injections, packet)
	}

	test.checkContexts = make([]*CheckContext, len(test.checks))

	t.Log("Running checks")
	for i, check := range test.checks {
		checkContext := &CheckContext{
			TestContext: context,
			gremlin:     g.G,
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
			t.Errorf("Test failed: %s", err)
			context.postmortem(t, test, time.Time{})
			return
		}
	}

	t.Log("Running tear down commands")
	if test.tearDownFunction != nil {
		if err = test.tearDownFunction(context); err != nil {
			execCmds(t, test.tearDownCmds...)
			context.getSystemState(t)
			t.Fatalf("Fail to tear test down: %s", err)
		}
	}

	execCmds(t, test.tearDownCmds...)

	if test.mode == Replay && agentTestsOnly == false {
		if topologyBackend != "memory" {
			for i, check := range test.checks {
				checkContext := test.checkContexts[i]
				checkContext.gremlin = checkContext.gremlin.Context(checkContext.time)
				t.Logf("Replaying test with time %s (Unix: %d), startTime %s (Unix: %d)", checkContext.time, checkContext.time.Unix(), checkContext.startTime, checkContext.startTime.Unix())
				err = common.Retry(func() error {
					return check(checkContext)
				}, retries, time.Second)

				if err != nil {
					t.Errorf("Failed to replay test: %s, graph: %s, flows: %s", err, checkContext.getWholeGraph(t, checkContext.time), checkContext.getAllFlows(t, checkContext.time))
				}
			}
		} else {
			t.Logf("Skipping replay as there is no persistent backend")
		}
	}
}

func pingRequest(t *testing.T, context *TestContext, packet *types.PacketInjection) error {
	return context.client.Create("injectpacket", packet)
}

func ping(t *testing.T, context *TestContext, ipVersion int, src, dst g.QueryString, count int64, id int64) error {
	packet := &types.PacketInjection{
		Src:      src.String(),
		Dst:      dst.String(),
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
	flag.BoolVar(&standalone, "standalone", false, "Start an analyzer and an agent")
	flag.BoolVar(&agentTestsOnly, "agenttestsonly", false, "run agent test only")
	flag.BoolVar(&noOFTests, "nooftests", false, "dont't run OpenFlow tests")
	flag.StringVar(&etcdServer, "etcd.server", "", "Etcd server")
	flag.StringVar(&topologyBackend, "analyzer.topology.backend", "memory", "Specify the graph storage backend used")
	flag.StringVar(&graphOutputFormat, "graph.output", "", "Graph output format (json, dot or ascii)")
	flag.StringVar(&flowBackend, "analyzer.flow.backend", "", "Specify the flow storage backend used")
	flag.StringVar(&analyzerListen, "analyzer.listen", "0.0.0.0:64500", "Specify the analyzer listen address")
	flag.StringVar(&analyzerProbes, "analyzer.topology.probes", "", "Specify the analyzer probes to enable")
	flag.Parse()

	http.DefaultTransport.(*http.Transport).MaxIdleConnsPerHost = 100

	if standalone {
		if err := initConfig(testConfig); err != nil {
			panic(fmt.Sprintf("Failed to initialize config: %s", err))
		}

		if err := config.InitLogging(); err != nil {
			panic(fmt.Sprintf("Failed to initialize logging system: %s", err))
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
