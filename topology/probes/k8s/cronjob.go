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

	"k8s.io/api/batch/v1beta1"
	"k8s.io/client-go/kubernetes"
)

// MetadataInnerCronJob contains the type specific fields
// easyjson:json
type MetadataInnerCronJob struct {
	MetadataInner
	Schedule  string `skydive:"string"`
	Suspended bool   `skydive:""`
}

// GetField implements Getter interface
func (inner *MetadataInnerCronJob) GetField(key string) (interface{}, error) {
	return GenericGetField(inner, key)
}

// GetFieldInt64 implements Getter interface
func (inner *MetadataInnerCronJob) GetFieldInt64(key string) (int64, error) {
	return GenericGetFieldInt64(inner, key)
}

// GetFieldString implements Getter interface
func (inner *MetadataInnerCronJob) GetFieldString(key string) (string, error) {
	return GenericGetFieldString(inner, key)
}

// GetFieldKeys implements Getter interface
func (inner *MetadataInnerCronJob) GetFieldKeys() []string {
	return GenericGetFieldKeys(inner)
}

// MetadataInnerCronJobDecoder implements a json message raw decoder
func MetadataInnerCronJobDecoder(raw json.RawMessage) (common.Getter, error) {
	var inner MetadataInnerCronJob
	return GenericMetadataDecoder(&inner, raw)
}

type cronJobHandler struct {
}

func (h *cronJobHandler) Dump(obj interface{}) string {
	cj := obj.(*v1beta1.CronJob)
	return fmt.Sprintf("cronjob{Namespace: %s, Name: %s}", cj.Namespace, cj.Name)
}

func (h *cronJobHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	cj := obj.(*v1beta1.CronJob)

	inner := new(MetadataInnerCronJob)
	inner.MetadataInner.Setup(&cj.ObjectMeta, cj)
	inner.Schedule = cj.Spec.Schedule
	inner.Suspended = cj.Spec.Suspend != nil && *cj.Spec.Suspend

	return graph.Identifier(cj.GetUID()), NewMetadata(Manager, "cronjob", inner.Name, inner)
}

func newCronJobProbe(client interface{}, g *graph.Graph) Subprobe {
	RegisterNodeDecoder(MetadataInnerCronJobDecoder, "cronjob")
	return NewResourceCache(client.(*kubernetes.Clientset).BatchV1beta1().RESTClient(), &v1beta1.CronJob{}, "cronjobs", g, &cronJobHandler{})
}
