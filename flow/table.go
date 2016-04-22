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
	"sync"
	"time"

	"github.com/redhat-cip/skydive/logging"
)

type FlowTable struct {
	lock    sync.RWMutex
	table   map[string]*Flow
	manager FlowTableManager
}

func NewFlowTable() *FlowTable {
	return &FlowTable{table: make(map[string]*Flow)}
}

func NewFlowTableFromFlows(flows []*Flow) *FlowTable {
	nft := NewFlowTable()
	nft.Update(flows)
	return nft
}

func (ft *FlowTable) String() string {
	ft.lock.RLock()
	defer ft.lock.RUnlock()
	return fmt.Sprintf("%d flows", len(ft.table))
}

func (ft *FlowTable) Update(flows []*Flow) {
	ft.lock.Lock()
	for _, f := range flows {
		if _, ok := ft.table[f.UUID]; !ok {
			ft.table[f.UUID] = f
		} else {
			ft.table[f.UUID].Statistics = f.Statistics
		}
	}
	ft.lock.Unlock()
}

func (ft *FlowTable) LookupFlowsByProbePath(p string) []*Flow {
	ft.lock.RLock()
	defer ft.lock.RUnlock()

	flows := []*Flow{}
	for _, f := range ft.table {
		if f.ProbeGraphPath == p {
			flows = append(flows, &*f)
		}
	}
	return flows
}

func (ft *FlowTable) GetFlows() []*Flow {
	ft.lock.RLock()
	defer ft.lock.RUnlock()

	flows := []*Flow{}
	for _, f := range ft.table {
		flows = append(flows, &*f)
	}
	return flows
}

func (ft *FlowTable) GetFlow(key string) *Flow {
	ft.lock.RLock()
	defer ft.lock.RUnlock()
	if flow, found := ft.table[key]; found {
		return flow
	}

	return nil
}

func (ft *FlowTable) GetOrCreateFlow(key string) (*Flow, bool) {
	ft.lock.Lock()
	defer ft.lock.Unlock()
	if flow, found := ft.table[key]; found {
		return flow, false
	}

	new := &Flow{}
	ft.table[key] = new

	return new, true
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
		layerFlow := f.GetStatistics().GetEndpointsType(endpointType)
		if layerFlow == nil || layerFlow.AB.Value == "ff:ff:ff:ff:ff:ff" || layerFlow.BA.Value == "ff:ff:ff:ff:ff:ff" {
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
