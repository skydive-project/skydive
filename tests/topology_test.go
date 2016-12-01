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
	"encoding/json"
	"testing"

	"github.com/gorilla/websocket"

	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/tests/helper"
	"github.com/skydive-project/skydive/topology/graph"
	"github.com/skydive-project/skydive/topology/graph/traversal"
)

const confTopology = `---
ws_pong_timeout: 15

agent:
  listen: :58081
  topology:
    probes:
      - netlink
      - netns
      - ovsdb
      - docker

cache:
  expire: 300
  cleanup: 30

sflow:
  listen: :55000

etcd:
  embedded: {{.EmbeddedEtcd}}
  port: 2374
  data_dir: /tmp
  servers: {{.EtcdServer}}

logging:
  default: {{.LogLevel}}
`

func processGraphMessage(g *graph.Graph, m []byte) error {
	g.Lock()
	defer g.Unlock()

	var msg shttp.WSMessage
	if err := json.Unmarshal(m, &msg); err != nil {
		return err
	}

	if msg.Namespace != "Graph" {
		return nil
	}

	msgType, obj, err := graph.UnmarshalWSMessage(msg)
	if err != nil {
		return err
	}

	switch msgType {
	case "NodeUpdated":
		n := obj.(*graph.Node)
		node := g.GetNode(n.ID)
		if node != nil {
			g.SetMetadata(node, n.Metadata())
		}
	case "NodeDeleted":
		g.DelNode(obj.(*graph.Node))
	case "NodeAdded":
		n := obj.(*graph.Node)
		if g.GetNode(n.ID) == nil {
			g.AddNode(n)
		}
	case "EdgeUpdated":
		e := obj.(*graph.Edge)
		edge := g.GetEdge(e.ID)
		if edge != nil {
			g.SetMetadata(edge, e.Metadata())
		}
	case "EdgeDeleted":
		g.DelEdge(obj.(*graph.Edge))
	case "EdgeAdded":
		e := obj.(*graph.Edge)
		if g.GetEdge(e.ID) == nil {
			g.AddEdge(e)
		}
	}

	return nil
}

func startTopologyClient(t *testing.T, g *graph.Graph, onReady func(*websocket.Conn), onChange func(*websocket.Conn)) error {
	// ready when got a first ping
	ws, err := helper.WSConnect("localhost:58081", 5, onReady)
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

		logging.GetLogger().Debugf("%s", string(m))
		logging.GetLogger().Debugf("%s", g.String())

		onChange(ws)
	}

	return nil
}

func testTopology(t *testing.T, g *graph.Graph, cmds []helper.Cmd, onChange func(ws *websocket.Conn)) {
	cmdIndex := 0
	cmdChan := make(chan helper.Cmd, len(cmds))
	defer close(cmdChan)

	go func() {
		for cmd := range cmdChan {
			helper.ExecCmds(t, cmd)
		}
	}()

	or := func(w *websocket.Conn) {
		// ready to exec the first cmd
		if cmdIndex < len(cmds) {
			cmdChan <- cmds[cmdIndex]
			cmdIndex++
		}
	}

	oc := func(ws *websocket.Conn) {
		onChange(ws)

		// exec the following command
		if cmdIndex < len(cmds) {
			cmdChan <- cmds[cmdIndex]
			cmdIndex++
		}
	}

	err := startTopologyClient(t, g, or, oc)
	if err != nil {
		t.Fatal(err.Error())
	}
}

func testCleanup(t *testing.T, g *graph.Graph, cmds []helper.Cmd, names []string) {
	// cleanup side on the test
	testPassed := false
	onChange := func(ws *websocket.Conn) {
		g.Lock()
		defer g.Unlock()

		if !testPassed {
			clean := true
			for _, name := range names {
				n := g.LookupFirstNode(graph.Metadata{"Name": name})
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

	helper.CleanGraph(g)
}

func TestBridgeOVS(t *testing.T) {
	g := helper.NewGraph(t)

	agent := helper.StartAgentWithConfig(t, confTopology)
	defer agent.Stop()

	setupCmds := []helper.Cmd{
		{"ovs-vsctl add-br br-test1", true},
	}

	tearDownCmds := []helper.Cmd{
		{"ovs-vsctl del-br br-test1", true},
	}

	tr := traversal.NewGraphTraversal(g)

	testPassed := false
	onChange := func(ws *websocket.Conn) {
		g.Lock()
		defer g.Unlock()

		if !testPassed {
			tv := tr.V().Has("Type", "ovsbridge", "Name", "br-test1")
			tv = tv.Out("Type", "ovsport", "Name", "br-test1")
			tv = tv.Out("Type", "internal", "Name", "br-test1", "Driver", "openvswitch")

			if len(tv.Values()) == 1 {
				testPassed = true
				ws.Close()
			}
		}
	}

	testTopology(t, g, setupCmds, onChange)
	if !testPassed {
		t.Error("test not executed or failed")
	}

	testCleanup(t, g, tearDownCmds, []string{"br-test1"})
}

func TestPatchOVS(t *testing.T) {
	g := helper.NewGraph(t)

	agent := helper.StartAgentWithConfig(t, confTopology)
	defer agent.Stop()

	setupCmds := []helper.Cmd{
		{"ovs-vsctl add-br br-test1", true},
		{"ovs-vsctl add-br br-test2", true},
		{"ovs-vsctl add-port br-test1 patch-br-test2 -- set interface patch-br-test2 type=patch", true},
		{"ovs-vsctl add-port br-test2 patch-br-test1 -- set interface patch-br-test1 type=patch", true},
		{"ovs-vsctl set interface patch-br-test2 option:peer=patch-br-test1", true},
		{"ovs-vsctl set interface patch-br-test1 option:peer=patch-br-test2", true},
	}

	tearDownCmds := []helper.Cmd{
		{"ovs-vsctl del-br br-test1", true},
		{"ovs-vsctl del-br br-test2", true},
	}

	tr := traversal.NewGraphTraversal(g)

	testPassed := false
	onChange := func(ws *websocket.Conn) {
		g.Lock()
		defer g.Unlock()

		if !testPassed {
			tv := tr.V().Has("Type", "patch", "Name", "patch-br-test1", "Driver", "openvswitch")
			tv = tv.Both("Type", "patch", "Name", "patch-br-test2", "Driver", "openvswitch")

			if len(tv.Values()) == 1 {
				testPassed = true
				ws.Close()
			}
		}
	}

	testTopology(t, g, setupCmds, onChange)
	if !testPassed {
		t.Error("test not executed or failed")
	}

	testCleanup(t, g, tearDownCmds, []string{"br-test1", "br-test2", "patch-br-test1", "patch-br-test2"})
}

func TestInterfaceOVS(t *testing.T) {
	g := helper.NewGraph(t)

	agent := helper.StartAgentWithConfig(t, confTopology)
	defer agent.Stop()

	setupCmds := []helper.Cmd{
		{"ovs-vsctl add-br br-test1", true},
		{"ovs-vsctl add-port br-test1 intf1 -- set interface intf1 type=internal", true},
	}

	tearDownCmds := []helper.Cmd{
		{"ovs-vsctl del-br br-test1", true},
	}

	tr := traversal.NewGraphTraversal(g)

	testPassed := false
	onChange := func(ws *websocket.Conn) {
		g.Lock()
		defer g.Unlock()

		if !testPassed {
			tv := tr.V().Has("Type", "internal", "Name", "intf1", "Driver", "openvswitch", "UUID", traversal.Ne(""), "MAC", traversal.Ne(""))

			if len(tv.Values()) == 1 && len(tr.V().Has("Name", "intf1", "Type", traversal.Ne("ovsport")).Values()) == 1 {
				testPassed = true
				ws.Close()
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
	g := helper.NewGraph(t)

	agent := helper.StartAgentWithConfig(t, confTopology)
	defer agent.Stop()

	setupCmds := []helper.Cmd{
		{"ovs-vsctl add-br br-test1", true},
		{"ip tuntap add mode tap dev intf1", true},
		{"ip tuntap add mode tap dev intf2", true},
		{"ovs-vsctl add-bond br-test1 bond0 intf1 intf2", true},
	}

	tearDownCmds := []helper.Cmd{
		{"ovs-vsctl del-br br-test1", true},
		{"ip link del intf1", true},
		{"ip link del intf2", true},
	}

	tr := traversal.NewGraphTraversal(g)

	testPassed := false
	onChange := func(ws *websocket.Conn) {
		g.Lock()
		defer g.Unlock()

		if !testPassed {
			tv := tr.V().Has("Type", "ovsport", "Name", "bond0")
			tv = tv.Out()

			if len(tv.Values()) == 2 {
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
	g := helper.NewGraph(t)

	agent := helper.StartAgentWithConfig(t, confTopology)
	defer agent.Stop()

	setupCmds := []helper.Cmd{
		{"ip l add vm1-veth0 type veth peer name vm1-veth1", true},
	}

	tearDownCmds := []helper.Cmd{
		{"ip link del vm1-veth0", true},
	}

	tr := traversal.NewGraphTraversal(g)

	testPassed := false
	onChange := func(ws *websocket.Conn) {
		g.Lock()
		defer g.Unlock()

		if !testPassed {
			tv := tr.V().Has("Type", "veth", "Name", "vm1-veth0")
			tv = tv.Both("Type", "veth", "Name", "vm1-veth1")

			if len(tv.Values()) == 1 {
				testPassed = true
				ws.Close()
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
	g := helper.NewGraph(t)

	agent := helper.StartAgentWithConfig(t, confTopology)
	defer agent.Stop()

	setupCmds := []helper.Cmd{
		{"brctl addbr br-test", true},
		{"ip tuntap add mode tap dev intf1", true},
		{"brctl addif br-test intf1", true},
	}

	tearDownCmds := []helper.Cmd{
		{"brctl delbr br-test", true},
		{"ip link del intf1", true},
	}

	tr := traversal.NewGraphTraversal(g)

	testPassed := false
	onChange := func(ws *websocket.Conn) {
		g.Lock()
		defer g.Unlock()

		if !testPassed {
			tv := tr.V().Has("Type", "bridge", "Name", "br-test")
			tv = tv.Out("Name", "intf1")

			if len(tv.Values()) == 1 {
				testPassed = true
				ws.Close()
			}
		}
	}

	testTopology(t, g, setupCmds, onChange)
	if !testPassed {
		t.Error("test not executed or failed")
	}

	testCleanup(t, g, tearDownCmds, []string{"br-test", "intf1"})
}

func TestMacNameUpdate(t *testing.T) {
	g := helper.NewGraph(t)

	agent := helper.StartAgentWithConfig(t, confTopology)
	defer agent.Stop()

	setupCmds := []helper.Cmd{
		{"ip l add vm1-veth0 type veth peer name vm1-veth1", true},
		{"ip l set vm1-veth1 name vm1-veth2", true},
		{"ip l set vm1-veth2 address 00:00:00:00:00:aa", true},
	}

	tearDownCmds := []helper.Cmd{
		{"ip link del vm1-veth0", true},
	}

	tr := traversal.NewGraphTraversal(g)

	testPassed := false
	onChange := func(ws *websocket.Conn) {
		g.Lock()
		defer g.Unlock()

		if !testPassed {
			tv := tr.V().Has("Name", "vm1-veth2", "MAC", "00:00:00:00:00:aa")

			if len(tv.Values()) == 1 && len(tr.V().Has("Name", "vm1-veth1").Values()) == 0 {
				testPassed = true
				ws.Close()
			}
		}
	}

	testTopology(t, g, setupCmds, onChange)
	if !testPassed {
		t.Error("test not executed or failed")
	}

	testCleanup(t, g, tearDownCmds, []string{"vm1-veth0", "vm1-veth1", "vm1-veth2"})
}

func TestNameSpace(t *testing.T) {
	g := helper.NewGraph(t)

	agent := helper.StartAgentWithConfig(t, confTopology)
	defer agent.Stop()

	setupCmds := []helper.Cmd{
		{"ip netns add ns1", true},
	}

	tearDownCmds := []helper.Cmd{
		{"ip netns del ns1", true},
	}

	tr := traversal.NewGraphTraversal(g)

	testPassed := false
	onChange := func(ws *websocket.Conn) {
		g.Lock()
		defer g.Unlock()

		if !testPassed {
			tv := tr.V().Has("Name", "ns1", "Type", "netns")

			if len(tv.Values()) == 1 {
				testPassed = true
				ws.Close()
			}
		}
	}

	testTopology(t, g, setupCmds, onChange)
	if !testPassed {
		t.Error("test not executed or failed")
	}

	testCleanup(t, g, tearDownCmds, []string{"ns1"})
}

func TestNameSpaceVeth(t *testing.T) {
	g := helper.NewGraph(t)

	agent := helper.StartAgentWithConfig(t, confTopology)
	defer agent.Stop()

	setupCmds := []helper.Cmd{
		{"ip netns add ns1", true},
		{"ip l add vm1-veth0 type veth peer name vm1-veth1 netns ns1", true},
	}

	tearDownCmds := []helper.Cmd{
		{"ip link del vm1-veth0", true},
		{"ip netns del ns1", true},
	}

	tr := traversal.NewGraphTraversal(g)

	testPassed := false
	onChange := func(ws *websocket.Conn) {
		g.Lock()
		defer g.Unlock()

		if !testPassed {
			tv := tr.V().Has("Name", "ns1", "Type", "netns")
			tv = tv.Out("Name", "vm1-veth1", "Type", "veth")

			if len(tv.Values()) == 1 {
				testPassed = true
				ws.Close()
			}
		}
	}

	testTopology(t, g, setupCmds, onChange)
	if !testPassed {
		t.Error("test not executed or failed")
	}

	testCleanup(t, g, tearDownCmds, []string{"ns1", "vm1-veth0"})
}

func TestNameSpaceOVSInterface(t *testing.T) {
	g := helper.NewGraph(t)

	agent := helper.StartAgentWithConfig(t, confTopology)
	defer agent.Stop()

	setupCmds := []helper.Cmd{
		{"ip netns add ns1", true},
		{"ovs-vsctl add-br br-test1", true},
		{"ovs-vsctl add-port br-test1 intf1 -- set interface intf1 type=internal", true},
		{"ip l set intf1 netns ns1", true},
	}

	tearDownCmds := []helper.Cmd{
		{"ovs-vsctl del-br br-test1", true},
		{"ip netns del ns1", true},
	}

	tr := traversal.NewGraphTraversal(g)

	testPassed := false
	onChange := func(ws *websocket.Conn) {
		g.Lock()
		defer g.Unlock()

		if !testPassed {
			tv := tr.V().Has("Name", "ns1", "Type", "netns")
			tv = tv.Out("Name", "intf1", "Type", "internal")

			if len(tv.Values()) == 1 && len(tr.V().Has("Name", "intf1", "Type", "internal").Values()) == 1 {
				testPassed = true
				ws.Close()
			}
		}
	}

	testTopology(t, g, setupCmds, onChange)
	if !testPassed {
		t.Error("test not executed or failed")
	}

	testCleanup(t, g, tearDownCmds, []string{"ns1", "br-test1"})
}

func TestDockerSimple(t *testing.T) {
	g := helper.NewGraph(t)

	agent := helper.StartAgentWithConfig(t, confTopology)
	defer agent.Stop()

	setupCmds := []helper.Cmd{
		{"docker run -d -t -i --name test-skydive-docker busybox", false},
	}

	tearDownCmds := []helper.Cmd{
		{"docker rm -f test-skydive-docker", false},
	}

	tr := traversal.NewGraphTraversal(g)

	testPassed := false
	onChange := func(ws *websocket.Conn) {
		g.Lock()
		defer g.Unlock()

		if !testPassed {
			tv := tr.V().Has("Name", "test-skydive-docker", "Type", "netns", "Manager", "docker")
			tv = tv.Out("Type", "container", "Docker/ContainerName", "/test-skydive-docker")

			if len(tv.Values()) == 1 {
				testPassed = true
				ws.Close()
			}
		}
	}

	testTopology(t, g, setupCmds, onChange)
	if !testPassed {
		t.Error("test not executed or failed")
	}

	testCleanup(t, g, tearDownCmds, []string{"test-skydive-docker"})
}

func TestDockerShareNamespace(t *testing.T) {
	g := helper.NewGraph(t)

	agent := helper.StartAgentWithConfig(t, confTopology)
	defer agent.Stop()

	setupCmds := []helper.Cmd{
		{"docker run -d -t -i --name test-skydive-docker busybox", false},
		{"docker run -d -t -i --name test-skydive-docker2 --net=container:test-skydive-docker busybox", false},
	}

	tearDownCmds := []helper.Cmd{
		{"docker rm -f test-skydive-docker", false},
		{"docker rm -f test-skydive-docker2", false},
	}

	tr := traversal.NewGraphTraversal(g)

	testPassed := false
	onChange := func(ws *websocket.Conn) {
		g.Lock()
		defer g.Unlock()

		if !testPassed {
			tv := tr.V().Has("Type", "netns", "Manager", "docker")
			logging.GetLogger().Warningf("Looking for docker namespace: %+v (%d)\n", tv.Values(), len(tv.Values()))

			if len(tv.Values()) > 1 {
				t.Error("There should be only one namespace managed by Docker")
				ws.Close()
			} else if len(tv.Values()) == 1 {
				tv = tv.Out().Has("Type", "container", "Docker/ContainerName", traversal.Within("/test-skydive-docker", "/test-skydive-docker2"))
				logging.GetLogger().Warningf("Found namespace, looking for containers: %+v (%d)\n", tv.Values(), len(tv.Values()))

				if len(tv.Values()) == 2 {
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

	testCleanup(t, g, tearDownCmds, []string{"test-skydive-docker"})
}

func TestDockerNetHost(t *testing.T) {
	g := helper.NewGraph(t)

	agent := helper.StartAgentWithConfig(t, confTopology)
	defer agent.Stop()

	setupCmds := []helper.Cmd{
		{"docker run -d -t -i --net=host --name test-skydive-docker busybox", false},
	}

	tearDownCmds := []helper.Cmd{
		{"docker rm -f test-skydive-docker", false},
	}

	tr := traversal.NewGraphTraversal(g)

	testPassed := false
	onChange := func(ws *websocket.Conn) {
		g.Lock()
		defer g.Unlock()

		if !testPassed {
			tv := tr.V().Has("Docker/ContainerName", "/test-skydive-docker", "Type", "container")

			if len(tv.Values()) == 1 {
				tv = tr.V().Has("Type", "netns", "Manager", "docker", "Name", "test-skydive-docker")
				if len(tv.Values()) == 1 {
					t.Error("There should be only no namespace managed by Docker")
				} else {
					testPassed = true
				}
				ws.Close()
			}
		}
	}

	testTopology(t, g, setupCmds, onChange)
	if !testPassed {
		t.Error("test not executed or failed")
	}

	testCleanup(t, g, tearDownCmds, []string{"test-skydive-docker"})
}
