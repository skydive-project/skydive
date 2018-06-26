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
	"net/http"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	g "github.com/skydive-project/skydive/gremlin"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/tests/helper"
	"github.com/skydive-project/skydive/topology"
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

		checks: []CheckFunction{func(c *CheckContext) error {
			gh := c.gh
			gremlin := g.G
			gremlin = gremlin.Context(c.time)

			gremlin = gremlin.V().Has("Type", "ovsbridge", "Name", "br-test1")
			gremlin = gremlin.Out("Type", "ovsport", "Name", "br-test1")
			gremlin = gremlin.Out("Type", "internal", "Name", "br-test1", "Driver", "openvswitch")

			// we have 2 links between ovsbridge and ovsport, this
			// results in 2 out nodes which are the same node so we Dedup
			gremlin = gremlin.Dedup()

			nodes, err := gh.GetNodes(gremlin)
			if err != nil {
				return err
			}

			if len(nodes) != 1 {
				return fmt.Errorf("Expected 1 node, got %+v", nodes)
			}

			return nil
		}},
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

		checks: []CheckFunction{func(c *CheckContext) error {
			gh := c.gh
			gremlin := g.G
			gremlin = gremlin.Context(c.time)

			gremlin = gremlin.V().Has("Type", "patch", "Name", "patch-br-test1", "Driver", "openvswitch")
			gremlin = gremlin.Both("Type", "patch", "Name", "patch-br-test2", "Driver", "openvswitch")

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
		}},
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

		checks: []CheckFunction{func(c *CheckContext) error {
			gh := c.gh
			prefix := g.G
			prefix = prefix.Context(c.time)
			gremlin := prefix.V().Has("Type", "internal", "Name", "intf1", "Driver", "openvswitch").HasKey("UUID").HasKey("MAC")
			nodes, err := gh.GetNodes(gremlin)
			if err != nil {
				return err
			}

			if len(nodes) != 1 {
				return fmt.Errorf("Expected one 'intf1' node with MAC and UUID attributes, got %+v", nodes)
			}

			gremlin = prefix.V().Has("Name", "intf1", "Type", g.Ne("ovsport"))
			nodes, err = gh.GetNodes(gremlin)
			if err != nil {
				return err
			}

			if len(nodes) != 1 {
				return fmt.Errorf("Expected one 'intf1' node with type different than 'ovsport', got %+v", nodes)
			}

			return nil
		}},
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

		checks: []CheckFunction{func(c *CheckContext) error {
			gh := c.gh
			prefix := g.G
			prefix = prefix.Context(c.time)

			nodes, err := gh.GetNodes(prefix.V().Has("Type", "veth", "Name", "vm1-veth0").Both("Type", "veth", "Name", "vm1-veth1"))
			if err != nil {
				return err
			}
			if len(nodes) != 1 {
				return fmt.Errorf("Expected 1 node, got %+v", nodes)
			}
			return nil
		}},
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

		checks: []CheckFunction{func(c *CheckContext) error {
			gh := c.gh
			prefix := g.G
			prefix = prefix.Context(c.time)

			nodes, err := gh.GetNodes(prefix.V().Has("Type", "bridge", "Name", "br-test").Out("Name", "intf1"))
			if err != nil {
				return err
			}

			if len(nodes) != 1 {
				return fmt.Errorf("Expected 1 node, got %+v", nodes)
			}

			return nil
		}},
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

		checks: []CheckFunction{func(c *CheckContext) error {
			gh := c.gh

			prefix := g.G
			prefix = prefix.Context(c.time)

			newNodes, err := gh.GetNodes(prefix.V().Has("Name", "vm1-veth2", "MAC", "00:00:00:00:00:aa"))
			if err != nil {
				return err
			}

			oldNodes, err := gh.GetNodes(prefix.V().Has("Name", "vm1-veth1"))
			if err != nil {
				return err
			}

			if len(newNodes) != 1 || len(oldNodes) != 0 {
				return fmt.Errorf("Expected one name named vm1-veth2 and zero named vm1-veth1")
			}

			return nil
		}},
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

		checks: []CheckFunction{func(c *CheckContext) error {
			gh := c.gh

			prefix := g.G
			prefix = prefix.Context(c.time)

			nodes, err := gh.GetNodes(prefix.V().Has("Name", "ns1", "Type", "netns"))
			if err != nil {
				return err
			}

			if len(nodes) != 1 {
				return fmt.Errorf("Expected 1 node, got %+v", nodes)
			}

			return nil
		}},
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

		checks: []CheckFunction{func(c *CheckContext) error {
			gh := c.gh
			prefix := g.G
			prefix = prefix.Context(c.time)

			nodes, err := gh.GetNodes(prefix.V().Has("Name", "ns1", "Type", "netns").Out("Name", "vm1-veth1", "Type", "veth"))
			if err != nil {
				return err
			}

			if len(nodes) != 1 {
				return fmt.Errorf("Expected 1 node, got %+v", nodes)
			}

			return nil
		}},
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

		checks: []CheckFunction{func(c *CheckContext) error {
			gh := c.gh
			prefix := g.G
			prefix = prefix.Context(c.time)

			nodes, err := gh.GetNodes(prefix.V().Has("Name", "ns1", "Type", "netns").Out("Name", "intf1", "Type", "internal"))
			if err != nil {
				return err
			}

			if len(nodes) != 1 {
				return fmt.Errorf("Expected 1 node of type internal, got %+v", nodes)
			}

			nodes, err = gh.GetNodes(prefix.V().Has("Name", "intf1", "Type", "internal"))
			if err != nil {
				return err
			}

			if len(nodes) != 1 {
				return fmt.Errorf("Expected 1 node, got %+v", nodes)
			}

			return nil
		}},
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

		checks: []CheckFunction{func(c *CheckContext) error {
			gh := c.gh

			now := time.Now()
			gremlin := g.G.Context(now, int(now.Sub(start).Seconds()))
			gremlin = gremlin.V().Has("Name", "iu", "Type", "netns").Out().Has("Name", "lo")

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
		}},
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

		checks: []CheckFunction{func(c *CheckContext) error {
			gh := c.gh

			gremlin := g.G.Context(c.startTime, c.startTime.Unix()-c.setupTime.Unix()+5)
			gremlin = gremlin.V().Has("Name", "im", "Type", "netns").Out().Has("Name", "lo").Metrics().Aggregates(10)

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
			for _, m := range metrics["Aggregated"] {
				if m.GetStart() < start {
					j, _ := json.MarshalIndent(metrics, "", "\t")
					return fmt.Errorf("Metrics not correctly sorted (%+v)", string(j))
				}
				start = m.GetStart()

				im := m.(*topology.InterfaceMetric)
				tx += im.TxPackets
			}

			// due to ratio applied during the aggregation we can't expect to get exactly
			// the sum of the metrics.
			if tx <= 25 {
				return fmt.Errorf("Expected at least TxPackets, got %d", tx)
			}

			gremlin += `.Sum()`

			m, err := gh.GetMetric(gremlin)
			if err != nil {
				return fmt.Errorf("Could not find metrics with: %s", gremlin)
			}

			im := m.(*topology.InterfaceMetric)
			if im.TxPackets != tx {
				return fmt.Errorf("Sum error %d vs %d", im.TxPackets, tx)
			}

			return nil
		}},
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

		checks: []CheckFunction{func(c *CheckContext) error {
			gh := c.gh
			prefix := g.G
			prefix = prefix.Context(c.time)

			intfs := []string{"patch-br-owner", "gre-br-owner", "vxlan-br-owner", "geneve-br-owner"}
			for _, intf := range intfs {
				gremlin := prefix.V().Has("Name", intf, "Type", g.Ne("ovsport")).InE().Has("RelationType", "ownership").InV().Has("Name", "br-owner")
				nodes, err := gh.GetNodes(gremlin)
				if err != nil {
					return err
				}

				// only the host node shouldn't have a parent ownership link
				if len(nodes) != 1 {
					return fmt.Errorf("tunneling and patch interface should have one ownership link to the bridge: %s", intf)
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
		}},
	}

	RunTest(t, test)
}

type TopologyInjecter struct {
	shttp.DefaultWSSpeakerEventHandler
	connected int32
}

func (t *TopologyInjecter) OnConnected(c shttp.WSSpeaker) {
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
			wspool := shttp.NewWSStructClientPool("TestQueryMetadata")
			for _, sa := range addresses {
				authClient := shttp.NewAuthenticationClient(config.GetURL("http", sa.Addr, sa.Port, ""), authOptions)
				client := shttp.NewWSClient(hostname+"-cli", common.UnknownService, config.GetURL("ws", sa.Addr, sa.Port, "/ws/publisher"), authClient, http.Header{}, 1000)
				wspool.AddClient(client)
			}

			masterElection := shttp.NewWSMasterElection(wspool)

			eventHandler := &TopologyInjecter{}
			wspool.AddEventHandler(eventHandler)
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

			msg := shttp.NewWSStructMessage(graph.Namespace, graph.NodeAddedMsgType, n)
			masterElection.SendMessageToMaster(msg)

			return nil
		},

		checks: []CheckFunction{func(c *CheckContext) error {
			gh := c.gh

			prefix := g.G
			prefix = prefix.Context(c.time)

			_, err := gh.GetNode(prefix.V().Has("A.F.G", 123))
			if err != nil {
				return err
			}

			_, err = gh.GetNode(prefix.V().Has("A.B.C", 123))
			if err != nil {
				return err
			}

			_, err = gh.GetNode(prefix.V().Has("A.B.D", 1))
			if err != nil {
				return err
			}

			return nil
		}},
	}

	RunTest(t, test)
}

//TestUserMetadata tests user metadata functionality
func TestUserMetadata(t *testing.T) {
	umd := types.NewUserMetadata(g.G.V().Has("Name", "br-umd", "Type", "ovsbridge").String(), "testKey", "testValue")
	test := &Test{
		setupCmds: []helper.Cmd{
			{"ovs-vsctl add-br br-umd", true},
		},

		setupFunction: func(c *TestContext) error {
			return c.client.Create("usermetadata", umd)
		},

		tearDownFunction: func(c *TestContext) error {
			c.client.Delete("usermetadata", umd.ID())
			return nil
		},

		tearDownCmds: []helper.Cmd{
			{"ovs-vsctl del-br br-umd", true},
		},

		checks: []CheckFunction{
			func(c *CheckContext) error {
				prefix := g.G
				prefix = prefix.Context(c.time)

				_, err := c.gh.GetNode(prefix.V().Has("UserMetadata.testKey", "testValue"))
				if err != nil {
					return fmt.Errorf("Failed to find a node with UserMetadata.testKey metadata")
				}

				return err
			},

			func(c *CheckContext) error {
				prefix := g.G
				prefix = prefix.Context(c.time)

				c.client.Delete("usermetadata", umd.ID())

				node, err := c.gh.GetNode(prefix.V().Has("UserMetadata.testKey", "testValue"))
				if err != common.ErrNotFound {
					return fmt.Errorf("Node %+v was found with metadata UserMetadata.testKey", node)
				}

				return nil
			},
		},
	}

	RunTest(t, test)
}

// TestAgentMetadata tests metadata set to the agent using the configuration file
func TestAgentMetadata(t *testing.T) {
	test := &Test{
		checks: []CheckFunction{
			func(c *CheckContext) error {
				prefix := g.G
				prefix = prefix.Context(c.time)

				_, err := c.gh.GetNode(prefix.V().Has("mydict.value", 123))
				if err != nil {
					return fmt.Errorf("Failed to find the host node with mydict.value metadata")
				}

				return nil
			},
		},
	}

	RunTest(t, test)
}

//TestRouteTable tests route table update
func TestRouteTable(t *testing.T) {
	test := &Test{
		mode: OneShot,

		setupCmds: []helper.Cmd{
			{"ovs-vsctl add-br br-rt", true},
			{"ip netns add rt-vm1", true},
			{"ip link add rt-vm1-eth0 type veth peer name rt-eth-src netns rt-vm1", true},
			{"ip link set rt-vm1-eth0 up", true},
			{"ip netns exec rt-vm1 ip link set rt-eth-src up", true},
			{"ip netns exec rt-vm1 ip address add 124.65.91.42/24 dev rt-eth-src", true},
			{"ovs-vsctl add-port br-rt rt-vm1-eth0", true},
			{"ip netns add rt-vm2", true},
			{"ip link add rt-vm2-eth0 type veth peer name rt-eth-dst netns rt-vm2", true},
			{"ip link set rt-vm2-eth0 up", true},
			{"ip netns exec rt-vm2 ip link set rt-eth-dst up", true},
			{"ip netns exec rt-vm2 ip address add 124.65.92.43/24 dev rt-eth-dst", true},
			{"ovs-vsctl add-port br-rt rt-vm2-eth0", true},
		},

		tearDownCmds: []helper.Cmd{
			{"ovs-vsctl del-br br-rt", true},
			{"ip link del rt-vm1-eth0", true},
			{"ip netns del rt-vm1", true},
			{"ip link del rt-vm2-eth0", true},
			{"ip netns del rt-vm2", true},
		},

		checks: []CheckFunction{
			func(c *CheckContext) error {
				prefix := g.G
				prefix = prefix.Context(c.time)

				node, err := c.gh.GetNode(prefix.V().Has("IPV4", "124.65.91.42/24"))
				if err != nil {
					return fmt.Errorf("Failed to find a node with IP 124.65.91.42/24")
				}

				routingTable := node.Metadata()["RoutingTable"].([]interface{})
				noOfRoutingTable := len(routingTable)

				helper.ExecCmds(t,
					helper.Cmd{Cmd: "ip netns exec rt-vm1 ip route add 124.65.92.0/24 via 124.65.91.42 table 2", Check: true},
					helper.Cmd{Cmd: "sleep 5", Check: false},
				)

				node, err = c.gh.GetNode(prefix.V().Has("IPV4", "124.65.91.42/24"))
				if err != nil {
					return fmt.Errorf("Failed to find a node with IP 124.65.91.42/24")
				}

				routingTable = node.Metadata()["RoutingTable"].([]interface{})
				newNoOfRoutingTable := len(routingTable)

				helper.ExecCmds(t,
					helper.Cmd{Cmd: "ip netns exec rt-vm1 ip route del 124.65.92.0/24 via 124.65.91.42 table 2", Check: true},
					helper.Cmd{Cmd: "sleep 5", Check: false},
				)
				if newNoOfRoutingTable <= noOfRoutingTable {
					return fmt.Errorf("Failed to add Route")
				}
				return nil
			},
		},
	}
	RunTest(t, test)
}

//TestRouteTableHistory tests route table update available in history
func TestRouteTableHistory(t *testing.T) {
	test := &Test{
		mode: OneShot,

		setupCmds: []helper.Cmd{
			{"ovs-vsctl add-br br-rth", true},
			{"ip netns add rth-vm1", true},
			{"ip link add rth-vm1-eth0 type veth peer name rth-eth-src netns rth-vm1", true},
			{"ip link set rth-vm1-eth0 up", true},
			{"ip netns exec rth-vm1 ip link set rth-eth-src up", true},
			{"ip netns exec rth-vm1 ip address add 124.65.75.42/24 dev rth-eth-src", true},
			{"ovs-vsctl add-port br-rth rth-vm1-eth0", true},
			{"ip netns add rth-vm2", true},
			{"ip link add rth-vm2-eth0 type veth peer name rth-eth-dst netns rth-vm2", true},
			{"ip link set rth-vm2-eth0 up", true},
			{"ip netns exec rth-vm2 ip link set rth-eth-dst up", true},
			{"ip netns exec rth-vm2 ip address add 124.65.76.43/24 dev rth-eth-dst", true},
			{"ovs-vsctl add-port br-rth rth-vm2-eth0", true},
			{"sleep 5", false},
			{"ip netns exec rth-vm1 ip route add 124.65.75.0/24 via 124.65.75.42 table 2", true},
		},

		tearDownCmds: []helper.Cmd{
			{"ovs-vsctl del-br br-rth", true},
			{"ip link del rth-vm1-eth0", true},
			{"ip netns del rth-vm1", true},
			{"ip link del rth-vm2-eth0", true},
			{"ip netns del rth-vm2", true},
		},

		checks: []CheckFunction{
			func(c *CheckContext) error {
				prefix := g.G.Context(time.Now())
				node, err := c.gh.GetNode(prefix.V().Has("IPV4", "124.65.75.42/24"))
				if err != nil {
					return fmt.Errorf("Failed to find a node with IP 124.65.75.42/24")
				}
				routingTable := node.Metadata()["RoutingTable"].([]interface{})
				foundNewTable := false
				for _, obj := range routingTable {
					rt := obj.(map[string]interface{})
					if rt["Id"].(int64) == 2 {
						foundNewTable = true
						break
					}
				}
				if !foundNewTable {
					return fmt.Errorf("Failed to get added Route from history")
				}
				return nil
			},
		},
	}
	RunTest(t, test)
}
