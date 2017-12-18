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

	api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

type kubeCache struct {
	cache          cache.Store
	controller     cache.Controller
	stopController chan (struct{})
}

type defaultKubeCacheEventHandler struct {
}

func (d *defaultKubeCacheEventHandler) OnAdd(obj interface{}) {
}

func (d *defaultKubeCacheEventHandler) OnUpdate(old, new interface{}) {
}

func (d *defaultKubeCacheEventHandler) OnDelete(obj interface{}) {
}

func newKubeCache(restClient rest.Interface, objType runtime.Object, resources string, handler cache.ResourceEventHandler) *kubeCache {
	watchlist := cache.NewListWatchFromClient(restClient, resources, api.NamespaceAll, fields.Everything())
	c := &kubeCache{stopController: make(chan struct{})}
	c.cache, c.controller = cache.NewInformer(watchlist, objType, 30*time.Minute, cache.ResourceEventHandlerFuncs{
		AddFunc:    handler.OnAdd,
		UpdateFunc: handler.OnUpdate,
		DeleteFunc: handler.OnDelete,
	})
	return c
}

func (c *kubeCache) Start() {
	c.cache.Resync()
	go c.controller.Run(c.stopController)
}

func (c *kubeCache) Stop() {
	c.stopController <- struct{}{}
}
