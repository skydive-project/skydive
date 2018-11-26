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

	"k8s.io/api/extensions/v1beta1"
	"k8s.io/client-go/kubernetes"
)

// MetadataInnerDaemonSet contains the type specific fields
// easyjson:json
type MetadataInnerDaemonSet struct {
	MetadataInner
	DesiredNumberScheduled int32 `skydive:"int"`
	CurrentNumberScheduled int32 `skydive:"int"`
	NumberMisscheduled     int32 `skydive:"int"`
}

// GetField implements Getter interface
func (inner *MetadataInnerDaemonSet) GetField(key string) (interface{}, error) {
	return GenericGetField(inner, key)
}

// GetFieldInt64 implements Getter interface
func (inner *MetadataInnerDaemonSet) GetFieldInt64(key string) (int64, error) {
	return GenericGetFieldInt64(inner, key)
}

// GetFieldString implements Getter interface
func (inner *MetadataInnerDaemonSet) GetFieldString(key string) (string, error) {
	return GenericGetFieldString(inner, key)
}

// GetFieldKeys implements Getter interface
func (inner *MetadataInnerDaemonSet) GetFieldKeys() []string {
	return GenericGetFieldKeys(inner)
}

// MetadataInnerDaemonSetDecoder implements a json message raw decoder
func MetadataInnerDaemonSetDecoder(raw json.RawMessage) (common.Getter, error) {
	var inner MetadataInnerDaemonSet
	return GenericMetadataDecoder(&inner, raw)
}

type daemonSetHandler struct {
}

func (h *daemonSetHandler) Dump(obj interface{}) string {
	ds := obj.(*v1beta1.DaemonSet)
	return fmt.Sprintf("daemonset{Namespace: %s, Name: %s}", ds.Namespace, ds.Name)
}

func (h *daemonSetHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	ds := obj.(*v1beta1.DaemonSet)

	inner := new(MetadataInnerDaemonSet)
	inner.MetadataInner.Setup(&ds.ObjectMeta, ds)
	inner.DesiredNumberScheduled = ds.Status.DesiredNumberScheduled
	inner.CurrentNumberScheduled = ds.Status.CurrentNumberScheduled
	inner.NumberMisscheduled = ds.Status.NumberMisscheduled

	return graph.Identifier(ds.GetUID()), NewMetadata(Manager, "daemonset", inner.Name, inner)
}

func newDaemonSetProbe(client interface{}, g *graph.Graph) Subprobe {
	RegisterNodeDecoder(MetadataInnerDaemonSetDecoder, "daemonset")
	return NewResourceCache(client.(*kubernetes.Clientset).ExtensionsV1beta1().RESTClient(), &v1beta1.DaemonSet{}, "daemonsets", g, &daemonSetHandler{})
}
