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

package k8s

import (
	"fmt"

	"github.com/skydive-project/skydive/topology/graph"

	"k8s.io/api/apps/v1beta1"
	"k8s.io/client-go/kubernetes"
)

type statefulSetHandler struct {
	DefaultResourceHandler
}

func (h *statefulSetHandler) Dump(obj interface{}) string {
	ss := obj.(*v1beta1.StatefulSet)
	return fmt.Sprintf("statefulset{Namespace: %s, Name: %s}", ss.Namespace, ss.Name)
}

func (h *statefulSetHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	ss := obj.(*v1beta1.StatefulSet)

	m := NewMetadata(Manager, "statefulset", ss, ss.Name, ss.Namespace)
	m.SetField("DesiredReplicas", int32ValueOrDefault(ss.Spec.Replicas, 1))
	m.SetField("ServiceName", ss.Spec.ServiceName) // FIXME: replace by link to Service
	m.SetField("Replicas", ss.Status.Replicas)
	m.SetField("ReadyReplicas", ss.Status.ReadyReplicas)
	m.SetField("CurrentReplicas", ss.Status.CurrentReplicas)
	m.SetField("UpdatedReplicas", ss.Status.UpdatedReplicas)
	m.SetField("CurrentRevision", ss.Status.CurrentRevision)
	m.SetField("UpdateRevision", ss.Status.UpdateRevision)

	return graph.Identifier(ss.GetUID()), m
}

func newStatefulSetProbe(clientset *kubernetes.Clientset, g *graph.Graph) Subprobe {
	return NewResourceCache(clientset.AppsV1beta1().RESTClient(), &v1beta1.StatefulSet{}, "statefulsets", g, &statefulSetHandler{})
}
