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

	"github.com/mitchellh/mapstructure"

	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/js"
)

func lookupWorkflow(client *http.CrudClient, name string) (*types.Workflow, error) {
	var workflows map[string]types.Workflow
	if err := client.List("workflow", &workflows); err != nil {
		return nil, err
	}

	for _, workflow := range workflows {
		if workflow.Name == name {
			return &workflow, nil
		}
	}

	return nil, fmt.Errorf("Could not find workflow %s", name)
}
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
			checkMTU, err := lookupWorkflow(c.client, "CheckMTU")
			if err != nil {
				return err
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

			result, err := runtime.ExecPromise(checkMTU.Source, srcNodeTID, dstNodeTID)
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

func TestFlowMatrixWorkflow(t *testing.T) {
	runtime, err := js.NewRuntime()
	if err != nil {
		t.Fatal(err)
	}

	test := &Test{
		setupFunction: func(c *TestContext) error {
			runtime.Start()
			runtime.RegisterAPIClient(c.client)

			return nil
		},

		tearDownFunction: func(c *TestContext) error {
			return nil
		},

		checks: []CheckFunction{func(c *CheckContext) error {
			flowMatrix, err := lookupWorkflow(c.client, "FlowMatrix")
			if err != nil {
				return err
			}

			result, err := runtime.ExecPromise(flowMatrix.Source, false, "graph", true)
			if err != nil {
				return fmt.Errorf("Error while calling workflow: %s", err)
			}

			g := &struct {
				Nodes map[string]graph.Node `mapstructure:"nodes"`
				Edges map[string]graph.Edge `mapstructure:"edges"`
			}{}

			obj, err := result.Export()
			if err != nil {
				return fmt.Errorf("Failed to export: %s", err)
			}

			if err := mapstructure.Decode(obj, g); err != nil {
				return fmt.Errorf("Failed to map: %s", err)
			}

			check, err := result.ToBoolean()
			if err != nil {
				return fmt.Errorf("Expected workflow to return a boolean: %s", err)
			}

			if !check {
				return fmt.Errorf("Expected FlowMatrix workflow to return true")
			}

			sa, err := common.ServiceAddressFromString(analyzerListen)
			if err != nil {
				return err
			}

			var process *graph.Node
			for _, n := range g.Nodes {
				name, _ := n.GetFieldString("Name")
				port, _ := n.GetFieldInt64("Port")
				if name == "functionals" && port == int64(sa.Port) {
					process = &n
					break
				}
			}

			if process == nil {
				return fmt.Errorf("Expected to find a process named 'functionals' with port %d", sa.Port)
			}

			var connection *graph.Edge
			for _, e := range g.Edges {
				port, _ := e.GetFieldInt64("ServicePort")
				relationType, _ := e.GetFieldString("RelationType")
				if e.Parent == process.ID && e.Child == process.ID && port == int64(sa.Port) && relationType == "connection" {
					connection = &e
					break
				}
			}

			if connection == nil {
				return fmt.Errorf("Expected to find a connection 'functionals' and itself on port 64500")
			}

			return nil
		}},
	}

	RunTest(t, test)
}
