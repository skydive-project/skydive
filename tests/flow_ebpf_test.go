// +build ebpf

/*
 * Copyright (C) 2016 Red Hat, Inc.
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
	"fmt"
	"testing"

	g "github.com/skydive-project/skydive/gremlin"
)

func TestFlowsEBPF(t *testing.T) {
	test := &Test{
		setupCmds: []Cmd{
			{"ovs-vsctl add-br br-ebpf", true},

			{"ip netns add src-vm", true},
			{"ip link add src-vm-eth0 type veth peer name ebpf-src-eth0 netns src-vm", true},
			{"ip link set src-vm-eth0 up", true},
			{"ip netns exec src-vm ip link set ebpf-src-eth0 up", true},
			{"ip netns exec src-vm ip address add 169.254.107.33/24 dev ebpf-src-eth0", true},

			{"ip netns add dst-vm", true},
			{"ip link add dst-vm-eth0 type veth peer name ebpf-dst-eth0 netns dst-vm", true},
			{"ip link set dst-vm-eth0 up", true},
			{"ip netns exec dst-vm ip link set ebpf-dst-eth0 up", true},
			{"ip netns exec dst-vm ip address add 169.254.107.34/24 dev ebpf-dst-eth0", true},

			{"ovs-vsctl add-port br-ebpf src-vm-eth0", true},
			{"ovs-vsctl add-port br-ebpf dst-vm-eth0", true},
		},

		injections: []TestInjection{{
			from:  g.G.V().Has("Name", "src-vm").Out().Has("Name", "ebpf-src-eth0"),
			to:    g.G.V().Has("Name", "dst-vm").Out().Has("Name", "ebpf-dst-eth0"),
			count: 10,
		}},

		tearDownCmds: []Cmd{
			{"ovs-vsctl del-br br-ebpf", true},
			{"ip link del dst-vm-eth0", true},
			{"ip link del src-vm-eth0", true},
			{"ip netns del src-vm", true},
			{"ip netns del dst-vm", true},
		},

		captures: []TestCapture{
			{gremlin: g.G.V().Has("Name", "ebpf-src-eth0"), kind: "ebpf"},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			gremlin := c.gremlin.Flows().Has("Network", "169.254.107.33", "LayersPath", "Ethernet/IPv4/ICMPv4").Dedup()
			flows, err := c.gh.GetFlows(gremlin)
			if err != nil {
				return err
			}
			if len(flows) != 1 || flows[0].Metric.ABPackets != 10 {
				return fmt.Errorf("Expected one flow, got %+v", flows)
			}
			return nil
		}},
	}
	RunTest(t, test)
}

func TestFlowsVLANEBPF(t *testing.T) {
	test := &Test{
		setupCmds: []Cmd{
			{"ovs-vsctl add-br br-ebpf", true},

			{"ip netns add src-vm", true},
			{"ip link add src-vm-eth0 type veth peer name ebpf-src-eth0 netns src-vm", true},
			{"ip link set src-vm-eth0 up", true},
			{"ip netns exec src-vm ip link set ebpf-src-eth0 up", true},
			{"ip netns exec src-vm ip link add link ebpf-src-eth0 name vlan1 type vlan id 8", true},
			{"ip netns exec src-vm ip link set vlan1 up", true},
			{"ip netns exec src-vm ip link add link vlan1 name vlan0 type vlan id 2050", true},
			{"ip netns exec src-vm ip address add 169.254.107.33/24 dev vlan0", true},
			{"ip netns exec src-vm ip link set vlan0 up", true},

			{"ip netns add dst-vm", true},
			{"ip link add dst-vm-eth0 type veth peer name ebpf-dst-eth0 netns dst-vm", true},
			{"ip link set dst-vm-eth0 up", true},
			{"ip netns exec dst-vm ip link set ebpf-dst-eth0 up", true},
			{"ip netns exec dst-vm ip link add link ebpf-dst-eth0 name vlan1 type vlan id 8", true},
			{"ip netns exec dst-vm ip link set vlan1 up", true},
			{"ip netns exec dst-vm ip link add link vlan1 name vlan0 type vlan id 2050", true},
			{"ip netns exec dst-vm ip address add 169.254.107.34/24 dev vlan0", true},
			{"ip netns exec dst-vm ip link set vlan0 up", true},

			{"ovs-vsctl add-port br-ebpf src-vm-eth0", true},
			{"ovs-vsctl add-port br-ebpf dst-vm-eth0", true},
		},

		injections: []TestInjection{{
			from:  g.G.V().Has("Name", "src-vm").Out().Has("Name", "vlan0"),
			to:    g.G.V().Has("Name", "dst-vm").Out().Has("Name", "vlan0"),
			count: 10,
		}},

		tearDownCmds: []Cmd{
			{"ovs-vsctl del-br br-ebpf", true},
			{"ip netns exec src-vm ip link del vlan0", true},
			{"ip netns exec dst-vm ip link del vlan0", true},
			{"ip link del src-vm-eth0", true},
			{"ip link del dst-vm-eth0", true},
			{"ip netns del src-vm", true},
			{"ip netns del dst-vm", true},
		},

		captures: []TestCapture{
			{gremlin: g.G.V().Has("Name", "ebpf-src-eth0"), kind: "ebpf"},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			gremlin := c.gremlin.Flows().Has("Network", "169.254.107.33", "LayersPath", "Ethernet/Dot1Q/Dot1Q/IPv4/ICMPv4").Dedup()
			flows, err := c.gh.GetFlows(gremlin)
			if err != nil {
				return err
			}
			if len(flows) != 1 || flows[0].Metric.ABPackets != 10 {
				return fmt.Errorf("Expected one flow, got %+v", flows)
			}
			return nil
		}},
	}
	RunTest(t, test)
}

func TestFlowsGREEBPF(t *testing.T) {
	test := testFlowTunnel(t, "br-greebpf", "gre", false, "192.168.0.1", "192.168.0.2",
		"172.16.0.1", "172.16.0.2", "192.168.0.0/24")
	for i := range test.captures {
		test.captures[i].kind = "ebpf"
	}
	RunTest(t, test)
}
