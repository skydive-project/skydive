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

package k8s

import (
	"fmt"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/probe"

	"k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/client-go/kubernetes"
)

type deploymentHandler struct {
}

func (h *deploymentHandler) Dump(obj interface{}) string {
	deployment := obj.(*v1beta1.Deployment)
	return fmt.Sprintf("deployment{Namespace: %s, Name: %s}", deployment.Namespace, deployment.Name)
}

func (h *deploymentHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	deployment := obj.(*v1beta1.Deployment)

	m := NewMetadataFields(&deployment.ObjectMeta)
	m.SetField("DesiredReplicas", int32ValueOrDefault(deployment.Spec.Replicas, 1))
	m.SetField("Replicas", deployment.Status.Replicas)
	m.SetField("ReadyReplicas", deployment.Status.ReadyReplicas)
	m.SetField("AvailableReplicas", deployment.Status.AvailableReplicas)
	m.SetField("UnavailableReplicas", deployment.Status.UnavailableReplicas)

	return graph.Identifier(deployment.GetUID()), NewMetadata(Manager, "deployment", m, deployment, deployment.Name)
}

func newDeploymentProbe(client interface{}, g *graph.Graph) Subprobe {
	return NewResourceCache(client.(*kubernetes.Clientset).ExtensionsV1beta1().RESTClient(), &v1beta1.Deployment{}, "deployments", g, &deploymentHandler{})
}

func deploymentPodAreLinked(a, b interface{}) bool {
	return matchLabelSelector(b.(*v1.Pod), a.(*v1beta1.Deployment).Spec.Selector)
}

func newDeploymentPodLinker(g *graph.Graph) probe.Probe {
	return NewABLinker(g, Manager, "deployment", Manager, "pod", deploymentPodAreLinked)
}

func deploymentReplicaSetAreLinked(a, b interface{}) bool {
	return matchLabelSelector(b.(*v1beta1.ReplicaSet), a.(*v1beta1.Deployment).Spec.Selector)
}

func newDeploymentReplicaSetLinker(g *graph.Graph) probe.Probe {
	return NewABLinker(g, Manager, "deployment", Manager, "replicaset", deploymentReplicaSetAreLinked)
}
