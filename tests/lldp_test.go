/*
 * Copyright (C) 2018 Red Hat, Inc.
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

func TestLLDP(t *testing.T) {
	test := &Test{
		setupCmds: []Cmd{
			{"ip link add lldp0 type dummy", false},
			{"ip link set lldp0 up", false},
			{"ip link set multicast on dev lldp0", false},
			{"ip addr add 10.10.20.1/24 dev lldp0", false},
			{"sleep 3", false}, // give some time to the LLDP probe to start capturing packets
		},

		tearDownCmds: []Cmd{
			{"ip link del lldp0", false},
		},

		injections: []TestInjection{{
			from: g.G.V().Has("Name", "lldp0"),
			pcap: "pcaptraces/lldp-detailed.pcap",
		}},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			gremlin := c.gremlin.V().Has("Name", "lldp0").Both("Type", "switchport").HasKey("LLDP")

			switchPort, err := c.gh.GetNode(gremlin)
			if err != nil {
				return fmt.Errorf("Failed to find node for port")
			}

			if mtu, _ := switchPort.GetFieldInt64("MTU"); mtu != 1522 {
				return fmt.Errorf("Expected MTU of 1522 for port switch")
			}

			portName := "Summit300-48-Port 1001"
			if name, _ := switchPort.GetFieldString("Name"); name != portName {
				return fmt.Errorf("Expected switch port to be named '%s' (%d), got '%s' (%d)", portName, len(portName), name, len(name))
			}

			gremlin = gremlin.Both("Type", "switch").Dedup()
			chassis, err := c.gh.GetNode(gremlin)
			if err != nil {
				return fmt.Errorf("Failed to find node for chassis")
			}

			id := "00:01:30:f9:ad:a0"
			if chassisID, _ := chassis.GetFieldString("LLDP.ChassisID"); chassisID != id {
				return fmt.Errorf("Expected LLDP.ChassisID to be '%s' (%d), got '%s' (%d)", id, len(id), chassisID, len(chassisID))
			}

			if name, _ := chassis.GetFieldString("Name"); name != "Summit300-48" {
				return fmt.Errorf("Expected chassis to be named 'Summit300-48'")
			}

			return nil
		}},
	}

	RunTest(t, test)
}
