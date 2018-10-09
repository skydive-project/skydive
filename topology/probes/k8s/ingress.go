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

	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology/graph"

	"k8s.io/api/extensions/v1beta1"
	"k8s.io/client-go/kubernetes"
)

type ingressHandler struct {
	DefaultResourceHandler
}

func (h *ingressHandler) Dump(obj interface{}) string {
	ingress := obj.(*v1beta1.Ingress)
	return fmt.Sprintf("ingress{Namespace: %s, Name: %s}", ingress.Namespace, ingress.Name)
}

func (h *ingressHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	ingress := obj.(*v1beta1.Ingress)

	m := NewMetadata(Manager, "ingress", ingress, ingress.Name, ingress.Namespace)
	m.SetFieldAndNormalize("Backend", ingress.Spec.Backend)
	m.SetFieldAndNormalize("TLS", ingress.Spec.TLS)
	m.SetFieldAndNormalize("Rules", ingress.Spec.Rules)

	return graph.Identifier(ingress.GetUID()), m
}

func newIngressProbe(clientset *kubernetes.Clientset, g *graph.Graph) Subprobe {
	return NewResourceCache(clientset.ExtensionsV1beta1().RESTClient(), &v1beta1.Ingress{}, "ingresses", g, &ingressHandler{})
}

func newIngressServiceLinker(g *graph.Graph, subprobes map[string]Subprobe) probe.Probe {
	return newResourceLinker(g, subprobes, "ingress", []string{"Namespace", "Backend.ServiceName"}, "service", []string{"Namespace", "Name"}, graph.Metadata{"RelationType": "ingress"})
}
