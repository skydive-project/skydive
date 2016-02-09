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
	"io/ioutil"
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

const conf = `
[default]
ws_pong_timeout = 1

[agent]
listen = 8081
flowtable_expire = 5

[cache]
expire = 300
cleanup = 30

[sflow]
listen = 5000

[ovs]
ovsdb = 6400
`

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

func initConfig(t *testing.T) {
	f, err := ioutil.TempFile("", "skydive_agent")
	if err != nil {
		t.Fatal(err.Error())
	}

	f.WriteString(conf)
	f.Close()

	err = config.InitConfigFromFile(f.Name())
	if err != nil {
		t.Fatal(err.Error())
	}
}

func startAgent(t *testing.T) {
	// FIX(safchain) has to be removed see comment around the variable declaration
	if globalAgent != nil {
		return
	}

	initConfig(t)

	err := logging.InitLogger()
	if err != nil {
		t.Fatal(err)
	}

	globalAgent = agent.NewAgent()
	go globalAgent.Start()
}

func startTopologyClient(t *testing.T, g *graph.Graph, onReady func(*websocket.Conn), onChange func(*websocket.Conn)) error {
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

		onChange(ws)
	}

	return nil
}

func testTopology(t *testing.T, g *graph.Graph, cmds []string, onChange func(ws *websocket.Conn)) {
	cmdIndex := 0
	or := func(w *websocket.Conn) {
		// ready to exec the first cmd
		if cmdIndex < len(cmds) {
			err := exec.Command("sudo", strings.Split(cmds[cmdIndex], " ")...).Run()
			if err != nil {
				t.Fatal("cmd : (sudo " + cmds[cmdIndex] + ") " + err.Error())
			}
			cmdIndex++
		}
	}

	oc := func(ws *websocket.Conn) {
		onChange(ws)

		// exec the following command
		if cmdIndex < len(cmds) {
			err := exec.Command("sudo", strings.Split(cmds[cmdIndex], " ")...).Run()
			if err != nil {
				t.Fatal("cmd : (sudo " + cmds[cmdIndex] + ") " + err.Error())
			}
			cmdIndex++
		}
	}

	err := startTopologyClient(t, g, or, oc)
	if err != nil {
		t.Fatal(err.Error())
	}
}

func testCleanup(t *testing.T, g *graph.Graph, cmds []string, ints []string) {
	// cleanup side on the test
	testPassed := false
	onChange := func(ws *websocket.Conn) {
		g.Lock()
		defer g.Unlock()

		if !testPassed {
			clean := true
			for _, intf := range ints {
				n := g.LookupFirstNode(graph.Metadatas{"Name": intf})
				if n != nil {
					clean = false
					break
				}
			}

			if clean {
				testPassed = true

				ws.Close()
			}
		}
	}

	testTopology(t, g, cmds, onChange)
	if !testPassed {
		t.Error("test not executed or failed")
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
				return
			}
			ovsports := g.LookupChildren(ovsbridge, graph.Metadatas{"Type": "ovsport"})
			if len(ovsports) != 1 {
				return
			}
			devices := g.LookupChildren(ovsports[0], graph.Metadatas{"Type": "internal", "Driver": "openvswitch"})
			if len(devices) != 1 {
				return
			}

			if ovsbridge.Metadatas()["Host"] == "" || ovsports[0].Metadatas()["Host"] == "" || devices[0].Metadatas()["Host"] == "" {
				return
			}

			testPassed = true

			ws.Close()
		}
	}

	testTopology(t, g, setupCmds, onChange)
	if !testPassed {
		t.Error("test not executed or failed")
	}

	testCleanup(t, g, tearDownCmds, []string{"br-test1"})
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
				return
			}

			patch2 := g.LookupFirstNode(graph.Metadatas{"Type": "patch", "Name": "patch-br-test2", "Driver": "openvswitch"})
			if patch2 == nil {
				return
			}

			if !g.AreLinked(patch1, patch2) {
				return
			}

			testPassed = true

			ws.Close()
		}
	}

	testTopology(t, g, setupCmds, onChange)
	if !testPassed {
		t.Error("test not executed or failed")
	}

	testCleanup(t, g, tearDownCmds, []string{"br-test1", "br-test2", "patch-br-test1", "patch-br-test2"})
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
						return
					}

					if _, ok := intf.Metadatas()["MAC"]; !ok {
						return
					}

					testPassed = true

					ws.Close()
				}
			}
		}
	}

	testTopology(t, g, setupCmds, onChange)
	if !testPassed {
		t.Error("test not executed or failed")
	}

	testCleanup(t, g, tearDownCmds, []string{"br-test1", "intf1"})
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
					return
				}

				testPassed = true

				ws.Close()
			}
		}
	}

	testTopology(t, g, setupCmds, onChange)
	if !testPassed {
		t.Error("test not executed or failed")
	}

	testCleanup(t, g, tearDownCmds, []string{"br-test1", "intf1", "intf2"})
}

func TestVeth(t *testing.T) {
	g := newGraph(t)

	startAgent(t)

	setupCmds := []string{
		"ip l add vm1-veth0 type veth peer name vm1-veth1",
	}

	tearDownCmds := []string{
		"ip link del vm1-veth0",
	}

	testPassed := false
	onChange := func(ws *websocket.Conn) {
		g.Lock()
		defer g.Unlock()

		if !testPassed && len(g.GetNodes()) >= 2 && len(g.GetEdges()) >= 1 {
			nodes := g.LookupNodes(graph.Metadatas{"Type": "veth"})
			if len(nodes) == 2 {
				if g.AreLinked(nodes[0], nodes[1]) {
					testPassed = true

					ws.Close()
				}
			}
		}
	}

	testTopology(t, g, setupCmds, onChange)
	if !testPassed {
		t.Error("test not executed or failed")
	}

	testCleanup(t, g, tearDownCmds, []string{"vm1-veth0", "vm1-veth1"})
}

func TestBridge(t *testing.T) {
	g := newGraph(t)

	startAgent(t)

	setupCmds := []string{
		"brctl addbr br-test",
		"ip tuntap add mode tap dev intf1",
		"brctl addif br-test intf1",
	}

	tearDownCmds := []string{
		"brctl delbr br-test",
		"ip link del intf1",
	}

	testPassed := false
	onChange := func(ws *websocket.Conn) {
		g.Lock()
		defer g.Unlock()

		if !testPassed && len(g.GetNodes()) >= 2 && len(g.GetEdges()) >= 1 {
			bridge := g.LookupFirstNode(graph.Metadatas{"Type": "bridge", "Name": "br-test"})
			if bridge != nil {
				nodes := g.LookupChildren(bridge, graph.Metadatas{"Name": "intf1"})
				if len(nodes) == 1 {
					testPassed = true

					ws.Close()
				}
			}

		}
	}

	testTopology(t, g, setupCmds, onChange)
	if !testPassed {
		t.Error("test not executed or failed")
	}

	testCleanup(t, g, tearDownCmds, []string{"br-test", "intf1"})
}
