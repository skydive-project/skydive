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
	"github.com/skydive-project/skydive/topology/graph"
)

func k8sConfigFile(name string) string {
	return "./k8s/" + name + ".yaml"
}

const (
	clusterName = "cluster"
	k8sRetry    = 3
	k8sDelay    = 10 * time.Second
	manager     = "k8s"
	objName     = "skydive-test"
)

var nodeName, _ = os.Hostname()

func setupFromConfigFile(file string) []helper.Cmd {
	return []helper.Cmd{
		{"kubectl create -f " + k8sConfigFile(file), true},
	}
}

func tearDownFromConfigFile(file string) []helper.Cmd {
	return []helper.Cmd{
		{"kubectl delete --grace-period=0 --force -f " + k8sConfigFile(file), false},
		{"sleep 2", true},
	}
}

func makeHasArgsType(ty interface{}, args1 ...interface{}) []interface{} {
	args := []interface{}{"Manager", "k8s", "Type", ty}
	args = append(args, args1...)
	return args
}

func makeHasArgsNode(node *graph.Node, args1 ...interface{}) []interface{} {
	m := node.Metadata()
	args := []interface{}{"Namespace", m["Namespace"], "Name", m["Name"]}
	args = append(args, args1...)
	return makeHasArgsType(m["Type"], args...)
}

func queryNodeCreation(t *testing.T, c *CheckContext, query g.QueryString) (node *graph.Node, err error) {
	node = nil
	err = common.Retry(func() error {
		const expectedNumNodes = 1

		t.Logf("Executing query '%s'", query)
		nodes, e := c.gh.GetNodes(query.String())
		if e != nil {
			e = fmt.Errorf("Failed executing query '%s': %s", query, e)
			t.Logf("%s", e)
			return e
		}

		if len(nodes) != expectedNumNodes {
			e = fmt.Errorf("Ran '%s', expected %d node, got %d nodes: %+v", query, expectedNumNodes, len(nodes), nodes)
			t.Logf("%s", e)
			return e
		}

		if expectedNumNodes > 0 {
			node = nodes[0]
		}
		return nil
	}, k8sRetry, k8sDelay)
	return
}

func checkNodeCreation(t *testing.T, c *CheckContext, ty string, values ...interface{}) (*graph.Node, error) {
	args := makeHasArgsType(ty, values...)
	query := c.gremlin.V().Has(args...)
	return queryNodeCreation(t, c, query)
}

func checkEdgeCreation(t *testing.T, c *CheckContext, from, to *graph.Node, relType string) error {
	fromArgs := makeHasArgsNode(from)
	toArgs := makeHasArgsNode(to)
	query := c.gremlin.V().Has(fromArgs...).OutE().Has("RelationType", relType).OutV().Has(toArgs...)
	_, err := queryNodeCreation(t, c, query)
	return err
}

func testRunner(t *testing.T, setupCmds, tearDownCmds []helper.Cmd, checks []CheckFunction) {
	test := &Test{
		mode:         Replay,
		retries:      1,
		preCleanup:   true,
		setupCmds:    setupCmds,
		tearDownCmds: tearDownCmds,
		checks:       checks,
	}
	RunTest(t, test)
}

func testNodeCreation(t *testing.T, setupCmds, tearDownCmds []helper.Cmd, typ, name string, fields ...string) {
	testRunner(t, setupCmds, tearDownCmds, []CheckFunction{
		func(c *CheckContext) error {
			obj, err := checkNodeCreation(t, c, typ, "Name", name)
			if err != nil {
				return err
			}

			m := obj.Metadata()
			for _, field := range fields {
				if _, ok := m[field]; !ok {

					return fmt.Errorf("Node '%s %s' missing field: %s", typ, name, field)
				}
			}

			return nil
		},
	})
}

func testNodeCreationFromConfig(t *testing.T, ty, name string, fields ...string) {
	file := ty
	setup := setupFromConfigFile(file)
	tearDown := tearDownFromConfigFile(file)
	testNodeCreation(t, setup, tearDown, ty, name, fields...)
}

/* -- test creation of single resource -- */
func TestK8sClusterNode(t *testing.T) {
	testNodeCreation(t, nil, nil, "cluster", clusterName)
}

func TestK8sContainerNode(t *testing.T) {
	testNodeCreationFromConfig(t, "container", objName+"-container", "Image", "Labels", "Pod")
}

func TestK8sCronJobNode(t *testing.T) {
	testNodeCreationFromConfig(t, "cronjob", objName+"-cronjob")
}

func TestK8sDeploymentNode(t *testing.T) {
	testNodeCreationFromConfig(t, "deployment", objName+"-deployment", "Selector", "DesiredReplicas", "Replicas", "ReadyReplicas", "AvailableReplicas", "UnavailableReplicas")
}

func TestK8sIngressNode(t *testing.T) {
	testNodeCreationFromConfig(t, "ingress", objName+"-ingress", "Backend", "TLS", "Rules")
}

func TestK8sJobNode(t *testing.T) {
	testNodeCreationFromConfig(t, "job", objName+"-job", "Parallelism", "Completions", "Active", "Succeeded", "Failed")
}

func TestK8sNamespaceNode(t *testing.T) {
	testNodeCreationFromConfig(t, "namespace", objName+"-namespace", "Cluster", "Labels", "Status")
}

func TestK8sDaemonSetNode(t *testing.T) {
	testNodeCreationFromConfig(t, "daemonset", objName+"-daemonset", "Labels", "DesiredNumberScheduled", "CurrentNumberScheduled", "NumberMisscheduled")
}

func TestK8sNetworkPolicyNode(t *testing.T) {
	testNodeCreationFromConfig(t, "networkpolicy", objName+"-networkpolicy", "Labels", "PodSelector")
}

func TestK8sNodeNode(t *testing.T) {
	testNodeCreation(t, nil, nil, "node", nodeName, "Arch", "Cluster", "Hostname", "InternalIP", "Labels", "OS")
}

func TestK8sPersistentVolumeNode(t *testing.T) {
	testNodeCreationFromConfig(t, "persistentvolume", objName+"-persistentvolume")
}

func TestK8sPersistentVolumeClaimNode(t *testing.T) {
	testNodeCreationFromConfig(t, "persistentvolumeclaim", objName+"-persistentvolumeclaim")
}

func TestK8sPodNode(t *testing.T) {
	testNodeCreationFromConfig(t, "pod", objName+"-pod", "Node", "Status")
}

func TestK8sReplicaSetNode(t *testing.T) {
	testNodeCreationFromConfig(t, "replicaset", objName+"-replicaset")
}

func TestK8sReplicationControllerNode(t *testing.T) {
	testNodeCreationFromConfig(t, "replicationcontroller", objName+"-replicationcontroller")
}

func TestK8sServiceNode(t *testing.T) {
	testNodeCreationFromConfig(t, "service", objName+"-service")
}

func TestK8sStatefulSetNode(t *testing.T) {
	testNodeCreationFromConfig(t, "statefulset", objName+"-statefulset")
}

func TestK8sStorageClassNode(t *testing.T) {
	testNodeCreationFromConfig(t, "storageclass", objName+"-storageclass")
}

/* -- test multi-node scenarios -- */
func TestHelloNodeScenario(t *testing.T) {
	testRunner(
		t,
		[]helper.Cmd{
			{"kubectl run hello-node --image=hello-node:v1 --port=8080", true},
		},
		[]helper.Cmd{
			{"kubectl delete --grace-period=0 --force deploy hello-node", false},
		},
		[]CheckFunction{
			func(c *CheckContext) error {
				// check nodes exist
				cluster, err := checkNodeCreation(t, c, "cluster")
				if err != nil {
					return err
				}

				container, err := checkNodeCreation(t, c, "container", "Name", "hello-node")
				if err != nil {
					return err
				}

				deployment, err := checkNodeCreation(t, c, "deployment", "Name", "hello-node")
				if err != nil {
					return err
				}

				namespace, err := checkNodeCreation(t, c, "namespace", "Name", "default")
				if err != nil {
					return err
				}

				pod, err := checkNodeCreation(t, c, "pod", "Name", g.Regex("%s-.*", "hello-node"))
				if err != nil {
					return err
				}

				// check edges exist
				if err = checkEdgeCreation(t, c, cluster, namespace, "ownership"); err != nil {
					return err
				}

				if err = checkEdgeCreation(t, c, namespace, deployment, "ownership"); err != nil {
					return err
				}

				if err = checkEdgeCreation(t, c, namespace, pod, "ownership"); err != nil {
					return err
				}

				if err = checkEdgeCreation(t, c, pod, container, "ownership"); err != nil {
					return err
				}
				return nil
			},
		},
	)
}

func TestK8sNetworkPolicyScenario1(t *testing.T) {
	file := "networkpolicy1"
	name := objName + "-" + file
	testRunner(
		t,
		setupFromConfigFile(file),
		tearDownFromConfigFile(file),
		[]CheckFunction{
			func(c *CheckContext) error {
				networkpolicy, err := checkNodeCreation(t, c, "networkpolicy", "Name", name)
				if err != nil {
					return err
				}

				namespace, err := checkNodeCreation(t, c, "namespace", "Name", name)
				if err != nil {
					return err
				}

				if err = checkEdgeCreation(t, c, networkpolicy, namespace, "association"); err != nil {
					return err
				}

				return nil
			},
		},
	)
}

func TestK8sNetworkPolicyScenario2(t *testing.T) {
	file := "networkpolicy2"
	name := objName + "-" + file
	testRunner(
		t,
		setupFromConfigFile(file),
		tearDownFromConfigFile(file),
		[]CheckFunction{
			func(c *CheckContext) error {
				networkpolicy, err := checkNodeCreation(t, c, "networkpolicy", "Name", name)
				if err != nil {
					return err
				}

				pod, err := checkNodeCreation(t, c, "pod", "Name", name)
				if err != nil {
					return err
				}

				if err = checkEdgeCreation(t, c, networkpolicy, pod, "association"); err != nil {
					return err
				}

				return nil
			},
		},
	)
}
