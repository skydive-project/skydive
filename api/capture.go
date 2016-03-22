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
	"errors"
	"fmt"
	"strings"
	"time"

	etcd "github.com/coreos/etcd/client"
	"golang.org/x/net/context"

	"github.com/redhat-cip/skydive/logging"
)

type Capture struct {
	ProbePath string `json:"ProbePath,omitempty"`
}

type CaptureHandler struct {
	EtcdKeyAPI etcd.KeysAPI
}

func (c *CaptureHandler) Name() string {
	return "capture"
}

func (c *CaptureHandler) New() interface{} {
	return &Capture{}
}

func (c *CaptureHandler) collectNodes(flatten map[string]interface{}, nodes etcd.Nodes) {
	for _, node := range nodes {
		if node.Dir {
			c.collectNodes(flatten, node.Nodes)
		} else {
			capture := &Capture{}
			json.Unmarshal([]byte(node.Value), &capture)
			flatten[capture.ProbePath] = capture
		}
	}
}

func (c *CaptureHandler) Index() map[string]interface{} {
	resp, err := c.EtcdKeyAPI.Get(context.Background(), "/capture/", &etcd.GetOptions{Recursive: true})
	captures := make(map[string]interface{})

	if err == nil {
		c.collectNodes(captures, resp.Node.Nodes)
	}

	return captures
}

func (c *CaptureHandler) Get(id string) (interface{}, bool) {
	resp, err := c.EtcdKeyAPI.Get(context.Background(), "/capture/"+id, nil)
	if err != nil {
		return nil, false
	}

	capture := &Capture{}
	json.Unmarshal([]byte(resp.Node.Value), &capture)
	return capture, true
}

func (c *CaptureHandler) Create(resource interface{}) error {
	capture := resource.(*Capture)
	if capture.ProbePath == "" {
		return errors.New("Invalid probe path")
	}

	data, err := json.Marshal(&resource)
	if err != nil {
		return err
	}

	etcdPath := fmt.Sprintf("/%s/%s", "capture", capture.ProbePath)
	_, err = c.EtcdKeyAPI.Set(context.Background(), etcdPath, string(data), nil)
	return err
}

func (c *CaptureHandler) Delete(id string) error {
	if _, err := c.EtcdKeyAPI.Delete(context.Background(), "/capture/"+id, nil); err != nil {
		return err
	}

	return nil
}

func (c *CaptureHandler) AsyncWatch(f ApiWatcherCallback) StoppableWatcher {
	watcher := c.EtcdKeyAPI.Watcher("/capture/", &etcd.WatcherOptions{Recursive: true})

	sw := StoppableWatcher{
		watcher: watcher,
	}

	sw.running.Store(true)
	go func() {
		for sw.running.Load() == true {
			resp, err := watcher.Next(context.Background())
			if err != nil {
				logging.GetLogger().Errorf("Error while watching etcd: %s", err.Error())

				time.Sleep(1 * time.Second)
				continue
			}

			if resp.Node.Dir {
				continue
			}

			id := strings.TrimPrefix(resp.Node.Key, "/capture/")
			f(resp.Action, id, resp.Node.Value)
		}
	}()

	return sw
}
