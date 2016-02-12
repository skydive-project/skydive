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
	"fmt"
	"io/ioutil"
	"net"
	"os/exec"
	"strings"
	"testing"

	"github.com/gorilla/mux"

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

	var logLevel string
	if testing.Verbose() {
		logLevel = "DEBUG"
	} else {
		logLevel = "INFO"
	}

	conf += fmt.Sprintf("\nlogging:\n  default: %s", logLevel)
	f.WriteString(conf)
	f.Close()

	err = config.InitConfigFromFile(f.Name())
	if err != nil {
		t.Fatal(err.Error())
	}

	err = logging.InitLogger()
	if err != nil {
		t.Fatal(err)
	}
}

func StartAgent() *agent.Agent {
	agent := agent.NewAgent()
	go agent.Start()
	return agent
}

func StartAgentWithConfig(t *testing.T, conf string) *agent.Agent {
	InitConfig(t, conf)

	return StartAgent()
}

func StartAgentAndAnalyzerWithConfig(t *testing.T, conf string, s storage.Storage) (*agent.Agent, *analyzer.Server) {
	InitConfig(t, conf)

	router := mux.NewRouter().StrictSlash(true)
	server, err := analyzer.NewServerFromConfig(router)
	if err != nil {
		t.Fatal(err)
	}

	server.SetStorage(s)

	go server.ListenAndServe()

	return StartAgent(), server
}

func ReplayTraceHelper(t *testing.T, trace string, target string) {
	t.Log("Replaying", trace)
	out, err := exec.Command("go", "run", "../cmd/pcap2sflow-replay/pcap2sflow-replay.go", "-trace", trace, target).CombinedOutput()
	if err != nil {
		t.Error(err.Error() + "\n" + string(out))
	}
	t.Log("Stdout/Stderr ", string(out))
}

func ExecCmds(t *testing.T, cmds ...Cmd) {
	for _, cmd := range cmds {
		err := exec.Command("sudo", strings.Split(cmd.Cmd, " ")...).Run()
		if err != nil && cmd.Check {
			t.Fatal("cmd : (sudo " + cmd.Cmd + ") " + err.Error())
		}
	}
}
