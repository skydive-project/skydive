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
	"os/exec"
	"runtime"
	"testing"
)

func TestLxdSimple(t *testing.T) {
	if runtime.GOARCH == "ppc64le" {
		t.Skip("lxd won't run on ppc64le")
	}
	if exec.Command("grep", "CentOS", "/etc/os-release").Run() == nil {
		t.Skip("lxd won't run on CentOS")
	}
	if exec.Command("grep", "Red Hat Enterprise Linux", "/etc/os-release").Run() == nil {
		t.Skip("lxd won't run on RHEL")
	}

	test := &Test{
		setupCmds: []Cmd{
			{"lxc launch images:fedora/27 test-skydive-lxd-simple", false},
		},

		tearDownCmds: []Cmd{
			{"lxc delete --force test-skydive-lxd-simple", false},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			gremlin := c.gremlin.V().Has("Type", "netns", "Manager", "lxd")
			gremlin = gremlin.Out("Type", "container", "Name", "test-skydive-lxd-simple")

			nodes, err := c.gh.GetNodes(gremlin)
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
