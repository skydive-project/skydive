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

package helper

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
	cmd "github.com/skydive-project/skydive/cmd/client"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/flow"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/storage"
	"github.com/skydive-project/skydive/topology/graph"
)

type Cmd struct {
	Cmd   string
	Check bool
}

type GremlinQueryHelper struct {
	authOptions *shttp.AuthenticationOpts
}

var (
	etcdServer   string
	graphBackend string
)

func init() {
	flag.StringVar(&etcdServer, "etcd.server", "", "Etcd server")
	flag.StringVar(&graphBackend, "graph.backend", "memory", "Specify the graph backend used")
	flag.Parse()
}

func SFlowSetup(t *testing.T) (*net.UDPConn, error) {
	addr := net.UDPAddr{
		Port: 0,
		IP:   net.ParseIP("localhost"),
	}
	conn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		t.Errorf("Unable to listen on UDP %s", err.Error())
		return nil, err
	}
	return conn, nil
}

func InitConfig(t *testing.T, conf string, params ...HelperParams) {
	f, err := ioutil.TempFile("", "skydive_agent")
	if err != nil {
		t.Fatal(err.Error())
	}

	if len(params) == 0 {
		params = []HelperParams{make(HelperParams)}
	}
	params[0]["AnalyzerPort"] = 64500
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
		params[0]["EtcdServer"] = "http://localhost:2374"
	}
	tmpl, err := template.New("config").Parse(conf)
	if err != nil {
		t.Fatal(err.Error())
	}
	buff := bytes.NewBufferString("")
	tmpl.Execute(buff, params[0])

	f.Write(buff.Bytes())
	f.Close()

	t.Logf("Configuration: %s", buff.String())

	err = config.InitConfig("file", f.Name())
	if err != nil {
		t.Fatal(err.Error())
	}

	err = logging.InitLogger()
	if err != nil {
		t.Fatal(err)
	}
}

type helperService int

const (
	start helperService = iota
	stop
	flush
)

type HelperAgentAnalyzer struct {
	t        *testing.T
	storage  storage.Storage
	Agent    *agent.Agent
	Analyzer *analyzer.Server

	service     chan helperService
	serviceDone chan bool
}

type HelperParams map[string]interface{}

func NewAgentAnalyzerWithConfig(t *testing.T, conf string, s storage.Storage, params ...HelperParams) *HelperAgentAnalyzer {
	InitConfig(t, conf, params...)
	agent := NewAgent()
	analyzer := NewAnalyzerStorage(t, s)

	helper := &HelperAgentAnalyzer{
		t:        t,
		storage:  s,
		Agent:    agent,
		Analyzer: analyzer,

		service:     make(chan helperService),
		serviceDone: make(chan bool),
	}

	go helper.run()
	return helper
}

func (h *HelperAgentAnalyzer) startAnalyzer() {
	h.Analyzer.ListenAndServe()
	WaitApi(h.t, h.Analyzer)
}

func (h *HelperAgentAnalyzer) Start() {
	h.service <- start
	<-h.serviceDone
}
func (h *HelperAgentAnalyzer) Stop() {
	h.service <- stop
	<-h.serviceDone
}
func (h *HelperAgentAnalyzer) Flush() {
	h.service <- flush
	<-h.serviceDone
}
func (h *HelperAgentAnalyzer) run() {
	for {
		switch <-h.service {
		case start:
			h.startAnalyzer()
			h.Agent.Start()
		case stop:
			h.Agent.Stop()
			h.Analyzer.Stop()
		case flush:
			h.Agent.FlowTableAllocator.Flush()
			time.Sleep(500 * time.Millisecond)
			h.Analyzer.FlowTable.Flush()
		}
		h.serviceDone <- true
	}
}

func NewAnalyzerStorage(t *testing.T, s storage.Storage) *analyzer.Server {
	server, err := analyzer.NewServerFromConfig()
	if err != nil {
		t.Fatal(err)
	}

	server.SetStorage(s)
	return server
}

func NewAgent() *agent.Agent {
	return agent.NewAgent()
}

func WaitApi(t *testing.T, analyzer *analyzer.Server) {
	// waiting for the api endpoint
	for i := 1; i <= 5; i++ {
		url := fmt.Sprintf("http://%s:%d/api", analyzer.HTTPServer.Addr, analyzer.HTTPServer.Port)
		_, err := http.Get(url)
		if err == nil {
			return
		}
		time.Sleep(time.Second)
	}

	t.Fatal("Fail to start the analyzer")
}

func StartAnalyzerWithConfig(t *testing.T, conf string, s storage.Storage, params ...HelperParams) *analyzer.Server {
	InitConfig(t, conf, params...)
	analyzer := NewAnalyzerStorage(t, s)
	analyzer.ListenAndServe()
	WaitApi(t, analyzer)
	return analyzer
}

func StartAgentWithConfig(t *testing.T, conf string, params ...HelperParams) *agent.Agent {
	InitConfig(t, conf, params...)
	agent := NewAgent()
	agent.Start()
	return agent
}

func ExecCmds(t *testing.T, cmds ...Cmd) {
	for _, cmd := range cmds {
		args := strings.Split(cmd.Cmd, " ")
		command := exec.Command(args[0], args[1:]...)
		stdouterr, err := command.CombinedOutput()
		if err != nil && cmd.Check {
			output := ""
			if testing.Verbose() {
				output = string(stdouterr)
			}
			t.Fatal("cmd : ("+cmd.Cmd+") ", err.Error(), output)
		}
	}
}

func NewGraph(t *testing.T) *graph.Graph {
	var backend graph.GraphBackend
	var err error
	switch graphBackend {
	case "gremlin-ws":
		backend, err = graph.NewGremlinBackend("ws://127.0.0.1:8182")
	case "gremlin-rest":
		backend, err = graph.NewGremlinBackend("http://127.0.0.1:8182?gremlin=")
	case "elasticsearch":
		backend, err = graph.NewElasticSearchBackend("127.0.0.1", "9200", 10, 60)
		if err == nil {
			backend, err = graph.NewShadowedBackend(backend)
		}
	case "orientdb":
		password := os.Getenv("ORIENTDB_ROOT_PASSWORD")
		if password == "" {
			password = "root"
		}
		backend, err = graph.NewOrientDBBackend("http://127.0.0.1:2480", "TestSkydive", "root", password)
	default:
		backend, err = graph.NewMemoryBackend()
	}

	if err != nil {
		t.Fatal(err.Error())
	}

	t.Logf("Using %s as backend", graphBackend)

	g, err := graph.NewGraphFromConfig(backend)
	if err != nil {
		t.Fatal(err.Error())
	}

	hostname, err := os.Hostname()
	if err != nil {
		t.Fatal(err.Error())
	}

	root := g.LookupFirstNode(graph.Metadata{"Name": hostname, "Type": "host"})
	if root == nil {
		root = g.NewNode(graph.Identifier(hostname), graph.Metadata{"Name": hostname, "Type": "host"})
		if root == nil {
			t.Fatal("fail while adding root node")
		}
	}

	return g
}

func (g *GremlinQueryHelper) GremlinQuery(t *testing.T, query string, values interface{}) {
	body, err := cmd.SendGremlinQuery(g.authOptions, query)
	if err != nil {
		t.Fatalf("Error while executing query %s: %s", query, err.Error())
	}

	err = json.NewDecoder(body).Decode(values)
	if err != nil {
		t.Fatal(err.Error())
	}
}

func (g *GremlinQueryHelper) GetNodesFromGremlinReply(t *testing.T, query string) []graph.Node {
	var values []interface{}
	g.GremlinQuery(t, query, &values)
	nodes := make([]graph.Node, len(values))
	for i, node := range values {
		if err := nodes[i].Decode(node); err != nil {
			t.Fatal(err.Error())
		}
	}
	return nodes
}

func (g *GremlinQueryHelper) GetNodeFromGremlinReply(t *testing.T, query string) *graph.Node {
	nodes := g.GetNodesFromGremlinReply(t, query)
	if len(nodes) > 0 {
		return &nodes[0]
	}
	return nil
}

func (g *GremlinQueryHelper) GetFlowsFromGremlinReply(t *testing.T, query string) (flows []*flow.Flow) {
	g.GremlinQuery(t, query, &flows)
	return
}

func (g *GremlinQueryHelper) GetFlowSetBandwidthFromGremlinReply(t *testing.T, query string) flow.FlowSetBandwidth {
	body, err := cmd.SendGremlinQuery(g.authOptions, query)
	if err != nil {
		t.Fatalf("%s: %s", query, err.Error())
	}

	var bw []flow.FlowSetBandwidth
	err = json.NewDecoder(body).Decode(&bw)
	if err != nil {
		t.Fatal(err.Error())
	}

	return bw[0]
}

func NewGremlinQueryHelper(authOptions *shttp.AuthenticationOpts) *GremlinQueryHelper {
	return &GremlinQueryHelper{
		authOptions: authOptions,
	}
}
