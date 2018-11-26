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
	"encoding/json"
	"fmt"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/probe"

	"k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/client-go/kubernetes"
)

// MetadataInnerDeployment contains the type specific fields
// easyjson:json
type MetadataInnerDeployment struct {
	MetadataInner
	DesiredReplicas     int32 `skydive:"int"`
	Replicas            int32 `skydive:"int"`
	ReadyReplicas       int32 `skydive:"int"`
	AvailableReplicas   int32 `skydive:"int"`
	UnavailableReplicas int32 `skydive:"int"`
}

// GetField implements Getter interface
func (inner *MetadataInnerDeployment) GetField(key string) (interface{}, error) {
	return GenericGetField(inner, key)
}

// GetFieldInt64 implements Getter interface
func (inner *MetadataInnerDeployment) GetFieldInt64(key string) (int64, error) {
	return GenericGetFieldInt64(inner, key)
}

// GetFieldString implements Getter interface
func (inner *MetadataInnerDeployment) GetFieldString(key string) (string, error) {
	return GenericGetFieldString(inner, key)
}

// GetFieldKeys implements Getter interface
func (inner *MetadataInnerDeployment) GetFieldKeys() []string {
	return GenericGetFieldKeys(inner)
}

// MetadataInnerDeploymentDecoder implements a json message raw decoder
func MetadataInnerDeploymentDecoder(raw json.RawMessage) (common.Getter, error) {
	var inner MetadataInnerDaemonSet
	return GenericMetadataDecoder(&inner, raw)
}

type deploymentHandler struct {
}

func (h *deploymentHandler) Dump(obj interface{}) string {
	deployment := obj.(*v1beta1.Deployment)
	return fmt.Sprintf("deployment{Namespace: %s, Name: %s}", deployment.Namespace, deployment.Name)
}

func (h *deploymentHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	deployment := obj.(*v1beta1.Deployment)

	inner := new(MetadataInnerDeployment)
	inner.MetadataInner.Setup(&deployment.ObjectMeta, deployment)
	inner.DesiredReplicas = int32ValueOrDefault(deployment.Spec.Replicas, 1)
	inner.Replicas = deployment.Status.Replicas
	inner.ReadyReplicas = deployment.Status.ReadyReplicas
	inner.AvailableReplicas = deployment.Status.AvailableReplicas
	inner.UnavailableReplicas = deployment.Status.UnavailableReplicas

	return graph.Identifier(deployment.GetUID()), NewMetadata(Manager, "deployment", inner.Name, inner)
}

func newDeploymentProbe(client interface{}, g *graph.Graph) Subprobe {
	RegisterNodeDecoder(MetadataInnerDeploymentDecoder, "deployment")
	return NewResourceCache(client.(*kubernetes.Clientset).ExtensionsV1beta1().RESTClient(), &v1beta1.Deployment{}, "deployments", g, &deploymentHandler{})
}

func deploymentPodAreLinked(a, b interface{}) bool {
	deployment := a.(*v1beta1.Deployment)
	pod := b.(*v1.Pod)
	return MatchNamespace(pod, deployment) && matchLabelSelector(pod, deployment.Spec.Selector)
}

func newDeploymentPodLinker(g *graph.Graph) probe.Probe {
	return NewABLinker(g, Manager, "deployment", Manager, "pod", deploymentPodAreLinked)
}

func deploymentReplicaSetAreLinked(a, b interface{}) bool {
	deployment := a.(*v1beta1.Deployment)
	replicaset := b.(*v1beta1.ReplicaSet)
	return MatchNamespace(replicaset, deployment) && matchLabelSelector(replicaset, deployment.Spec.Selector)
}

func newDeploymentReplicaSetLinker(g *graph.Graph) probe.Probe {
	return NewABLinker(g, Manager, "deployment", Manager, "replicaset", deploymentReplicaSetAreLinked)
}
