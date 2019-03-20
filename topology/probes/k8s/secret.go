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
	"fmt"

	"github.com/skydive-project/skydive/graffiti/graph"

	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

type secretHandler struct {
}

func (h *secretHandler) Dump(obj interface{}) string {
	secret := obj.(*v1.Secret)
	return fmt.Sprintf("secret{Namespace: %s, Name: %s}", secret.Namespace, secret.Name)
}

func (h *secretHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	secret := obj.(*v1.Secret)
	m := NewMetadataFields(&secret.ObjectMeta)
	m.SetField("Type", secret.Type)
	m.SetField("Data", mapBytesToList(secret.Data))
	return graph.Identifier(secret.GetUID()), NewMetadata(Manager, "secret", m, secret, secret.Name)
}

func newSecretProbe(client interface{}, g *graph.Graph) Subprobe {
	return NewResourceCache(client.(*kubernetes.Clientset).CoreV1().RESTClient(), &v1.Secret{}, "secrets", g, &secretHandler{})
}
