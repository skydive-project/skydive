/*
 * Copyright 2016 IBM Corp.
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

package probes

import (
	"fmt"
	"strings"

	"github.com/FlorianOtel/client-go/kubernetes"
	"github.com/FlorianOtel/client-go/pkg/api/v1"
	"github.com/FlorianOtel/client-go/pkg/types"
	"github.com/FlorianOtel/client-go/tools/clientcmd"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"
)

type K8SMessage struct {
	msgType string
	obj     interface{}
}

type K8SProbe struct {
	Graph           *graph.Graph
	nodeUpdaterChan chan graph.Identifier
	uid             map[types.UID]bool
	clientset       *kubernetes.Clientset
}

// check whether podName is a k8s pod
func (k8s *K8SProbe) isK8S(nodeName string) bool {
	// gets pods from k8s
	pods, err := k8s.clientset.Core().Pods("").List(v1.ListOptions{})
	if err != nil {
		logging.GetLogger().Error(err)
		return false
	}
	logging.GetLogger().Debugf("There are %d pods in the cluster", len(pods.Items))

	for _, pod := range pods.Items {
		logging.GetLogger().Debugf("Compare nodeName %s with uid %s", nodeName, string(pod.GetUID()))
		if strings.Contains(nodeName, string(pod.GetUID())) {
			// update k8sPods
			logging.GetLogger().Debugf("Found match! nodeName %s with uid %s", nodeName, string(pod.GetUID()))
			return true
		}
	}
	logging.GetLogger().Debugf("No match is found for nodeName %s", nodeName)
	return false
}

func (k8s *K8SProbe) enhanceNode(id graph.Identifier) {
	k8s.Graph.RLock()
	n := k8s.Graph.GetNode(id)
	k8s.Graph.RUnlock()
	if n == nil {
		logging.GetLogger().Debugf("WARNING: Cannot Find Node %s", id)
	} else {
		k8s.Graph.RLock()
		nodeName, _ := n.GetFieldString("PodName")
		k8s.Graph.RUnlock()

		logging.GetLogger().Debugf("netns added/(updated): %s", nodeName)
		isk8s := k8s.isK8S(nodeName)
		if isk8s {
			// set metadata
			tr := k8s.Graph.StartMetadataTransaction(n)
			tr.AddMetadata("Orchestrator", "k8s")
			tr.AddMetadata("K8S.Type", "Pod")
			tr.Commit()
		}
	}
}

func (k8s *K8SProbe) OnNodeUpdated(n *graph.Node) {
	nodeType, _ := n.GetFieldString("Type")
	if nodeType == "netns" {
		k8s.nodeUpdaterChan <- n.ID
	}
}

func (k8s *K8SProbe) OnNodeAdded(n *graph.Node) {
	nodeType, _ := n.GetFieldString("Type")
	if nodeType == "netns" {
		k8s.nodeUpdaterChan <- n.ID
	}
}

func (k8s *K8SProbe) OnNodeDeleted(n *graph.Node) {
}

func (k8s *K8SProbe) OnEdgeUpdated(e *graph.Edge) {
}

func (k8s *K8SProbe) OnEdgeAdded(e *graph.Edge) {
}

func (k8s *K8SProbe) OnEdgeDeleted(e *graph.Edge) {
}

func (k8s *K8SProbe) nodeMessagesUpdater() {
	logging.GetLogger().Debugf("k8s probe: Starting messages updater")
	for message := range k8s.nodeUpdaterChan {
		k8s.enhanceNode(message)
	}
	logging.GetLogger().Debugf("k8s probe: Stopping messages updater")
}

func (k8s *K8SProbe) Start() {
	logging.GetLogger().Debugf("k8s probe: got command - Start")
	go k8s.nodeMessagesUpdater()
}

func (k8s *K8SProbe) Stop() {
	k8s.Graph.RemoveEventListener(k8s)
	close(k8s.nodeUpdaterChan)
	logging.GetLogger().Debugf("k8s probe: got command - Stop")
}

func NewK8SProbe(g *graph.Graph) (*K8SProbe, error) {
	logging.GetLogger().Debugf("k8s probe: got command - NewK8SProbe")
	k8s := &K8SProbe{
		Graph: g,
	}
	kubeconfig := config.GetConfig().GetString("k8s.config_file")
	// uses the current context in kubeconfig
	logging.GetLogger().Debugf("kubeconfig: %s", kubeconfig)
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("k8s probe: %s (k8s config_file = %s)", err.Error(), kubeconfig)
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("k8s probe: %s", err.Error())
	}
	k8s.clientset = clientset

	k8s.nodeUpdaterChan = make(chan graph.Identifier, 500)
	g.AddEventListener(k8s)
	return k8s, nil
}
