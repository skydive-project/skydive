// +build k8s

/*
 * Copyright (C) 2018 IBM, Inc.
 *
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 *
 */

package tests

import (
	"fmt"
	"os"
	"testing"

	g "github.com/skydive-project/skydive/gremlin"
	"github.com/skydive-project/skydive/tests/helper"
)

func k8sConfigFile(name string) string {
	return "./k8s/" + name + ".yaml"
}

func k8sObjectName(name string) string {
	prefix := "skydive-test"
	if name != "" {
		return prefix + "-" + name
	}
	return prefix
}

const (
	manager = "k8s"
)

var (
	nodeName, _       = os.Hostname()
	podName           = k8sObjectName("pod")
	containerName     = k8sObjectName("container")
	networkPolicyName = k8sObjectName("")
	namespaceName     = k8sObjectName("")
)

func makeCmdWaitUntilStatus(ty, name, status string) string {
	return fmt.Sprintf("echo 'for i in {1..10}; do sleep 1; kubectl get %s %s %s break; done' | bash", ty, name, status)
}

func makeCmdWaitUntilCreated(ty, name string) string {
	return makeCmdWaitUntilStatus(ty, name, "&&")
}

func makeCmdWaitUntilDeleted(ty, name string) string {
	return makeCmdWaitUntilStatus(ty, name, "||")
}

func setupFromDeploymnet(name string) []helper.Cmd {
	return []helper.Cmd{
		{"kubectl run " + k8sObjectName(name) +
			"  --image=gcr.io/google_containers/echoserver:1.4" +
			"  --port=8080", true},
		{makeCmdWaitUntilCreated("deployment", k8sObjectName(name)), true},
	}
}

func tearDownFromDeployment(name string) []helper.Cmd {
	return []helper.Cmd{
		{"kubectl delete deployment " + k8sObjectName(name), false},
		{makeCmdWaitUntilDeleted("deployment", k8sObjectName(name)), true},
	}
}

func setupFromConfigFile(ty, name string) []helper.Cmd {
	return []helper.Cmd{
		{"kubectl create -f " + k8sConfigFile(ty), true},
		{makeCmdWaitUntilCreated(ty, name), true},
	}
}

func tearDownFromConfigFile(ty, name string) []helper.Cmd {
	return []helper.Cmd{
		{"kubectl delete -f " + k8sConfigFile(ty), false},
		{makeCmdWaitUntilDeleted(ty, name), true},
	}
}

func testNodeCreation(t *testing.T, setupCmds, tearDownCmds []helper.Cmd, ty, name g.ValueString) {
	test := &Test{
		mode:         OneShot,
		retries:      3,
		setupCmds:    append(tearDownCmds, setupCmds...),
		tearDownCmds: tearDownCmds,
		checks: []CheckFunction{func(c *CheckContext) error {
			query := g.G.V().Has(g.Quote("Manager"), g.Quote("k8s"), g.Quote("Type"), ty, g.Quote("Name"), name)
			fmt.Printf("Gremlin Query: %s\n", query)

			nodes, err := c.gh.GetNodes(query.String())
			if err != nil {
				return err
			}

			if len(nodes) != 1 {
				return fmt.Errorf("Ran \"%s\", expected 1 node, got %d nodes: %+v", query, len(nodes), nodes)
			}

			return nil
		}},
	}
	RunTest(t, test)
}

func TestK8sContainerNode(t *testing.T) {
	testNodeCreation(t, setupFromDeploymnet("container"), tearDownFromDeployment("container"), g.Quote("container"), g.Quote(containerName))
}

func TestK8sPodNode(t *testing.T) {
	testNodeCreation(t, setupFromDeploymnet("pod"), tearDownFromDeployment("pod"), g.Quote("pod"), g.StartsWith(podName))
}

func TestK8sNetworkPolicyNode(t *testing.T) {
	testNodeCreation(t, setupFromConfigFile("networkpolicy", networkPolicyName), tearDownFromConfigFile("networkpolicy", networkPolicyName), g.Quote("networkpolicy"), g.Quote(networkPolicyName))
}

func TestK8sNodeNode(t *testing.T) {
	testNodeCreation(t, nil, nil, g.Quote("node"), g.Quote(nodeName))
}

func TestK8sNamespaceNode(t *testing.T) {
	testNodeCreation(t, setupFromConfigFile("namespace", namespaceName), tearDownFromConfigFile("namespace", namespaceName), g.Quote("namespace"), g.Quote(namespaceName))
}
