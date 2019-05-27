/*
 * Copyright (C) 2015 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy ofthe License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specificlanguage governing permissions and
 * limitations under the License.
 *
 */

package tests

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/graffiti/graph"
	g "github.com/skydive-project/skydive/gremlin"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/probes/netlink"
)

func TestBridgeOVS(t *testing.T) {
	test := &Test{
		setupCmds: []Cmd{
			{"ovs-vsctl add-br br-testbovs1", true},
		},

		tearDownCmds: []Cmd{
			{"ovs-vsctl del-br br-testbovs1", true},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			gh := c.gh
			gremlin := c.gremlin.V().Has("Type", "ovsbridge", "Name", "br-testbovs1")
			gremlin = gremlin.Out("Type", "ovsport", "Name", "br-testbovs1")
			gremlin = gremlin.Out("Type", "internal", "Name", "br-testbovs1", "Driver", "openvswitch")

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
		setupCmds: []Cmd{
			{"ovs-vsctl add-br br-testpaovs1", true},
			{"ovs-vsctl add-br br-testpaovs2", true},
			{"ovs-vsctl add-port br-testpaovs1 patch-br-testpaovs2 -- set interface patch-br-testpaovs2 type=patch", true},
			{"ovs-vsctl add-port br-testpaovs2 patch-br-testpaovs1 -- set interface patch-br-testpaovs1 type=patch", true},
			{"ovs-vsctl set interface patch-br-testpaovs2 option:peer=patch-br-testpaovs1", true},
			{"ovs-vsctl set interface patch-br-testpaovs1 option:peer=patch-br-testpaovs2", true},
		},

		tearDownCmds: []Cmd{
			{"ovs-vsctl del-br br-testpaovs1", true},
			{"ovs-vsctl del-br br-testpaovs2", true},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			gh := c.gh
			gremlin := c.gremlin.V().Has("Type", "patch", "Name", "patch-br-testpaovs1", "Driver", "openvswitch")
			gremlin = gremlin.Both("Type", "patch", "Name", "patch-br-testpaovs2", "Driver", "openvswitch")

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
		setupCmds: []Cmd{
			{"ovs-vsctl add-br br-test1", true},
			{"ovs-vsctl add-port br-test1 intf1 -- set interface intf1 type=internal", true},
		},

		tearDownCmds: []Cmd{
			{"ovs-vsctl del-br br-test1", true},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			gh := c.gh
			prefix := c.gremlin
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
		setupCmds: []Cmd{
			{"ip l add vm1-veth0 type veth peer name vm1-veth1", true},
		},

		tearDownCmds: []Cmd{
			{"ip link del vm1-veth0", true},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			gh := c.gh
			prefix := c.gremlin

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
		setupCmds: []Cmd{
			{"brctl addbr br-test", true},
			{"ip tuntap add mode tap dev intf1", true},
			{"brctl addif br-test intf1", true},
		},

		tearDownCmds: []Cmd{
			{"brctl delbr br-test", true},
			{"ip link del intf1", true},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			gh := c.gh
			prefix := c.gremlin

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
		setupCmds: []Cmd{
			{"ip l add vm1-veth0 type veth peer name vm1-veth1", true},
			{"ip l set vm1-veth1 name vm1-veth2", true},
			{"ip l set vm1-veth2 address 00:00:00:00:00:aa", true},
		},

		tearDownCmds: []Cmd{
			{"ip link del vm1-veth0", true},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			gh := c.gh
			prefix := c.gremlin

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
		setupCmds: []Cmd{
			{"ip netns add ns1", true},
		},

		tearDownCmds: []Cmd{
			{"ip netns del ns1", true},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			gh := c.gh
			prefix := c.gremlin

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
		setupCmds: []Cmd{
			{"ip netns add ns1", true},
			{"ip l add vm1-veth0 type veth peer name vm1-veth1 netns ns1", true},
		},

		tearDownCmds: []Cmd{
			{"ip link del vm1-veth0", true},
			{"ip netns del ns1", true},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			gh := c.gh
			prefix := c.gremlin

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

func TestNameSpaceTwoVeth(t *testing.T) {
	test := &Test{
		setupCmds: []Cmd{
			{"ip netns add ns1", true},
			{"ip netns add ns2", true},
			{"ip l add vm1-veth0 type veth peer name vm2-veth0 netns ns1", true},
			{"ip l set vm1-veth0 netns ns2", true},
		},

		tearDownCmds: []Cmd{
			{"ip netns exec ns1 ip link del vm2-veth0", true},
			{"ip netns del ns1", true},
			{"ip netns del ns2", true},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			gh := c.gh
			prefix := c.gremlin

			nodes, err := gh.GetNodes(prefix.V().Has("LinkNetNsName", "ns1"))
			if err != nil {
				return err
			}

			if len(nodes) != 1 {
				return fmt.Errorf("Expected 1 node, got %+v", nodes)
			}

			nodes, err = gh.GetNodes(prefix.V().Has("LinkNetNsName", "ns2"))
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
		setupCmds: []Cmd{
			{"ip netns add ns1", true},
			{"ovs-vsctl add-br br-test1", true},
			{"ovs-vsctl add-port br-test1 intf1 -- set interface intf1 type=internal", true},
			{"ip l set intf1 netns ns1", true},
		},

		tearDownCmds: []Cmd{
			{"ovs-vsctl del-br br-test1", true},
			{"ip netns del ns1", true},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			gh := c.gh
			prefix := c.gremlin

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
		setupCmds: []Cmd{
			{"ip netns add iu", true},
			{"sleep 5", false},
			{"ip netns exec iu ip link set lo up", true},
		},

		tearDownCmds: []Cmd{
			{"ip netns del iu", true},
		},

		mode: OneShot,

		checks: []CheckFunction{func(c *CheckContext) error {
			gh := c.gh
			now := time.Now()
			gremlin := c.gremlin.Context("NOW", int(now.Sub(start).Seconds()))
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
				hasDown = hasDown || topology.IsInterfaceUp(nodes[i])
				hasUp = hasUp || topology.IsInterfaceUp(nodes[i])
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
		setupCmds: []Cmd{
			{"ip netns add im", true},
			{"ip netns exec im ip link set lo up", true},
			{"sleep 2", false},
		},

		setupFunction: func(c *TestContext) error {
			execCmds(t,
				Cmd{Cmd: "ip netns exec im ping -c 15 127.0.0.1", Check: true},
				Cmd{Cmd: "sleep 5", Check: false},
			)
			return nil
		},

		tearDownCmds: []Cmd{
			{"ip netns del im", true},
		},

		mode: OneShot,

		checks: []CheckFunction{func(c *CheckContext) error {
			gh := c.gh
			gremlin := c.gremlin.Context(c.startTime, c.startTime.Unix()-c.setupTime.Unix()+5)
			gremlin = gremlin.V().Has("Name", "im", "Type", "netns").Out().Has("Name", "lo").Metrics().Aggregates(10)

			metrics, err := gh.GetInterfaceMetrics(gremlin)
			if err != nil {
				return err
			}

			if len(metrics) != 1 {
				return fmt.Errorf("Expected one aggregated metric, got %+v", metrics)
			}

			if len(metrics["Aggregated"]) <= 1 {
				return fmt.Errorf("Should have more metrics entry, got %+v", metrics["Aggregated"])
			}

			var start, totalTx int64
			for _, m := range metrics["Aggregated"] {
				if m.GetStart() < start {
					j, _ := json.MarshalIndent(metrics, "", "\t")
					return fmt.Errorf("Metrics not correctly sorted (%+v)", string(j))
				}
				start = m.GetStart()

				tx, _ := m.GetFieldInt64("TxPackets")
				totalTx += tx
			}

			// due to ratio applied during the aggregation we can't expect to get exactly
			// the sum of the metrics.
			if totalTx <= 25 {
				return fmt.Errorf("Expected at least TxPackets, got %d", totalTx)
			}

			gremlin += `.Sum()`

			m, err := gh.GetInterfaceMetric(gremlin)
			if err != nil {
				return fmt.Errorf("Could not find metrics with: %s", gremlin)
			}

			if tx, _ := m.GetFieldInt64("TxPackets"); tx != totalTx {
				return fmt.Errorf("Sum error %d vs %d", totalTx, tx)
			}

			return nil
		}},
	}

	RunTest(t, test)
}

func TestOVSOwnershipLink(t *testing.T) {
	test := &Test{
		setupCmds: []Cmd{
			{"ovs-vsctl add-br br-owner", true},
			{"ovs-vsctl add-port br-owner patch-br-owner -- set interface patch-br-owner type=patch", true},
			{"ovs-vsctl add-port br-owner gre-br-owner -- set interface gre-br-owner type=gre", true},
			{"ovs-vsctl add-port br-owner vxlan-br-owner -- set interface vxlan-br-owner type=vxlan", true},
			{"ovs-vsctl add-port br-owner geneve-br-owner -- set interface geneve-br-owner type=geneve", true},
			{"ovs-vsctl add-port br-owner intf-owner -- set interface intf-owner type=internal", true},
		},

		tearDownCmds: []Cmd{
			{"ovs-vsctl del-br br-owner", true},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			gh := c.gh
			prefix := c.gremlin

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

func TestNodeRuleCreate(t *testing.T) {
	nodeRule := &types.NodeRule{
		Action:   "create",
		Metadata: graph.Metadata{"Name": "TestNode", "Type": "fabric"},
	}

	test := &Test{
		setupFunction: func(c *TestContext) error {
			return c.client.Create("noderule", nodeRule)
		},

		tearDownFunction: func(c *TestContext) error {
			c.client.Delete("noderule", nodeRule.ID())
			return nil
		},

		mode: Replay,

		checks: []CheckFunction{
			func(c *CheckContext) error {
				if _, err := c.gh.GetNode(c.gremlin.V().Has("Name", "TestNode")); err != nil {
					return errors.New("Failed to find a node with name TestNode")
				}

				return nil
			},

			func(c *CheckContext) error {
				c.client.Delete("noderule", nodeRule.ID())

				if node, err := c.gh.GetNode(c.gremlin.V().Has("Name", "TestNode")); err != common.ErrNotFound {
					return fmt.Errorf("Node %+v found with name TestNode", node)
				}

				return nil
			},
		},
	}

	RunTest(t, test)
}

func TestNodeRuleUpdate(t *testing.T) {
	nodeRule := &types.NodeRule{
		Action:   "update",
		Query:    "G.V().Has('Name', 'br-umd', 'Type', 'ovsbridge')",
		Metadata: graph.Metadata{"testKey": "testValue"},
	}

	test := &Test{
		setupCmds: []Cmd{
			{"ovs-vsctl add-br br-umd", true},
		},

		setupFunction: func(c *TestContext) error {
			return c.client.Create("noderule", nodeRule)
		},

		tearDownFunction: func(c *TestContext) error {
			c.client.Delete("noderule", nodeRule.ID())
			return nil
		},

		tearDownCmds: []Cmd{
			{"ovs-vsctl del-br br-umd", true},
		},

		mode: Replay,

		checks: []CheckFunction{
			func(c *CheckContext) error {
				if _, err := c.gh.GetNode(c.gremlin.V().Has("testKey", "testValue")); err != nil {
					return fmt.Errorf("Failed to find a node with testKey metadata")
				}

				return nil
			},

			func(c *CheckContext) error {
				c.client.Delete("noderule", nodeRule.ID())

				if node, err := c.gh.GetNode(c.gremlin.V().Has("testKey", "testValue")); err != common.ErrNotFound {
					return fmt.Errorf("Node %+v was found with metadata testKey", node)
				}

				return nil
			},
		},
	}

	RunTest(t, test)
}

func TestEdgeRuleCreate(t *testing.T) {
	edgeRule := &types.EdgeRule{
		Src:      "G.V().Has('Name', 'br-srcnode', 'Type', 'ovsbridge')",
		Dst:      "G.V().Has('Name', 'br-dstnode', 'Type', 'ovsbridge')",
		Metadata: graph.Metadata{"RelationType": "layer2"},
	}

	test := &Test{
		setupCmds: []Cmd{
			{"ovs-vsctl add-br br-srcnode", true},
			{"ovs-vsctl add-br br-dstnode", true},
			{"sleep 5", false},
		},

		setupFunction: func(c *TestContext) error {
			return c.client.Create("edgerule", edgeRule)
		},

		tearDownFunction: func(c *TestContext) error {
			c.client.Delete("edgerule", edgeRule.ID())
			return nil
		},

		tearDownCmds: []Cmd{
			{"ovs-vsctl del-br br-srcnode", true},
			{"ovs-vsctl del-br br-dstnode", true},
		},

		mode: Replay,

		checks: []CheckFunction{
			func(c *CheckContext) error {
				query := c.gremlin.V().Has("Name", "br-srcnode", "Type", "ovsbridge")
				query = query.OutE().Has("RelationType", "layer2")
				query = query.OutV().Has("Name", "br-dstnode", "Type", "ovsbridge")
				if _, err := c.gh.GetNode(query); err != nil {
					return fmt.Errorf("Failed to find a layer2 link, error: %v", err)
				}

				return nil
			},

			func(c *CheckContext) error {
				query := c.gremlin.V().Has("Name", "br-srcnode", "Type", "ovsbridge")
				query = query.OutE().Has("RelationType", "layer2")
				query = query.OutV().Has("Name", "br-dstnode", "Type", "ovsbridge")

				if c.successTime.IsZero() {
					c.client.Delete("edgerule", edgeRule.ID())
				}

				if _, err := c.gh.GetNode(query); err != common.ErrNotFound {
					return errors.New("Found a layer2 link")
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
		mode: Replay,

		checks: []CheckFunction{
			func(c *CheckContext) error {
				if _, err := c.gh.GetNode(c.gremlin.V().Has("mydict.value", 123)); err != nil {
					return err
				}

				if _, err := c.gh.GetNode(c.gremlin.V().Has("myarrays.integers", 1)); err != nil {
					return err
				}

				if _, err := c.gh.GetNode(c.gremlin.V().Has("myarrays.strings", "cat")); err != nil {
					return err
				}

				_, err := c.gh.GetNode(c.gremlin.V().Has("myarrays.bools", true))
				return err
			},
		},
	}

	RunTest(t, test)
}

//TestRouteTable tests route table update
func TestRouteTable(t *testing.T) {
	test := &Test{
		setupCmds: []Cmd{
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

		tearDownCmds: []Cmd{
			{"ovs-vsctl del-br br-rt", true},
			{"ip link del rt-vm1-eth0", true},
			{"ip netns del rt-vm1", true},
			{"ip link del rt-vm2-eth0", true},
			{"ip netns del rt-vm2", true},
		},

		mode: OneShot,

		checks: []CheckFunction{
			func(c *CheckContext) error {
				prefix := c.gremlin

				node, err := c.gh.GetNode(prefix.V().Has("IPV4", "124.65.91.42/24"))
				if err != nil {
					return fmt.Errorf("Failed to find a node with IP 124.65.91.42/24")
				}

				routingTables, ok := node.Metadata["RoutingTables"].(*netlink.RoutingTables)
				if !ok {
					return fmt.Errorf("Wrong metadata type for RoutingTables: %+v", node.Metadata["RoutingTables"])
				}
				noOfRoutingTable := len(*routingTables)

				execCmds(t,
					Cmd{Cmd: "ip netns exec rt-vm1 ip route add 124.65.92.0/24 via 124.65.91.42 table 2", Check: true},
					Cmd{Cmd: "sleep 5", Check: false},
				)

				node, err = c.gh.GetNode(prefix.V().Has("IPV4", "124.65.91.42/24"))
				if err != nil {
					return fmt.Errorf("Failed to find a node with IP 124.65.91.42/24")
				}

				routingTables, ok = node.Metadata["RoutingTables"].(*netlink.RoutingTables)
				if !ok {
					return fmt.Errorf("Wrong metadata type for RoutingTables: %+v", node.Metadata["RoutingTables"])
				}
				newNoOfRoutingTable := len(*routingTables)

				execCmds(t,
					Cmd{Cmd: "ip netns exec rt-vm1 ip route del 124.65.92.0/24 via 124.65.91.42 table 2", Check: true},
					Cmd{Cmd: "sleep 5", Check: false},
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
		setupCmds: []Cmd{
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

		tearDownCmds: []Cmd{
			{"ovs-vsctl del-br br-rth", true},
			{"ip link del rth-vm1-eth0", true},
			{"ip netns del rth-vm1", true},
			{"ip link del rth-vm2-eth0", true},
			{"ip netns del rth-vm2", true},
		},

		mode: OneShot,

		checks: []CheckFunction{
			func(c *CheckContext) error {
				prefix := g.G.Context("NOW")
				node, err := c.gh.GetNode(prefix.V().Has("IPV4", "124.65.75.42/24"))
				if err != nil {
					return fmt.Errorf("Failed to find a node with IP 124.65.75.42/24")
				}

				routingTables, ok := node.Metadata["RoutingTables"].(*netlink.RoutingTables)
				if !ok {
					return fmt.Errorf("Wrong metadata type for RoutingTables: %+v", reflect.TypeOf(node.Metadata["RoutingTables"]))
				}

				foundNewTable := false
				for _, rt := range *routingTables {
					if rt.ID == 2 {
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

func TestInterfaceFeatures(t *testing.T) {
	test := &Test{
		setupCmds: []Cmd{
			{"brctl addbr br-features", true},
		},

		tearDownCmds: []Cmd{
			{"brctl delbr br-features", true},
		},

		checks: []CheckFunction{
			func(c *CheckContext) error {
				_, err := c.gh.GetNode(c.gremlin.V().Has("Name", "br-features", "Features.highdma", true))
				return err
			},
		},
	}
	RunTest(t, test)
}

func TestSFlowMetric(t *testing.T) {
	test := &Test{
		setupCmds: []Cmd{
			{"ovs-vsctl add-br br-sfmt", true},

			{"ip netns add vm1", true},
			{"ip link add vm1-eth0 type veth peer name eth0 netns vm1", true},
			{"ip link set vm1-eth0 up", true},
			{"ovs-vsctl add-port br-sfmt vm1-eth0", true},
			{"ip netns exec vm1 ip link set eth0 up", true},
			{"ip netns exec vm1 ip address add 192.168.0.11/24 dev eth0", true},
			{"ip netns add vm2", true},
			{"ip link add vm2-eth0 type veth peer name eth0 netns vm2", true},
			{"ip link set vm2-eth0 up", true},
			{"ovs-vsctl add-port br-sfmt vm2-eth0", true},
			{"ip netns exec vm2 ip link set eth0 up", true},
			{"ip netns exec vm2 ip address add 192.168.0.21/24 dev eth0", true},
		},

		injections: []TestInjection{{
			from:  g.G.V().Has("Name", "vm1").Out().Has("Name", "eth0"),
			to:    g.G.V().Has("Name", "vm2").Out().Has("Name", "eth0"),
			count: 1,
		}},

		tearDownCmds: []Cmd{
			{"ovs-vsctl del-br br-sfmt", true},
			{"ip link set vm1-eth0 down", true},
			{"ip link del vm1-eth0", true},
			{"ip netns del vm1", true},
			{"ip link set vm2-eth0 down", true},
			{"ip link del vm2-eth0", true},
			{"ip netns del vm2", true},
		},

		captures: []TestCapture{
			{gremlin: g.G.V().Has("Type", "host").Out().Has("Name", "br-sfmt", "Type", "ovsbridge"), kind: "ovssflow", samplingRate: 1, pollingInterval: 10},
		},

		mode: OneShot,

		checks: []CheckFunction{func(c *CheckContext) error {
			sfmetrics, err := c.gh.GetSFlowMetrics(c.gremlin.V().Metrics("SFlow.LastUpdateMetric").Aggregates())
			if err != nil {
				return err
			}

			if len(sfmetrics) != 1 {
				return fmt.Errorf("We should receive only one unique element in  Array of Metric, got: %d", len(sfmetrics))
			}

			if len(sfmetrics["Aggregated"]) < 1 {
				return fmt.Errorf("Should have one or more metrics entry, got %+v", sfmetrics["Aggregated"])
			}

			var start, totalInUc int64
			for _, m := range sfmetrics["Aggregated"] {
				if m.GetStart() < start {
					j, _ := json.MarshalIndent(sfmetrics, "", "\t")
					return fmt.Errorf("Metrics not correctly sorted (%+v)", string(j))
				}
				start = m.GetStart()

				inUc, _ := m.GetFieldInt64("IfInUcastPkts")
				totalInUc += inUc
			}

			// due to ratio applied during the aggregation we can't expect to get exactly
			// the sum of the metrics.
			if totalInUc <= 1 {
				return fmt.Errorf("Expected at least IfInUcastPkts, got %d", totalInUc)
			}

			m, err := c.gh.GetSFlowMetric(c.gremlin.V().Metrics("SFlow.LastUpdateMetric").Aggregates().Sum())
			if err != nil {
				return fmt.Errorf("Could not find metrics with: %s", "c.gremlin.V().Metrics('SFlow.LastUpdateMetric').Aggregates().Sum()")
			}

			if inUc, _ := m.GetFieldInt64("IfInUcastPkts"); inUc != totalInUc {
				return fmt.Errorf("Sum error %d vs %d", totalInUc, inUc)
			}

			return nil
		}},
	}

	RunTest(t, test)
}
