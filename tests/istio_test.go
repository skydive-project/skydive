// +build istio

/*
 * Copyright (C) 2018 IBM, Inc.
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
	"testing"

	"github.com/skydive-project/skydive/graffiti/graph"
	g "github.com/skydive-project/skydive/gremlin"
	"github.com/skydive-project/skydive/topology/probes/istio"
	"github.com/skydive-project/skydive/topology/probes/k8s"
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
				virtualservice, err := checkNodeCreation(t, c, istio.Manager, "virtualservice", name)
				if err != nil {
					return err
				}
				_, err = checkNodeCreation(t, c, istio.Manager, "destinationrule", name)
				if err != nil {
					return err
				}
				pod, err := checkNodeCreation(t, c, k8s.Manager, "pod", name)
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
	bookinfo := "WITH_ISTIO=true ./bookinfo/bookinfo.sh"
	testRunner(
		t,
		[]Cmd{
			{bookinfo + " start", true},
		},
		[]Cmd{
			{bookinfo + " stop", false},
		},
		[]CheckFunction{
			func(c *CheckContext) error {

				// check nodes exist

				drDetails, err := checkNodeCreation(t, c, istio.Manager, "destinationrule", "details")
				if err != nil {
					return err
				}

				drProductpage, err := checkNodeCreation(t, c, istio.Manager, "destinationrule", "productpage")
				if err != nil {
					return err
				}

				drRatings, err := checkNodeCreation(t, c, istio.Manager, "destinationrule", "ratings")
				if err != nil {
					return err
				}

				drReviews, err := checkNodeCreation(t, c, istio.Manager, "destinationrule", "reviews")
				if err != nil {
					return err
				}

				vs, err := checkNodeCreation(t, c, istio.Manager, "virtualservice", "bookinfo")
				if err != nil {
					return err
				}

				podProductpage, err := checkNodeCreation(t, c, k8s.Manager, "pod", g.Regex("%s-.*", "productpage"))
				if err != nil {
					return err
				}

				serviceDetails, err := checkNodeCreation(t, c, k8s.Manager, "service", "details")
				if err != nil {
					return err
				}

				serviceProductpage, err := checkNodeCreation(t, c, k8s.Manager, "service", "productpage")
				if err != nil {
					return err
				}

				serviceRatings, err := checkNodeCreation(t, c, k8s.Manager, "service", "ratings")
				if err != nil {
					return err
				}

				serviceReviews, err := checkNodeCreation(t, c, k8s.Manager, "service", "reviews")
				if err != nil {
					return err
				}

				gateway, err := checkNodeCreation(t, c, istio.Manager, "gateway", "bookinfo-gateway")
				if err != nil {
					return err
				}

				// check edges exist

				if err = checkEdge(t, c, vs, podProductpage, "virtualservice"); err != nil {
					return err
				}

				if err = checkEdge(t, c, vs, serviceProductpage, "virtualservice"); err != nil {
					return err
				}

				if err = checkEdge(t, c, vs, drProductpage, "virtualservice"); err != nil {
					return err
				}

				if err = checkEdge(t, c, drDetails, serviceDetails, "destinationrule"); err != nil {
					return err
				}

				if err = checkEdge(t, c, drProductpage, serviceProductpage, "destinationrule"); err != nil {
					return err
				}

				if err = checkEdge(t, c, drRatings, serviceRatings, "destinationrule"); err != nil {
					return err
				}

				if err = checkEdge(t, c, drReviews, serviceReviews, "destinationrule"); err != nil {
					return err
				}

				if err = checkEdge(t, c, gateway, vs, "gateway"); err != nil {
					return err
				}

				return nil
			},
		},
	)
}
