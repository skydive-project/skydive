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
	"time"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/logging"

	api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

type k8sHandler interface {
	OnAdd(obj interface{})
	OnUpdate(oldObj, newObj interface{})
	OnDelete(obj interface{})
}

// KubeCache describes a generic cache for Kubernetes resources.
type KubeCache struct {
	cache          cache.Indexer
	controller     cache.Controller
	stopController chan (struct{})
	handlers       []k8sHandler
}

// List returns a list of resources
func (c *KubeCache) List() []interface{} {
	return c.cache.List()
}

func (c *KubeCache) getByKey(namespace, name string) interface{} {
	key := ""
	if len(namespace) > 0 {
		key = namespace + "/"
	}
	key += name
	if obj, found, _ := c.cache.GetByKey(key); found {
		return obj
	}
	return nil
}

// GetByNode returns graph node according to name and namespace
func (c *KubeCache) GetByNode(node *graph.Node) interface{} {
	namespace, _ := node.GetFieldString(MetadataField("Namespace"))
	name, _ := node.GetFieldString("Name")
	if name == "" {
		return nil
	}
	return c.getByKey(namespace, name)
}

func (c *KubeCache) getByNamespace(namespace string) []interface{} {
	if namespace == api.NamespaceAll {
		return c.List()
	}

	objects, _ := c.cache.ByIndex("namespace", namespace)
	return objects
}

func (c *KubeCache) getBySelector(g *graph.Graph, namespace string, selector *metav1.LabelSelector) []metav1.Object {
	if objects := c.getByNamespace(namespace); len(objects) > 0 {
		return filterObjectsBySelector(objects, selector)
	}
	return nil
}

// Start begin waiting on Kubernetes events
func (c *KubeCache) Start() error {
	c.cache.Resync()
	go c.controller.Run(c.stopController)
	return nil
}

// Stop end waiting on Kubernetes events
func (c *KubeCache) Stop() {
	c.stopController <- struct{}{}
}

// NewKubeCache returns a new cache using the associed Kubernetes client.
func NewKubeCache(restClient rest.Interface, objType runtime.Object, resources string) *KubeCache {
	watchlist := cache.NewListWatchFromClient(restClient, resources, api.NamespaceAll, fields.Everything())

	c := &KubeCache{
		handlers:       []k8sHandler{},
		stopController: make(chan struct{}),
	}

	cacheHandler := cache.ResourceEventHandlerFuncs{}
	cacheHandler.AddFunc = c.onAdd
	cacheHandler.UpdateFunc = c.onUpdate
	cacheHandler.DeleteFunc = c.onDelete

	indexers := cache.Indexers{"namespace": cache.MetaNamespaceIndexFunc}
	c.cache, c.controller = cache.NewIndexerInformer(watchlist, objType, 30*time.Minute, cacheHandler, indexers)
	return c
}

var kubeCacheMap = make(map[string]*KubeCache)

// RegisterKubeCache registers resource handler to kubernetes events.
func RegisterKubeCache(restClient rest.Interface, objType runtime.Object, resources string, handler k8sHandler) *KubeCache {
	if _, ok := kubeCacheMap[resources]; !ok {
		kubeCacheMap[resources] = NewKubeCache(restClient, objType, resources)
	}
	c := kubeCacheMap[resources]

	c.handlers = append(c.handlers, handler)

	return c
}

func (c *KubeCache) onAdd(obj interface{}) {
	for _, h := range c.handlers {
		h.OnAdd(obj)
	}
}

func (c *KubeCache) onUpdate(oldObj, newObj interface{}) {
	for _, h := range c.handlers {
		h.OnUpdate(oldObj, newObj)
	}
}

func (c *KubeCache) onDelete(obj interface{}) {
	for _, h := range c.handlers {
		h.OnDelete(obj)
	}
}

// MatchNamespace true if namespaces are identical
func MatchNamespace(obj1, obj2 metav1.Object) bool {
	return obj1.GetNamespace() == obj2.GetNamespace()
}

func matchSelector(obj metav1.Object, selector labels.Selector, matchEmpty ...bool) bool {
	if selector.Empty() && len(matchEmpty) > 0 && !matchEmpty[0] {
		return false
	}
	return selector.Matches(labels.Set(obj.GetLabels()))
}

func matchLabelSelector(obj metav1.Object, labelSelector *metav1.LabelSelector, matchEmpty ...bool) bool {
	selector, err := metav1.LabelSelectorAsSelector(labelSelector)
	return err == nil && matchSelector(obj, selector, matchEmpty...)
}

func matchMapSelector(obj metav1.Object, mapSelector map[string]string, matchEmpty ...bool) bool {
	labelSelector := &metav1.LabelSelector{MatchLabels: mapSelector}
	return matchLabelSelector(obj, labelSelector, matchEmpty...)
}

func filterObjectsBySelector(objects []interface{}, labelSelector *metav1.LabelSelector, namespace ...string) (out []metav1.Object) {
	selector, _ := metav1.LabelSelectorAsSelector(labelSelector)
	for _, obj := range objects {
		obj := obj.(metav1.Object)
		if len(namespace) > 0 && obj.GetNamespace() != namespace[0] {
			continue
		}
		if matchSelector(obj, selector) {
			out = append(out, obj)
		}
	}
	return
}

// ResourceHandler is used to map Kubernetes resources to objets in the graph
type ResourceHandler interface {
	Map(obj interface{}) (graph.Identifier, graph.Metadata)
	Dump(obj interface{}) string
}

// ResourceCache describes a cache for a specific kind of Kubernetes resource.
// It is in charge of listening to Kubernetes events and creating the
// according resource in the graph with the informations returned by
// the associated resource handler
type ResourceCache struct {
	*graph.EventHandler
	*KubeCache
	graph       *graph.Graph
	handler     ResourceHandler
	clusterName string
}

// OnAdd is called when a new Kubernetes resource has been created
func (c *ResourceCache) OnAdd(obj interface{}) {
	c.graph.Lock()
	defer c.graph.Unlock()

	id, metadata := c.handler.Map(obj)
	if len(c.clusterName) > 0 {
		metadata.SetField(ClusterNameField, c.clusterName)
	}

	node, err := c.graph.NewNode(id, metadata, "")
	if err != nil {
		logging.GetLogger().Error(err)
		return
	}
	c.NotifyEvent(graph.NodeAdded, node)
	logging.GetLogger().Debugf("Added %s", c.handler.Dump(obj))
}

// OnUpdate is called when a Kubernetes resource has been updated
func (c *ResourceCache) OnUpdate(oldObj, newObj interface{}) {
	c.graph.Lock()
	defer c.graph.Unlock()

	id, metadata := c.handler.Map(newObj)
	if len(c.clusterName) > 0 {
		metadata.SetFieldAndNormalize(ClusterNameField, c.clusterName)
	}
	if node := c.graph.GetNode(id); node != nil {
		if err := c.graph.SetMetadata(node, metadata); err != nil {
			logging.GetLogger().Error(err)
			return
		}
		c.NotifyEvent(graph.NodeUpdated, node)
		logging.GetLogger().Debugf("Updated %s", c.handler.Dump(newObj))
	}
}

// OnDelete is called when a Kubernetes resource has been deleted
func (c *ResourceCache) OnDelete(obj interface{}) {
	c.graph.Lock()
	defer c.graph.Unlock()

	id, _ := c.handler.Map(obj)
	if node := c.graph.GetNode(id); node != nil {
		if err := c.graph.DelNode(node); err != nil {
			logging.GetLogger().Error(err)
			return
		}
		c.NotifyEvent(graph.NodeDeleted, node)
		logging.GetLogger().Debugf("Deleted %s", c.handler.Dump(obj))
	}
}

// NewResourceCache returns a new cache using the associed Kubernetes
// client and with the handler for the resource that this cache manages.
func NewResourceCache(restClient rest.Interface, objType runtime.Object, resources string, g *graph.Graph, handler ResourceHandler) *ResourceCache {
	c := &ResourceCache{
		EventHandler: graph.NewEventHandler(100),
		graph:        g,
		handler:      handler,
	}
	c.KubeCache = RegisterKubeCache(restClient, objType, resources, c)
	return c
}

func mapStringToList(data map[string]string) []string {
	newData := []string{}
	for key := range data {
		newData = append(newData, key)
	}
	return newData
}

func mapBytesToList(data map[string][]byte) []string {
	newData := []string{}
	for key := range data {
		newData = append(newData, key)
	}
	return newData
}
