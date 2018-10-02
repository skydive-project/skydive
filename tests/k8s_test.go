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
	"testing"
	"time"

	"github.com/skydive-project/skydive/common"
	g "github.com/skydive-project/skydive/gremlin"
	"github.com/skydive-project/skydive/topology/graph"
	"github.com/skydive-project/skydive/topology/probes/k8s"
)

func k8sConfigFile(mngr, name string) string {
	return fmt.Sprintf("./%s/%s.yaml", mngr, name)
}

const (
	k8sRetry = 3
	k8sDelay = 10 * time.Second
	objName  = "skydive-test"
)

func setupFromConfigFile(mngr, file string) []Cmd {
	return []Cmd{
		{"kubectl create -f " + k8sConfigFile(mngr, file), true},
	}
}

func tearDownFromConfigFile(mngr, file string) []Cmd {
	return []Cmd{
		{"kubectl delete --grace-period=0 --force -f " + k8sConfigFile(mngr, file), false},
		{"sleep 5", true},
	}
}

func makeHasArgsType(mngr, ty interface{}, args1 ...interface{}) []interface{} {
	args := []interface{}{"Manager", mngr, "Type", ty}
	args = append(args, args1...)
	return args
}

func makeHasArgsNode(node *graph.Node, args1 ...interface{}) []interface{} {
	m := node.Metadata()
	args := []interface{}{}
	for _, key := range []string{"Namespace", "Name"} {
		if val, ok := m[key]; ok {
			args = append(args, key, val)
		}
	}
	args = append(args, args1...)
	return makeHasArgsType(m["Manager"], m["Type"], args...)
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

func checkNodeCreation(t *testing.T, c *CheckContext, mngr, ty string, values ...interface{}) (*graph.Node, error) {
	args := makeHasArgsType(mngr, ty, values...)
	query := c.gremlin.V().Has(args...)
	return queryNodeCreation(t, c, query)
}

func checkEdge(t *testing.T, c *CheckContext, from, to *graph.Node, relType string, edgeArgs ...interface{}) error {
	edgeArgs = append([]interface{}{"RelationType", relType}, edgeArgs...)
	fromArgs := makeHasArgsNode(from)
	toArgs := makeHasArgsNode(to)
	query := c.gremlin.V().Has(fromArgs...).OutE().Has(edgeArgs...).OutV().Has(toArgs...)
	_, err := queryNodeCreation(t, c, query)
	return err
}

func checkEdgeLink(t *testing.T, c *CheckContext, from, to *graph.Node, edgeArgs ...interface{}) error {
	return checkEdge(t, c, from, to, "association", edgeArgs...)
}

func checkEdgeNetworkPolicy(t *testing.T, c *CheckContext, from, to *graph.Node, ty k8s.PolicyType, target k8s.PolicyTarget, point k8s.PolicyPoint, edgeArgs ...interface{}) error {
	edgeArgs = append([]interface{}{
		"PolicyType", ty,
		"PolicyTarget", target,
		"PolicyPoint", point}, edgeArgs...)
	return checkEdge(t, c, from, to, "networkpolicy", edgeArgs...)
}

func checkEdgeOwnership(t *testing.T, c *CheckContext, from, to *graph.Node, edgeArgs ...interface{}) error {
	return checkEdge(t, c, from, to, "ownership", edgeArgs...)
}

func checkEdgeAssociation(t *testing.T, c *CheckContext, from, to *graph.Node, edgeArgs ...interface{}) error {
	return checkEdge(t, c, from, to, "association", edgeArgs...)
}

func checkEdgeService(t *testing.T, c *CheckContext, from, to *graph.Node, edgeArgs ...interface{}) error {
	return checkEdge(t, c, from, to, "service", edgeArgs...)
}

func testRunner(t *testing.T, setupCmds, tearDownCmds []Cmd, checks []CheckFunction) {
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

func testNodeCreation(t *testing.T, setupCmds, tearDownCmds []Cmd, mngr, typ, name string, fields ...string) {
	testRunner(t, setupCmds, tearDownCmds, []CheckFunction{
		func(c *CheckContext) error {
			var values []interface{}
			if name != "" {
				values = append(values, "Name", name)
			}
			obj, err := checkNodeCreation(t, c, mngr, typ, values...)
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

func testNodeCreationFromConfig(t *testing.T, mngr, ty, name string, fields ...string) {
	file := ty
	setup := setupFromConfigFile(mngr, file)
	tearDown := tearDownFromConfigFile(mngr, file)
	testNodeCreation(t, setup, tearDown, mngr, ty, name, fields...)
}

/* -- test creation of single resource -- */
func TestK8sClusterNode(t *testing.T) {
	testNodeCreation(t, nil, nil, k8s.Manager, "cluster", k8s.ClusterName)
}

func TestK8sContainerNode(t *testing.T) {
	testNodeCreationFromConfig(t, k8s.Manager, "container", objName+"-container", "Image", "Pod")
}

func TestK8sCronJobNode(t *testing.T) {
	testNodeCreationFromConfig(t, k8s.Manager, "cronjob", objName+"-cronjob")
}

func TestK8sDeploymentNode(t *testing.T) {
	testNodeCreationFromConfig(t, k8s.Manager, "deployment", objName+"-deployment", "Selector", "DesiredReplicas", "Replicas", "ReadyReplicas", "AvailableReplicas", "UnavailableReplicas")
}

func TestK8sEndpointsNode(t *testing.T) {
	testNodeCreationFromConfig(t, k8s.Manager, "endpoints", objName+"-endpoints")
}

func TestK8sIngressNode(t *testing.T) {
	testNodeCreationFromConfig(t, k8s.Manager, "ingress", objName+"-ingress", "Backend", "TLS", "Rules")
}

func TestK8sJobNode(t *testing.T) {
	testNodeCreationFromConfig(t, k8s.Manager, "job", objName+"-job", "Parallelism", "Completions", "Active", "Succeeded", "Failed")
}

func TestK8sNamespaceNode(t *testing.T) {
	testNodeCreationFromConfig(t, k8s.Manager, "namespace", objName+"-namespace", "Cluster", "Labels", "Status")
}

func TestK8sDaemonSetNode(t *testing.T) {
	testNodeCreationFromConfig(t, k8s.Manager, "daemonset", objName+"-daemonset", "Labels", "DesiredNumberScheduled", "CurrentNumberScheduled", "NumberMisscheduled")
}

func TestK8sNetworkPolicyNode(t *testing.T) {
	testNodeCreationFromConfig(t, k8s.Manager, "networkpolicy", objName+"-networkpolicy")
}

func TestK8sNodeNode(t *testing.T) {
	testNodeCreation(t, nil, nil, k8s.Manager, "node", "", "Arch", "Cluster", "Hostname", "InternalIP", "Labels", "OS")
}

func TestK8sPersistentVolumeNode(t *testing.T) {
	testNodeCreationFromConfig(t, k8s.Manager, "persistentvolume", objName+"-persistentvolume", "Capacity", "AccessModes", "VolumeMode", "StorageClassName", "Status")
}

func TestK8sPersistentVolumeClaimNode(t *testing.T) {
	testNodeCreationFromConfig(t, k8s.Manager, "persistentvolumeclaim", objName+"-persistentvolumeclaim", "AccessModes", "VolumeName", "StorageClassName", "VolumeMode", "Status")
}

func TestK8sPodNode(t *testing.T) {
	testNodeCreationFromConfig(t, k8s.Manager, "pod", objName+"-pod", "Node", "Status")
}

func TestK8sReplicaSetNode(t *testing.T) {
	testNodeCreationFromConfig(t, k8s.Manager, "replicaset", objName+"-replicaset")
}

func TestK8sReplicationControllerNode(t *testing.T) {
	testNodeCreationFromConfig(t, k8s.Manager, "replicationcontroller", objName+"-replicationcontroller")
}

func TestK8sServiceNode(t *testing.T) {
	testNodeCreationFromConfig(t, k8s.Manager, "service", objName+"-service", "Ports", "ClusterIP", "ServiceType", "SessionAffinity", "LoadBalancerIP", "ExternalName")
}

func TestK8sStatefulSetNode(t *testing.T) {
	testNodeCreationFromConfig(t, k8s.Manager, "statefulset", objName+"-statefulset", "DesiredReplicas", "ServiceName", "Replicas", "ReadyReplicas", "CurrentReplicas", "UpdatedReplicas", "CurrentRevision", "UpdateRevision")
}

func TestK8sStorageClassNode(t *testing.T) {
	testNodeCreationFromConfig(t, k8s.Manager, "storageclass", objName+"-storageclass")
}

/* -- test multi-node scenarios -- */
func TestK8sIngressScenario1(t *testing.T) {
	file := "ingress1"
	name := objName + "-" + file
	testRunner(
		t,
		setupFromConfigFile(k8s.Manager, file),
		tearDownFromConfigFile(k8s.Manager, file),
		[]CheckFunction{
			func(c *CheckContext) error {
				ingress, err := checkNodeCreation(t, c, k8s.Manager, "ingress", "Name", name)
				if err != nil {
					return err
				}

				service, err := checkNodeCreation(t, c, k8s.Manager, "service", "Name", name)
				if err != nil {
					return err
				}

				if err = checkEdge(t, c, ingress, service, "ingress"); err != nil {
					return err
				}

				return nil
			},
		},
	)
}

func TestHelloNodeScenario(t *testing.T) {
	testRunner(
		t,
		[]Cmd{
			{"kubectl run hello-node --image=hello-node:v1 --port=8080", true},
		},
		[]Cmd{
			{"kubectl delete --grace-period=0 --force deploy hello-node", false},
		},
		[]CheckFunction{
			func(c *CheckContext) error {
				// check nodes exist
				cluster, err := checkNodeCreation(t, c, k8s.Manager, "cluster")
				if err != nil {
					return err
				}

				container, err := checkNodeCreation(t, c, k8s.Manager, "container", "Name", "hello-node")
				if err != nil {
					return err
				}

				deployment, err := checkNodeCreation(t, c, k8s.Manager, "deployment", "Name", "hello-node")
				if err != nil {
					return err
				}

				namespace, err := checkNodeCreation(t, c, k8s.Manager, "namespace", "Name", "default")
				if err != nil {
					return err
				}

				service, err := checkNodeCreation(t, c, k8s.Manager, "service", "Name", "kubernetes")
				if err != nil {
					return err
				}

				replicaset, err := checkNodeCreation(t, c, k8s.Manager, "replicaset", "Name", g.Regex("%s-.*", "hello-node"))
				if err != nil {
					return err
				}

				node, err := checkNodeCreation(t, c, k8s.Manager, "node")
				if err != nil {
					return err
				}

				pod, err := checkNodeCreation(t, c, k8s.Manager, "pod", "Name", g.Regex("%s-.*", "hello-node"))
				if err != nil {
					return err
				}

				// check edges exist
				if err = checkEdgeOwnership(t, c, cluster, namespace); err != nil {
					return err
				}

				if err = checkEdgeOwnership(t, c, namespace, deployment); err != nil {
					return err
				}

				if err = checkEdgeOwnership(t, c, namespace, replicaset); err != nil {
					return err
				}

				if err = checkEdgeOwnership(t, c, namespace, pod); err != nil {
					return err
				}

				if err = checkEdgeOwnership(t, c, pod, container); err != nil {
					return err
				}

				if err = checkEdgeService(t, c, service, pod); err != nil {
					return err
				}

				if err = checkEdgeAssociation(t, c, node, pod); err != nil {
					return err
				}
				return nil
			},
		},
	)
}

func TestK8sNetworkPolicyScenario1(t *testing.T) {
	file := "networkpolicy-namespace"
	name := objName + "-" + file
	testRunner(
		t,
		setupFromConfigFile(k8s.Manager, file),
		tearDownFromConfigFile(k8s.Manager, file),
		[]CheckFunction{
			func(c *CheckContext) error {
				networkpolicy, err := checkNodeCreation(t, c, k8s.Manager, "networkpolicy", "Name", name)
				if err != nil {
					return err
				}

				pod, err := checkNodeCreation(t, c, k8s.Manager, "pod", "Name", name)
				if err != nil {
					return err
				}

				if err = checkEdgeNetworkPolicy(t, c, networkpolicy, pod, k8s.PolicyTypeIngress, k8s.PolicyTargetAllow, k8s.PolicyPointBegin); err != nil {
					return err
				}

				return nil
			},
		},
	)
}

func TestK8sNetworkPolicyScenario2(t *testing.T) {
	file := "networkpolicy-pod"
	name := objName + "-" + file
	testRunner(
		t,
		setupFromConfigFile(k8s.Manager, file),
		tearDownFromConfigFile(k8s.Manager, file),
		[]CheckFunction{
			func(c *CheckContext) error {
				networkpolicy, err := checkNodeCreation(t, c, k8s.Manager, "networkpolicy", "Name", name)
				if err != nil {
					return err
				}

				pod, err := checkNodeCreation(t, c, k8s.Manager, "pod", "Name", name)
				if err != nil {
					return err
				}

				if err = checkEdgeNetworkPolicy(t, c, networkpolicy, pod, k8s.PolicyTypeIngress, k8s.PolicyTargetAllow, k8s.PolicyPointBegin); err != nil {
					return err
				}

				return nil
			},
		},
	)
}

func testK8sNetworkPolicyDefaultScenario(t *testing.T, policyType k8s.PolicyType, policyTarget k8s.PolicyTarget) {
	file := fmt.Sprintf("networkpolicy-%s-%s", policyType, policyTarget)
	name := objName + "-" + file
	testRunner(
		t,
		setupFromConfigFile(k8s.Manager, file),
		tearDownFromConfigFile(k8s.Manager, file),
		[]CheckFunction{
			func(c *CheckContext) error {
				np, err := checkNodeCreation(t, c, k8s.Manager, "networkpolicy", "Name", name)
				if err != nil {
					return err
				}

				pod, err := checkNodeCreation(t, c, k8s.Manager, "pod", "Name", name)
				if err != nil {
					return err
				}

				if err = checkEdgeNetworkPolicy(t, c, np, pod, policyType, policyTarget, k8s.PolicyPointBegin); err != nil {
					return err
				}

				return nil
			},
		},
	)
}

func TestK8sNetworkPolicyDenyIngressScenario(t *testing.T) {
	testK8sNetworkPolicyDefaultScenario(t, k8s.PolicyTypeIngress, k8s.PolicyTargetDeny)
}

func TestK8sNetworkPolicyAllowIngressScenario(t *testing.T) {
	testK8sNetworkPolicyDefaultScenario(t, k8s.PolicyTypeIngress, k8s.PolicyTargetAllow)
}

func TestK8sNetworkPolicyDenyEgressScenario(t *testing.T) {
	testK8sNetworkPolicyDefaultScenario(t, k8s.PolicyTypeEgress, k8s.PolicyTargetDeny)
}

func TestK8sNetworkPolicyAllowEgressScenario(t *testing.T) {
	testK8sNetworkPolicyDefaultScenario(t, k8s.PolicyTypeEgress, k8s.PolicyTargetAllow)
}

func testK8sNetworkPolicyObjectToObjectScenario(t *testing.T, policyType k8s.PolicyType, policyTarget k8s.PolicyTarget, fileSuffix string, edgeArgs ...interface{}) {
	file := fmt.Sprintf("networkpolicy-%s-%s-%s", policyType, policyTarget, fileSuffix)
	name := objName + "-" + file
	testRunner(
		t,
		setupFromConfigFile(k8s.Manager, file),
		tearDownFromConfigFile(k8s.Manager, file),
		[]CheckFunction{
			func(c *CheckContext) error {
				np, err := checkNodeCreation(t, c, k8s.Manager, "networkpolicy", "Name", name)
				if err != nil {
					return err
				}

				begin, err := checkNodeCreation(t, c, k8s.Manager, "pod", "Name", name+"-to")
				if err != nil {
					return err
				}

				end, err := checkNodeCreation(t, c, k8s.Manager, "pod", "Name", name+"-from")
				if err != nil {
					return err
				}

				if err = checkEdgeNetworkPolicy(t, c, np, begin, policyType, policyTarget, k8s.PolicyPointBegin); err != nil {
					return err
				}

				if err = checkEdgeNetworkPolicy(t, c, np, end, policyType, policyTarget, k8s.PolicyPointEnd, edgeArgs...); err != nil {
					return err
				}

				return nil
			},
		},
	)
}

func TestK8sNetworkPolicyAllowIngressPodToPodScenario(t *testing.T) {
	testK8sNetworkPolicyObjectToObjectScenario(t, k8s.PolicyTypeIngress, k8s.PolicyTargetAllow, "pod")
}

func TestK8sNetworkPolicyAllowIngressNamespaceToNamespaceScenario(t *testing.T) {
	testK8sNetworkPolicyObjectToObjectScenario(t, k8s.PolicyTypeIngress, k8s.PolicyTargetAllow, "namespace")
}

func TestK8sNetworkPolicyAllowIngressPodToPodPortsScenario(t *testing.T) {
	testK8sNetworkPolicyObjectToObjectScenario(t, k8s.PolicyTypeIngress, k8s.PolicyTargetAllow, "ports", "Ports", ":80")
}

func TestK8sServicePodScenario(t *testing.T) {
	file := "service-pod"
	name := objName + "-" + file
	testRunner(
		t,
		setupFromConfigFile(k8s.Manager, file),
		tearDownFromConfigFile(k8s.Manager, file),
		[]CheckFunction{
			func(c *CheckContext) error {
				service, err := checkNodeCreation(t, c, k8s.Manager, "service", "Name", name)
				if err != nil {
					return err
				}

				pod, err := checkNodeCreation(t, c, k8s.Manager, "pod", "Name", name)
				if err != nil {
					return err
				}

				if err = checkEdge(t, c, service, pod, "service"); err != nil {
					return err
				}

				return nil
			},
		},
	)
}
