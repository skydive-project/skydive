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

	"encoding/json"

	"github.com/redhat-cip/skydive/logging"
)

type FlowTable struct {
	lock    sync.RWMutex
	table   map[string]*Flow
	manager FlowTableManager
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
		if layerFlow == nil {
			continue
		}

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

type DiscoType int

const (
	BYTES DiscoType = 1 + iota
	PACKETS
)

type DiscoNode struct {
	name     string
	size     uint64
	children map[string]*DiscoNode
}

func (d *DiscoNode) MarshalJSON() ([]byte, error) {
	str := "{"
	str += fmt.Sprintf(`"name":"%s",`, d.name)
	if d.size > 0 {
		str += fmt.Sprintf(`"size": %d,`, d.size)
	}
	str += fmt.Sprintf(`"children": [`)
	idx := 0
	for _, child := range d.children {
		bytes, err := child.MarshalJSON()
		if err != nil {
			return []byte(str), err
		}
		str += string(bytes)
		if idx != len(d.children)-1 {
			str += ","
		}
		idx++
	}
	str += "]"
	str += "}"
	return []byte(str), nil
}

func NewDiscoNode() *DiscoNode {
	return &DiscoNode{
		children: make(map[string]*DiscoNode),
	}
}

func (ft *FlowTable) JSONFlowDiscovery(DiscoType DiscoType) string {
	// {"name":"root","children":[{"name":"Ethernet","children":[{"name":"IPv4","children":[{"name":"UDP","children":[{"name":"Payload","size":360,"children":[]}]},{"name":"TCP","children":[{"name":"Payload","size":240,"children":[]}]}]}]}]}

	pathMap := make(map[string]FlowEndpointStatistics)

	ft.lock.RLock()
	for _, f := range ft.table {
		p, _ := pathMap[f.LayersPath]
		p.Bytes += f.GetStatistics().Endpoints[FlowEndpointType_ETHERNET.Value()].AB.Bytes
		p.Bytes += f.GetStatistics().Endpoints[FlowEndpointType_ETHERNET.Value()].BA.Bytes
		p.Packets += f.GetStatistics().Endpoints[FlowEndpointType_ETHERNET.Value()].AB.Packets
		p.Packets += f.GetStatistics().Endpoints[FlowEndpointType_ETHERNET.Value()].BA.Packets
		pathMap[f.LayersPath] = p
	}
	ft.lock.RUnlock()

	root := NewDiscoNode()
	root.name = "root"
	for path, stat := range pathMap {
		node := root
		layers := strings.Split(path, "/")
		for i, layer := range layers {
			l, found := node.children[layer]
			if !found {
				node.children[layer] = NewDiscoNode()
				l = node.children[layer]
				l.name = layer
			}
			if len(layers)-1 == i {
				switch DiscoType {
				case BYTES:
					l.size = stat.Bytes
				case PACKETS:
					l.size = stat.Packets
				}
			}
			node = l
		}
	}

	bytes, err := json.Marshal(root)
	if err != nil {
		logging.GetLogger().Fatal(err)
	}
	return string(bytes)
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

/*
 * Following function are FlowTable manager helpers
 */

func (ft *FlowTable) Expire(now time.Time) {
	timepoint := now.Unix() - int64((ft.manager.expire.duration).Seconds())
	ft.lock.Lock()
	ft.expire(ft.manager.expire.callback, timepoint)
	ft.lock.Unlock()
}

/* Internal call only, Must be called under ft.lock.Lock() */
func (ft *FlowTable) expire(fn ExpireUpdateFunc, expireBefore int64) {
	var expiredFlows []*Flow
	flowTableSzBefore := len(ft.table)
	for _, f := range ft.table {
		fs := f.GetStatistics()
		if fs.Last < expireBefore {
			duration := time.Duration(fs.Last - fs.Start)
			logging.GetLogger().Debugf("Expire flow %s Duration %v", f.UUID, duration)
			expiredFlows = append(expiredFlows, f)
		}
	}
	/* Advise Clients */
	fn(expiredFlows)
	for _, f := range expiredFlows {
		delete(ft.table, f.UUID)
	}
	flowTableSz := len(ft.table)
	logging.GetLogger().Debugf("Expire Flow : removed %v ; new size %v", flowTableSzBefore-flowTableSz, flowTableSz)
}

func (ft *FlowTable) Updated(now time.Time) {
	timepoint := now.Unix() - int64((ft.manager.updated.duration).Seconds())
	ft.lock.RLock()
	ft.updated(ft.manager.updated.callback, timepoint)
	ft.lock.RUnlock()
}

/* Internal call only, Must be called under ft.lock.RLock() */
func (ft *FlowTable) updated(fn ExpireUpdateFunc, updateFrom int64) {
	var updatedFlows []*Flow
	for _, f := range ft.table {
		fs := f.GetStatistics()
		if fs.Last > updateFrom {
			updatedFlows = append(updatedFlows, f)
		}
	}
	/* Advise Clients */
	fn(updatedFlows)
	logging.GetLogger().Debugf("Send updated Flow %d", len(updatedFlows))
}

func (ft *FlowTable) ExpireNow() {
	const Now = int64(^uint64(0) >> 1)
	ft.lock.Lock()
	ft.expire(ft.manager.expire.callback, Now)
	ft.lock.Unlock()
}

/* Asynchrnously Register an expire callback fn with last updated flow 'since', each 'since' tick  */
func (ft *FlowTable) RegisterExpire(fn ExpireUpdateFunc, every time.Duration) {
	ft.lock.Lock()
	ft.manager.expire.Register(&FlowTableManagerAsyncParam{ft.expire, fn, every, every})
	ft.lock.Unlock()
}

/* Asynchrnously call the callback fn with last updated flow 'since', each 'since' tick  */
func (ft *FlowTable) RegisterUpdated(fn ExpireUpdateFunc, since time.Duration) {
	ft.lock.Lock()
	ft.manager.updated.Register(&FlowTableManagerAsyncParam{ft.updated, fn, since, since + 2})
	ft.lock.Unlock()
}

func (ft *FlowTable) UnregisterAll() {
	ft.lock.Lock()
	if ft.manager.updated.running {
		ft.manager.updated.Unregister()
	}
	if ft.manager.expire.running {
		ft.manager.expire.Unregister()
	}
	ft.lock.Unlock()

	ft.ExpireNow()
}

func (ft *FlowTable) GetExpireTicker() <-chan time.Time {
	return ft.manager.expire.ticker.C
}

func (ft *FlowTable) GetUpdatedTicker() <-chan time.Time {
	return ft.manager.updated.ticker.C
}
