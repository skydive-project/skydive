// +build vpp

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
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/topology/probes/vpp"
)

func createLoopback(t *testing.T) string {
	cmd := "vppctl loopback create-interface"
	out, err := exec.Command("/bin/sh", "-c", cmd).CombinedOutput()
	if err != nil {
		t.Error("Can't create vpp loopback interface ", err)
	}
	time.Sleep(2 * vpp.VPPPollingTime * time.Millisecond)
	return strings.Trim(string(out), "\r\n")
}

func deleteLoopback(t *testing.T, intf string) string {
	cmd := "vppctl loopback delete-interface intfc " + intf
	out, err := exec.Command("/bin/sh", "-c", cmd).CombinedOutput()
	if err != nil {
		t.Error("Can't delete vpp loopback interface ", intf, " : ", err)
	}
	time.Sleep(2 * vpp.VPPPollingTime * time.Millisecond)
	return strings.Trim(string(out), "\r\n")
}

func TestVPPLoopback(t *testing.T) {
	test := &Test{
		setupCmds: []Cmd{
			{"vppctl show interface", true},
		},

		tearDownCmds: []Cmd{
			{"vppctl show interface", true},
		},

		mode: OneShot,

		checks: []CheckFunction{func(c *CheckContext) error {
			var nodes []*graph.Node
			var err error
			loop1 := createLoopback(t)
			gremlin := c.gremlin.V().Has("Driver", "vpp", "Name", loop1)
			if nodes, err = c.gh.GetNodes(gremlin); err != nil {
				return err
			}
			if len(nodes) < 1 {
				return fmt.Errorf("Expected one interface, got %+v", nodes)
			}

			loop2 := createLoopback(t)
			gremlin = c.gremlin.V().Has("Driver", "vpp", "Name", loop2)
			if nodes, err = c.gh.GetNodes(gremlin); err != nil {
				return err
			}
			if len(nodes) < 1 {
				return fmt.Errorf("Expected one interface, got %+v", nodes)
			}

			deleteLoopback(t, loop1)
			gremlin = c.gremlin.V().Has("Driver", "vpp", "Name", loop1)
			if nodes, err = c.gh.GetNodes(gremlin); err != nil {
				return err
			}
			if len(nodes) != 0 {
				return fmt.Errorf("Expected no interface, got %+v", nodes)
			}

			deleteLoopback(t, loop2)

			return nil
		}},
	}
	RunTest(t, test)
}
