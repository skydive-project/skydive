/*
 * Copyright (C) 2017 Red Hat, Inc.
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

	"github.com/mohae/deepcopy"
	"github.com/skydive-project/skydive/graffiti/graph"

	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

type podHandler struct {
	graph.DefaultGraphListener
	graph *graph.Graph
	cache *ResourceCache
}

func (h *podHandler) Dump(obj interface{}) string {
	pod := obj.(*v1.Pod)
	return fmt.Sprintf("pod{Namespace: %s, Name: %s}", pod.Namespace, pod.Name)
}

func (h *podHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	// we make a copy of pod before modifaying the pod object so
	// that don't interfere with the container subprobe
	pod := deepcopy.Copy(obj).(*v1.Pod)

	pod.Spec.Containers = nil

	m := NewMetadataFields(&pod.ObjectMeta)
	m.SetField("Node", pod.Spec.NodeName)
	podIP := pod.Status.PodIP
	if podIP != "" {
		m.SetField("IP", podIP)
	}
	reason := string(pod.Status.Phase)
	if pod.Status.Reason != "" {
		reason = pod.Status.Reason
	}
	m.SetField("Status", reason)

	return graph.Identifier(pod.GetUID()), NewMetadata(Manager, "pod", m, pod, pod.Name)
}

func newPodProbe(client interface{}, g *graph.Graph) Subprobe {
	return NewResourceCache(client.(*kubernetes.Clientset).CoreV1().RESTClient(), &v1.Pod{}, "pods", g, &podHandler{graph: g})
}
