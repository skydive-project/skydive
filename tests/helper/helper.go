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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/skydive-project/skydive/agent"
	"github.com/skydive-project/skydive/analyzer"
	cmd "github.com/skydive-project/skydive/cmd/client"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/flow/storage"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
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
	etcdServer             string
	graphBackend           string
	storageBackend         string
	useFlowsConnectionType string
)

func init() {
	flag.StringVar(&etcdServer, "etcd.server", "", "Etcd server")
	flag.StringVar(&graphBackend, "graph.backend", "memory", "Specify the graph backend used")
	flag.StringVar(&storageBackend, "storage.backend", "", "Specify the storage backend used")
	flag.StringVar(&useFlowsConnectionType, "use.FlowsConnectionType", "UDP", "Specify the flows connection type between Agent(s) and Analyzer")
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
	if storageBackend != "" {
		params[0]["Storage"] = storageBackend
	}
	if storageBackend == "orientdb" || graphBackend == "orientdb" {
		orientDBPassword := os.Getenv("ORIENTDB_ROOT_PASSWORD")
		if orientDBPassword == "" {
			orientDBPassword = "root"
		}
		params[0]["OrientDBRootPassword"] = orientDBPassword
	}
	if graphBackend != "" {
		params[0]["GraphBackend"] = graphBackend
	}
	if useFlowsConnectionType == "TLS" {
		cert, key := GenerateFakeX509Certificate("server")
		params[0]["AnalyzerX509_cert"] = cert
		params[0]["AnalyzerX509_key"] = key
		cert, key = GenerateFakeX509Certificate("client")
		params[0]["AgentX509_cert"] = cert
		params[0]["AgentX509_key"] = key
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

	CleanGraph(h.Analyzer.GraphServer.Graph)
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

	if server.Storage == nil {
		server.SetStorage(s)
	}

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
	s.Start()
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
		logging.GetLogger().Debugf("Executing command %s with args %+v", args[0], args[1:])
		stdouterr, err := command.CombinedOutput()
		logging.GetLogger().Debugf("Command returned %s", stdouterr)
		if err != nil && cmd.Check {
			t.Fatal("cmd : ("+cmd.Cmd+") returned ", err.Error())
		}
	}
}

func NewGraph(t *testing.T) *graph.Graph {
	var backend graph.GraphBackend
	var err error
	switch graphBackend {
	case "elasticsearch":
		backend, err = graph.NewElasticSearchBackend("127.0.0.1", "9200", 10, 60, 1)
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

	g := graph.NewGraphFromConfig(backend)

	hostname, err := os.Hostname()
	if err != nil {
		t.Fatal(err.Error())
	}

	root := g.LookupFirstNode(graph.Metadata{"Name": hostname, "Type": "host"})
	if root == nil {
		root = agent.CreateRootNode(g)
		if root == nil {
			t.Fatal("fail while adding root node")
		}
	}

	return g
}

func CleanGraph(g *graph.Graph) {
	g.Lock()
	defer g.Unlock()
	hostname, _ := os.Hostname()
	g.DelHostGraph(hostname)
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

func (g *GremlinQueryHelper) GetFlowMetricFromGremlinReply(t *testing.T, query string) flow.FlowMetric {
	body, err := cmd.SendGremlinQuery(g.authOptions, query)
	if err != nil {
		t.Fatalf("%s: %s", query, err.Error())
	}

	var metric flow.FlowMetric
	err = json.NewDecoder(body).Decode(&metric)
	if err != nil {
		t.Fatal(err.Error())
	}

	return metric
}

func NewGremlinQueryHelper(authOptions *shttp.AuthenticationOpts) *GremlinQueryHelper {
	return &GremlinQueryHelper{
		authOptions: authOptions,
	}
}

func FilterIPv6AddrAnd(flows []*flow.Flow, A, B string) (r []*flow.Flow) {
	for _, f := range flows {
		if f.Network == nil || (f.Network.Protocol != flow.FlowProtocol_IPV6) {
			continue
		}
		if strings.HasPrefix(f.Network.A, A) && strings.HasPrefix(f.Network.B, B) {
			r = append(r, f)
		}
		if strings.HasPrefix(f.Network.A, B) && strings.HasPrefix(f.Network.B, A) {
			r = append(r, f)
		}
	}
	return r
}

func publicKey(priv interface{}) interface{} {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return &k.PublicKey
	case *ecdsa.PrivateKey:
		return &k.PublicKey
	default:
		return nil
	}
}

func pemBlockForKey(priv interface{}) *pem.Block {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)}
	case *ecdsa.PrivateKey:
		b, err := x509.MarshalECPrivateKey(k)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to marshal ECDSA private key: %v", err)
			os.Exit(2)
		}
		return &pem.Block{Type: "EC PRIVATE KEY", Bytes: b}
	default:
		return nil
	}
}

func GenerateFakeX509Certificate(certType string) (string, string) {
	priv, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	if err != nil {
		logging.GetLogger().Fatal("ECDSA GenerateKey failed : " + err.Error())
	}
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		logging.GetLogger().Fatal("Serial number generation error : " + err.Error())
	}
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Skydive Co"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(1 * time.Hour),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	if certType == "server" {
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	} else {
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	}

	hosts := strings.Split("127.0.0.1,::1", ",")
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, h)
		}
	}
	template.DNSNames = append(template.DNSNames, "localhost")
	template.EmailAddresses = append(template.EmailAddresses, "skydive@skydive-test.com")
	/* Generate CA */
	template.IsCA = true
	template.KeyUsage |= x509.KeyUsageCertSign

	/* Certificate */
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, publicKey(priv), priv)
	if err != nil {
		logging.GetLogger().Fatalf("Failed to create certificate: %s", err)
	}

	basedir, err := ioutil.TempDir("", "skydive-keys")
	if err != nil {
		logging.GetLogger().Fatal("can't create tempdir skydive-keys")
	}
	certFilename := filepath.Join(basedir, "cert.pem")
	certOut, err := os.Create(certFilename)
	if err != nil {
		logging.GetLogger().Fatalf("failed to open %s for writing: %s", certFilename, err)
	}
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	certOut.Close()

	/* Private Key */
	privKeyFilename := filepath.Join(basedir, "key.pem")
	keyOut, err := os.OpenFile(privKeyFilename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		logging.GetLogger().Fatalf("failed to open %s for writing: %s", privKeyFilename, err)
	}
	pem.Encode(keyOut, pemBlockForKey(priv))
	keyOut.Close()

	return certFilename, privKeyFilename
}

func newWSClient(endpoint string) (*websocket.Conn, error) {
	conn, err := net.Dial("tcp", endpoint)
	if err != nil {
		return nil, err
	}

	endpoint = fmt.Sprintf("ws://%s/ws", endpoint)
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	wsConn, _, err := websocket.NewClient(conn, u, http.Header{"Origin": {endpoint}}, 1024, 1024)
	if err != nil {
		return nil, err
	}

	return wsConn, nil
}

func WSConnect(endpoint string, timeout int, onReady func(*websocket.Conn)) (*websocket.Conn, error) {
	var ws *websocket.Conn
	var err error

	t := 0
	for {
		if t > timeout {
			return nil, errors.New("Connection to Agent : timeout reached")
		}

		ws, err = newWSClient(endpoint)
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
		t++
	}

	ready := false
	h := func(message string) error {
		err := ws.WriteControl(websocket.PongMessage, []byte(message), time.Now().Add(time.Second))
		if err != nil {
			return err
		}
		if !ready {
			ready = true
			onReady(ws)
		}
		return nil
	}
	ws.SetPingHandler(h)

	return ws, nil
}
