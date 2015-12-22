/*
 * Copyright (C) 2015 Red Hat, Inc.
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

package mappings

import (
	"time"

	"github.com/pmylund/go-cache"

	"github.com/redhat-cip/skydive/config"
	"github.com/redhat-cip/skydive/flow"
	"github.com/redhat-cip/skydive/logging"
	"github.com/redhat-cip/skydive/topology/graph"
)

type GraphMappingDriver struct {
	Graph            *graph.Graph
	cache            *cache.Cache
	cacheUpdaterChan chan string
}

func (m *GraphMappingDriver) cacheUpdater() {
	logging.GetLogger().Debug("Start GraphMappingDriver cache updater")

	var mac string
	for {
		mac = <-m.cacheUpdaterChan

		logging.GetLogger().Debug("GraphMappingDriver request received: %s", mac)

		m.Graph.Lock()
		intf := m.Graph.LookupNode(graph.Metadatas{"MAC": mac})
		m.Graph.Unlock()

		if intf != nil {
			m.cache.Set(mac, intf.Host, cache.DefaultExpiration)
		}
	}
}

func (m *GraphMappingDriver) Enhance(mac string, attrs *flow.Flow_InterfaceAttributes) {
	h, f := m.cache.Get(mac)
	if f {
		host := h.(string)
		attrs.Host = &host
		return
	}

	m.cacheUpdaterChan <- mac
}

func NewGraphMappingDriver(g *graph.Graph) (*GraphMappingDriver, error) {
	mapper := &GraphMappingDriver{
		Graph: g,
	}

	expire, err := config.GetConfig().Section("cache").Key("expire").Int()
	if err != nil {
		return nil, err
	}
	cleanup, err := config.GetConfig().Section("cache").Key("cleanup").Int()
	if err != nil {
		return nil, err
	}
	mapper.cache = cache.New(time.Duration(expire)*time.Second, time.Duration(cleanup)*time.Second)
	mapper.cacheUpdaterChan = make(chan string, 200)
	go mapper.cacheUpdater()

	return mapper, nil
}
