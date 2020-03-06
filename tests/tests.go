/*
 * Copyright (C) 2015 Red Hat, Inc.
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
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/avast/retry-go"
	shellquote "github.com/kballard/go-shellquote"
	"gopkg.in/mcuadros/go-syslog.v2"

	"github.com/skydive-project/skydive/agent"
	"github.com/skydive-project/skydive/analyzer"
	"github.com/skydive-project/skydive/api/client"
	apiclient "github.com/skydive-project/skydive/api/client"
	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/flow/probes"
	"github.com/skydive-project/skydive/graffiti/service"
	g "github.com/skydive-project/skydive/gremlin"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/profiling"
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
    probes: {{block "analyzerprobes" .}}{{"\n"}}{{range .AnalyzerProbes}}{{println "    -" .}}{{end}}{{end}}
  startup:
    capture_gremlin: "g.V().Has('Name','startup-vm2')"

agent:
  listen: {{.AgentAddr}}:{{.AgentPort}}
  topology:
    probes:
    - ovsdb
    - blockdev
    - docker
    - lxd
    - lldp
    - runc
    - socketinfo
    - libvirt{{block "agentProbes" .}}{{"\n"}}{{range .AgentProbes}}{{println "    -" .}}{{end}}{{end}}
    netlink:
      metrics_update: 5
    lldp:
      interfaces:
      - lldp0

  metadata:
    mydict:
      value: 123
    myarrays:
      integers:
      - 1
      - 2
      - 3
      bools:
      - true
      - true
      strings:
      - dog
      - cat
      - frog

flow:
  expire: 600
  update: 10

ovs:
  oflow:
    enable: true
    native: {{.OvsOflowNative}}
    address:
      br-test: tcp:127.0.0.1:16633

storage:
  orientdb:
    addr: http://127.0.0.1:2480
    database: Skydive
    username: root
    password: {{.OrientDBRootPassword}}

logging:
  level: DEBUG
  syslog:
    address: /tmp/skydive-test-syslog

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
	gh              *apiclient.GremlinQueryHelper
	client          *shttp.CrudClient
	captures        []*types.Capture
	injections      []*types.PacketInjection
	setupTime       time.Time
	data            map[string]interface{}
	setupCmdOutputs []string
}

// TestCapture describes a capture to be created in tests
type TestCapture struct {
	gremlin         g.QueryString
	kind            string
	bpf             string
	rawPackets      int
	port            int
	samplingRate    uint32 // default-value : 1
	pollingInterval uint32 // default-value : 10
	target          string
	targetType      string
}

// TestInjection describes a packet injection to be created in tests
type TestInjection struct {
	intf     g.QueryString
	from     g.QueryString
	fromPort uint16
	to       g.QueryString
	toMAC    string
	toIP     string
	toPort   uint16
	protocol string // icmp if not set
	ipv6     bool
	count    uint64
	id       uint64
	mode     string
	payload  string
	pcap     string
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
	retries          uint
	mode             int
	checks           []CheckFunction
	checkContexts    []*CheckContext
}

var (
	agentProbes       string
	analyzerListen    string
	analyzerProbes    string
	etcdServer        string
	testLogs          string
	flowBackend       string
	graphOutputFormat string
	ovsOflowNative    bool
	standalone        bool
	topologyBackend   string
	profile           bool
)

func initConfig(conf string, params ...helperParams) error {
	f, err := ioutil.TempFile("", "skydive_agent")
	if err != nil {
		return err
	}

	if len(params) == 0 {
		params = []helperParams{make(helperParams)}
	}

	sa, err := service.AddressFromString(analyzerListen)
	if err != nil {
		return err
	}
	params[0]["AnalyzerAddr"] = sa.Addr
	params[0]["AnalyzerPort"] = sa.Port
	params[0]["AgentAddr"] = sa.Addr
	params[0]["AgentPort"] = sa.Port - 1
	params[0]["OvsOflowNative"] = strconv.FormatBool(ovsOflowNative)

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
	if agentProbes != "" {
		params[0]["AgentProbes"] = strings.Split(agentProbes, ",")
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

func execCmds(t *testing.T, cmds ...Cmd) (outputs []string, e error) {
	outputs = make([]string, len(cmds))
	for i, cmd := range cmds {
		args, err := shellquote.Split(cmd.Cmd)
		if err != nil {
			return nil, err
		}
		command := exec.Command(args[0], args[1:]...)
		logging.GetLogger().Debugf("Executing command %+v", args)
		stdouterr, err := command.CombinedOutput()
		if stdouterr != nil {
			logging.GetLogger().Debugf("Command returned %s", string(stdouterr))
			outputs[i] = string(stdouterr)
		}
		if err != nil {
			if cmd.Check {
				return nil, fmt.Errorf("cmd : (%s) returned error %s with output %s", cmd.Cmd, err, string(stdouterr))
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
		header.Set("Accept", "text/vnd.graphviz")

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

		var out bytes.Buffer
		json.Indent(&out, data, "", "\t")

		ioutil.WriteFile("graph.json", out.Bytes(), 0644)

		return out.String()
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
	if standalone && !initStandalone {
		runStandalone()
	}

	client, err := client.NewCrudClientFromConfig(&shttp.AuthenticationOpts{})
	if err != nil {
		t.Fatalf("Failed to create client: %s", err)
	}

	var captures []*types.Capture
	defer func() {
		t.Log("Removing existing captures")
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
		capture.SamplingRate = tc.samplingRate
		capture.PollingInterval = tc.pollingInterval
		capture.Target = tc.target
		capture.TargetType = tc.targetType
		if err = client.Create("capture", capture, nil); err != nil {
			t.Fatal(err)
		}
		captures = append(captures, capture)
	}

	t.Log("Executing setup commands")
	if test.preCleanup {
		execCmds(t, test.tearDownCmds...)
	}

	context := &TestContext{
		gh:       apiclient.NewGremlinQueryHelper(&shttp.AuthenticationOpts{}),
		client:   client,
		captures: captures,
		data:     make(map[string]interface{}),
	}

	if context.setupCmdOutputs, err = execCmds(t, test.setupCmds...); err != nil {
		execCmds(t, test.tearDownCmds...)
		t.Fatal(err)
	}

	t.Log("Checking captures are correctly set up")
	err = retry.Do(func() error {
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
				if err != nil || !probes.IsCaptureAllowed(tp) {
					continue
				}

				findCapture := func(id string) *probes.CaptureMetadata {
					field, err := node.GetField("Captures")
					if err != nil {
						return nil
					}

					if captures, ok := field.(*probes.Captures); ok {
						for _, capture := range *captures {
							if capture.ID == id {
								return capture
							}
						}
					}
					return nil
				}

				captureMetadata := findCapture(capture.ID())
				if captureMetadata == nil {
					return fmt.Errorf("Node %+v matched the capture but capture %s is not enabled, graph: %s", node, capture.ID(), context.getWholeGraph(t, time.Time{}))
				}

				if captureMetadata.State != "active" {
					return fmt.Errorf("Capture %s is not active, graph: %s", capture.ID(), context.getWholeGraph(t, time.Time{}))
				}

				return nil
			}
		}

		return nil
	}, retry.Attempts(15), retry.Delay(time.Second), retry.DelayType(retry.FixedDelay))

	if err != nil {
		context.postmortem(t, test, time.Now())
		t.Fatalf("Failed to setup captures: %s", err)
	}

	// wait a bit after the capture creation
	time.Sleep(2 * time.Second)

	retries := test.retries
	if retries <= 0 {
		retries = 30
	}

	settleTime := time.Now()

	t.Log("Executing settle function")
	if test.settleFunction != nil {
		err = retry.Do(func() error {
			return test.settleFunction(context)
		}, retry.Attempts(uint(retries)), retry.Delay(time.Second), retry.DelayType(retry.FixedDelay))

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
	err = retry.Do(func() error {
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
	}, retry.Attempts(15), retry.Delay(time.Second), retry.DelayType(retry.FixedDelay))

	if err != nil {
		context.postmortem(t, test, time.Time{})
		t.Fatalf("Failed to setup test: %s", err)
	}

	defer func() {
		t.Log("Removing existing injections")
		for _, injection := range context.injections {
			client.Delete("injectpacket", injection.ID())
		}
	}()

	for _, injection := range test.injections {
		if injection.protocol == "" {
			injection.protocol = "icmp"
		}
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
			Src:      src,
			SrcMAC:   srcMAC,
			SrcIP:    srcIP,
			SrcPort:  injection.fromPort,
			Dst:      injection.to.String(),
			DstMAC:   injection.toMAC,
			DstIP:    injection.toIP,
			DstPort:  injection.toPort,
			Type:     fmt.Sprintf("%s%d", injection.protocol, ipVersion),
			Count:    injection.count,
			ICMPID:   uint16(injection.id),
			Mode:     injection.mode,
			Payload:  injection.payload,
			Interval: 1000,
			Pcap:     pcap,
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

		err = retry.Do(func() error {
			if err = check(checkContext); err != nil {
				return err
			}
			checkContext.successTime = time.Now()
			if checkContext.time.IsZero() {
				checkContext.time = checkContext.successTime
			}
			return nil
		}, retry.Attempts(retries), retry.Delay(time.Second), retry.DelayType(retry.FixedDelay))

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

	if _, err := execCmds(t, test.tearDownCmds...); err != nil {
		t.Fatal(err)
	}

	if test.mode == Replay {
		if topologyBackend != "memory" {
			for i, check := range test.checks {
				checkContext := test.checkContexts[i]
				checkContext.gremlin = checkContext.gremlin.Context(checkContext.time)
				t.Logf("Replaying test with time %s (Unix: %d), startTime %s (Unix: %d)", checkContext.time, checkContext.time.Unix(), checkContext.startTime, checkContext.startTime.Unix())
				err = retry.Do(func() error {
					return check(checkContext)
				}, retry.Attempts(retries), retry.Delay(time.Second), retry.DelayType(retry.FixedDelay))

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
	return context.client.Create("injectpacket", packet, nil)
}

func ping(t *testing.T, context *TestContext, ipVersion int, src, dst g.QueryString, count uint64, id uint16) error {
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

var initStandalone = false

func runStandalone() {
	if profile {
		go profiling.Profile("/tmp/skydive-test-")
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
	initStandalone = true
}

func init() {
	flag.BoolVar(&standalone, "standalone", false, "Start an analyzer and an agent")
	flag.StringVar(&testLogs, "logs", "", "Capture test logs using syslog and write them to the specified file")
	flag.StringVar(&etcdServer, "etcd.server", "", "Etcd server")
	flag.StringVar(&topologyBackend, "analyzer.topology.backend", "memory", "Specify the graph storage backend used")
	flag.StringVar(&graphOutputFormat, "graph.output", "", "Graph output format (json, dot or ascii)")
	flag.StringVar(&flowBackend, "analyzer.flow.backend", "", "Specify the flow storage backend used")
	flag.StringVar(&analyzerListen, "analyzer.listen", "0.0.0.0:64500", "Specify the analyzer listen address")
	flag.StringVar(&analyzerProbes, "analyzer.topology.probes", "", "Specify the analyzer probes to enable")
	flag.StringVar(&agentProbes, "agent.topology.probes", "", "Specify the extra agent probes to enable")
	flag.BoolVar(&ovsOflowNative, "ovs.oflow.native", false, "Use native OpenFlow protocol instead of ovs-ofctl")
	flag.BoolVar(&profile, "profile", false, "Start profiling")
	flag.Parse()

	http.DefaultTransport.(*http.Transport).MaxIdleConnsPerHost = 100

	if err := initConfig(testConfig); err != nil {
		panic(fmt.Sprintf("Failed to initialize config: %s", err))
	}

	if testLogs != "" {
		loggingBackends := config.GetStringSlice("logging.backends")
		sort.Strings(loggingBackends)
		if n := sort.SearchStrings(loggingBackends, "syslog"); n == len(loggingBackends) || loggingBackends[n] != "syslog" {
			config.Set("logging.backends", append(loggingBackends, "syslog"))
		}

		channel := make(syslog.LogPartsChannel)
		handler := syslog.NewChannelHandler(channel)

		syslogServer := syslog.NewServer()
		syslogServer.SetFormat(syslog.RFC3164)
		syslogServer.SetHandler(handler)

		if err := os.MkdirAll(filepath.Dir(testLogs), os.ModeDir); err != nil {
			panic(err)
		}

		os.Remove("/tmp/skydive-test-syslog")
		if err := syslogServer.ListenUnixgram("/tmp/skydive-test-syslog"); err != nil {
			panic(err)
		}

		if err := syslogServer.Boot(); err != nil {
			panic(err)
		}

		f, err := os.Create(testLogs)
		if err != nil {
			panic(err)
		}

		go func(channel syslog.LogPartsChannel) {
			for logParts := range channel {
				fmt.Fprintln(f, logParts["content"])
			}
		}(channel)
	}

	if err := config.InitLogging(); err != nil {
		panic(fmt.Sprintf("Failed to initialize logging system: %s", err))
	}
}
