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

type GraphFlowEnhancer struct {
	Graph            *graph.Graph
	cache            *cache.Cache
	cacheUpdaterChan chan string
}

func (gfe *GraphFlowEnhancer) cacheUpdater() {
	logging.GetLogger().Debug("Start GraphFlowEnhancer cache updater")

	var mac string
	for {
		mac = <-gfe.cacheUpdaterChan

		logging.GetLogger().Debug("GraphFlowEnhancer request received: %s", mac)

		gfe.Graph.Lock()
		intf := gfe.Graph.LookupNode(graph.Metadatas{"MAC": mac})

		if intf != nil {
			ancestors, ok := intf.GetAncestorsTo(graph.Metadatas{"Type": "host"})
			if ok {
				var path string
				for i := len(ancestors) - 1; i >= 0; i-- {
					if len(path) > 0 {
						path += "/"
					}
					path += ancestors[i].Metadatas["Name"].(string)
				}

				gfe.cache.Set(mac, path, cache.DefaultExpiration)
			}
		}
		gfe.Graph.Unlock()
	}
}

func (gfe *GraphFlowEnhancer) getPath(mac string) *string {
	if mac == "ff:ff:ff:ff:ff:ff" {
		return new(string)
	}

	p, f := gfe.cache.Get(mac)
	if f {
		path := p.(string)
		return &path
	}

	gfe.cacheUpdaterChan <- mac

	return nil
}

func (gfe *GraphFlowEnhancer) Enhance(flow *flow.Flow) {
	if flow.ProbeGraphPath == nil {
		flow.ProbeGraphPath = gfe.getPath(flow.GetProbeMAC())
	}

	if flow.IfSrcGraphPath == nil {
		flow.IfSrcGraphPath = gfe.getPath(flow.GetEtherSrc())
	}
	if flow.IfDstGraphPath == nil {
		flow.IfDstGraphPath = gfe.getPath(flow.GetEtherDst())
	}
}

func NewGraphFlowEnhancer(g *graph.Graph) (*GraphFlowEnhancer, error) {
	mapper := &GraphFlowEnhancer{
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
