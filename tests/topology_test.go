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
	"errors"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/redhat-cip/skydive/agent"
	"github.com/redhat-cip/skydive/config"
	"github.com/redhat-cip/skydive/logging"
	"github.com/redhat-cip/skydive/topology/graph"
)

// FIX(safchain) has to be removed when will be able to stop agent
var globalAgent *agent.Agent

func newClient() (*websocket.Conn, error) {
	conn, err := net.Dial("tcp", "127.0.0.1:8081")
	if err != nil {
		return nil, err
	}

	endpoint := "ws://127.0.0.1:8081/ws/graph"
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

func connectToAgent(timeout int, onReady func(*websocket.Conn)) (*websocket.Conn, error) {
	var ws *websocket.Conn
	var err error

	t := 0
	for {
		if t > timeout {
			return nil, errors.New("Connection timeout reached")
		}

		ws, err = newClient()
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

func processGraphMessage(g *graph.Graph, m []byte) error {
	g.Lock()
	defer g.Unlock()

	msg, err := graph.UnmarshalWSMessage(m)
	if err != nil {
		return err
	}

	switch msg.Type {
	case "NodeUpdated":
		n := msg.Obj.(*graph.Node)
		node := g.GetNode(n.ID)
		if node != nil {
			g.SetMetadatas(node, n.Metadatas())
		}
	case "NodeDeleted":
		g.DelNode(msg.Obj.(*graph.Node))
	case "NodeAdded":
		n := msg.Obj.(*graph.Node)
		if g.GetNode(n.ID) == nil {
			g.AddNode(n)
		}
	case "EdgeUpdated":
		e := msg.Obj.(*graph.Edge)
		edge := g.GetEdge(e.ID)
		if edge != nil {
			g.SetMetadatas(edge, e.Metadatas())
		}
	case "EdgeDeleted":
		g.DelEdge(msg.Obj.(*graph.Edge))
	case "EdgeAdded":
		e := msg.Obj.(*graph.Edge)
		if g.GetEdge(e.ID) == nil {
			g.AddEdge(e)
		}
	}

	return nil
}

func startAgent(t *testing.T) {
	// FIX(safchain) has to be removed see comment around the variable declaration
	if globalAgent != nil {
		return
	}

	config.InitEmptyConfig()

	section, err := config.GetConfig().NewSection("default")
	if err != nil {
		t.Fatal(err.Error())
	}
	section.NewKey("ws_pong_timeout", "1")

	section, err = config.GetConfig().NewSection("agent")
	if err != nil {
		t.Fatal(err.Error())
	}
	section.NewKey("listen", "8081")
	section.NewKey("flowtable_expire", "5")

	section, err = config.GetConfig().NewSection("cache")
	if err != nil {
		t.Fatal(err.Error())
	}
	section.NewKey("expire", "300")
	section.NewKey("cleanup", "30")

	section, err = config.GetConfig().NewSection("sflow")
	if err != nil {
		t.Fatal(err.Error())
	}
	section.NewKey("listen", "5000")

	section, err = config.GetConfig().NewSection("ovs")
	if err != nil {
		t.Fatal(err.Error())
	}
	section.NewKey("ovsdb", "6400")

	globalAgent = agent.NewAgent()
	go globalAgent.Start()
}

func startTopologyClient(t *testing.T, g *graph.Graph, onReady func(*websocket.Conn), onChange func()) error {
	// ready when got a first ping
	ws, err := connectToAgent(5, onReady)
	if err != nil {
		return err
	}

	for {
		_, m, err := ws.ReadMessage()
		if err != nil {
			break
		}

		err = processGraphMessage(g, m)
		if err != nil {
			return err
		}

		logging.GetLogger().Debug("%s", string(m))
		logging.GetLogger().Debug("%s", g.String())

		onChange()
	}

	return nil
}

func testTopology(t *testing.T, g *graph.Graph, cmds []string, onChange func(ws *websocket.Conn)) {
	var ws *websocket.Conn

	cmdIndex := 0
	or := func(w *websocket.Conn) {
		ws = w

		// ready to exec the first cmd
		err := exec.Command("sudo", strings.Split(cmds[cmdIndex], " ")...).Run()
		if err != nil {
			t.Fatal(err.Error())
		}
		cmdIndex++
	}

	oc := func() {
		onChange(ws)

		// exec the following command
		if cmdIndex < len(cmds) {
			err := exec.Command("sudo", strings.Split(cmds[cmdIndex], " ")...).Run()
			if err != nil {
				t.Fatal(err.Error())
			}
			cmdIndex++
		}
	}

	err := startTopologyClient(t, g, or, oc)
	if err != nil {
		t.Fatal(err.Error())
	}
}

func testCleanup(t *testing.T, g *graph.Graph, cmds []string) {
	// cleanup side on the test
	testPassed := false
	onChange := func(ws *websocket.Conn) {
		g.Lock()
		defer g.Unlock()

		if !testPassed && len(g.GetNodes()) == 0 && len(g.GetEdges()) == 0 {
			testPassed = true

			ws.Close()
		}
	}

	testTopology(t, g, cmds, onChange)
	if !testPassed {
		t.Error("test not executed")
	}
}

func newGraph(t *testing.T) *graph.Graph {
	backend, err := graph.NewMemoryBackend()
	if err != nil {
		t.Fatal(err.Error())
	}

	g, err := graph.NewGraph(backend)
	if err != nil {
		t.Fatal(err.Error())
	}

	return g
}

func TestBridgeOVS(t *testing.T) {
	g := newGraph(t)

	startAgent(t)

	setupCmds := []string{
		"ovs-vsctl add-br br-test1",
	}

	tearDownCmds := []string{
		"ovs-vsctl del-br br-test1",
	}

	testPassed := false
	onChange := func(ws *websocket.Conn) {
		g.Lock()
		defer g.Unlock()

		if !testPassed && len(g.GetNodes()) >= 3 && len(g.GetEdges()) >= 2 {
			ovsbridge := g.LookupFirstNode(graph.Metadatas{"Type": "ovsbridge", "Name": "br-test1"})
			if ovsbridge == nil {
				t.Error("ovs bridge not found")
			}
			ovsports := g.LookupChildren(ovsbridge, graph.Metadatas{"Type": "ovsport"})
			if len(ovsports) != 1 {
				t.Error("ovs port not found or not unique")
			}
			devices := g.LookupChildren(ovsports[0], graph.Metadatas{"Type": "internal", "Driver": "openvswitch"})
			if len(devices) != 1 {
				t.Error("device not found or not unique")
			}

			if ovsbridge.Metadatas()["Host"] == "" || ovsports[0].Metadatas()["Host"] == "" || devices[0].Metadatas()["Host"] == "" {
				t.Error("host binding not found")
			}

			testPassed = true

			ws.Close()
		}
	}

	testTopology(t, g, setupCmds, onChange)
	if !testPassed {
		t.Error("test not executed")
	}

	testCleanup(t, g, tearDownCmds)
}

func TestPatchOVS(t *testing.T) {
	g := newGraph(t)

	startAgent(t)

	setupCmds := []string{
		"ovs-vsctl add-br br-test1",
		"ovs-vsctl add-br br-test2",
		"ovs-vsctl add-port br-test1 patch-br-test2 -- set interface patch-br-test2 type=patch",
		"ovs-vsctl add-port br-test2 patch-br-test1 -- set interface patch-br-test1 type=patch",
		"ovs-vsctl set interface patch-br-test2 option:peer=patch-br-test1",
		"ovs-vsctl set interface patch-br-test1 option:peer=patch-br-test2",
	}

	tearDownCmds := []string{
		"ovs-vsctl del-br br-test1",
		"ovs-vsctl del-br br-test2",
	}

	testPassed := false
	onChange := func(ws *websocket.Conn) {
		g.Lock()
		defer g.Unlock()

		if !testPassed && len(g.GetNodes()) >= 10 && len(g.GetEdges()) >= 9 {
			patch1 := g.LookupFirstNode(graph.Metadatas{"Type": "patch", "Name": "patch-br-test1", "Driver": "openvswitch"})
			if patch1 == nil {
				t.Error("patch not found")
			}

			patch2 := g.LookupFirstNode(graph.Metadatas{"Type": "patch", "Name": "patch-br-test2", "Driver": "openvswitch"})
			if patch2 == nil {
				t.Error("patch not found")
			}

			if !g.AreLinked(patch1, patch2) {
				t.Error("patch interfaces not linked")
			}

			testPassed = true

			ws.Close()
		}
	}

	testTopology(t, g, setupCmds, onChange)
	if !testPassed {
		t.Error("test not executed")
	}

	testCleanup(t, g, tearDownCmds)
}

func TestInterfaceOVS(t *testing.T) {
	g := newGraph(t)

	startAgent(t)

	setupCmds := []string{
		"ovs-vsctl add-br br-test1",
		"ovs-vsctl add-port br-test1 intf1 -- set interface intf1 type=internal",
	}

	tearDownCmds := []string{
		"ovs-vsctl del-br br-test1",
	}

	testPassed := false
	onChange := func(ws *websocket.Conn) {
		g.Lock()
		defer g.Unlock()

		if !testPassed && len(g.GetNodes()) >= 5 && len(g.GetEdges()) >= 4 {
			intf := g.LookupFirstNode(graph.Metadatas{"Type": "internal", "Name": "intf1", "Driver": "openvswitch"})
			if intf != nil {
				if _, ok := intf.Metadatas()["UUID"]; ok {
					// check we don't have another interface potentially added by netlink
					// should only have ovsport and interface
					others := g.LookupNodes(graph.Metadatas{"Name": "intf1"})
					if len(others) > 2 {
						t.Error("found more interface than expected")
					}

					if _, ok := intf.Metadatas()["MAC"]; !ok {
						t.Error("mac not found")
					}

					testPassed = true

					ws.Close()
				}
			}
		}
	}

	testTopology(t, g, setupCmds, onChange)
	if !testPassed {
		t.Error("test not executed")
	}

	testCleanup(t, g, tearDownCmds)
}

func TestBondOVS(t *testing.T) {
	g := newGraph(t)

	startAgent(t)

	setupCmds := []string{
		"ovs-vsctl add-br br-test1",
		"ip tuntap add mode tap dev intf1",
		"ip tuntap add mode tap dev intf2",
		"ovs-vsctl add-bond br-test1 bond0 intf1 intf2",
	}

	tearDownCmds := []string{
		"ovs-vsctl del-br br-test1",
		"ip link del intf1",
		"ip link del intf2",
	}

	testPassed := false
	onChange := func(ws *websocket.Conn) {
		g.Lock()
		defer g.Unlock()

		if !testPassed && len(g.GetNodes()) >= 6 && len(g.GetEdges()) >= 5 {
			bond := g.LookupFirstNode(graph.Metadatas{"Type": "ovsport", "Name": "bond0"})
			if bond != nil {
				intfs := g.LookupChildren(bond, nil)
				if len(intfs) != 2 {
					t.Error("bond interfaces not found")
				}

				testPassed = true

				ws.Close()
			}

		}
	}

	testTopology(t, g, setupCmds, onChange)
	if !testPassed {
		t.Error("test not executed")
	}

	testCleanup(t, g, tearDownCmds)
}
