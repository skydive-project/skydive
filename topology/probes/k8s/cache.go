/*
 * Copyright (C) 2017 Red Hat, Inc.
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
	"time"

	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"

	api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

// ResourceHandler is used to map Kubernetes resources to objets in the graph
type ResourceHandler interface {
	Map(obj interface{}) (graph.Identifier, graph.Metadata)
	Dump(obj interface{}) string
	IsTopLevel() bool
}

// DefaultResourceHandler defines a default Kubernetes resource handler
type DefaultResourceHandler struct {
}

// IsTopLevel returns whether the resource is top level (no parent)
func (h *DefaultResourceHandler) IsTopLevel() bool {
	return false
}

// ResourceCache describes a cache for a specific kind of Kubernetes resource.
// It is in charge of listening to Kubernetes events and creating the
// according resource in the graph with the informations returned by
// the associated resource handler
type ResourceCache struct {
	*graph.EventHandler
	cache          cache.Indexer
	controller     cache.Controller
	stopController chan (struct{})
	handler        ResourceHandler
	graph          *graph.Graph
}

func (c *ResourceCache) list() []interface{} {
	return c.cache.List()
}

func (c *ResourceCache) getByKey(namespace, name string) interface{} {
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

func (c *ResourceCache) getByNode(node *graph.Node) interface{} {
	namespace, _ := node.GetFieldString("Namespace")
	name, _ := node.GetFieldString("Name")
	if name == "" {
		return nil
	}
	return c.getByKey(namespace, name)
}

func (c *ResourceCache) getByNamespace(namespace string) []interface{} {
	if namespace == api.NamespaceAll {
		return c.list()
	}

	objects, _ := c.cache.ByIndex("namespace", namespace)
	return objects
}

func (c *ResourceCache) getBySelector(g *graph.Graph, namespace string, selector *metav1.LabelSelector) []metav1.Object {
	if objects := c.getByNamespace(namespace); len(objects) > 0 {
		return filterObjectsBySelector(objects, selector)
	}
	return nil
}

// OnAdd is called when a new Kubernetes resource has been created
func (c *ResourceCache) OnAdd(obj interface{}) {
	c.graph.Lock()
	defer c.graph.Unlock()

	id, metadata := c.handler.Map(obj)
	node := c.graph.NewNode(id, metadata, "")
	c.NotifyEvent(graph.NodeAdded, node)
	logging.GetLogger().Debugf("Added %s", c.handler.Dump(obj))
}

// OnUpdate is called when a Kubernetes resource has been updated
func (c *ResourceCache) OnUpdate(oldObj, newObj interface{}) {
	c.graph.Lock()
	defer c.graph.Unlock()

	id, metadata := c.handler.Map(newObj)
	if node := c.graph.GetNode(id); node != nil {
		c.graph.SetMetadata(node, metadata)
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
		c.graph.DelNode(node)
		c.NotifyEvent(graph.NodeDeleted, node)
		logging.GetLogger().Debugf("Deleted %s", c.handler.Dump(obj))
	}
}

// Start begin waiting on Kubernetes events
func (c *ResourceCache) Start() {
	c.cache.Resync()
	go c.controller.Run(c.stopController)
}

// Stop end waiting on Kubernetes events
func (c *ResourceCache) Stop() {
	c.stopController <- struct{}{}
}

// NewResourceCache returns a new cache using the associed Kubernetes
// client and with the handler for the resource that this cache manages.
func NewResourceCache(restClient rest.Interface, objType runtime.Object, resources string, g *graph.Graph, handler ResourceHandler) *ResourceCache {
	watchlist := cache.NewListWatchFromClient(restClient, resources, api.NamespaceAll, fields.Everything())

	c := &ResourceCache{
		EventHandler:   graph.NewEventHandler(100),
		graph:          g,
		handler:        handler,
		stopController: make(chan struct{}),
	}

	cacheHandler := cache.ResourceEventHandlerFuncs{}
	if handler != nil {
		cacheHandler.AddFunc = c.OnAdd
		cacheHandler.UpdateFunc = c.OnUpdate
		cacheHandler.DeleteFunc = c.OnDelete
	}

	indexers := cache.Indexers{"namespace": cache.MetaNamespaceIndexFunc}
	c.cache, c.controller = cache.NewIndexerInformer(watchlist, objType, 30*time.Minute, cacheHandler, indexers)
	return c
}

func matchSelector(obj metav1.Object, selector labels.Selector) bool {
	return selector.Matches(labels.Set(obj.GetLabels()))
}

func filterObjectsBySelector(objects []interface{}, labelSelector *metav1.LabelSelector) (out []metav1.Object) {
	selector, _ := metav1.LabelSelectorAsSelector(labelSelector)
	for _, obj := range objects {
		obj := obj.(metav1.Object)
		if matchSelector(obj, selector) {
			out = append(out, obj)
		}
	}
	return
}
