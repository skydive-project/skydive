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
	g "github.com/skydive-project/skydive/gremlin"
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

func TestCheckConnectivityWorkflow(t *testing.T) {
	runtime, err := js.NewRuntime()
	if err != nil {
		t.Fatal(err)
	}

	test := &Test{
		retries: 5,

		setupCmds: []Cmd{
			{"ovs-vsctl add-br br-int", true},

			{"ip netns add vm1", true},
			{"ip link add vm1-eth0 type veth peer name intf1 netns vm1", true},
			{"ip link set vm1-eth0 up", true},
			{"ip netns exec vm1 ip link set intf1 up", true},
			{"ip netns exec vm1 ip address add 192.168.0.1/24 dev intf1", true},

			{"ip netns add vm2", true},
			{"ip link add vm2-eth0 type veth peer name intf2 netns vm2", true},
			{"ip link set vm2-eth0 up", true},
			{"ip netns exec vm2 ip link set intf2 up", true},
			{"ip netns exec vm2 ip address add 192.168.0.2/24 dev intf2", true},

			{"ovs-vsctl add-port br-int vm1-eth0", true},
			{"ovs-vsctl add-port br-int vm2-eth0", true},
		},

		tearDownCmds: []Cmd{
			{"ovs-vsctl del-br br-int", true},
			{"ip link del vm1-eth0", true},
			{"ip link del vm2-eth0", true},
			{"ip netns del vm1", true},
			{"ip netns del vm2", true},
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
			checkConnectivity, err := lookupWorkflow(c.client, "CheckConnectivity")
			if err != nil {
				return err
			}

			srcNode, err := c.gh.GetNode(c.gremlin.V().Has("Name", "intf1"))
			if err != nil {
				return err
			}

			dstNode, err := c.gh.GetNode(c.gremlin.V().Has("Name", "intf2"))
			if err != nil {
				return err
			}

			srcNodeTID, _ := srcNode.GetFieldString("TID")
			dstNodeTID, _ := dstNode.GetFieldString("TID")

			result, err := runtime.ExecFunction(checkConnectivity.Source, srcNodeTID, dstNodeTID)
			if err != nil {
				return fmt.Errorf("Error while calling workflow: %s", err)
			}

			obj, err := result.Export()
			if err != nil {
				return fmt.Errorf("Failed to export: %s", err)
			}

			output := &struct {
				Connectivity bool          `mapstructure:"connectivity"`
				Flows        []interface{} `mapstructure:"flows"`
			}{}

			if err := mapstructure.Decode(obj, output); err != nil {
				return fmt.Errorf("Failed to decode %+v: %s", obj, err)
			}

			if !output.Connectivity {
				return fmt.Errorf("Expected CheckConnectivity workflow to return true and got %v", obj)
			}

			return nil
		}},
	}

	RunTest(t, test)
}

func TestCheckMTUWorkflow(t *testing.T) {
	runtime, err := js.NewRuntime()
	if err != nil {
		t.Fatal(err)
	}

	test := &Test{
		retries: 5,

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
				return fmt.Errorf("Expected CheckMTU workflow to return true and got %t", check)
			}

			return nil
		}},
	}

	RunTest(t, test)
}

func TestFlowValidation1(t *testing.T) {
	runtime, err := js.NewRuntime()
	if err != nil {
		t.Fatal(err)
	}

	test := &Test{
		retries: 5,

		setupCmds: []Cmd{
			{"ovs-vsctl add-br br-int", true},

			{"ip netns add vm1", true},
			{"ip link add vm1-eth0 type veth peer name intf1 netns vm1", true},
			{"ip link set vm1-eth0 up", true},
			{"ip netns exec vm1 ip link set intf1 up", true},
			{"ip netns exec vm1 ip address add 192.168.0.1/24 dev intf1", true},

			{"ip netns add vm2", true},
			{"ip link add vm2-eth0 type veth peer name intf2 netns vm2", true},
			{"ip link set vm2-eth0 up", true},
			{"ip netns exec vm2 ip link set intf2 up", true},
			{"ip netns exec vm2 ip address add 192.168.0.2/24 dev intf2", true},

			{"ovs-vsctl add-port br-int vm1-eth0", true},
			{"ovs-vsctl add-port br-int vm2-eth0", true},
		},

		tearDownCmds: []Cmd{
			{"ovs-vsctl del-br br-int", true},
			{"ip link del vm1-eth0", true},
			{"ip link del vm2-eth0", true},
			{"ip netns del vm1", true},
			{"ip netns del vm2", true},
		},

		injections: []TestInjection{
			{from: g.G.V().Has("Name", "intf1"), to: g.G.V().Has("Name", "intf2"), count: 60, id: 123},
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
			flowValidation, err := lookupWorkflow(c.client, "FlowValidation")
			if err != nil {
				return err
			}

			checkPoint1, err := c.gh.GetNode(c.gremlin.V().Has("Name", "intf1"))
			if err != nil {
				return err
			}

			checkPoint2, err := c.gh.GetNode(c.gremlin.V().Has("Name", "intf2"))
			if err != nil {
				return err
			}

			checkPoint1TID, _ := checkPoint1.GetFieldString("TID")
			checkPoint2TID, _ := checkPoint2.GetFieldString("TID")

			result, err := runtime.ExecFunction(flowValidation.Source, checkPoint1TID, checkPoint2TID, true, "icmp", "192.168.0.1", "192.168.0.2", "123", 10)
			if err != nil {
				return fmt.Errorf("Error while calling workflow: %s", err)
			}

			obj, err := result.Export()
			if err != nil {
				return fmt.Errorf("Failed to export: %s", err)
			}

			output := &struct {
				Connectivity bool   `mapstructure:"Connectivity"`
				Error        string `mapstructure:"Error"`
			}{}

			if err := mapstructure.Decode(obj, output); err != nil {
				return fmt.Errorf("Failed to decode %+v: %s", obj, err)
			}

			if output.Error != "" || !output.Connectivity {
				return fmt.Errorf("Expected FlowValidation workflow to return a 'true' connectivity and got %+v", output)
			}

			return nil
		}},
	}

	RunTest(t, test)
}

func TestFlowValidation2(t *testing.T) {
	runtime, err := js.NewRuntime()
	if err != nil {
		t.Fatal(err)
	}

	test := &Test{
		retries: 5,

		setupCmds: []Cmd{
			{"ovs-vsctl add-br br-int", true},

			{"ip netns add vm1", true},
			{"ip link add vm1-eth0 type veth peer name intf1 netns vm1", true},
			{"ip link set vm1-eth0 up", true},
			{"ip netns exec vm1 ip link set intf1 up", true},
			{"ip netns exec vm1 ip address add 192.168.0.1/24 dev intf1", true},

			{"ip netns add vm2", true},
			{"ip link add vm2-eth0 type veth peer name intf2 netns vm2", true},
			{"ip link set vm2-eth0 up", true},
			{"ip netns exec vm2 ip link set intf2 up", true},
			{"ip netns exec vm2 ip address add 192.168.0.2/24 dev intf2", true},

			{"ovs-vsctl add-port br-int vm1-eth0", true},
			{"ovs-vsctl add-port br-int vm2-eth0", true},
			{"ovs-ofctl add-flow br-int nw_dst=192.168.0.2,actions=drop", true},
		},

		tearDownCmds: []Cmd{
			{"ovs-vsctl del-br br-int", true},
			{"ip link del vm1-eth0", true},
			{"ip link del vm2-eth0", true},
			{"ip netns del vm1", true},
			{"ip netns del vm2", true},
		},

		injections: []TestInjection{
			{from: g.G.V().Has("Name", "intf1"), to: g.G.V().Has("Name", "intf2"), count: 60, id: 123},
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
			flowValidation, err := lookupWorkflow(c.client, "FlowValidation")
			if err != nil {
				return err
			}

			checkPoint1, err := c.gh.GetNode(c.gremlin.V().Has("Name", "intf1"))
			if err != nil {
				return err
			}

			checkPoint2, err := c.gh.GetNode(c.gremlin.V().Has("Name", "intf2"))
			if err != nil {
				return err
			}

			checkPoint1TID, _ := checkPoint1.GetFieldString("TID")
			checkPoint2TID, _ := checkPoint2.GetFieldString("TID")

			result, err := runtime.ExecFunction(flowValidation.Source, checkPoint1TID, checkPoint2TID, true, "icmp", "192.168.0.1", "192.168.0.2", "123", 10)
			if err != nil {
				return fmt.Errorf("Error while calling workflow: %s", err)
			}

			obj, err := result.Export()
			if err != nil {
				return fmt.Errorf("Failed to export: %s", err)
			}

			output := &struct {
				Connectivity    bool          `mapstructure:"Connectivity"`
				NotReachedNodes []interface{} `mapstructure:"NotReachedNodes"`
				Error           string        `mapstructure:"Error"`
			}{}

			if err := mapstructure.Decode(obj, output); err != nil {
				return fmt.Errorf("Failed to decode %+v: %s", obj, err)
			}

			if output.Connectivity {
				return fmt.Errorf("Expected FlowValidation workflow to return a 'false' connectivity and got %v", obj)
			} else if len(output.NotReachedNodes) != 3 {
				return fmt.Errorf("Expected FlowValidation workflow to return that flows are not captured at 3 Nodes and got %v", obj)
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
