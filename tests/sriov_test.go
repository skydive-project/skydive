// +build sriov_tests

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
	"io/ioutil"
	"os"
	"path"
	"strings"
	"testing"
)

const sysNet = "/sys/class/net"

func findSRIOV() string {
	files, err := ioutil.ReadDir(sysNet)
	if err != nil {
		return ""
	}
	for _, info := range files {
		device := info.Name()
		sriovFile := path.Join(sysNet, device, "device", "sriov_numvfs")
		if _, err = os.Stat(sriovFile); os.IsNotExist(err) {
			continue
		}
		operstateFile := path.Join(sysNet, device, "operstate")
		contents, err := ioutil.ReadFile(operstateFile)
		if err == nil && strings.Trim(string(contents), "\r\n") == "up" {
			return device
		}
	}
	return ""
}

func TestSRIOV(t *testing.T) {
	device := findSRIOV()
	if device == "" {
		t.Skip("No SRIOV device found.")
	}
	sriovFile := path.Join(sysNet, device, "device", "sriov_numvfs")
	test := &Test{
		setupFunction: func(c *TestContext) error {
			return ioutil.WriteFile(sriovFile, []byte("2"), 0664)
		},

		tearDownFunction: func(c *TestContext) error {
			return ioutil.WriteFile(sriovFile, []byte("0"), 0664)
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			gremlin := c.gremlin.V().Has("Name", device).OutE("RelationType", "vf").OutV()
			nodes, err := c.gh.GetNodes(gremlin)
			if err != nil {
				return err
			}
			if len(nodes) != 2 {
				return fmt.Errorf("Expected 2 nodes, got %+v (sriov intf: %s)", nodes, device)
			}
			return nil
		}},
	}
	RunTest(t, test)
}
