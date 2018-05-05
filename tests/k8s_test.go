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
	"time"

	"github.com/skydive-project/skydive/common"
	g "github.com/skydive-project/skydive/gremlin"
	"github.com/skydive-project/skydive/tests/helper"
)

func k8sConfigFile(name string) string {
	return "./k8s/" + name + ".yaml"
}

const (
	manager         = "k8s"
	objName         = "skydive-test"
	k8sRetry        = 10
	k8sDelaySeconds = 1
)

var (
	nodeName, _       = os.Hostname()
	podName           = objName
	containerName     = objName
	networkPolicyName = objName
	namespaceName     = objName
	clusterName       = "cluster"
)

func makeCmdWaitUntilStatus(ty, name, status string) string {
	return fmt.Sprintf("echo 'for i in {1..%d}; do sleep %d; kubectl get %s %s %s break; done' | bash", k8sRetry, k8sDelaySeconds, ty, name, status)
}

func makeCmdWaitUntilCreated(ty, name string) string {
	return makeCmdWaitUntilStatus(ty, name, "&&")
}

func makeCmdWaitUntilDeleted(ty, name string) string {
	return makeCmdWaitUntilStatus(ty, name, "||")
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

func testNodeCreation(t *testing.T, setupCmds, tearDownCmds []helper.Cmd, typ, name string) {
	test := &Test{
		mode:         OneShot,
		retries:      3,
		setupCmds:    append(tearDownCmds, setupCmds...),
		tearDownCmds: tearDownCmds,
		checks: []CheckFunction{func(c *CheckContext) error {
			return common.Retry(func() error {
				query := g.G.V().Has("Manager", "k8s", "Type", typ, "Name", name)
				t.Log("Gremlin Query: " + query)

				nodes, err := c.gh.GetNodes(query.String())
				if err != nil {
					return err
				}

				if len(nodes) != 1 {
					return fmt.Errorf("Ran '%s', expected 1 node, got %d nodes: %+v", query, len(nodes), nodes)
				}

				return nil
			}, k8sRetry, k8sDelaySeconds*time.Second)
		}},
	}
	RunTest(t, test)
}

func testNodeCreationFromConfig(t *testing.T, typ, name string) {
	setup := setupFromConfigFile(typ, name)
	tearDown := tearDownFromConfigFile(typ, name)
	testNodeCreation(t, setup, tearDown, typ, name)
}

func TestK8sContainerNode(t *testing.T) {
	testNodeCreationFromConfig(t, "container", containerName)
}

func TestK8sNamespaceNode(t *testing.T) {
	testNodeCreationFromConfig(t, "namespace", namespaceName)
}

func TestK8sNetworkPolicyNode(t *testing.T) {
	testNodeCreationFromConfig(t, "networkpolicy", networkPolicyName)
}

func TestK8sNodeNode(t *testing.T) {
	testNodeCreation(t, nil, nil, "node", nodeName)
}

func TestK8sClusterNode(t *testing.T) {
	testNodeCreation(t, nil, nil, "cluster", clusterName)
}

func TestK8sPodNode(t *testing.T) {
	testNodeCreationFromConfig(t, "pod", podName)
}
