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

	"k8s.io/api/batch/v1beta1"
	"k8s.io/client-go/kubernetes"
)

type cronJobHandler struct {
}

func (h *cronJobHandler) Dump(obj interface{}) string {
	cj := obj.(*v1beta1.CronJob)
	return fmt.Sprintf("cronjob{Namespace: %s, Name: %s}", cj.Namespace, cj.Name)
}

func (h *cronJobHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	cj := obj.(*v1beta1.CronJob)

	m := NewMetadataFields(&cj.ObjectMeta)
	m.SetField("Schedule", cj.Spec.Schedule)
	m.SetField("Suspended", cj.Spec.Suspend != nil && *cj.Spec.Suspend)

	return graph.Identifier(cj.GetUID()), NewMetadata(Manager, "cronjob", m, cj, cj.Name)
}

func newCronJobProbe(client interface{}, g *graph.Graph) Subprobe {
	return NewResourceCache(client.(*kubernetes.Clientset).BatchV1beta1().RESTClient(), &v1beta1.CronJob{}, "cronjobs", g, &cronJobHandler{})
}
