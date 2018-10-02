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

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/client-go/kubernetes"
)

type jobHandler struct {
	DefaultResourceHandler
}

func (h *jobHandler) Dump(obj interface{}) string {
	job := obj.(*batchv1.Job)
	return fmt.Sprintf("job{Namespace: %s, Name: %s}", job.Namespace, job.Name)
}

func (h *jobHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	job := obj.(*batchv1.Job)

	m := NewMetadata(Manager, "job", job, job.Name, job.Namespace)
	m.SetField("Parallelism", job.Spec.Parallelism)
	m.SetField("Completions", job.Spec.Completions)
	m.SetField("Active", job.Status.Active)
	m.SetField("Succeeded", job.Status.Succeeded)
	m.SetField("Failed", job.Status.Failed)

	return graph.Identifier(job.GetUID()), m
}

func newJobProbe(clientset *kubernetes.Clientset, graph *graph.Graph) Subprobe {
	return NewResourceCache(clientset.BatchV1().RESTClient(), &batchv1.Job{}, "jobs", graph, &jobHandler{})
}
