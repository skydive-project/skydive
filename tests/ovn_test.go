/*
 * Copyright (C) 2019 Red Hat, Inc.
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
)

func TestOVNLsAdd(t *testing.T) {
	test := &Test{
		setupCmds: []Cmd{
			{"ovn-nbctl ls-add ls-test", false},
		},

		tearDownCmds: []Cmd{
			{"ovn-nbctl ls-del ls-test", false},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			gremlin := c.gremlin.V().Has("Name", "ls-test", "Type", "logical_switch")

			_, err := c.gh.GetNode(gremlin)
			if err != nil {
				return fmt.Errorf("Failed to find a logical switch 'ls-test'")
			}

			return nil
		}},
	}

	RunTest(t, test)
}

func TestOVNLpAdd(t *testing.T) {
	test := &Test{
		setupCmds: []Cmd{
			{"ovn-nbctl ls-add ls-test", false},
			{"ovn-nbctl lsp-add ls-test lsp-test", false},
		},

		tearDownCmds: []Cmd{
			{"ovn-nbctl lsp-del lsp-test", false},
			{"ovn-nbctl ls-del ls-test", false},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			gremlin := c.gremlin.V().Has("Name", "ls-test", "Type", "logical_switch")
			gremlin = gremlin.Out("Type", "logical_port", "Name", "lsp-test")

			_, err := c.gh.GetNode(gremlin)
			if err != nil {
				return fmt.Errorf("Failed to find a logical port 'lsp-test' on switch 'ls-test'")
			}

			return nil
		}},
	}

	RunTest(t, test)
}

func TestOVNAclAdd(t *testing.T) {
	test := &Test{
		setupCmds: []Cmd{
			{"ovn-nbctl ls-add ls-test", false},
			{"ovn-nbctl acl-add ls-test from-lport 1000 eth.type==0x1234 drop", false},
		},

		tearDownCmds: []Cmd{
			{"ovn-nbctl acl-del ls-test from-lport 1000 eth.type==0x1234", false},
			{"ovn-nbctl ls-del ls-test", false},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			gremlin := c.gremlin.V().Has("Name", "ls-test", "Type", "logical_switch")
			gremlin = gremlin.Out("Type", "acl")

			_, err := c.gh.GetNode(gremlin)
			if err != nil {
				return fmt.Errorf("Failed to find an ACL on switch 'ls-test'")
			}

			return nil
		}},
	}

	RunTest(t, test)
}

func TestOVNRouting(t *testing.T) {
	test := &Test{
		setupCmds: []Cmd{
			{"ovn-nbctl ls-add inside", false},
			{"ovn-nbctl ls-add dmz", false},

			{"ovn-nbctl lr-add tenant1", false},

			{"ovn-nbctl lrp-add tenant1 tenant1-dmz 02:ac:10:ff:01:29 172.16.255.129/26", false},
			{"ovn-nbctl lsp-add dmz dmz-tenant1", false},
			{"ovn-nbctl lsp-set-type dmz-tenant1 router", false},
			{"ovn-nbctl lsp-set-addresses dmz-tenant1 02:ac:10:ff:01:29", false},
			{"ovn-nbctl lsp-set-options dmz-tenant1 router-port=tenant1-dmz", false},

			{"ovn-nbctl lrp-add tenant1 tenant1-inside 02:ac:10:ff:01:93 172.16.255.193/26", false},
			{"ovn-nbctl lsp-add inside inside-tenant1", false},
			{"ovn-nbctl lsp-set-type inside-tenant1 router", false},
			{"ovn-nbctl lsp-set-addresses inside-tenant1 02:ac:10:ff:01:93", false},
			{"ovn-nbctl lsp-set-options inside-tenant1 router-port=tenant1-inside", false},

			{"ovs-vsctl add-br br-ovn", false},

			{"ovn-nbctl lsp-add dmz dmz-vm1", false},
			{"ovn-nbctl lsp-set-addresses dmz-vm1 \"02:ac:10:ff:01:30 172.16.255.130\"", false},
			{"ovn-nbctl lsp-set-port-security dmz-vm1 \"02:ac:10:ff:01:30 172.16.255.130\"", false},

			{"ovn-nbctl lsp-add inside inside-vm3", false},
			{"ovn-nbctl lsp-set-addresses inside-vm3 \"02:ac:10:ff:01:94 172.16.255.194\"", false},
			{"ovn-nbctl lsp-set-port-security inside-vm3 \"02:ac:10:ff:01:94 172.16.255.194\"", false},

			{"ip netns add vm1", false},
			{"ovs-vsctl add-port br-ovn vm1 -- set interface vm1 type=internal", false},
			{"ip link set vm1 address 02:ac:10:ff:01:30", false},
			{"ip link set vm1 netns vm1", false},
			{"ovs-vsctl set Interface vm1 external_ids:iface-id=dmz-vm1", false},

			{"ip netns add vm3", false},
			{"ovs-vsctl add-port br-ovn vm3 -- set interface vm3 type=internal", false},
			{"ip link set vm3 address 02:ac:10:ff:01:94", false},
			{"ip link set vm3 netns vm3", false},
			{"ovs-vsctl set Interface vm3 external_ids:iface-id=inside-vm3", false},
		},

		tearDownCmds: []Cmd{
			{"ip netns del vm1", false},
			{"ip netns del vm3", false},
			{"ovs-vsctl del-br br-ovn", false},
			{"ovn-nbctl lr-del tenant1", false},
			{"ovn-nbctl ls-del inside", false},
			{"ovn-nbctl ls-del dmz", false},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			gremlin := c.gremlin.V().Has("Name", "dmz", "Type", "logical_switch")
			_, err := c.gh.GetNode(gremlin)
			if err != nil {
				return fmt.Errorf("Failed to find switch 'dmz': %s", err)
			}

			gremlin = gremlin.Out().Has("Name", "dmz-tenant1", "Type", "logical_port")
			if _, err = c.gh.GetNode(gremlin); err != nil {
				return fmt.Errorf("Failed to find port 'dmz-tenant1': %s", err)
			}

			gremlin = gremlin.Out().Has("Name", "tenant1-dmz", "Type", "logical_port")
			if _, err = c.gh.GetNode(gremlin); err != nil {
				return fmt.Errorf("Failed to find router port 'tenant1-dmz': %s", err)
			}

			gremlin = gremlin.In().Has("Name", "tenant1", "Type", "logical_router")
			if _, err = c.gh.GetNode(gremlin); err != nil {
				return fmt.Errorf("Failed to find logical router 'tenant1': %s", err)
			}

			gremlin = gremlin.Out().Has("Name", "tenant1-inside", "Type", "logical_port")
			if _, err = c.gh.GetNode(gremlin); err != nil {
				return fmt.Errorf("Failed to find logical port 'tenant1-inside': %s", err)
			}

			gremlin = gremlin.In().Has("Name", "inside-tenant1", "Type", "logical_port")
			if _, err = c.gh.GetNode(gremlin); err != nil {
				return fmt.Errorf("Failed to find logical port 'inside-tenant1': %s", err)
			}

			gremlin = gremlin.In().Has("Name", "inside", "Type", "logical_switch")
			if _, err = c.gh.GetNode(gremlin); err != nil {
				return fmt.Errorf("Failed to find logical switch 'inside': %s", err)
			}

			gremlin = gremlin.Out().Has("Name", "inside-vm3", "Type", "logical_port")
			if _, err = c.gh.GetNode(gremlin); err != nil {
				return fmt.Errorf("Failed to find logical switch 'inside': %s", err)
			}

			gremlin = gremlin.In()
			if _, err = c.gh.GetNodes(gremlin); err != nil {
				return fmt.Errorf("Failed to find parent node for 'inside-vm3': %s", err)
			}

			gremlin = gremlin.Has("Name", "vm3", "Driver", "openvswitch")
			if _, err = c.gh.GetNode(gremlin); err != nil {
				bytes, _ := c.gh.Query(gremlin.String())
				return fmt.Errorf("Failed to find interface 'vm3' linked to switch 'inside-vm3': %s (%s) (%s)", err, gremlin, string(bytes))
			}

			return nil
		}},
	}

	RunTest(t, test)
}
