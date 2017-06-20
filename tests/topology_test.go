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
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/tests/helper"
	"github.com/skydive-project/skydive/topology/graph"
)

func TestBridgeOVS(t *testing.T) {
	test := &Test{
		setupCmds: []helper.Cmd{
			{"ovs-vsctl add-br br-test1", true},
		},

		tearDownCmds: []helper.Cmd{
			{"ovs-vsctl del-br br-test1", true},
		},

		check: func(c *TestContext) error {
			gh := c.gh
			gremlin := "g"
			if !c.time.IsZero() {
				gremlin += fmt.Sprintf(".Context(%d)", common.UnixMillis(c.time))
			}

			gremlin += `.V().Has("Type", "ovsbridge", "Name", "br-test1")`
			gremlin += `.Out("Type", "ovsport", "Name", "br-test1")`
			gremlin += `.Out("Type", "internal", "Name", "br-test1", "Driver", "openvswitch")`

			// we have 2 links between ovsbridge and ovsport, this
			// results in 2 out nodes which are the same node so we Dedup
			gremlin += ".Dedup()"

			nodes, err := gh.GetNodes(gremlin)
			if err != nil {
				return err
			}

			if len(nodes) != 1 {
				return fmt.Errorf("Expected 1 node, got %+v", nodes)
			}

			return nil
		},
	}

	RunTest(t, test)
}

func TestPatchOVS(t *testing.T) {
	test := &Test{
		setupCmds: []helper.Cmd{
			{"ovs-vsctl add-br br-test1", true},
			{"ovs-vsctl add-br br-test2", true},
			{"ovs-vsctl add-port br-test1 patch-br-test2 -- set interface patch-br-test2 type=patch", true},
			{"ovs-vsctl add-port br-test2 patch-br-test1 -- set interface patch-br-test1 type=patch", true},
			{"ovs-vsctl set interface patch-br-test2 option:peer=patch-br-test1", true},
			{"ovs-vsctl set interface patch-br-test1 option:peer=patch-br-test2", true},
		},

		tearDownCmds: []helper.Cmd{
			{"ovs-vsctl del-br br-test1", true},
			{"ovs-vsctl del-br br-test2", true},
		},

		check: func(c *TestContext) error {
			gh := c.gh
			gremlin := "g"
			if !c.time.IsZero() {
				gremlin += fmt.Sprintf(".Context(%d)", common.UnixMillis(c.time))
			}

			gremlin += `.V().Has("Type", "patch", "Name", "patch-br-test1", "Driver", "openvswitch")`
			gremlin += `.Both("Type", "patch", "Name", "patch-br-test2", "Driver", "openvswitch")`

			nodes, err := gh.GetNodes(gremlin)
			if err != nil {
				return err
			}

			if len(nodes) != 1 {
				return fmt.Errorf("Expected 1 node, got %+v", nodes)
			}

			gremlin += `.Dedup()`

			if nodes, err = gh.GetNodes(gremlin); err != nil {
				return err
			}

			if len(nodes) != 1 {
				return fmt.Errorf("Expected 1 node, got %+v", nodes)
			}

			return nil
		},
	}

	RunTest(t, test)
}

func TestInterfaceOVS(t *testing.T) {
	test := &Test{
		setupCmds: []helper.Cmd{
			{"ovs-vsctl add-br br-test1", true},
			{"ovs-vsctl add-port br-test1 intf1 -- set interface intf1 type=internal", true},
		},

		tearDownCmds: []helper.Cmd{
			{"ovs-vsctl del-br br-test1", true},
		},

		check: func(c *TestContext) error {
			gh := c.gh
			prefix := "g"
			if !c.time.IsZero() {
				prefix += fmt.Sprintf(".Context(%d)", common.UnixMillis(c.time))
			}

			gremlin := prefix + `.V().Has("Type", "internal", "Name", "intf1", "Driver", "openvswitch").HasKey("UUID").HasKey("MAC")`
			nodes, err := gh.GetNodes(gremlin)
			if err != nil {
				return err
			}

			if len(nodes) != 1 {
				return fmt.Errorf("Expected one 'intf1' node with MAC and UUID attributes, got %+v", nodes)
			}

			gremlin = prefix + `.V().Has("Name", "intf1", "Type", Ne("ovsport"))`
			nodes, err = gh.GetNodes(gremlin)
			if err != nil {
				return err
			}

			if len(nodes) != 1 {
				return fmt.Errorf("Expected one 'intf1' node with type different than 'ovsport', got %+v", nodes)
			}

			return nil
		},
	}

	RunTest(t, test)
}

func TestVeth(t *testing.T) {
	test := &Test{
		setupCmds: []helper.Cmd{
			{"ip l add vm1-veth0 type veth peer name vm1-veth1", true},
		},

		tearDownCmds: []helper.Cmd{
			{"ip link del vm1-veth0", true},
		},

		check: func(c *TestContext) error {
			gh := c.gh
			prefix := "g"
			if !c.time.IsZero() {
				prefix += fmt.Sprintf(".Context(%d)", common.UnixMillis(c.time))
			}

			nodes, err := gh.GetNodes(prefix + `.V().Has("Type", "veth", "Name", "vm1-veth0").Both("Type", "veth", "Name", "vm1-veth1")`)
			if err != nil {
				return err
			}
			if len(nodes) != 1 {
				return fmt.Errorf("Expected 1 node, got %+v", nodes)
			}
			return nil
		},
	}

	RunTest(t, test)
}

func TestBridge(t *testing.T) {
	test := &Test{
		setupCmds: []helper.Cmd{
			{"brctl addbr br-test", true},
			{"ip tuntap add mode tap dev intf1", true},
			{"brctl addif br-test intf1", true},
		},

		tearDownCmds: []helper.Cmd{
			{"brctl delbr br-test", true},
			{"ip link del intf1", true},
		},

		check: func(c *TestContext) error {
			gh := c.gh
			prefix := "g"
			if !c.time.IsZero() {
				prefix += fmt.Sprintf(".Context(%d)", common.UnixMillis(c.time))
			}

			nodes, err := gh.GetNodes(prefix + `.V().Has("Type", "bridge", "Name", "br-test").Out("Name", "intf1")`)
			if err != nil {
				return err
			}

			if len(nodes) != 1 {
				return fmt.Errorf("Expected 1 node, got %+v", nodes)
			}

			return nil
		},
	}

	RunTest(t, test)
}

func TestMacNameUpdate(t *testing.T) {
	test := &Test{
		setupCmds: []helper.Cmd{
			{"ip l add vm1-veth0 type veth peer name vm1-veth1", true},
			{"ip l set vm1-veth1 name vm1-veth2", true},
			{"ip l set vm1-veth2 address 00:00:00:00:00:aa", true},
		},

		tearDownCmds: []helper.Cmd{
			{"ip link del vm1-veth0", true},
		},

		check: func(c *TestContext) error {
			gh := c.gh

			prefix := "g"
			if !c.time.IsZero() {
				prefix += fmt.Sprintf(".Context(%d)", common.UnixMillis(c.time))
			}

			newNodes, err := gh.GetNodes(prefix + `.V().Has("Name", "vm1-veth2", "MAC", "00:00:00:00:00:aa")`)
			if err != nil {
				return err
			}

			oldNodes, err := gh.GetNodes(prefix + `.V().Has("Name", "vm1-veth1")`)
			if err != nil {
				return err
			}

			if len(newNodes) != 1 || len(oldNodes) != 0 {
				return fmt.Errorf("Expected one name named vm1-veth2 and zero named vm1-veth1")
			}

			return nil
		},
	}

	RunTest(t, test)
}

func TestNameSpace(t *testing.T) {
	test := &Test{
		setupCmds: []helper.Cmd{
			{"ip netns add ns1", true},
		},

		tearDownCmds: []helper.Cmd{
			{"ip netns del ns1", true},
		},

		check: func(c *TestContext) error {
			gh := c.gh

			prefix := "g"
			if !c.time.IsZero() {
				prefix += fmt.Sprintf(".Context(%d)", common.UnixMillis(c.time))
			}

			nodes, err := gh.GetNodes(prefix + `.V().Has("Name", "ns1", "Type", "netns")`)
			if err != nil {
				return err
			}

			if len(nodes) != 1 {
				return fmt.Errorf("Expected 1 node, got %+v", nodes)
			}

			return nil
		},
	}

	RunTest(t, test)
}

func TestNameSpaceVeth(t *testing.T) {
	test := &Test{
		setupCmds: []helper.Cmd{
			{"ip netns add ns1", true},
			{"ip l add vm1-veth0 type veth peer name vm1-veth1 netns ns1", true},
		},

		tearDownCmds: []helper.Cmd{
			{"ip link del vm1-veth0", true},
			{"ip netns del ns1", true},
		},

		check: func(c *TestContext) error {
			gh := c.gh
			prefix := "g"
			if !c.time.IsZero() {
				prefix += fmt.Sprintf(".Context(%d)", common.UnixMillis(c.time))
			}

			nodes, err := gh.GetNodes(prefix + `.V().Has("Name", "ns1", "Type", "netns").Out("Name", "vm1-veth1", "Type", "veth")`)
			if err != nil {
				return err
			}

			if len(nodes) != 1 {
				return fmt.Errorf("Expected 1 node, got %+v", nodes)
			}

			return nil
		},
	}

	RunTest(t, test)
}

func TestNameSpaceOVSInterface(t *testing.T) {
	test := &Test{
		setupCmds: []helper.Cmd{
			{"ip netns add ns1", true},
			{"ovs-vsctl add-br br-test1", true},
			{"ovs-vsctl add-port br-test1 intf1 -- set interface intf1 type=internal", true},
			{"ip l set intf1 netns ns1", true},
		},

		tearDownCmds: []helper.Cmd{
			{"ovs-vsctl del-br br-test1", true},
			{"ip netns del ns1", true},
		},

		check: func(c *TestContext) error {
			gh := c.gh
			prefix := "g"
			if !c.time.IsZero() {
				prefix += fmt.Sprintf(".Context(%d)", common.UnixMillis(c.time))
			}

			nodes, err := gh.GetNodes(prefix + `.V().Has("Name", "ns1", "Type", "netns").Out("Name", "intf1", "Type", "internal")`)
			if err != nil {
				return err
			}

			if len(nodes) != 1 {
				return fmt.Errorf("Expected 1 node of type internal, got %+v", nodes)
			}

			nodes, err = gh.GetNodes(prefix + `.V().Has("Name", "intf1", "Type", "internal")`)
			if err != nil {
				return err
			}

			if len(nodes) != 1 {
				return fmt.Errorf("Expected 1 node, got %+v", nodes)
			}

			return nil
		},
	}

	RunTest(t, test)
}

func TestDockerSimple(t *testing.T) {
	test := &Test{
		setupCmds: []helper.Cmd{
			{"docker run -d -t -i --name test-skydive-docker-simple busybox", false},
		},

		tearDownCmds: []helper.Cmd{
			{"docker rm -f test-skydive-docker-simple", false},
		},

		check: func(c *TestContext) error {
			gh := c.gh
			gremlin := "g"
			if !c.time.IsZero() {
				gremlin += fmt.Sprintf(".Context(%d)", common.UnixMillis(c.time))
			}

			gremlin += `.V().Has("Name", "test-skydive-docker-simple", "Type", "netns", "Manager", "docker")`
			gremlin += `.Out("Type", "container", "Docker.ContainerName", "/test-skydive-docker-simple")`

			nodes, err := gh.GetNodes(gremlin)
			if err != nil {
				return err
			}

			if len(nodes) != 1 {
				return fmt.Errorf("Expected 1 node, got %+v", nodes)
			}

			return nil
		},
	}

	RunTest(t, test)
}

func TestDockerShareNamespace(t *testing.T) {
	test := &Test{
		setupCmds: []helper.Cmd{
			{"docker run -d -t -i --name test-skydive-docker-share-ns busybox", false},
			{"docker run -d -t -i --name test-skydive-docker-share-ns2 --net=container:test-skydive-docker-share-ns busybox", false},
		},

		tearDownCmds: []helper.Cmd{
			{"docker rm -f test-skydive-docker-share-ns", false},
			{"docker rm -f test-skydive-docker-share-ns2", false},
		},

		check: func(c *TestContext) error {
			gh := c.gh

			gremlin := "g"
			if !c.time.IsZero() {
				gremlin += fmt.Sprintf(".Context(%d)", common.UnixMillis(c.time))
			}

			gremlin += `.V().Has("Type", "netns", "Manager", "docker")`
			nodes, err := gh.GetNodes(gremlin)
			if err != nil {
				return err
			}

			switch len(nodes) {
			case 0:
				return errors.New("No namespace found")
			case 1:
				gremlin += `.Out().Has("Type", "container", "Docker.ContainerName", Within("/test-skydive-docker-share-ns", "/test-skydive-docker-share-ns2"))`

				nodes, err = gh.GetNodes(gremlin)
				if err != nil {
					return err
				}

				if len(nodes) != 2 {
					return fmt.Errorf("Expected 2 nodes, got %+v", nodes)
				}
			default:
				return fmt.Errorf("There should be only one namespace managed by Docker, got %+v", nodes)
			}

			return nil
		},
	}

	RunTest(t, test)
}

func TestDockerNetHost(t *testing.T) {
	test := &Test{
		setupCmds: []helper.Cmd{
			{"docker run -d -t -i --net=host --name test-skydive-docker-net-host busybox", false},
		},

		tearDownCmds: []helper.Cmd{
			{"docker rm -f test-skydive-docker-net-host", false},
		},

		check: func(c *TestContext) error {
			gh := c.gh

			prefix := "g"
			if !c.time.IsZero() {
				prefix += fmt.Sprintf(".Context(%d)", common.UnixMillis(c.time))
			}

			gremlin := prefix + `.V().Has("Docker.ContainerName", "/test-skydive-docker-net-host", "Type", "container")`
			nodes, err := gh.GetNodes(gremlin)
			if err != nil {
				return err
			}

			if len(nodes) != 1 {
				return fmt.Errorf("Expected 1 container, got %+v", nodes)
			}

			gremlin = prefix + `.V().Has("Type", "netns", "Manager", "docker", "Name", "test-skydive-docker-net-host")`
			nodes, err = gh.GetNodes(gremlin)
			if err != nil {
				return err
			}

			if len(nodes) != 0 {
				return fmt.Errorf("There should be only no namespace managed by Docker, got %+v", nodes)
			}

			return nil
		},
	}

	RunTest(t, test)
}

func TestInterfaceUpdate(t *testing.T) {
	start := time.Now()

	test := &Test{
		mode: OneShot,
		setupCmds: []helper.Cmd{
			{"ip netns add iu", true},
			{"sleep 5", false},
			{"ip netns exec iu ip link set lo up", true},
		},

		tearDownCmds: []helper.Cmd{
			{"ip netns del iu", true},
		},

		check: func(c *TestContext) error {
			gh := c.gh

			now := time.Now()
			gremlin := fmt.Sprintf("g.Context(%d, %d)", common.UnixMillis(now), int(now.Sub(start).Seconds()))
			gremlin += `.V().Has("Name", "iu", "Type", "netns").Out().Has("Name", "lo")`

			nodes, err := gh.GetNodes(gremlin)
			if err != nil {
				return err
			}

			if len(nodes) < 2 {
				return fmt.Errorf("Expected at least 2 nodes, got %+v", nodes)
			}

			hasDown := false
			hasUp := false
			for i := range nodes {
				if !hasDown && nodes[i].Metadata()["State"].(string) == "DOWN" {
					hasDown = true
				}
				if !hasUp && nodes[i].Metadata()["State"].(string) == "UP" {
					hasUp = true
				}
			}

			if !hasUp || !hasDown {
				return fmt.Errorf("Expected one node up and one node down, got %+v", nodes)
			}

			return nil
		},
	}

	RunTest(t, test)
}

func TestInterfaceMetrics(t *testing.T) {
	test := &Test{
		mode: OneShot,
		setupCmds: []helper.Cmd{
			{"ip netns add im", true},
			{"ip netns exec im ip link set lo up", true},
			{"sleep 2", false},
		},

		setupFunction: func(c *TestContext) error {
			helper.ExecCmds(t,
				helper.Cmd{Cmd: "ip netns exec im ping -c 15 127.0.0.1", Check: true},
				helper.Cmd{Cmd: "sleep 5", Check: false},
			)
			return nil
		},

		tearDownCmds: []helper.Cmd{
			{"ip netns del im", true},
		},

		check: func(c *TestContext) error {
			gh := c.gh

			gremlin := fmt.Sprintf("g.Context(%d, %d)", common.UnixMillis(c.startTime), c.startTime.Unix()-c.setupTime.Unix()+5)
			gremlin += `.V().Has("Name", "im", "Type", "netns").Out().Has("Name", "lo").Metrics().Aggregates()`

			metrics, err := gh.GetMetrics(gremlin)
			if err != nil {
				return err
			}

			if len(metrics) != 1 {
				return fmt.Errorf("Expected one aggregated metric, got %+v", metrics)
			}

			if len(metrics["Aggregated"]) <= 1 {
				return fmt.Errorf("Should have more metrics entry, got %+v", metrics["Aggregated"])
			}

			var start, tx int64
			for _, tm := range metrics["Aggregated"] {
				if tm.Start < start {
					j, _ := json.MarshalIndent(metrics, "", "\t")
					return fmt.Errorf("Metrics not correctly sorted (%+v)", string(j))
				}
				start = tm.Start

				im := tm.Metric.(*graph.InterfaceMetric)
				tx += im.TxPackets
			}

			if tx != 30 {
				return fmt.Errorf("Expected 30 TxPackets, got %d", tx)
			}

			gremlin += `.Sum()`

			tm, err := gh.GetMetric(gremlin)
			if err != nil {
				return fmt.Errorf("Could not find metrics with: %s", gremlin)
			}

			im := tm.Metric.(*graph.InterfaceMetric)
			if im.TxPackets != 30 {
				return fmt.Errorf("Expected 30 TxPackets, got %d", tx)
			}

			return nil
		},
	}

	RunTest(t, test)
}
func TestOVSOwnershipLink(t *testing.T) {
	test := &Test{
		setupCmds: []helper.Cmd{
			{"ovs-vsctl add-br br-owner", true},
			{"ovs-vsctl add-port br-owner patch-br-owner -- set interface patch-br-owner type=patch", true},
			{"ovs-vsctl add-port br-owner gre-br-owner -- set interface gre-br-owner type=gre", true},
			{"ovs-vsctl add-port br-owner vxlan-br-owner -- set interface vxlan-br-owner type=vxlan", true},
			{"ovs-vsctl add-port br-owner geneve-br-owner -- set interface geneve-br-owner type=geneve", true},
			{"ovs-vsctl add-port br-owner intf-owner -- set interface intf-owner type=internal", true},
		},

		tearDownCmds: []helper.Cmd{
			{"ovs-vsctl del-br br-owner", true},
		},

		check: func(c *TestContext) error {
			gh := c.gh
			prefix := "g"
			if !c.time.IsZero() {
				prefix += fmt.Sprintf(".Context(%d)", common.UnixMillis(c.time))
			}

			intfs := []string{"patch-br-owner", "gre-br-owner", "vxlan-br-owner", "geneve-br-owner"}
			for _, intf := range intfs {
				gremlin := prefix + fmt.Sprintf(`.V().Has('Name', '%s', 'Type', NE('ovsport')).InE().Has('RelationType', 'ownership').InV().Has('Name', 'br-owner')`, intf)
				nodes, err := gh.GetNodes(gremlin)
				if err != nil {
					return err
				}

				// only the host node shouldn't have a parent ownership link
				if len(nodes) != 1 {
					return errors.New("tunneling and patch interface should have one ownership link to the bridge")
				}
			}

			gremlin := prefix + `.V().Has('Name', 'intf-owner', 'Type', NE('ovsport')).InE().Has('RelationType', 'ownership').InV().Has('Type', 'host')`
			nodes, err := gh.GetNodes(gremlin)
			if err != nil {
				return err
			}

			// only the host node shouldn't have a parent ownership link
			if len(nodes) != 1 {
				return errors.New("internal interface should have one ownership link to the host")
			}

			return nil
		},
	}

	RunTest(t, test)
}

type TopologyInjecter struct {
	shttp.DefaultWSClientEventHandler
	connected int32
}

func (t *TopologyInjecter) OnConnected(c *shttp.WSAsyncClient) {
	atomic.StoreInt32(&t.connected, 1)
}

func TestQueryMetadata(t *testing.T) {
	test := &Test{
		setupFunction: func(c *TestContext) error {
			authOptions := &shttp.AuthenticationOpts{}
			addresses, err := config.GetAnalyzerServiceAddresses()
			if err != nil || len(addresses) == 0 {
				return fmt.Errorf("Unable to get the analyzers list: %s", err.Error())
			}

			hostname, _ := os.Hostname()
			wspool := shttp.NewWSAsyncClientPool()
			for _, sa := range addresses {
				authClient := shttp.NewAuthenticationClient(sa.Addr, sa.Port, authOptions)
				wsclient := shttp.NewWSAsyncClient(hostname+"-cli", "", sa.Addr, sa.Port, "/ws", authClient)
				wspool.AddWSAsyncClient(wsclient)
			}

			eventHandler := &TopologyInjecter{}
			wspool.AddEventHandler(eventHandler, []string{"*"})
			wspool.ConnectAll()

			err = common.Retry(func() error {
				if atomic.LoadInt32(&eventHandler.connected) != 1 {
					return errors.New("Not connected through WebSocket")
				}
				return nil
			}, 10, time.Second)

			if err != nil {
				return err
			}

			n := new(graph.Node)
			n.Decode(map[string]interface{}{
				"ID":   "123",
				"Host": "test",
				"Metadata": map[string]interface{}{
					"A": map[string]interface{}{
						"B": map[string]interface{}{
							"C": 123,
							"D": []interface{}{1, 2, 3},
						},
						"F": map[string]interface{}{
							"G": 123,
						},
					},
				},
			})

			msg := shttp.NewWSMessage(graph.Namespace, graph.NodeAddedMsgType, n)
			wspool.MasterClient().SendWSMessage(msg)

			return nil
		},
		check: func(c *TestContext) error {
			gh := c.gh

			prefix := "g"
			if !c.time.IsZero() {
				prefix += fmt.Sprintf(".Context(%d)", common.UnixMillis(c.time))
			}

			_, err := gh.GetNode(prefix + `.V().Has("A.F.G", 123)`)
			if err != nil {
				return err
			}

			_, err = gh.GetNode(prefix + `.V().Has("A.B.C", 123)`)
			if err != nil {
				return err
			}

			_, err = gh.GetNode(prefix + `.V().Has("A.B.D", Contains(1))`)
			if err != nil {
				return err
			}

			return nil
		},
	}

	RunTest(t, test)
}
