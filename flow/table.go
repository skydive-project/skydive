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
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/redhat-cip/skydive/logging"
)

type TableQuery struct {
	Obj interface{}
}

type TableReply struct {
	Status int
	Obj    interface{}
}

type FlowSearchQuery struct {
	ProbeNodeUUID string
}

type FlowSearchReply struct {
	Flows []*Flow
}

type sortByLast []*Flow

type FlowQueryFilter struct {
	// TODO add more filter elements
	ProbeNodeUUID string
}

type Table struct {
	lock        sync.RWMutex
	table       map[string]*Flow
	manager     tableManager
	defaultFunc func()
	flush       chan bool
	flushDone   chan bool
	query       chan *TableQuery
	reply       chan *TableReply
	running     atomic.Value
	wg          sync.WaitGroup
}

func NewTable() *Table {
	return &Table{
		table:     make(map[string]*Flow),
		flush:     make(chan bool),
		flushDone: make(chan bool),
		query:     make(chan *TableQuery),
		reply:     make(chan *TableReply),
	}
}

func NewTableFromFlows(flows []*Flow) *Table {
	nft := NewTable()
	nft.Update(flows)
	return nft
}

func (ft *Table) String() string {
	ft.lock.RLock()
	defer ft.lock.RUnlock()
	return fmt.Sprintf("%d flows", len(ft.table))
}

func (ft *Table) Update(flows []*Flow) {
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

func matchQueryFilter(f *Flow, filter *FlowQueryFilter) bool {
	if filter.ProbeNodeUUID != "" && f.ProbeNodeUUID != filter.ProbeNodeUUID {
		return false
	}

	return true
}

func (ft *Table) GetFlows(filters ...FlowQueryFilter) []*Flow {
	ft.lock.RLock()
	defer ft.lock.RUnlock()

	flows := make([]*Flow, 0, len(ft.table))
	for _, f := range ft.table {
		if len(filters) == 0 || matchQueryFilter(f, &filters[0]) {
			flows = append(flows, &*f)
		}
	}
	return flows
}

func (ft *Table) GetFlow(key string) *Flow {
	ft.lock.RLock()
	defer ft.lock.RUnlock()
	if flow, found := ft.table[key]; found {
		return flow
	}

	return nil
}

func (ft *Table) GetOrCreateFlow(key string) (*Flow, bool) {
	ft.lock.Lock()
	defer ft.lock.Unlock()
	if flow, found := ft.table[key]; found {
		return flow, false
	}

	new := &Flow{}
	ft.table[key] = new

	return new, true
}

/* Return a new flow.Table that contain <last> active flows */
func (ft *Table) FilterLast(last time.Duration) []*Flow {
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

func (ft *Table) SelectLayer(endpointType FlowEndpointType, list []string) []*Flow {
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
 * Following function are Table manager helpers
 */
func (ft *Table) Expire(now time.Time) {
	timepoint := now.Unix() - int64((ft.manager.expire.duration).Seconds())
	ft.lock.Lock()
	ft.expire(ft.manager.expire.callback, timepoint)
	ft.lock.Unlock()
}

/* Internal call only, Must be called under ft.lock.Lock() */
func (ft *Table) expire(fn ExpireUpdateFunc, expireBefore int64) {
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

func (ft *Table) Updated(now time.Time) {
	timepoint := now.Unix() - int64((ft.manager.updated.duration).Seconds())
	ft.lock.RLock()
	ft.updated(ft.manager.updated.callback, timepoint)
	ft.lock.RUnlock()
}

/* Internal call only, Must be called under ft.lock.RLock() */
func (ft *Table) updated(fn ExpireUpdateFunc, updateFrom int64) {
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

func (ft *Table) expireNow() {
	const Now = int64(^uint64(0) >> 1)
	ft.lock.Lock()
	ft.expire(ft.manager.expire.callback, Now)
	ft.lock.Unlock()
}

/* Asynchrnously Register an expire callback fn with last updated flow 'since', each 'since' tick  */
func (ft *Table) RegisterExpire(fn ExpireUpdateFunc, every time.Duration, windowSize time.Duration) {
	ft.lock.Lock()
	ft.manager.expire.Register(&tableManagerAsyncParam{ft.expire, fn, every, windowSize})
	ft.lock.Unlock()
}

/* Asynchrnously call the callback fn with last updated flow 'since', each 'since' tick  */
func (ft *Table) RegisterUpdated(fn ExpireUpdateFunc, since time.Duration, windowSize time.Duration) {
	ft.lock.Lock()
	ft.manager.updated.Register(&tableManagerAsyncParam{ft.updated, fn, since, windowSize + 2})
	ft.lock.Unlock()
}

func (ft *Table) RegisterDefault(fn func()) {
	ft.lock.Lock()
	ft.defaultFunc = fn
	ft.lock.Unlock()
}

func (ft *Table) UnregisterAll() {
	ft.lock.Lock()
	if ft.manager.updated.running {
		ft.manager.updated.Unregister()
	}
	if ft.manager.expire.running {
		ft.manager.expire.Unregister()
	}
	ft.lock.Unlock()

	ft.expireNow()
}

func (ft *Table) Flush() {
	ft.flush <- true
	<-ft.flushDone
}

func (s sortByLast) Len() int {
	return len(s)
}

func (s sortByLast) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s sortByLast) Less(i, j int) bool {
	return s[i].GetStatistics().Last > s[j].GetStatistics().Last
}

func (ft *Table) onFlowSearchQueryMessage(o interface{}) (*FlowSearchReply, int) {
	var fq FlowSearchQuery
	err := mapstructure.Decode(o, &fq)
	if err != nil {
		logging.GetLogger().Errorf("Unable to decode flow search message %v", o)
		return nil, 500
	}

	flows := ft.GetFlows(FlowQueryFilter{
		ProbeNodeUUID: fq.ProbeNodeUUID,
	})

	if len(flows) == 0 {
		return &FlowSearchReply{
			Flows: flows,
		}, 404
	}

	sort.Sort(sortByLast(flows))

	return &FlowSearchReply{
		Flows: flows,
	}, 200
}

func (ft *Table) onQuery(q *TableQuery) *TableReply {

	var obj interface{}
	status := 500

	switch q.Obj.(type) {
	case *FlowSearchQuery:
		obj, status = ft.onFlowSearchQueryMessage(q.Obj)
	}

	return &TableReply{
		Status: status,
		Obj:    obj,
	}
}

func (ft *Table) Query(query *TableQuery) *TableReply {
	ft.query <- query
	return <-ft.reply
}

func (ft *Table) Start() {
	ft.wg.Add(1)
	defer ft.wg.Done()

	ft.running.Store(true)
	for ft.running.Load() == true {
		select {
		case now := <-ft.manager.expire.ticker.C:
			ft.Expire(now)
		case now := <-ft.manager.updated.ticker.C:
			ft.Updated(now)
		case <-ft.flush:
			ft.expireNow()
			ft.flushDone <- true
		case query := <-ft.query:
			ft.reply <- ft.onQuery(query)
		default:
			if ft.defaultFunc != nil {
				ft.defaultFunc()
			} else {
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
	ft.expireNow()
}

func (ft *Table) Stop() {
	if ft.running.Load() == true {
		ft.running.Store(false)
		ft.wg.Wait()
	}
	ft.expireNow()
}
