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
	"fmt"
	"html/template"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/redhat-cip/skydive/agent"
	"github.com/redhat-cip/skydive/analyzer"
	"github.com/redhat-cip/skydive/config"
	"github.com/redhat-cip/skydive/logging"
	"github.com/redhat-cip/skydive/storage"
)

type Cmd struct {
	Cmd   string
	Check bool
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

func InitConfig(t *testing.T, conf string) {
	f, err := ioutil.TempFile("", "skydive_agent")
	if err != nil {
		t.Fatal(err.Error())
	}

	param := struct {
		AnalyzerPort int
		LogLevel     string
	}{
		AnalyzerPort: rand.Intn(400) + 64500,
	}

	if testing.Verbose() {
		param.LogLevel = "DEBUG"
	} else {
		param.LogLevel = "INFO"
	}

	tmpl, err := template.New("config").Parse(conf)
	if err != nil {
		t.Fatal(err.Error())
	}
	buff := bytes.NewBufferString("")
	tmpl.Execute(buff, param)

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

func NewAgentAnalyzerWithConfig(t *testing.T, conf string, s storage.Storage) *HelperAgentAnalyzer {
	InitConfig(t, conf)
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

	// waiting for the api endpoint
	for i := 1; i <= 5; i++ {
		url := fmt.Sprintf("http://%s:%d/api", h.Analyzer.HTTPServer.Addr, h.Analyzer.HTTPServer.Port)
		_, err := http.Get(url)
		if err == nil {
			return
		}
		time.Sleep(time.Second)
	}

	h.t.Fatal("Fail to start the analyzer")
	return
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
			h.Agent.FlowProbeBundle.Flush()
			time.Sleep(500 * time.Millisecond)
			h.Analyzer.Flush()
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

func StartAgent() *agent.Agent {
	agent := NewAgent()
	agent.Start()
	return agent
}

func StartAgentWithConfig(t *testing.T, conf string) *agent.Agent {
	InitConfig(t, conf)
	return StartAgent()
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
