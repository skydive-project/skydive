/*
 * Copyright 2017 IBM Corp.
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
	corev1 "k8s.io/api/core/v1"

	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology/graph"
)

func dumpObject(obj interface{}) string {
	switch obj := obj.(type) {
	case *corev1.Namespace:
		return dumpNamespace(obj)
	case *corev1.Pod:
		return dumpPod(obj)
	default:
		return "<nil>"
	}
}

func int32ValueOrDefault(value *int32, defaultValue int32) int32 {
	if value == nil {
		return defaultValue
	}
	return *value
}

// Probe for tracking k8s events
type Probe struct {
	bundle *probe.ProbeBundle
}

type ProbeHandler func(g *graph.Graph) probe.Probe
type ProbeMap map[string]ProbeHandler

// ProcessProbeBundle using config will create the runtime probes map
func (m *ProbeMap) newProbeBundle(g *graph.Graph, context string) *probe.ProbeBundle {
	configProbes := config.GetStringSlice(context + ".probes")
	if len(configProbes) == 0 {
		for name := range *m {
			configProbes = append(configProbes, name)
		}
	}
	logging.GetLogger().Infof("%s probes: %v", context, configProbes)

	probes := make(map[string]probe.Probe)
	for _, name := range configProbes {
		if ctor, ok := (*m)[name]; ok {
			probes[name] = ctor(g)
		} else {
			logging.GetLogger().Errorf("skipping unsupported %s probe %v", context, name)
		}
	}
	return probe.NewProbeBundle(probes)
}

// Start k8s probe
func (p *Probe) Start() {
	p.bundle.Start()
}

// Stop k8s probe
func (p *Probe) Stop() {
	p.bundle.Stop()
}

// NewProbeHelper create the Probe for tracking events
func NewProbeHelper(g *graph.Graph, context string, m *ProbeMap) (*Probe, error) {
	return &Probe{
		bundle: m.newProbeBundle(g, context),
	}, nil
}

// NewProbe create the Probe for tracking k8s events
func NewProbe(g *graph.Graph) (*Probe, error) {
	err := initClientset()
	if err != nil {
		return nil, err
	}

	name2ctor := ProbeMap{
		"cluster":               newClusterProbe,
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
	return NewProbeHelper(g, "k8s", &name2ctor)
}
