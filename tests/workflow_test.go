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
	"net"
	"testing"
	"time"

	"github.com/mitchellh/mapstructure"

	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/js"
)

func lookupWorkflow(client *shttp.CrudClient, name string) (*types.Workflow, error) {
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

			result, err := runtime.ExecFunction(checkMTU.Source, srcNodeTID, dstNodeTID)
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

func flowMatrixServer(t *testing.T, readyChan chan bool) net.Listener {
	l, err := net.Listen("tcp", ":22222")
	if err != nil {
		t.Fatal(err)
	}
	readyChan <- true

	go func() {

		c, err := l.Accept()
		if err != nil {
			t.Fatal(err)
		}

		for {
			var buf []byte

			if _, err := c.Write(buf); err != nil {
				return
			}

			time.Sleep(100 * time.Millisecond)
		}
	}()

	return l
}

func flowMatrixClient(t *testing.T) {
	c, err := net.Dial("tcp", "127.0.0.1:22222")
	if err != nil {
		t.Fatal(err)
	}
	for {
		var buf []byte

		if _, err := c.Read(buf); err != nil {
			return
		}
	}
}

func TestFlowMatrixWorkflow(t *testing.T) {
	runtime, err := js.NewRuntime()
	if err != nil {
		t.Fatal(err)
	}

	var l net.Listener
	readyChan := make(chan bool, 1)

	test := &Test{
		setupFunction: func(c *TestContext) error {
			runtime.Start()
			runtime.RegisterAPIClient(c.client)

			l = flowMatrixServer(t, readyChan)
			<-readyChan

			go flowMatrixClient(t)

			return nil
		},

		tearDownFunction: func(c *TestContext) error {
			l.Close()

			return nil
		},

		checks: []CheckFunction{func(c *CheckContext) error {
			flowMatrix, err := lookupWorkflow(c.client, "FlowMatrix")
			if err != nil {
				return err
			}

			result, err := runtime.ExecFunction(flowMatrix.Source, false, "graph", true)
			if err != nil {
				return fmt.Errorf("Error while calling workflow: %s", err)
			}

			subgraph := &struct {
				Nodes map[string]*graph.Node `mapstructure:"nodes"`
				Edges map[string]*graph.Edge `mapstructure:"edges"`
			}{}

			obj, err := result.Export()
			if err != nil {
				return fmt.Errorf("Failed to export: %s", err)
			}

			if err := mapstructure.Decode(obj, subgraph); err != nil {
				return fmt.Errorf("Failed to map: %s", err)
			}

			check, err := result.ToBoolean()
			if err != nil {
				return fmt.Errorf("Expected workflow to return a boolean: %s", err)
			}

			if !check {
				return fmt.Errorf("Expected FlowMatrix workflow to return true")
			}

			backend, _ := graph.NewMemoryBackend()
			g := graph.NewGraph("test", backend, common.UnknownService)

			for _, n := range subgraph.Nodes {
				g.NodeAdded(n)
			}

			for _, e := range subgraph.Edges {
				g.EdgeAdded(e)
			}

			tr := traversal.NewGraphTraversal(g, false)

			ctx := traversal.StepContext{}
			tv := tr.V(ctx).Has(ctx, "Name", "functionals", "ServiceType", "client").OutE(ctx).Has(ctx, "RelationType", "connection", "Port", 22222).OutV(ctx).Has(ctx, "ServiceType", "server", "Name", "functionals")
			if len(tv.Values()) == 0 {
				return fmt.Errorf("Should return at least one connection node, returned: %v, graph: %v, subgraph: %v", tv.Values(), g.String(), subgraph)
			}

			return nil
		}},
	}

	RunTest(t, test)
}
