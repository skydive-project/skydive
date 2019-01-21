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
	"testing"
)

func TestSRIOV(t *testing.T) {
	test := &Test{
		setupFunction: func(c *TestContext) error {
			return ioutil.WriteFile("/sys/class/net/enp5s0f0/device/sriov_numvfs", []byte("2"), 0664)
		},

		tearDownFunction: func(c *TestContext) error {
			return ioutil.WriteFile("/sys/class/net/enp5s0f0/device/sriov_numvfs", []byte("0"), 0664)
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			gremlin := c.gremlin.V().Has("Name", "enp5s0f0").OutE("RelationType", "vf").OutV()
			nodes, err := c.gh.GetNodes(gremlin)
			if err != nil {
				return err
			}

			if len(nodes) != 2 {
				return fmt.Errorf("Expected 2 nodes, got %+v", nodes)
			}

			return nil
		}},
	}
	RunTest(t, test)
}
