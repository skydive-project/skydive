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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hydrogen18/stoppableListener"
	"github.com/skydive-project/skydive/alert"
	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/topology/graph"
	ws "github.com/skydive-project/skydive/websocket"
)

var alertLock sync.Mutex

func checkMessage(t *testing.T, b []byte, al *types.Alert, nsName string) (bool, error) {
	alertLock.Lock()
	defer alertLock.Unlock()

	var alertMsg alert.Message
	if err := common.JSONDecode(bytes.NewReader(b), &alertMsg); err == nil {
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

			if len(nodes) > 0 {
				if name, _ := nodes[0].GetFieldString("Name"); name == nsName {
					return true, nil
				}
			}
		}
	}
	return false, nil
}

func wsClose(ws *websocket.Conn) error {
	if err := ws.WriteControl(websocket.CloseMessage, nil, time.Now().Add(3*time.Second)); err != nil {
		return err
	}
	return ws.Close()
}

func decodeStructMessageJSON(b []byte) *ws.StructMessage {
	mJSON := ws.StructMessageJSON{}
	if err := json.Unmarshal(b, &mJSON); err != nil {
		return nil
	}
	msg := &ws.StructMessage{
		Protocol:  ws.JSONProtocol,
		Namespace: mJSON.Namespace,
		Type:      mJSON.Type,
		UUID:      mJSON.UUID,
		Status:    mJSON.Status,
		JSONObj:   mJSON.Obj,
	}
	return msg
}

func newClient(endpoint string) (*websocket.Conn, error) {
	conn, err := net.Dial("tcp", endpoint)
	if err != nil {
		return nil, err
	}

	scheme := "ws"
	if config.IsTLSEnabled() == true {
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

func connect(endpoint string, timeout int, onReady func(*websocket.Conn)) (*websocket.Conn, error) {
	var ws *websocket.Conn
	var err error

	t := 0
	for {
		if t > timeout {
			return nil, errors.New("Connection to Agent : timeout reached")
		}

		ws, err = newClient(endpoint)
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

func TestAlertWebhook(t *testing.T) {
	var (
		err        error
		al         *types.Alert
		sl         *stoppableListener.StoppableListener
		wg         sync.WaitGroup
		testPassed atomic.Value
	)

	testPassed.Store(false)

	agent1IP := os.Getenv("AGENT1_IP")
	if agent1IP == "" {
		agent1IP = "localhost"
	}

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
				result, _ := checkMessage(t, b, al, "alert-ns-webhook")
				testPassed.Store(result)
			}
		})

		go func() {
			http.Serve(sl, nil)
			wg.Done()
		}()
	}

	test := &Test{
		setupCmds: []Cmd{
			{"ip netns add alert-ns-webhook", true},
		},

		setupFunction: func(c *TestContext) error {
			wg.Add(1)
			ListenAndServe(agent1IP, 8080)

			alertLock.Lock()
			defer alertLock.Unlock()

			al = types.NewAlert()
			al.Expression = "G.V().Has('Name', 'alert-ns-webhook', 'Type', 'netns')"
			al.Action = fmt.Sprintf("http://%s:8080/", agent1IP)

			if err = c.client.Create("alert", al); err != nil {
				return fmt.Errorf("Failed to create alert: %s", err.Error())
			}

			return nil
		},

		tearDownCmds: []Cmd{
			{"ip netns del alert-ns-webhook", true},
		},

		tearDownFunction: func(c *TestContext) error {
			sl.Close()
			wg.Wait()

			return c.client.Delete("alert", al.ID())
		},

		checks: []CheckFunction{func(c *CheckContext) error {
			if testPassed.Load() == false {
				if err != nil {
					return err
				}
				return errors.New("Webhook was not triggered")
			}
			return nil
		}},
	}

	RunTest(t, test)
}

func TestAlertScript(t *testing.T) {
	if agentTestsOnly {
		t.Skip("this test works only when agent and analyzers are on the same host")
	}

	var (
		err        error
		al         *types.Alert
		testPassed = false
	)

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

	test := &Test{
		setupCmds: []Cmd{
			{"ip netns add alert-ns-script", true},
		},

		setupFunction: func(c *TestContext) error {
			al = types.NewAlert()
			al.Expression = "G.V().Has('Name', 'alert-ns-script', 'Type', 'netns')"
			al.Action = "file://" + tmpfile.Name()

			if err = c.client.Create("alert", al); err != nil {
				return fmt.Errorf("Failed to create alert: %s", err.Error())
			}

			return nil
		},

		tearDownCmds: []Cmd{
			{"ip netns del alert-ns-script", true},
		},

		tearDownFunction: func(c *TestContext) error {
			return c.client.Delete("alert", al.ID())
		},

		checks: []CheckFunction{func(c *CheckContext) error {
			if _, err := os.Stat(cookie.Name()); err != nil {
				return errors.New("No alert was triggered")
			}

			b, err := ioutil.ReadFile(cookie.Name())
			if err != nil {
				return errors.New("No alert was triggered")
			}

			testPassed, err = checkMessage(t, b, al, "alert-ns-script")
			if !testPassed {
				return fmt.Errorf("Wrong message %+v (error: %+v)", string(b), err)
			}

			return nil
		}},
	}

	RunTest(t, test)
}

func TestAlertWithTimer(t *testing.T) {
	var (
		err error
		ws  *websocket.Conn
		al  *types.Alert
	)

	test := &Test{
		retries: 1,

		setupCmds: []Cmd{
			{"ip netns add alert-ns-timer", true},
		},

		setupFunction: func(c *TestContext) error {
			ws, err = connect(config.GetStringSlice("analyzers")[0], 5, nil)
			if err != nil {
				return err
			}

			al = types.NewAlert()
			al.Expression = "G.V().Has('Name', 'alert-ns-timer', 'Type', 'netns')"
			al.Trigger = "duration:+1s"

			if err = c.client.Create("alert", al); err != nil {
				return fmt.Errorf("Failed to create alert: %s", err.Error())
			}

			return nil
		},

		tearDownCmds: []Cmd{
			{"ip netns del alert-ns-timer", true},
		},

		tearDownFunction: func(c *TestContext) error {
			wsClose(ws)
			return c.client.Delete("alert", al.ID())
		},

		checks: []CheckFunction{func(c *CheckContext) error {
			for {
				_, m, err := ws.ReadMessage()
				if err != nil {
					return err
				}

				msg := decodeStructMessageJSON(m)
				if msg == nil {
					t.Fatal("Failed to unmarshal message")
				}
				if msg.Namespace != "Alert" {
					continue
				}

				testPassed, err := checkMessage(t, []byte(*msg.JSONObj), al, "alert-ns-timer")
				if err != nil {
					return err
				}

				if !testPassed {
					return fmt.Errorf("Wrong alert message: %+v (error: %+v)", string(*msg.JSONObj), err)
				}

				break
			}

			return nil
		}},
	}

	RunTest(t, test)
}

func TestMultipleTriggering(t *testing.T) {
	var (
		err error
		ws  *websocket.Conn
		al  *types.Alert
	)

	test := &Test{
		setupCmds: []Cmd{
			{"ip netns add alert-lo-down", true},
		},

		setupFunction: func(c *TestContext) error {
			ws, err = connect(config.GetStringSlice("analyzers")[0], 5, nil)
			if err != nil {
				return err
			}

			al = types.NewAlert()
			al.Expression = "G.V().Has('Name', 'alert-lo-down', 'Type', 'netns').Out('Name','lo').Values('State')"

			if err = c.client.Create("alert", al); err != nil {
				return fmt.Errorf("Failed to create alert: %s", err.Error())
			}
			t.Logf("alert created with UUID : %s", al.UUID)

			return nil
		},

		tearDownCmds: []Cmd{
			{"ip netns del alert-lo-down", true},
		},

		tearDownFunction: func(c *TestContext) error {
			wsClose(ws)
			return c.client.Delete("alert", al.ID())
		},

		retries: 1,

		checks: []CheckFunction{func(c *CheckContext) error {
			alertNumber := 0
			cmd := []Cmd{
				{"ip netns exec alert-lo-down ip l set lo up", true},
			}
			downLo := []Cmd{
				{"ip netns exec alert-lo-down ip l set lo down", true},
			}
			for alertNumber < 2 {
				_, m, err := ws.ReadMessage()
				if err != nil {
					return err
				}

				msg := decodeStructMessageJSON(m)
				if msg == nil {
					t.Fatal("Failed to unmarshal message")
				}

				if msg.Namespace != "Alert" {
					continue
				}

				var alertMsg alert.Message
				if err := msg.DecodeObj(&alertMsg); err != nil {
					t.Fatalf("Failed to unmarshal alert : %s", err.Error())
				}

				t.Logf("ws msg received with namespace %s and alertMsg UUID %s", msg.Namespace, alertMsg.UUID)
				if alertMsg.UUID != al.UUID {
					continue
				}
				alertNumber++
				execCmds(t, cmd...)
				cmd = downLo
			}

			return nil
		}},
	}

	RunTest(t, test)
}
