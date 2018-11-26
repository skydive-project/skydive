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

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/client-go/kubernetes"
)

// MetadataInnerJob contains the type specific fields
// easyjson:json
type MetadataInnerJob struct {
	MetadataInner
	Parallelism int32 `skydive:"int"`
	Completions int32 `skydive:"int"`
	Active      int32 `skydive:"int"`
	Succeeded   int32 `skydive:"int"`
	Failed      int32 `skydive:"int"`
}

// GetField implements Getter interface
func (inner *MetadataInnerJob) GetField(key string) (interface{}, error) {
	return GenericGetField(inner, key)
}

// GetFieldInt64 implements Getter interface
func (inner *MetadataInnerJob) GetFieldInt64(key string) (int64, error) {
	return GenericGetFieldInt64(inner, key)
}

// GetFieldString implements Getter interface
func (inner *MetadataInnerJob) GetFieldString(key string) (string, error) {
	return GenericGetFieldString(inner, key)
}

// GetFieldKeys implements Getter interface
func (inner *MetadataInnerJob) GetFieldKeys() []string {
	return GenericGetFieldKeys(inner)
}

// MetadataInnerJobDecoder implements a json message raw decoder
func MetadataInnerJobDecoder(raw json.RawMessage) (common.Getter, error) {
	var inner MetadataInnerJob
	return GenericMetadataDecoder(&inner, raw)
}

type jobHandler struct {
}

func (h *jobHandler) Dump(obj interface{}) string {
	job := obj.(*batchv1.Job)
	return fmt.Sprintf("job{Namespace: %s, Name: %s}", job.Namespace, job.Name)
}

func (h *jobHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	job := obj.(*batchv1.Job)

	inner := new(MetadataInnerJob)
	inner.MetadataInner.Setup(&job.ObjectMeta, job)
	inner.Parallelism = *job.Spec.Parallelism
	inner.Completions = *job.Spec.Completions
	inner.Active = job.Status.Active
	inner.Succeeded = job.Status.Succeeded
	inner.Failed = job.Status.Failed

	return graph.Identifier(job.GetUID()), NewMetadata(Manager, "job", inner.Name, inner)
}

func newJobProbe(client interface{}, g *graph.Graph) Subprobe {
	RegisterNodeDecoder(MetadataInnerJobDecoder, "job")
	return NewResourceCache(client.(*kubernetes.Clientset).BatchV1().RESTClient(), &batchv1.Job{}, "jobs", g, &jobHandler{})
}
