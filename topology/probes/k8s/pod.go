/*
 * Copyright (C) 2017 Red Hat, Inc.
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

	"github.com/mohae/deepcopy"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/probe"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

type podHandler struct {
	graph.DefaultGraphListener
	graph *graph.Graph
	cache *ResourceCache
}

func (h *podHandler) Dump(obj interface{}) string {
	pod := obj.(*v1.Pod)
	return fmt.Sprintf("pod{Namespace: %s, Name: %s}", pod.Namespace, pod.Name)
}

func (h *podHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	// we make a copy of pod before modifaying the pod object so
	// that don't interfere with the container subprobe
	pod := deepcopy.Copy(obj).(*v1.Pod)

	pod.Spec.Containers = nil

	m := NewMetadataFields(&pod.ObjectMeta)
	m.SetField("Node", pod.Spec.NodeName)
	podIP := pod.Status.PodIP
	if podIP != "" {
		m.SetField("IP", podIP)
	}
	reason := string(pod.Status.Phase)
	if pod.Status.Reason != "" {
		reason = pod.Status.Reason
	}
	m.SetField("Status", reason)

	metadata := NewMetadata(Manager, "pod", m, pod, pod.Name)
	SetState(&metadata, reason == "Running")

	return graph.Identifier(pod.GetUID()), metadata
}

func newPodProbe(client interface{}, g *graph.Graph) Subprobe {
	return NewResourceCache(client.(*kubernetes.Clientset).CoreV1().RESTClient(), &v1.Pod{}, "pods", g, &podHandler{graph: g})
}

func podPVCAreLinked(a, b interface{}) bool {
	pod := a.(*v1.Pod)
	pvc := b.(*v1.PersistentVolumeClaim)

	if !MatchNamespace(pod, pvc) {
		return false
	}

	for _, vol := range pod.Spec.Volumes {
		if vol.VolumeSource.PersistentVolumeClaim != nil && vol.VolumeSource.PersistentVolumeClaim.ClaimName == pvc.Name {
			return true
		}
	}
	return false
}

func newPodPVCLinker(g *graph.Graph) probe.Handler {
	return NewABLinker(g, Manager, "pod", Manager, "persistentvolumeclaim", podPVCAreLinked)
}

func podConfigMapAreLinked(a, b interface{}) bool {
	pod := a.(*v1.Pod)
	cm := b.(*v1.ConfigMap)

	if !MatchNamespace(pod, cm) {
		return false
	}

	for _, container := range pod.Spec.Containers {
		for _, envVar := range container.Env {
			if envVar.ValueFrom != nil && envVar.ValueFrom.ConfigMapKeyRef != nil && envVar.ValueFrom.ConfigMapKeyRef.Name == cm.Name {
				return true
			}
		}

		for _, envFrom := range container.EnvFrom {
			if envFrom.ConfigMapRef != nil && envFrom.ConfigMapRef.Name == cm.Name {
				return true
			}
		}
	}

	for _, vol := range pod.Spec.Volumes {
		if vol.ConfigMap != nil && vol.ConfigMap.Name == cm.Name {
			return true
		}
	}

	return false
}

func newPodConfigMapLinker(g *graph.Graph) probe.Handler {
	return NewABLinker(g, Manager, "pod", Manager, "configmap", podConfigMapAreLinked)
}

func podSecretAreLinked(a, b interface{}) bool {
	pod := a.(*v1.Pod)
	secret := b.(*v1.Secret)

	if !MatchNamespace(pod, secret) {
		return false
	}

	for _, container := range pod.Spec.Containers {
		for _, envVar := range container.Env {
			if envVar.ValueFrom != nil && envVar.ValueFrom.SecretKeyRef != nil && envVar.ValueFrom.SecretKeyRef.Name == secret.Name {
				return true
			}
		}

		for _, envFrom := range container.EnvFrom {
			if envFrom.SecretRef != nil && envFrom.SecretRef.Name == secret.Name {
				return true
			}
		}
	}

	for _, vol := range pod.Spec.Volumes {
		if vol.Secret != nil && vol.Secret.SecretName == secret.Name {
			return true
		}
	}

	return false
}

func newPodSecretLinker(g *graph.Graph) probe.Handler {
	return NewABLinker(g, Manager, "pod", Manager, "secret", podSecretAreLinked)
}
