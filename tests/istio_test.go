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
	"testing"
	"github.com/skydive-project/skydive/topology/probes/istio"
)

/* -- test creation of single resource -- */
func TestIstioClusterNode(t *testing.T) {
	testNodeCreation(t, nil, nil, istio.Manager, "cluster", istio.ClusterName)
}

func TestIstioDestinationRuleNode(t *testing.T) {
	testNodeCreationFromConfig(t, istio.Manager, "destinationrule", objName+"-destinationrule")
}

func TestBookInfoScenario(t *testing.T) {
        testRunner(
                t,
                []Cmd{
                        {"kubectl apply -f /usr/bin/istio-1.0.1/samples/bookinfo/networking/destination-rule-all.yaml", true},
                },
                []Cmd{
                        {"kubectl apply -f /usr/bin/istio-1.0.1/samples/bookinfo/platform/kube/cleanup.sh", false},
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

                                return nil
                        },
                },
        )
}

