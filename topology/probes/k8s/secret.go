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

	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// MetadataInnerSecret contains the type specific fields
// easyjson:json
type MetadataInnerSecret struct {
	MetadataInner
	Type string `skydive:"string"`
}

// GetField implements Getter interface
func (inner *MetadataInnerSecret) GetField(key string) (interface{}, error) {
	return GenericGetField(inner, key)
}

// GetFieldInt64 implements Getter interface
func (inner *MetadataInnerSecret) GetFieldInt64(key string) (int64, error) {
	return GenericGetFieldInt64(inner, key)
}

// GetFieldString implements Getter interface
func (inner *MetadataInnerSecret) GetFieldString(key string) (string, error) {
	return GenericGetFieldString(inner, key)
}

// GetFieldKeys implements Getter interface
func (inner *MetadataInnerSecret) GetFieldKeys() []string {
	return GenericGetFieldKeys(inner)
}

// MetadataInnerSecretDecoder implements a json message raw decoder
func MetadataInnerSecretDecoder(raw json.RawMessage) (common.Getter, error) {
	var inner MetadataInnerSecret
	return GenericMetadataDecoder(&inner, raw)
}

type secretHandler struct {
}

func (h *secretHandler) Dump(obj interface{}) string {
	secret := obj.(*v1.Secret)
	return fmt.Sprintf("secret{Namespace: %s, Name: %s}", secret.Namespace, secret.Name)
}

func (h *secretHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	secret := obj.(*v1.Secret)

	inner := new(MetadataInnerSecret)
	inner.MetadataInner.Setup(&secret.ObjectMeta, secret)
	inner.Type = string(secret.Type)
	return graph.Identifier(secret.GetUID()), NewMetadata(Manager, "secret", inner.Name, inner)
}

func newSecretProbe(client interface{}, g *graph.Graph) Subprobe {
	RegisterNodeDecoder(MetadataInnerDecoder, "secret")
	return NewResourceCache(client.(*kubernetes.Clientset).CoreV1().RESTClient(), &v1.Secret{}, "secrets", g, &secretHandler{})
}
