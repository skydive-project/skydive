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
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

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

// TableOpts defines flow table options
type TableOpts struct {
	RawPacketLimit int64
	ExtraTCPMetric bool
	IPDefrag       bool
	ReassembleTCP  bool
	LayerKeyMode   LayerKeyMode
	ExtraLayers    ExtraLayers
}

// Table store the flow table and related metrics mechanism
type Table struct {
	Opts          TableOpts
	packetSeqChan chan *PacketSequence
	flowChan      chan *Flow
	table         map[string]*Flow
	flush         chan bool
	flushDone     chan bool
	query         chan *TableQuery
	reply         chan *TableReply
	state         int64
	lockState     common.RWMutex
	wg            sync.WaitGroup
	quit          chan bool
	updateHandler *Handler
	lastUpdate    int64
	updateVersion int64
	expireHandler *Handler
	lastExpire    int64
	nodeTID       string
	ipDefragger   *IPDefragger
	tcpAssembler  *TCPAssembler
	flowOpts      Opts
	appPortMap    *ApplicationPortMap
}

// NewTable creates a new flow table
func NewTable(updateHandler *Handler, expireHandler *Handler, nodeTID string, opts ...TableOpts) *Table {
	t := &Table{
		packetSeqChan: make(chan *PacketSequence, 1000),
		flowChan:      make(chan *Flow, 1000),
		table:         make(map[string]*Flow),
		flush:         make(chan bool),
		flushDone:     make(chan bool),
		state:         common.StoppedState,
		quit:          make(chan bool),
		updateHandler: updateHandler,
		expireHandler: expireHandler,
		nodeTID:       nodeTID,
		ipDefragger:   NewIPDefragger(),
		tcpAssembler:  NewTCPAssembler(),
		appPortMap:    NewApplicationPortMapFromConfig(),
	}
	if len(opts) > 0 {
		t.Opts = opts[0]
	}

	t.flowOpts = Opts{
		TCPMetric:    t.Opts.ExtraTCPMetric,
		IPDefrag:     t.Opts.IPDefrag,
		LayerKeyMode: t.Opts.LayerKeyMode,
		AppPortMap:   t.appPortMap,
		ExtraLayers:  t.Opts.ExtraLayers,
	}

	t.updateVersion = 0
	return t
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

	if query == nil {
		return flowset
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

func (ft *Table) replaceFlow(key string, f *Flow) *Flow {
	prev, _ := ft.table[key]
	ft.table[key] = f

	return prev
}

func (ft *Table) expire(expireBefore int64) {
	var expiredFlows []*Flow
	flowTableSzBefore := len(ft.table)
	for k, f := range ft.table {
		if f.Last < expireBefore {
			duration := time.Duration(f.Last - f.Start)
			if f.XXX_state.updateVersion > ft.updateVersion {
				ft.updateMetric(f, ft.lastUpdate, f.Last)
			}

			logging.GetLogger().Debugf("Expire flow %s Duration %v", f.UUID, duration)
			expiredFlows = append(expiredFlows, f)

			// need to use the key as the key could be not equal to the UUID
			delete(ft.table, k)
		}
	}

	/* Advise Clients */
	ft.expireHandler.callback(expiredFlows)

	flowTableSz := len(ft.table)
	logging.GetLogger().Debugf("Expire Flow : removed %v ; new size %v", flowTableSzBefore-flowTableSz, flowTableSz)
}

func (ft *Table) updateAt(now time.Time) {
	updateTime := common.UnixMillis(now)
	ft.update(ft.lastUpdate, updateTime)
	ft.lastUpdate = updateTime
	ft.updateVersion++
}

func (ft *Table) updateMetric(f *Flow, start, last int64) {
	if f.LastUpdateMetric == nil {
		f.LastUpdateMetric = &FlowMetric{}
	}
	f.LastUpdateMetric.ABPackets = f.Metric.ABPackets
	f.LastUpdateMetric.ABBytes = f.Metric.ABBytes
	f.LastUpdateMetric.BAPackets = f.Metric.BAPackets
	f.LastUpdateMetric.BABytes = f.Metric.BABytes

	// subtract previous values to get the diff so that we store the
	// amount of data between two updates
	if lm := f.XXX_state.lastMetric; lm != nil {
		f.LastUpdateMetric.ABPackets -= lm.ABPackets
		f.LastUpdateMetric.ABBytes -= lm.ABBytes
		f.LastUpdateMetric.BAPackets -= lm.BAPackets
		f.LastUpdateMetric.BABytes -= lm.BABytes
		f.LastUpdateMetric.Start = start
	} else {
		f.LastUpdateMetric.Start = f.Start
	}

	f.LastUpdateMetric.Last = last
}

func (ft *Table) update(updateFrom, updateTime int64) {
	logging.GetLogger().Debugf("flow table update: %d, %d", updateFrom, updateTime)

	var updatedFlows []*Flow
	for _, f := range ft.table {
		if f.XXX_state.updateVersion > ft.updateVersion {
			ft.updateMetric(f, updateFrom, updateTime)
			updatedFlows = append(updatedFlows, f)
		} else {
			f.LastUpdateMetric = &FlowMetric{Start: updateFrom, Last: updateTime}
		}

		f.XXX_state.lastMetric = f.Metric.Copy()
	}

	if len(updatedFlows) != 0 {
		/* Advise Clients */
		ft.updateHandler.callback(updatedFlows)
		logging.GetLogger().Debugf("Send updated Flows: %d", len(updatedFlows))

		// cleanup raw packets
		if ft.Opts.RawPacketLimit > 0 {
			for _, f := range updatedFlows {
				f.LastRawPackets = f.LastRawPackets[:0]
			}
		}
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
			logging.GetLogger().Errorf("Unable to decode the flow search query: %s", err)
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

func (ft *Table) packetToFlow(packet *Packet, parentUUID string) *Flow {
	key := packet.Key(parentUUID, ft.flowOpts)
	flow, new := ft.getOrCreateFlow(key)
	if new {
		uuids := UUIDs{
			ParentUUID: parentUUID,
		}

		if ft.Opts.ReassembleTCP {
			if layer := packet.GoPacket.TransportLayer(); layer != nil && layer.LayerType() == layers.LayerTypeTCP {
				ft.tcpAssembler.RegisterFlow(flow, packet.GoPacket)
			}
		}

		flow.initFromPacket(key, packet, ft.nodeTID, uuids, ft.flowOpts)
	} else {
		if ft.Opts.ReassembleTCP {
			if layer := packet.GoPacket.TransportLayer(); layer != nil && layer.LayerType() == layers.LayerTypeTCP {
				ft.tcpAssembler.Assemble(packet.GoPacket)
			}
		}

		flow.Update(packet, ft.flowOpts)
	}

	flow.XXX_state.updateVersion = ft.updateVersion + 1

	if ft.Opts.RawPacketLimit != 0 && flow.RawPacketsCaptured < ft.Opts.RawPacketLimit {
		flow.RawPacketsCaptured++
		data := &RawPacket{
			Timestamp: common.UnixMillis(packet.GoPacket.Metadata().CaptureInfo.Timestamp),
			Index:     flow.RawPacketsCaptured,
			Data:      packet.Data,
		}
		flow.LastRawPackets = append(flow.LastRawPackets, data)
	}

	return flow
}

func (ft *Table) processPacketSeq(ps *PacketSequence) {
	var parentUUID string
	logging.GetLogger().Debugf("%d Packets received for capture node %s", len(ps.Packets), ft.nodeTID)
	for _, packet := range ps.Packets {
		f := ft.packetToFlow(packet, parentUUID)
		parentUUID = f.UUID
	}
}

func (ft *Table) processFlow(fl *Flow) {
	prev := ft.replaceFlow(fl.UUID, fl)
	if prev != nil {
		fl.LastUpdateMetric = prev.LastUpdateMetric

		fl.XXX_state = prev.XXX_state
		fl.XXX_state.updateVersion = ft.updateVersion + 1
	}
}

// State returns the state of the flow table, stopped, running...
func (ft *Table) State() int64 {
	return atomic.LoadInt64(&ft.state)
}

// Run background jobs, like update/expire entries event
func (ft *Table) Run() {
	ft.wg.Add(1)
	defer ft.wg.Done()

	updateTicker := time.NewTicker(ft.updateHandler.every)
	defer updateTicker.Stop()

	expireTicker := time.NewTicker(ft.expireHandler.every)
	defer expireTicker.Stop()

	// ticker used internally to track fragment and tcp connections
	const ctDuration = 30 * time.Second

	ctTicker := time.NewTicker(ctDuration)
	defer ctTicker.Stop()

	nowTicker := time.NewTicker(time.Second * 1)
	defer nowTicker.Stop()

	ft.query = make(chan *TableQuery, 100)
	ft.reply = make(chan *TableReply, 100)

	atomic.StoreInt64(&ft.state, common.RunningState)
	for {
		select {
		case <-ft.quit:
			return
		case now := <-expireTicker.C:
			ft.expireAt(now)
		case now := <-updateTicker.C:
			ft.updateAt(now)
		case <-ft.flush:
			ft.tcpAssembler.FlushAll()

			ft.expireNow()
			ft.flushDone <- true
		case query, ok := <-ft.query:
			if ok {
				ft.reply <- ft.onQuery(query)
			}
		case ps := <-ft.packetSeqChan:
			ft.processPacketSeq(ps)
		case fl := <-ft.flowChan:
			ft.processFlow(fl)
		case now := <-ctTicker.C:
			t := now.Add(-ctDuration)
			ft.tcpAssembler.FlushOlderThan(t)
			ft.ipDefragger.FlushOlderThan(t)
		}
	}
}

// IPDefragger returns the ipDefragger if enabled
func (ft *Table) IPDefragger() *IPDefragger {
	if ft.Opts.IPDefrag {
		return ft.ipDefragger
	}
	return nil
}

// FeedWithGoPacket feeds the table with a gopacket
func (ft *Table) FeedWithGoPacket(packet gopacket.Packet, bpf *BPF) {
	if ps := PacketSeqFromGoPacket(packet, 0, bpf, ft.ipDefragger); len(ps.Packets) > 0 {
		ft.packetSeqChan <- ps
	}
}

// FeedWithSFlowSample feeds the table with sflow samples
func (ft *Table) FeedWithSFlowSample(sample *layers.SFlowFlowSample, bpf *BPF) {
	for _, ps := range PacketSeqFromSFlowSample(sample, bpf, ft.ipDefragger) {
		ft.packetSeqChan <- ps
	}
}

// Start the flow table
func (ft *Table) Start() (chan *PacketSequence, chan *Flow) {
	go ft.Run()
	return ft.packetSeqChan, ft.flowChan
}

// Stop the flow table
func (ft *Table) Stop() {
	ft.lockState.Lock()
	defer ft.lockState.Unlock()

	if atomic.CompareAndSwapInt64(&ft.state, common.RunningState, common.StoppingState) {
		ft.quit <- true
		ft.wg.Wait()

		close(ft.query)
		close(ft.reply)

		for len(ft.packetSeqChan) != 0 {
			ps := <-ft.packetSeqChan
			ft.processPacketSeq(ps)
		}

		for len(ft.flowChan) != 0 {
			fl := <-ft.flowChan
			ft.processFlow(fl)
		}

		close(ft.packetSeqChan)
		close(ft.flowChan)
	}

	ft.expireNow()
}
