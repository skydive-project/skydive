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
	"testing"
	"time"

	g "github.com/skydive-project/skydive/gremlin"
	"github.com/skydive-project/skydive/topology/probe/vpp"
)

func createLoopback(t *testing.T) string {
	cmd := "vppctl loopback create-interface | tr -d '\r'"
	out, err := exec.Command("/bin/sh", "-c", cmd).CombinedOutput()
	if err != nil {
		t.Error("Can't create vpp loopback interface ", err)
	}
	time.Sleep(vpp.VPPPollingTime * time.Millisecond)
	return string(out)
}

func deleteLoopback(t *testing.T, intf string) string {
	cmd := "vppctl loopback delete-interface intfc " + intf
	out, err := exec.Command("/bin/sh", "-c", cmd).CombinedOutput()
	if err != nil {
		t.Error("Can't delete vpp loopback interface ", intf, " : ", err)
	}
	time.Sleep(vpp.VPPPollingTime * time.Millisecond)
	return string(out)
}

func TestVPPLoopback(t *testing.T) {
	test := &Test{
		setupCmds: []Cmd{
			{"vppctl show interface", true},
		},

		tearDownCmds: []Cmd{
			{"vppctl show interface", true},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			loop1 := createLoopback()
			gremlin := c.gremlin.Has("Driver", "vpp", "Name", loop1)
			if len(gremlin) < 1 {
				return fmt.Errorf("Expected one interface, got %+v", gremlin)
			}

			loop2 := createLoopback()
			gremlin := c.gremlin.Has("Driver", "vpp", "Name", loop2)
			if len(gremlin) < 1 {
				return fmt.Errorf("Expected one interface, got %+v", gremlin)
			}

			deleteLoopback(loop1)
			gremlin := c.gremlin.Has("Driver", "vpp", "Name", loop1)
			if len(gremlin) != 0 {
				return fmt.Errorf("Expected no interface, got %+v", gremlin)
			}

			deleteLoopback(loop2)

			return nil
		}},
	}
	RunTest(t, test)
}
