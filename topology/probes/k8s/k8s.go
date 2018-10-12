/*
 * Copyright 2018 Red Hat
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

package k8s

import (
	"fmt"

	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology/graph"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// ClusterName is the name to give to the probe cluster node
const ClusterName = "cluster"

type resourceHandler func(clientset *kubernetes.Clientset, graph *graph.Graph) Subprobe
type linkHandler func(g *graph.Graph, subprobes map[string]Subprobe) probe.Probe

// NewConfig returns a new Kubernetes configuration object
func NewConfig(kubeConfig string) (*rest.Config, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
	if err != nil {
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()

		configOverrides := &clientcmd.ConfigOverrides{}

		kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
		config, err = kubeConfig.ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("Failed to load Kubernetes config: %s", err.Error())
		}
	}
	return config, nil
}

// NewK8sProbe returns a new Kubernetes probe
func NewK8sProbe(g *graph.Graph) (*Probe, error) {
	configFile := config.GetString("analyzer.topology.k8s.config_file")
	enabledSubprobes := config.GetStringSlice("analyzer.topology.k8s.probes")

	config, err := NewConfig(configFile)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("Failed to create Kubernetes client: %s", err.Error())
	}

	resourceHandlers := map[string]resourceHandler{
		"container":             newContainerProbe,
		"cronjob":               newCronJobProbe,
		"daemonset":             newDaemonSetProbe,
		"deployment":            newDeploymentProbe,
		"endpoints":             newEndpointsProbe,
		"ingress":               newIngressProbe,
		"job":                   newJobProbe,
		"namespace":             newNamespaceProbe,
		"networkpolicy":         newNetworkPolicyProbe,
		"node":                  newNodeProbe,
		"persistentvolume":      newPersistentVolumeProbe,
		"persistentvolumeclaim": newPersistentVolumeClaimProbe,
		"pod":                   newPodProbe,
		"replicaset":            newReplicaSetProbe,
		"replicationcontroller": newReplicationControllerProbe,
		"service":               newServiceProbe,
		"statefulset":           newStatefulSetProbe,
		"storageclass":          newStorageClassProbe,
	}

	if len(enabledSubprobes) == 0 {
		for name := range resourceHandlers {
			enabledSubprobes = append(enabledSubprobes, name)
		}
	}

	subprobes := make(map[string]Subprobe)
	for _, name := range enabledSubprobes {
		if probeHandler, ok := resourceHandlers[name]; ok {
			subprobes[name] = probeHandler(clientset, g)
		} else {
			logging.GetLogger().Errorf("skipping unsupported probe %v", name)
		}
	}

	linkerHandlers := []linkHandler{
		newContainerLinker,
		newHostNodeLinker,
		newNodePodLinker,
		newIngressServiceLinker,
		newNamespaceLinker,
		newNetworkPolicyLinker,
		newServicePodLinker,
	}

	var linkers []probe.Probe
	for _, linkHandler := range linkerHandlers {
		if linker := linkHandler(g, subprobes); linker != nil {
			linkers = append(linkers, linker)
		}
	}

	return NewProbe("k8s", ClusterName, g, subprobes, linkers)
}
