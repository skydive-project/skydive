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
	"strings"
	"testing"
)

func TestRuncSimple(t *testing.T) {
	test := &Test{
		setupCmds: []Cmd{
			{"podman run -d --name nginx --label Name=test nginx", true},
		},

		tearDownCmds: []Cmd{
			{"podman kill nginx", false},
			{"podman rm -f nginx", false},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			outputLines := strings.Split(strings.TrimSpace(c.setupCmdOutputs[0]), "\n")
			containerID := outputLines[len(outputLines)-1]

			// Newer version of runc switched to bolt to store labels so
			// we disable the test on labels for now
			gremlin := c.gremlin.V().Has("Type", "container", "Manager", "runc", "Runc.ContainerID", containerID) // , "Runc.CreateConfig.Labels.Name", "test")
			gremlin = gremlin.In("Type", "netns", "Manager", "runc")

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
