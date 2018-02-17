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
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
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
	Standalone        bool
	GraphOutputFormat string

	etcdServer     string
	graphBackend   string
	storageBackend string
	analyzerProbes string
)

type HelperParams map[string]interface{}

func init() {
	flag.BoolVar(&Standalone, "standalone", false, "Start an analyzer and an agent")
	flag.StringVar(&etcdServer, "etcd.server", "", "Etcd server")
	flag.StringVar(&graphBackend, "graph.backend", "memory", "Specify the graph backend used")
	flag.StringVar(&GraphOutputFormat, "graph.output", "", "Graph output format (json, dot or ascii)")
	flag.StringVar(&storageBackend, "storage.backend", "", "Specify the storage backend used")
	flag.StringVar(&analyzerProbes, "analyzer.topology.probes", "", "Specify the analyzer probes to enable")
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
		params[0]["EtcdServer"] = "http://localhost:12379"
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

	err = config.InitConfig("file", []string{f.Name()})
	if err != nil {
		return err
	}

	return nil
}

func ExecCmds(t *testing.T, cmds ...Cmd) error {
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
		}
	}
	return nil
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

func newWSClient(endpoint string) (*websocket.Conn, error) {
	conn, err := net.Dial("tcp", endpoint)
	if err != nil {
		return nil, err
	}

	scheme := "ws"
	if config.IsTLSenabled() == true {
		scheme = "wss"
	}
	endpoint = fmt.Sprintf("%s://%s/ws/subscriber", scheme, endpoint)
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

func WSClose(ws *websocket.Conn) error {
	if err := ws.WriteControl(websocket.CloseMessage, nil, time.Now().Add(3*time.Second)); err != nil {
		return err
	}
	return ws.Close()
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
