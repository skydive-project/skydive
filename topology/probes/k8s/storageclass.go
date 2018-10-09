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

	"k8s.io/api/storage/v1"
	"k8s.io/client-go/kubernetes"
)

type storageClassHandler struct {
	DefaultResourceHandler
}

func (h *storageClassHandler) Dump(obj interface{}) string {
	sc := obj.(*v1.StorageClass)
	return fmt.Sprintf("storageclass{Namespace: %s, Name: %s}", sc.Namespace, sc.Name)
}

func (h *storageClassHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	sc := obj.(*v1.StorageClass)

	m := NewMetadata(Manager, "storageclass", sc, sc.Name, sc.Namespace)
	m.SetField("Provisioner", sc.Provisioner)

	return graph.Identifier(sc.GetUID()), m
}

func newStorageClassProbe(clientset *kubernetes.Clientset, g *graph.Graph) Subprobe {
	return NewResourceCache(clientset.StorageV1().RESTClient(), &v1.StorageClass{}, "storageclasses", g, &storageClassHandler{})
}
