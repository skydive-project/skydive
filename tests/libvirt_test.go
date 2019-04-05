// +build libvirt_tests

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

func TestLibvirtBasic(t *testing.T) {
	test := &Test{
		setupCmds: []Cmd{
			{"touch /tmp/rawdisk", false},
			{"virsh net-create --file libvirt/network.xml", false},
			{"virsh create --file libvirt/vm.xml", false},
			{"virsh attach-device testvm libvirt/device.xml", false},
		},

		tearDownCmds: []Cmd{
			{"virsh destroy testvm", false},
			{"virsh net-destroy testnet", false},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			gremlin := c.gremlin.V().Has("Type", "libvirt", "Name", "testvm")
			_, err := c.gh.GetNode(gremlin)
			if err != nil {
				return fmt.Errorf("error from query '%s': %s", gremlin, err)
			}

			gremlin = gremlin.InE("RelationType", "vlayer2").InV("Name", "testintf", "Driver", "tun", "Libvirt.Alias", "net0")
			if _, err = c.gh.GetNode(gremlin); err != nil {
				return fmt.Errorf("error from query '%s': %s", gremlin, err)
			}

			gremlin = gremlin.BothE("RelationType", "layer2").BothV("Type", "bridge")
			if _, err = c.gh.GetNode(gremlin); err != nil {
				return fmt.Errorf("error from query '%s': %s", gremlin, err)
			}

			return nil
		}},
	}
	RunTest(t, test)
}
