// +build istio

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
	g "github.com/skydive-project/skydive/gremlin"
	"github.com/skydive-project/skydive/topology/graph"
	"github.com/skydive-project/skydive/topology/probes/istio"
	"github.com/skydive-project/skydive/topology/probes/k8s"
	"testing"
)

const (
	bookinfo = "https://raw.githubusercontent.com/istio/istio/release-1.0/samples/bookinfo"
)

/* -- test creation of single resource -- */
func TestIstioDestinationRuleNode(t *testing.T) {
	testNodeCreationFromConfig(t, istio.Manager, "destinationrule", objName+"-destinationrule")
}

func TestIstioGatewayNode(t *testing.T) {
	testNodeCreationFromConfig(t, istio.Manager, "gateway", objName+"-gateway")
}

func TestIstioServiceEntryNode(t *testing.T) {
	testNodeCreationFromConfig(t, istio.Manager, "serviceentry", objName+"-serviceentry")
}

func TestIstioQuotaSpecNode(t *testing.T) {
	testNodeCreationFromConfig(t, istio.Manager, "quotaspec", objName+"-quotaspec")
}

func TestIstioQuotaSpecBindingNode(t *testing.T) {
	testNodeCreationFromConfig(t, istio.Manager, "quotaspecbinding", objName+"-quotaspecbinding")
}

func TestIstioVirtualServiceNode(t *testing.T) {
	testNodeCreationFromConfig(t, istio.Manager, "virtualservice", objName+"-virtualservice")
}

func checkEdgeVirtualService(t *testing.T, c *CheckContext, from, to *graph.Node, edgeArgs ...interface{}) error {
	return checkEdge(t, c, from, to, "virtualservice", edgeArgs...)
}

func TestIstioVirtualServicePodScenario(t *testing.T) {
	file := "virtualservice-pod"
	name := objName + "-" + file
	testRunner(
		t,
		setupFromConfigFile(istio.Manager, file),
		tearDownFromConfigFile(istio.Manager, file),
		[]CheckFunction{
			func(c *CheckContext) error {
				virtualservice, err := checkNodeCreation(t, c, istio.Manager, "virtualservice", "Name", name)
				if err != nil {
					return err
				}
				_, err = checkNodeCreation(t, c, istio.Manager, "destinationrule", "Name", name)
				if err != nil {
					return err
				}
				pod, err := checkNodeCreation(t, c, k8s.Manager, "pod", "Name", name)
				if err != nil {
					return err
				}
				if err = checkEdgeVirtualService(t, c, virtualservice, pod); err != nil {
					return err
				}
				return nil
			},
		},
	)
}

func TestBookInfoScenario(t *testing.T) {
	testRunner(
		t,
		[]Cmd{
			{"kubectl apply -f " + bookinfo + "/networking/destination-rule-all.yaml", true},
			{"kubectl apply -f " + bookinfo + "/networking/bookinfo-gateway.yaml", true},
			{"kubectl apply -f " + bookinfo + "/platform/kube/bookinfo.yaml", true},
		},
		[]Cmd{
			{"istioctl delete virtualservice bookinfo", false},
			{"istioctl delete gateway bookinfo-gateway", false},
			{"istioctl delete destinationrule details productpage ratings reviews", false},
			{"kubectl delete deployment details-v1 productpage-v1 ratings-v1 reviews-v1 reviews-v2 reviews-v3", false},
		},
		[]CheckFunction{
			func(c *CheckContext) error {
				// check nodes exist
				_, err := checkNodeCreation(t, c, istio.Manager, "destinationrule", "Name", "details")
				if err != nil {
					return err
				}

				_, err = checkNodeCreation(t, c, istio.Manager, "destinationrule", "Name", "productpage")
				if err != nil {
					return err
				}

				_, err = checkNodeCreation(t, c, istio.Manager, "destinationrule", "Name", "ratings")
				if err != nil {
					return err
				}

				_, err = checkNodeCreation(t, c, istio.Manager, "destinationrule", "Name", "reviews")
				if err != nil {
					return err
				}

				_, err = checkNodeCreation(t, c, istio.Manager, "gateway", "Name", "bookinfo-gateway")
				if err != nil {
					return err
				}

				virtualservice, err := checkNodeCreation(t, c, istio.Manager, "virtualservice", "Name", "bookinfo")
				if err != nil {
					return err
				}

				pod, err := checkNodeCreation(t, c, k8s.Manager, "pod", "Name", g.Regex("%s-.*", "productpage"))
				if err != nil {
					return err
				}

				if err = checkEdgeVirtualService(t, c, virtualservice, pod); err != nil {
					return err
				}

				return nil
			},
		},
	)
}
