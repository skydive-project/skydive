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
	"fmt"
	"time"

	"github.com/skydive-project/skydive/config"

	api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

var clientset *kubernetes.Clientset

func NewConfig() (*rest.Config, error) {
	file := config.GetString("analyzer.topology.k8s.config_file")
	config, err := clientcmd.BuildConfigFromFlags("", file)
	if err != nil {
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()

		configOverrides := &clientcmd.ConfigOverrides{}

		kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
		config, err = kubeConfig.ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("Failed to load Kubernetes config: %s", err.Error())
		}
	}

	return config, err
}

func newClientset() (*kubernetes.Clientset, error) {
	config, err := NewConfig()
	if err != nil {
		return nil, err
	}

	clntset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("Failed to create Kubernetes client: %s", err.Error())
	}

	return clntset, nil
}

func initClientset() (err error) {
	clientset, err = newClientset()
	return
}

func getClientset() *kubernetes.Clientset {
	if clientset == nil {
		panic("clientset was not initialized, aborting!")
	}
	return clientset
}

type KubeCache struct {
	cache          cache.Store
	controller     cache.Controller
	stopController chan (struct{})
}

func (c *KubeCache) list() []interface{} {
	return c.cache.List()
}

func (c *KubeCache) listByNamespace(namespace string) (objList []interface{}) {
	if namespace == api.NamespaceAll {
		return c.list()
	}
	for _, obj := range c.list() {
		ns := obj.(*api.Pod).GetNamespace()
		if len(ns) == 0 || ns == namespace {
			objList = append(objList, obj)
		}
	}
	return
}

func (c *KubeCache) getByKey(namespace, name string) interface{} {
	key := ""
	if len(namespace) > 0 {
		key += namespace + "/"
	}
	key += name
	if obj, found, _ := c.cache.GetByKey(key); found {
		return obj
	}
	return nil
}

type DefaultKubeCacheEventHandler struct {
}

func (d *DefaultKubeCacheEventHandler) OnAdd(obj interface{}) {
}

func (d *DefaultKubeCacheEventHandler) OnUpdate(old, new interface{}) {
}

func (d *DefaultKubeCacheEventHandler) OnDelete(obj interface{}) {
}

func NewKubeCache(restClient rest.Interface, objType runtime.Object, resources string, handler cache.ResourceEventHandler) *KubeCache {
	watchlist := cache.NewListWatchFromClient(restClient, resources, api.NamespaceAll, fields.Everything())

	cacheHandler := cache.ResourceEventHandlerFuncs{}
	if handler != nil {
		cacheHandler.AddFunc = handler.OnAdd
		cacheHandler.UpdateFunc = handler.OnUpdate
		cacheHandler.DeleteFunc = handler.OnDelete
	}

	c := &KubeCache{stopController: make(chan struct{})}
	c.cache, c.controller = cache.NewInformer(watchlist, objType, 30*time.Minute, cacheHandler)
	return c
}

func (c *KubeCache) Start() {
	c.cache.Resync()
	go c.controller.Run(c.stopController)
}

func (c *KubeCache) Stop() {
	c.stopController <- struct{}{}
}
