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
	"syscall"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/logging"
)

type Cmd struct {
	Cmd   string
	Check bool
}

var (
	Standalone bool

	etcdServer             string
	graphBackend           string
	storageBackend         string
	useFlowsConnectionType string
)

type HelperParams map[string]interface{}

func init() {
	flag.BoolVar(&Standalone, "standalone", false, "Start an analyzer and an agent")
	flag.StringVar(&etcdServer, "etcd.server", "", "Etcd server")
	flag.StringVar(&graphBackend, "graph.backend", "memory", "Specify the graph backend used")
	flag.StringVar(&storageBackend, "storage.backend", "", "Specify the storage backend used")
	flag.StringVar(&useFlowsConnectionType, "use.FlowsConnectionType", "UDP", "Specify the flows connection type between Agent(s) and Analyzer")
	flag.Parse()
}

func InitConfig(conf string, params ...HelperParams) error {
	f, err := ioutil.TempFile("", "skydive_agent")
	if err != nil {
		return err
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
		params[0]["EtcdServer"] = "http://localhost:2379"
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
		return err
	}
	buff := bytes.NewBufferString("")
	tmpl.Execute(buff, params[0])

	f.Write(buff.Bytes())
	f.Close()

	fmt.Printf("Config: %s\n", string(buff.Bytes()))

	err = config.InitConfig("file", f.Name())
	if err != nil {
		return err
	}

	err = logging.InitLogger()
	if err != nil {
		return err
	}

	return nil
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
			if onReady != nil {
				onReady(ws)
			}
		}
		return nil
	}
	ws.SetPingHandler(h)

	return ws, nil
}

func SendPCAPFile(filename string, socket string) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("Failed to open file %s: %s", filename, err.Error())
	}

	stats, err := file.Stat()
	if err != nil {
		return fmt.Errorf("Failed to get informations for %s: %s", filename, err.Error())
	}

	tcpAddr, err := net.ResolveTCPAddr("tcp", socket)
	if err != nil {
		return fmt.Errorf("Failed to parse address %s: %s", tcpAddr.String(), err.Error())
	}

	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		return fmt.Errorf("Failed to connect to TCP socket %s: %s", tcpAddr.String(), err.Error())
	}

	unixFile, err := conn.File()
	if err != nil {
		return fmt.Errorf("Failed to get file description from socket %s: %s", socket, err.Error())
	}
	defer unixFile.Close()

	dst := unixFile.Fd()
	src := file.Fd()

	_, err = syscall.Sendfile(int(dst), int(src), nil, int(stats.Size()))
	if err != nil {
		logging.GetLogger().Fatalf("Failed to send file %s to socket %s: %s", filename, socket, err.Error())
	}

	return nil
}

func FlowsToString(flows []*flow.Flow) string {
	s := fmt.Sprintf("%d flows:\n", len(flows))
	b, _ := json.MarshalIndent(flows, "", "\t")
	s += string(b) + "\n"
	return s
}
