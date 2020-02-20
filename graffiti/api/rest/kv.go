/*
 * Copyright (C) 2016 Red Hat, Inc.
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

package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	etcd "github.com/coreos/etcd/client"
	uuid "github.com/nu7hatch/gouuid"
	"github.com/skydive-project/skydive/logging"
)

// BasicAPIHandler basic implementation of an Handler, should be used as embedded struct
// for the most part of the resource
type BasicAPIHandler struct {
	ResourceHandler ResourceHandler
	EtcdKeyAPI      etcd.KeysAPI
}

// Name returns the resource name
func (h *BasicAPIHandler) Name() string {
	return h.ResourceHandler.Name()
}

// New creates a new resource
func (h *BasicAPIHandler) New() Resource {
	return h.ResourceHandler.New()
}

// Unmarshal deserialize a resource
func (h *BasicAPIHandler) Unmarshal(b []byte) (resource Resource, err error) {
	resource = h.ResourceHandler.New()
	err = json.Unmarshal(b, resource)
	return
}

// Decorate the resource
func (h *BasicAPIHandler) Decorate(resource Resource) {
}

func (h *BasicAPIHandler) collectNodes(flatten map[string]Resource, nodes etcd.Nodes) {
	for _, node := range nodes {
		if node.Dir {
			h.collectNodes(flatten, node.Nodes)
		} else {
			resource, err := h.Unmarshal([]byte(node.Value))
			if err != nil {
				logging.GetLogger().Warningf("Failed to unmarshal capture: %s", err.Error())
				continue
			}
			flatten[resource.ID()] = resource
		}
	}
}

// Index returns the list of resource available in Etcd
func (h *BasicAPIHandler) Index() map[string]Resource {
	etcdPath := fmt.Sprintf("/%s/", h.ResourceHandler.Name())

	resp, err := h.EtcdKeyAPI.Get(context.Background(), etcdPath, &etcd.GetOptions{Recursive: true})
	resources := make(map[string]Resource)

	if err == nil {
		h.collectNodes(resources, resp.Node.Nodes)
	}

	return resources
}

// Get a specific resource
func (h *BasicAPIHandler) Get(id string) (Resource, bool) {
	etcdPath := fmt.Sprintf("/%s/%s", h.ResourceHandler.Name(), id)

	resp, err := h.EtcdKeyAPI.Get(context.Background(), etcdPath, nil)
	if err != nil {
		return nil, false
	}

	resource, err := h.Unmarshal([]byte(resp.Node.Value))
	return resource, err == nil
}

// Create a new resource in Etcd
func (h *BasicAPIHandler) Create(resource Resource, createOpts *CreateOptions) error {
	id, _ := uuid.NewV4()
	resource.SetID(id.String())

	data, err := json.Marshal(&resource)
	if err != nil {
		return err
	}

	var setOptions *etcd.SetOptions
	if createOpts != nil {
		setOptions = &etcd.SetOptions{TTL: createOpts.TTL}
	}

	etcdPath := fmt.Sprintf("/%s/%s", h.ResourceHandler.Name(), id)
	_, err = h.EtcdKeyAPI.Set(context.Background(), etcdPath, string(data), setOptions)
	return err
}

// Delete a resource
func (h *BasicAPIHandler) Delete(id string) error {
	etcdPath := fmt.Sprintf("/%s/%s", h.ResourceHandler.Name(), id)

	_, err := h.EtcdKeyAPI.Delete(context.Background(), etcdPath, nil)
	return err
}

// Update a resource
func (h *BasicAPIHandler) Update(id string, resource Resource) error {
	data, err := json.Marshal(&resource)
	if err != nil {
		return err
	}

	etcdPath := fmt.Sprintf("/%s/%s", h.ResourceHandler.Name(), id)
	_, err = h.EtcdKeyAPI.Update(context.Background(), etcdPath, string(data))
	return err
}

// AsyncWatch registers a new resource watcher
func (h *BasicAPIHandler) AsyncWatch(f WatcherCallback) StoppableWatcher {
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
				if err == context.Canceled {
					break
				}

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

// BasicStoppableWatcher basic implementation of a resource watcher
type BasicStoppableWatcher struct {
	StoppableWatcher
	watcher etcd.Watcher
	running atomic.Value
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// Stop the resource watcher
func (s *BasicStoppableWatcher) Stop() {
	s.cancel()
	s.running.Store(false)
	s.wg.Wait()
}
