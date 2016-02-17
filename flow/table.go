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

package flow

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/redhat-cip/skydive/logging"
)

type FlowTable struct {
	lock  sync.RWMutex
	table map[string]*Flow
}

type FlowTableAsyncNotificationUpdate interface {
	AsyncNotificationUpdate(every time.Duration)
}

func NewFlowTable() *FlowTable {
	return &FlowTable{table: make(map[string]*Flow)}
}

func (ft *FlowTable) String() string {
	ft.lock.RLock()
	defer ft.lock.RUnlock()
	return fmt.Sprintf("%d flows", len(ft.table))
}

func (ft *FlowTable) Update(flows []*Flow) {
	ft.lock.Lock()
	for _, f := range flows {
		_, found := ft.table[f.UUID]
		if !found {
			ft.table[f.UUID] = f
		} else if f.UUID != ft.table[f.UUID].UUID {
			logging.GetLogger().Errorf("FlowTable Collision %s %s", f.UUID, ft.table[f.UUID].UUID)
		}
	}
	ft.lock.Unlock()
}

type ExpireFunc func(f *Flow)

func (ft *FlowTable) expire(fn ExpireFunc, expire int64) {
	ft.lock.Lock()
	flowTableSzBefore := len(ft.table)
	for key, f := range ft.table {
		fs := f.GetStatistics()
		if fs.Last < expire {
			duration := time.Duration(fs.Last - fs.Start)
			logging.GetLogger().Debugf("Expire flow %s Duration %v", f.UUID, duration)
			/* Advise Clients */
			fn(f)
			delete(ft.table, key)
		}
	}
	flowTableSz := len(ft.table)
	ft.lock.Unlock()
	logging.GetLogger().Debugf("Expire Flow : removed %v ; new size %v", flowTableSzBefore-flowTableSz, flowTableSz)
}

func (ft *FlowTable) AsyncExpire(fn ExpireFunc, every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		now := <-ticker.C
		expire := now.Unix() - int64((every).Seconds())

		ft.expire(fn, expire)
	}
}

func (ft *FlowTable) IsExist(f *Flow) bool {
	ft.lock.RLock()
	_, found := ft.table[f.UUID]
	ft.lock.RUnlock()
	return found
}

func (ft *FlowTable) GetFlow(key string) (flow *Flow, new bool) {
	ft.lock.Lock()
	flow, found := ft.table[key]
	if found == false {
		flow = &Flow{}
		ft.table[key] = flow
	}
	ft.lock.Unlock()
	return flow, !found
}

func (ft *FlowTable) JSONFlowConversationEthernetPath(EndpointType FlowEndpointType) string {
	str := ""
	str += "{"
	//	{"nodes":[{"name":"Myriel","group":1}, ... ],"links":[{"source":1,"target":0,"value":1},...]}

	var strNodes, strLinks string
	strNodes += "\"nodes\":["
	strLinks += "\"links\":["
	pathMap := make(map[string]int)
	layerMap := make(map[string]int)
	ft.lock.RLock()
	for _, f := range ft.table {
		_, found := pathMap[f.LayersPath]
		if !found {
			pathMap[f.LayersPath] = len(pathMap)
		}

		layerFlow := f.GetStatistics().Endpoints[EndpointType.Value()]
		if _, found := layerMap[layerFlow.AB.Value]; !found {
			layerMap[layerFlow.AB.Value] = len(layerMap)
			strNodes += fmt.Sprintf("{\"name\":\"%s\",\"group\":%d},", layerFlow.AB.Value, pathMap[f.LayersPath])
		}
		if _, found := layerMap[layerFlow.BA.Value]; !found {
			layerMap[layerFlow.BA.Value] = len(layerMap)
			strNodes += fmt.Sprintf("{\"name\":\"%s\",\"group\":%d},", layerFlow.BA.Value, pathMap[f.LayersPath])
		}
		strLinks += fmt.Sprintf("{\"source\":%d,\"target\":%d,\"value\":%d},", layerMap[layerFlow.AB.Value], layerMap[layerFlow.BA.Value], layerFlow.AB.Bytes+layerFlow.BA.Bytes)
	}
	ft.lock.RUnlock()
	strNodes = strings.TrimRight(strNodes, ",")
	strNodes += "]"
	strLinks = strings.TrimRight(strLinks, ",")
	strLinks += "]"
	str += strNodes + "," + strLinks
	str += "}"
	return str
}

func (ft *FlowTable) NewFlowTableFromFlows(flows []*Flow) *FlowTable {
	nft := NewFlowTable()
	nft.Update(flows)
	return nft
}

/* Return a new FlowTable that contain <last> active flows */
func (ft *FlowTable) FilterLast(last time.Duration) []*Flow {
	var flows []*Flow
	selected := time.Now().Unix() - int64((last).Seconds())
	ft.lock.RLock()
	for _, f := range ft.table {
		fs := f.GetStatistics()
		if fs.Last >= selected {
			flows = append(flows, f)
		}
	}
	ft.lock.RUnlock()
	return flows
}

func (ft *FlowTable) SelectLayer(endpointType FlowEndpointType, list []string) []*Flow {
	meth := make(map[string][]*Flow)
	ft.lock.RLock()
	for _, f := range ft.table {
		layerFlow := f.GetStatistics().Endpoints[endpointType.Value()]
		if layerFlow.AB.Value == "ff:ff:ff:ff:ff:ff" || layerFlow.BA.Value == "ff:ff:ff:ff:ff:ff" {
			continue
		}
		meth[layerFlow.AB.Value] = append(meth[layerFlow.AB.Value], f)
		meth[layerFlow.BA.Value] = append(meth[layerFlow.BA.Value], f)
	}
	ft.lock.RUnlock()

	mflows := make(map[*Flow]struct{})
	var flows []*Flow
	for _, eth := range list {
		if flist, ok := meth[eth]; ok {
			for _, f := range flist {
				if _, found := mflows[f]; !found {
					mflows[f] = struct{}{}
					flows = append(flows, f)
				}
			}
		}
	}
	return flows
}
