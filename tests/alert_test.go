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

package tests

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/hydrogen18/stoppableListener"
	"github.com/skydive-project/skydive/alert"
	"github.com/skydive-project/skydive/api"
	"github.com/skydive-project/skydive/config"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/tests/helper"
	"github.com/skydive-project/skydive/topology/graph"
)

func checkMessage(t *testing.T, b []byte, al *api.Alert) (bool, error) {
	var alertMsg alert.AlertMessage
	if err := json.Unmarshal(b, &alertMsg); err == nil {
		if alertMsg.UUID == al.UUID {
			var nodes []*graph.Node
			switch arr := alertMsg.ReasonData.(type) {
			case []interface{}:
				for _, obj := range arr {
					n := new(graph.Node)
					if err := n.Decode(obj); err != nil {
						return false, err
					}
					nodes = append(nodes, n)
				}
			}

			if len(nodes) > 0 && nodes[0].Metadata()["Name"].(string) == "alert-ns" {
				return true, nil
			}
		}
	}
	return false, nil
}

func TestAlertWebhook(t *testing.T) {
	aa := helper.NewAgentAnalyzerWithConfig(t, confAgentAnalyzer, NewTestStorage())
	aa.Start()
	defer aa.Stop()

	client, err := api.NewCrudClientFromConfig(&shttp.AuthenticationOpts{})
	if err != nil {
		t.Fatal(err.Error())
	}

	al := api.NewAlert()
	al.Expression = `Gremlin("G.V().Has('Name', 'alert-ns', 'Type', 'netns')")`
	al.Action = "http://localhost:8080/"

	if err := client.Create("alert", al); err != nil {
		t.Fatalf("Failed to create alert: %s", err.Error())
	}
	defer client.Delete("alert", al.ID())

	testPassed := false
	var sl *stoppableListener.StoppableListener
	var wg sync.WaitGroup
	ListenAndServe := func(addr string, port int) {
		listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", addr, port))
		if err != nil {
			t.Fatalf("Failed to listen on %s:%d: %s", addr, port, err.Error())
		}

		sl, err = stoppableListener.New(listener)
		if err != nil {
			t.Fatalf("Failed to create stoppable listener: %s", err.Error())
		}

		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			wg.Add(1)
			defer wg.Done()

			if r.Method == "POST" {
				b, _ := ioutil.ReadAll(r.Body)
				testPassed, err = checkMessage(t, b, al)
				if !testPassed {
					t.Errorf("Wrong message %+v (error: %+v)", string(b), err)
				}
			}
		})

		go func() {
			http.Serve(sl, nil)
			wg.Done()
		}()
	}

	wg.Add(1)
	ListenAndServe("localhost", 8080)

	setupCmds := []helper.Cmd{
		{"ip netns add alert-ns", true},
	}

	tearDownCmds := []helper.Cmd{
		{"ip netns del alert-ns", true},
	}

	helper.ExecCmds(t, setupCmds...)
	defer helper.ExecCmds(t, tearDownCmds...)

	time.Sleep(3 * time.Second)

	sl.Close()
	wg.Wait()

	if !testPassed {
		t.Error("No alert was triggered")
	}
}

func TestAlertScript(t *testing.T) {
	cookie, err := ioutil.TempFile("", "test-alert-script")
	if err == nil {
		err = os.Remove(cookie.Name())
	}

	if err != nil {
		t.Fatalf(err.Error())
		return
	}

	tmpfile, err := ioutil.TempFile("", "example")
	if err == nil {
		if _, err = tmpfile.Write([]byte(fmt.Sprintf("#!/bin/sh\ncat > %s", cookie.Name()))); err == nil {
			err = os.Chmod(tmpfile.Name(), 0755)
		}
	}

	if err != nil {
		t.Fatalf(err.Error())
		return
	}

	tmpfile.Close()
	defer os.Remove(tmpfile.Name())

	aa := helper.NewAgentAnalyzerWithConfig(t, confAgentAnalyzer, NewTestStorage())
	aa.Start()
	defer aa.Stop()

	client, err := api.NewCrudClientFromConfig(&shttp.AuthenticationOpts{})
	if err != nil {
		t.Fatal(err.Error())
	}

	al := api.NewAlert()
	al.Expression = "G.V().Has('Name', 'alert-ns', 'Type', 'netns')"
	al.Action = "file://" + tmpfile.Name()

	if err := client.Create("alert", al); err != nil {
		t.Fatalf("Failed to create alert: %s", err.Error())
	}
	defer client.Delete("alert", al.ID())

	setupCmds := []helper.Cmd{
		{"ip netns add alert-ns", true},
	}

	tearDownCmds := []helper.Cmd{
		{"ip netns del alert-ns", true},
	}

	helper.ExecCmds(t, setupCmds...)
	defer helper.ExecCmds(t, tearDownCmds...)

	time.Sleep(3 * time.Second)

	if _, err := os.Stat(cookie.Name()); err != nil {
		t.Error("No alert was triggered")
	}

	b, err := ioutil.ReadFile(cookie.Name())
	if err != nil {
		t.Error("No alert was triggered")
		return
	}

	testPassed, err := checkMessage(t, b, al)
	if !testPassed {
		t.Errorf("Wrong message %+v (error: %+v)", string(b), err)
	}
}

func TestAlertWithTimer(t *testing.T) {
	aa := helper.NewAgentAnalyzerWithConfig(t, confAgentAnalyzer, NewTestStorage())
	aa.Start()
	defer aa.Stop()

	client, err := api.NewCrudClientFromConfig(&shttp.AuthenticationOpts{})
	if err != nil {
		t.Fatal(err.Error())
	}

	ws, err := helper.WSConnect(config.GetConfig().GetString("analyzer.listen"), 5, func(*websocket.Conn) {
		helper.ExecCmds(t, helper.Cmd{Cmd: "ip netns add alert-ns", Check: true})
	})
	defer helper.ExecCmds(t, helper.Cmd{Cmd: "ip netns del alert-ns", Check: true})

	if err != nil {
		t.Fatal(err.Error())
	}

	al := api.NewAlert()
	al.Expression = "G.V().Has('Name', 'alert-ns', 'Type', 'netns')"
	al.Trigger = "duration:+1s"

	if err := client.Create("alert", al); err != nil {
		t.Fatalf("Failed to create alert: %s", err.Error())
	}
	defer client.Delete("alert", al.ID())

	time.Sleep(3 * time.Second)

	for {
		_, m, err := ws.ReadMessage()
		if err != nil {
			break
		}

		var msg shttp.WSMessage
		if err = json.Unmarshal(m, &msg); err != nil {
			t.Fatalf("Failed to unmarshal message: %s", err.Error())
		}

		if msg.Namespace != "Alert" {
			continue
		}

		testPassed, err := checkMessage(t, []byte(*msg.Obj), al)
		if !testPassed {
			t.Errorf("Wrong alert message: %+v (error: %+v)", string([]byte(*msg.Obj)), err)
		}

		break
	}
}
