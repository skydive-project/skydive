/*
 * Copyright (C) 2016 Red Hat, Inc.
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

package api

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	etcd "github.com/coreos/etcd/client"
	"golang.org/x/net/context"

	"github.com/skydive-project/skydive/logging"
)

type ApiResource interface {
	ID() string
	SetID(string)
}

type ApiHandler interface {
	Name() string
	New() ApiResource
	Index() map[string]ApiResource
	Get(id string) (ApiResource, bool)
	Create(resource ApiResource) error
	Delete(id string) error
	AsyncWatch(f ApiWatcherCallback) StoppableWatcher
}

type ResourceHandler interface {
	Name() string
	New() ApiResource
}

// basic implementation of an ApiHandler, should be used as embedded struct
// for the most part of the resources
type BasicApiHandler struct {
	ResourceHandler ResourceHandler
	EtcdKeyAPI      etcd.KeysAPI
}

type ApiWatcherCallback func(action string, id string, resource ApiResource)

type StoppableWatcher interface {
	Stop()
}

type BasicStoppableWatcher struct {
	watcher etcd.Watcher
	running atomic.Value
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

type ApiResourceWatcher interface {
	AsyncWatch(f ApiWatcherCallback) StoppableWatcher
}

func (s *BasicStoppableWatcher) Stop() {
	s.cancel()
	s.running.Store(false)
	s.wg.Wait()
}

func (h *BasicApiHandler) Name() string {
	return h.ResourceHandler.Name()
}

func (h *BasicApiHandler) New() ApiResource {
	return h.ResourceHandler.New()
}

func (h *BasicApiHandler) collectNodes(flatten map[string]ApiResource, nodes etcd.Nodes) {
	for _, node := range nodes {
		if node.Dir {
			h.collectNodes(flatten, node.Nodes)
		} else {
			resource := h.ResourceHandler.New()

			json.Unmarshal([]byte(node.Value), resource)
			flatten[resource.ID()] = resource
		}
	}
}

func (h *BasicApiHandler) Index() map[string]ApiResource {
	etcdPath := fmt.Sprintf("/%s/", h.ResourceHandler.Name())

	resp, err := h.EtcdKeyAPI.Get(context.Background(), etcdPath, &etcd.GetOptions{Recursive: true})
	resources := make(map[string]ApiResource)

	if err == nil {
		h.collectNodes(resources, resp.Node.Nodes)
	}

	return resources
}

func (h *BasicApiHandler) Get(id string) (ApiResource, bool) {
	etcdPath := fmt.Sprintf("/%s/%s", h.ResourceHandler.Name(), id)

	resp, err := h.EtcdKeyAPI.Get(context.Background(), etcdPath, nil)
	if err != nil {
		return nil, false
	}

	resource := h.ResourceHandler.New()
	json.Unmarshal([]byte(resp.Node.Value), resource)
	return resource, true
}

func (h *BasicApiHandler) Create(resource ApiResource) error {
	data, err := json.Marshal(&resource)
	if err != nil {
		return err
	}

	etcdPath := fmt.Sprintf("/%s/%s", h.ResourceHandler.Name(), resource.ID())
	_, err = h.EtcdKeyAPI.Set(context.Background(), etcdPath, string(data), nil)
	return err
}

func (h *BasicApiHandler) Delete(id string) error {
	etcdPath := fmt.Sprintf("/%s/%s", h.ResourceHandler.Name(), id)

	if _, err := h.EtcdKeyAPI.Delete(context.Background(), etcdPath, nil); err != nil {
		return err
	}

	return nil
}

func (h *BasicApiHandler) AsyncWatch(f ApiWatcherCallback) StoppableWatcher {
	etcdPath := fmt.Sprintf("/%s/", h.ResourceHandler.Name())

	watcher := h.EtcdKeyAPI.Watcher(etcdPath, &etcd.WatcherOptions{Recursive: true})

	ctx, cancel := context.WithCancel(context.Background())
	sw := &BasicStoppableWatcher{
		watcher: watcher,
		ctx:     ctx,
		cancel:  cancel,
	}

	// init phase retrieve all the previous value and use init as action for the
	// callback
	for id, node := range h.Index() {
		f("init", id, node)
	}

	sw.wg.Add(1)
	sw.running.Store(true)
	go func() {
		defer sw.wg.Done()

		for sw.running.Load() == true {
			resp, err := watcher.Next(sw.ctx)
			if err != nil {
				logging.GetLogger().Errorf("Error while watching etcd: %s", err.Error())

				time.Sleep(1 * time.Second)
				continue
			}

			if resp.Node.Dir {
				continue
			}

			id := strings.TrimPrefix(resp.Node.Key, etcdPath)

			resource := h.ResourceHandler.New()

			switch resp.Action {
			case "expire", "delete":
				json.Unmarshal([]byte(resp.PrevNode.Value), resource)
			default:
				json.Unmarshal([]byte(resp.Node.Value), resource)
			}

			f(resp.Action, id, resource)
		}
	}()

	return sw
}
