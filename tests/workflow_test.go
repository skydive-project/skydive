/*
 * Copyright (C) 2015 Red Hat, Inc.
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

	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/js"
)

func TestCheckMTUWorkflow(t *testing.T) {
	runtime, err := js.NewRuntime()
	if err != nil {
		t.Fatal(err)
	}

	test := &Test{
		setupCmds: []Cmd{
			{"ip l add vm1-veth0 type veth peer name vm1-veth1", true},
		},

		tearDownCmds: []Cmd{
			{"ip link del vm1-veth0", true},
		},

		setupFunction: func(c *TestContext) error {
			runtime.Start()
			runtime.RegisterAPIClient(c.client)

			return nil
		},

		tearDownFunction: func(c *TestContext) error {
			return nil
		},

		checks: []CheckFunction{func(c *CheckContext) error {
			var workflows map[string]types.Workflow
			if err := c.client.List("workflow", &workflows); err != nil {
				return err
			}

			var checkMTU *types.Workflow
			for _, workflow := range workflows {
				if workflow.Name == "CheckMTU" {
					checkMTU = &workflow
					break
				}
			}

			if checkMTU == nil {
				return fmt.Errorf("Could not find workflow CheckMTU")
			}

			srcNode, err := c.gh.GetNode(c.gremlin.V().Has("Name", "vm1-veth0"))
			if err != nil {
				return err
			}

			dstNode, err := c.gh.GetNode(c.gremlin.V().Has("Name", "vm1-veth1"))
			if err != nil {
				return err
			}

			srcNodeTID, _ := srcNode.GetFieldString("TID")
			dstNodeTID, _ := dstNode.GetFieldString("TID")

			result, err := runtime.CallFunction(checkMTU.Source, srcNodeTID, dstNodeTID)
			if err != nil {
				return fmt.Errorf("Error while calling workflow: %s", err)
			}

			check, err := result.ToBoolean()
			if err != nil {
				return fmt.Errorf("Expected workflow to return a boolean: %s", err)
			}

			if !check {
				return fmt.Errorf("Expected CheckMTU workflow to return true")
			}

			return nil
		}},
	}

	RunTest(t, test)
}
