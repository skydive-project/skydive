/*
 * Copyright (C) 2017 Red Hat, Inc.
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

	"github.com/skydive-project/skydive/topology/graph"

	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

type podHandler struct {
	DefaultResourceHandler
	graph.DefaultGraphListener
	graph *graph.Graph
	cache *ResourceCache
}

func (h *podHandler) Dump(obj interface{}) string {
	pod := obj.(*v1.Pod)
	return fmt.Sprintf("pod{Namespace: %s, Name: %s}", pod.Namespace, pod.Name)
}

func (h *podHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	pod := obj.(*v1.Pod)

	m := NewMetadata(Manager, "pod", pod, pod.Name, pod.Namespace)
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

	return graph.Identifier(pod.GetUID()), m
}

func newPodProbe(clientset *kubernetes.Clientset, g *graph.Graph) Subprobe {
	return NewResourceCache(clientset.CoreV1().RESTClient(), &v1.Pod{}, "pods", g, &podHandler{graph: g})
}
