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
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/filters"
	"github.com/skydive-project/skydive/logging"
)

// TableQuery contains a type and a query obj as an array of bytes.
// The query can be encoded in different ways according the type.
type TableQuery struct {
	Type string
	Obj  []byte
}

// TableReply is the response to a TableQuery containing a Status and an array
// of replies that can be encoded in many ways, ex: json, protobuf.
type TableReply struct {
	Obj    [][]byte
	status int
}

// ExpireUpdateFunc defines expire and updates callback
type ExpireUpdateFunc func(f []*Flow)

// Handler defines a flow callback called every time
type Handler struct {
	callback ExpireUpdateFunc
	every    time.Duration
}

// NewFlowHandler creates a flow callback handler that will be asynchronously called every time
func NewFlowHandler(callback ExpireUpdateFunc, every time.Duration) *Handler {
	return &Handler{
		callback: callback,
		every:    every,
	}
}

// Table store the flow table and related metrics mechanism
type Table struct {
	PacketsChan   chan *Packets
	table         map[string]*Flow
	stats         map[string]*FlowMetric
	flush         chan bool
	flushDone     chan bool
	query         chan *TableQuery
	reply         chan *TableReply
	state         int64
	lockState     sync.RWMutex
	wg            sync.WaitGroup
	updateHandler *Handler
	lastUpdate    int64
	expireHandler *Handler
	lastExpire    int64
	tableClock    int64
	nodeTID       string
	pipeline      *EnhancerPipeline
}

// NewTable creates a new flow table
func NewTable(updateHandler *Handler, expireHandler *Handler, pipeline *EnhancerPipeline) *Table {
	t := &Table{
		PacketsChan:   make(chan *Packets, 1000),
		table:         make(map[string]*Flow),
		stats:         make(map[string]*FlowMetric),
		flush:         make(chan bool),
		flushDone:     make(chan bool),
		state:         common.StoppedState,
		updateHandler: updateHandler,
		expireHandler: expireHandler,
		pipeline:      pipeline,
	}
	t.tableClock = common.UnixMillis(time.Now())
	t.lastUpdate = t.tableClock
	return t
}

// SetNodeTID set the nodeTID of a flow table
func (ft *Table) SetNodeTID(tid string) {
	ft.nodeTID = tid
}

func (ft *Table) getFlows(query *filters.SearchQuery) *FlowSet {
	flowset := NewFlowSet()
	for _, f := range ft.table {
		if query == nil || query.Filter == nil || query.Filter.Eval(f) {
			if flowset.Start == 0 || flowset.Start > f.Start {
				flowset.Start = f.Start
			}
			if flowset.End == 0 || flowset.Start < f.Last {
				flowset.End = f.Last
			}
			flowset.Flows = append(flowset.Flows, f)
		}
	}

	if query.Sort {
		flowset.Sort(common.SortOrder(query.SortOrder), query.SortBy)
	}

	if query.Dedup {
		flowset.Dedup(query.DedupBy)
	}

	if query.PaginationRange != nil {
		flowset.Slice(int(query.PaginationRange.From), int(query.PaginationRange.To))
	}

	return flowset
}

func (ft *Table) getOrCreateFlow(key string) (*Flow, bool) {
	if flow, found := ft.table[key]; found {
		return flow, false
	}

	new := NewFlow()
	ft.table[key] = new

	return new, true
}

func (ft *Table) expire(expireBefore int64) {
	var expiredFlows []*Flow
	flowTableSzBefore := len(ft.table)
	for k, f := range ft.table {
		if f.Last < expireBefore {
			duration := time.Duration(f.Last - f.Start)
			if f.Last >= ft.lastUpdate {
				ft.updateMetric(f, ft.lastUpdate, f.Last)
			}

			logging.GetLogger().Debugf("Expire flow %s Duration %v", f.UUID, duration)
			expiredFlows = append(expiredFlows, f)

			// need to use the key as the key could be not equal to the UUID
			delete(ft.table, k)

			// stats are always indexed by UUID
			delete(ft.stats, f.UUID)
		}
	}
	/* Advise Clients */
	if ft.expireHandler != nil {
		ft.expireHandler.callback(expiredFlows)
	}

	flowTableSz := len(ft.table)
	logging.GetLogger().Debugf("Expire Flow : removed %v ; new size %v", flowTableSzBefore-flowTableSz, flowTableSz)
}

func (ft *Table) updateAt(now time.Time) {
	updateTime := common.UnixMillis(now)
	ft.update(ft.lastUpdate, updateTime)
	ft.lastUpdate = updateTime
}

func (ft *Table) updateMetric(f *Flow, start, last int64) {
	f.LastUpdateMetric.ABPackets = f.Metric.ABPackets
	f.LastUpdateMetric.ABBytes = f.Metric.ABBytes
	f.LastUpdateMetric.BAPackets = f.Metric.BAPackets
	f.LastUpdateMetric.BABytes = f.Metric.BABytes

	// subtract previous values to get the diff so that we store the
	// amount of data between two updates
	if s, ok := ft.stats[f.UUID]; ok {
		f.LastUpdateMetric.ABPackets -= s.ABPackets
		f.LastUpdateMetric.ABBytes -= s.ABBytes
		f.LastUpdateMetric.BAPackets -= s.BAPackets
		f.LastUpdateMetric.BABytes -= s.BABytes
		f.LastUpdateStart = start
	} else {
		f.LastUpdateStart = f.Start
	}

	f.LastUpdateLast = last
}

func (ft *Table) update(updateFrom, updateTime int64) {
	logging.GetLogger().Debugf("flow table update: %d, %d", updateFrom, updateTime)

	var updatedFlows []*Flow
	for _, f := range ft.table {
		if f.Last >= updateFrom {
			ft.updateMetric(f, updateFrom, updateTime)
			updatedFlows = append(updatedFlows, f)
		} else {
			f.LastUpdateMetric = &FlowMetric{}
			f.LastUpdateStart = updateFrom
			f.LastUpdateLast = updateTime
		}

		ft.stats[f.UUID] = f.Metric.Copy()
	}

	/* Advise Clients */
	if ft.updateHandler != nil && len(updatedFlows) != 0 {
		ft.updateHandler.callback(updatedFlows)

		logging.GetLogger().Debugf("Send updated Flows: %d", len(updatedFlows))
	}
}

func (ft *Table) expireNow() {
	const Now = int64(^uint64(0) >> 1)
	ft.expire(Now)
}

func (ft *Table) expireAt(now time.Time) {
	ft.expire(ft.lastExpire)
	ft.lastExpire = common.UnixMillis(now)
}

func (ft *Table) onSearchQueryMessage(fsq *filters.SearchQuery) (*FlowSearchReply, int) {
	flowset := ft.getFlows(fsq)
	if len(flowset.Flows) == 0 {
		return &FlowSearchReply{
			FlowSet: flowset,
		}, http.StatusNoContent
	}

	return &FlowSearchReply{
		FlowSet: flowset,
	}, http.StatusOK
}

func (ft *Table) onQuery(query *TableQuery) *TableReply {
	reply := &TableReply{
		status: http.StatusBadRequest,
		Obj:    make([][]byte, 0),
	}

	switch query.Type {
	case "SearchQuery":
		var fsq filters.SearchQuery
		if err := proto.Unmarshal(query.Obj, &fsq); err != nil {
			logging.GetLogger().Errorf("Unable to decode the flow search query: %s", err.Error())
			break
		}

		fsr, status := ft.onSearchQueryMessage(&fsq)
		if status != http.StatusOK {
			reply.status = status
			break
		}
		pb, _ := proto.Marshal(fsr)

		// TableReply returns an array of replies so in that case an array of
		// protobuf replies
		reply.Obj = append(reply.Obj, pb)

		reply.status = http.StatusOK
	}

	return reply
}

// Query a flow table
func (ft *Table) Query(query *TableQuery) *TableReply {
	ft.lockState.Lock()
	defer ft.lockState.Unlock()

	if atomic.LoadInt64(&ft.state) == common.RunningState {
		ft.query <- query

		timer := time.NewTicker(1 * time.Second)
		defer timer.Stop()

		for {
			select {
			case r := <-ft.reply:
				return r
			case <-timer.C:
				if atomic.LoadInt64(&ft.state) != common.RunningState {
					return nil
				}
			}
		}
	}
	return nil
}

func (ft *Table) flowPacketToFlow(packet *Packet, parentUUID string, t int64, L2ID int64, L3ID int64) *Flow {
	key := KeyFromGoPacket(packet.gopacket, parentUUID).String()
	flow, new := ft.getOrCreateFlow(key)
	if new {
		flow.Init(key, t, packet.gopacket, packet.length, ft.nodeTID, parentUUID, L2ID, L3ID)
		ft.pipeline.EnhanceFlow(flow)
	} else {
		flow.Update(t, packet.gopacket, packet.length)
	}
	return flow
}

func (ft *Table) flowPacketsToFlow(flowPackets *Packets) {
	t := flowPackets.Timestamp
	if t == -1 {
		t = ft.tableClock
	}

	var parentUUID string
	var L2ID int64
	var L3ID int64
	logging.GetLogger().Debugf("%d Packets received for capture node %s", len(flowPackets.Packets), ft.nodeTID)
	for _, packet := range flowPackets.Packets {
		f := ft.flowPacketToFlow(&packet, parentUUID, t, L2ID, L3ID)
		parentUUID = f.UUID
		if f.Link != nil {
			L2ID = f.Link.ID
		}
		if f.Network != nil {
			L3ID = f.Network.ID
		}
	}
}

// Run background jobs, like update/expire entries event
func (ft *Table) Run() {
	ft.wg.Add(1)
	defer ft.wg.Done()

	updateTicker := time.NewTicker(ft.updateHandler.every)
	defer updateTicker.Stop()

	expireTicker := time.NewTicker(ft.expireHandler.every)
	defer expireTicker.Stop()

	nowTicker := time.NewTicker(time.Second * 1)
	defer nowTicker.Stop()

	ft.query = make(chan *TableQuery, 100)
	ft.reply = make(chan *TableReply, 100)

	atomic.StoreInt64(&ft.state, common.RunningState)
	for atomic.LoadInt64(&ft.state) == common.RunningState {
		select {
		case now := <-expireTicker.C:
			ft.expireAt(now)
		case now := <-updateTicker.C:
			ft.updateAt(now)
		case <-ft.flush:
			ft.expireNow()
			ft.flushDone <- true
		case query, ok := <-ft.query:
			if ok {
				ft.reply <- ft.onQuery(query)
			}
		case now := <-nowTicker.C:
			ft.tableClock = common.UnixMillis(now)
		case packets := <-ft.PacketsChan:
			ft.flowPacketsToFlow(packets)
		}
	}
}

// Start the flow table
func (ft *Table) Start() chan *Packets {
	go ft.Run()
	return ft.PacketsChan
}

// Stop the flow table
func (ft *Table) Stop() {
	ft.lockState.Lock()
	defer ft.lockState.Unlock()

	if atomic.CompareAndSwapInt64(&ft.state, common.RunningState, common.StoppingState) {
		ft.wg.Wait()

		close(ft.query)
		close(ft.reply)

		for len(ft.PacketsChan) != 0 {
			packets := <-ft.PacketsChan
			ft.flowPacketsToFlow(packets)
		}

		close(ft.PacketsChan)
	}

	ft.expireNow()
}
