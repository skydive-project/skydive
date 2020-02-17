/*
 * Copyright (C) 2018 Red Hat, Inc.
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

	"github.com/skydive-project/skydive/filters"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	dockerContainerNameField = "Docker.Labels.io.kubernetes.container.name"
	dockerPodNameField       = "Docker.Labels.io.kubernetes.pod.name"
	dockerPodNamespaceField  = "Docker.Labels.io.kubernetes.pod.namespace"
)

type containerProbe struct {
	*graph.EventHandler
	*KubeCache
	graph            *graph.Graph
	containerIndexer *graph.MetadataIndexer
}

func (c *containerProbe) newMetadata(pod *v1.Pod, container *v1.Container) graph.Metadata {
	m := NewMetadataFields(&pod.ObjectMeta)
	m.SetField("Pod", pod.Name)
	m.SetField("Name", container.Name)
	m.SetField("Image", container.Image)
	return NewMetadata(Manager, "container", m, container, container.Name)
}

func (c *containerProbe) dump(pod *v1.Pod, name string) string {
	return fmt.Sprintf("container{Namespace: %s, Pod: %s, Name: %s}", pod.Namespace, pod.Name, name)
}

// OnAdd is called when a new Kubernetes resource has been created
func (c *containerProbe) OnAdd(obj interface{}) {
	if pod, ok := obj.(*v1.Pod); ok {
		logging.GetLogger().Debugf("Added/Updated pod{Namespace: %s, Name: %s}", pod.Namespace, pod.Name)

		c.graph.Lock()
		defer c.graph.Unlock()

		wasUpdated := make(map[string]bool)

		for _, container := range pod.Spec.Containers {
			uid := graph.GenID(string(pod.GetUID()), container.Name)
			m := c.newMetadata(pod, &container)
			if node := c.graph.GetNode(uid); node == nil {
				var err error

				node, err = c.graph.NewNode(uid, m)
				if err != nil {
					logging.GetLogger().Error(err)
					continue
				}
				c.NotifyEvent(graph.NodeAdded, node)
				logging.GetLogger().Debugf("Added %s", c.dump(pod, container.Name))
			} else {
				if err := c.graph.SetMetadata(node, m); err != nil {
					logging.GetLogger().Error(err)
					continue
				}
				c.NotifyEvent(graph.NodeUpdated, node)
				logging.GetLogger().Debugf("Updated %s", c.dump(pod, container.Name))
			}
			wasUpdated[container.Name] = true
		}

		nodes, _ := c.containerIndexer.Get(pod.Namespace, pod.Name)
		for _, node := range nodes {
			name, _ := node.GetFieldString(MetadataField("Name"))
			if !wasUpdated[name] {
				if err := c.graph.DelNode(node); err != nil {
					logging.GetLogger().Error(err)
					continue
				}
				c.NotifyEvent(graph.NodeDeleted, node)
				logging.GetLogger().Debugf("Deleted %s", c.dump(pod, name))
			}
		}
	}
}

// OnUpdate is called when a Kubernetes resource has been updated
func (c *containerProbe) OnUpdate(oldObj, newObj interface{}) {
	c.OnAdd(newObj)
}

// OnDelete is called when a Kubernetes resource has been deleted
func (c *containerProbe) OnDelete(obj interface{}) {
	if pod, ok := obj.(*v1.Pod); ok {
		logging.GetLogger().Debugf("Deleted pod{Namespace: %s, Name: %s}", pod.Namespace, pod.Name)

		c.graph.Lock()
		defer c.graph.Unlock()

		containerNodes, _ := c.containerIndexer.Get(pod.Namespace, pod.Name)
		for _, containerNode := range containerNodes {
			name, _ := containerNode.GetFieldString(MetadataField("Name"))
			if err := c.graph.DelNode(containerNode); err != nil {
				logging.GetLogger().Error(err)
				continue
			}
			c.NotifyEvent(graph.NodeDeleted, containerNode)
			logging.GetLogger().Debugf("Deleted %s", c.dump(pod, name))
		}
	}
}

func newContainerProbe(client interface{}, g *graph.Graph) Subprobe {
	c := &containerProbe{
		EventHandler: graph.NewEventHandler(100),
		graph:        g,
	}

	containerFilter := newTypesFilter(Manager, "container")
	c.containerIndexer = newObjectIndexerFromFilter(g, c, containerFilter, MetadataFields("Namespace", "Pod")...)
	c.containerIndexer.Start()
	c.KubeCache = RegisterKubeCache(client.(*kubernetes.Clientset).CoreV1().RESTClient(), &v1.Pod{}, "pods", c)
	return c
}

func newPodContainerLinker(g *graph.Graph) probe.Handler {
	podIndexer := newResourceIndexer(g, Manager, "pod", MetadataFields("Namespace", "Name"))
	containerIndexer := newResourceIndexer(g, Manager, "container", MetadataFields("Namespace", "Pod"))
	return newResourceLinker(g, podIndexer, containerIndexer, topology.OwnershipMetadata())
}

func newDockerIndexer(g *graph.Graph) *graph.MetadataIndexer {
	m := graph.NewElementFilter(filters.NewAndFilter(
		filters.NewTermStringFilter("Manager", "docker"),
		filters.NewTermStringFilter("Type", "container"),
		filters.NewNotNullFilter(dockerPodNamespaceField),
		filters.NewNotNullFilter(dockerPodNameField),
		filters.NewNotNullFilter(dockerContainerNameField),
	))

	return graph.NewMetadataIndexer(g, g, m, dockerPodNamespaceField, dockerPodNameField, dockerContainerNameField)
}

func newContainerDockerLinker(g *graph.Graph) probe.Handler {
	containerProbe := GetSubprobe(Manager, "container")
	if containerProbe == nil {
		return nil
	}

	containerFilter := newTypesFilter(Manager, "container")
	containerIndexer := newObjectIndexerFromFilter(g, containerProbe, containerFilter, MetadataFields("Namespace", "Pod", "Name")...)
	containerIndexer.Start()

	dockerIndexer := newDockerIndexer(g)
	dockerIndexer.Start()

	ml := graph.NewMetadataIndexerLinker(g, containerIndexer, dockerIndexer, NewEdgeMetadata(Manager, "container"))

	linker := &Linker{
		ResourceLinker: ml.ResourceLinker,
	}
	ml.AddEventListener(linker)

	return linker
}
