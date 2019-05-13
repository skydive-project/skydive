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

package flow

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"github.com/hashicorp/golang-lru/simplelru"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/filters"
	"github.com/skydive-project/skydive/logging"
)

// HoldTimeoutMilliseconds is the number of milliseconds for holding ended flows before they get deleted from the flow table
const HoldTimeoutMilliseconds = int64(time.Duration(15) * time.Second / time.Millisecond)

// ExpireUpdateFunc defines expire and updates callback
type ExpireUpdateFunc func(f *FlowArray)

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
	Opts              TableOpts
	packetSeqChan     chan *PacketSequence
	flowChanOperation chan *Operation
	flowEBPFChan      chan *EBPFFlow
	table             *simplelru.LRU
	flush             chan bool
	flushDone         chan bool
	query             chan *TableQuery
	reply             chan []byte
	state             int64
	lockState         common.RWMutex
	wg                sync.WaitGroup
	quit              chan bool
	updateHandler     *Handler
	lastUpdate        int64
	updateVersion     int64
	expireHandler     *Handler
	lastExpire        int64
	nodeTID           string
	ipDefragger       *IPDefragger
	tcpAssembler      *TCPAssembler
	flowOpts          Opts
	appPortMap        *ApplicationPortMap
	appTimeout        map[string]int64
}

// OperationType operation type of a Flow in a flow table
type OperationType int

const (
	// ReplaceOperation replace the flow
	ReplaceOperation OperationType = iota
	// UpdateOperation update the flow
	UpdateOperation
)

// Operation describes a flow operation
type Operation struct {
	Key  string
	Flow *Flow
	Type OperationType
}

func updateTCPFlagTime(prevFlagTime int64, currFlagTime int64) int64 {
	if prevFlagTime != 0 {
		return prevFlagTime
	}
	return currFlagTime
}

// NewTable creates a new flow table
func NewTable(updateHandler *Handler, expireHandler *Handler, nodeTID string, opts ...TableOpts) *Table {
	appTimeout := make(map[string]int64)
	for key := range config.GetConfig().GetStringMap("flow.application_timeout") {
		// convert seconds to milleseconds
		appTimeout[strings.ToUpper(key)] = int64(1000 * config.GetConfig().GetInt("flow.application_timeout."+key))
	}
	LRU, _ := simplelru.NewLRU(500000, nil)
	t := &Table{
		packetSeqChan:     make(chan *PacketSequence, 1000),
		flowChanOperation: make(chan *Operation, 1000),
		flowEBPFChan:      make(chan *EBPFFlow, 1000),
		table:             LRU,
		flush:             make(chan bool),
		flushDone:         make(chan bool),
		state:             common.StoppedState,
		quit:              make(chan bool),
		updateHandler:     updateHandler,
		expireHandler:     expireHandler,
		nodeTID:           nodeTID,
		ipDefragger:       NewIPDefragger(),
		tcpAssembler:      NewTCPAssembler(),
		appPortMap:        NewApplicationPortMapFromConfig(),
		appTimeout:        appTimeout,
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
	keys := ft.table.Keys()
	for _, k := range keys {
		fl, _ := ft.table.Peek(k)
		f := fl.(*Flow)
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
	if flow, found := ft.table.Get(key); found {
		return flow.(*Flow), false
	}

	new := NewFlow()
	ft.table.Add(key, new)
	return new, true
}

func (ft *Table) replaceFlow(key string, f *Flow) *Flow {
	prev, _ := ft.table.Get(key)
	ft.table.Add(key, f)
	if prev == nil {
		return nil
	}
	return prev.(*Flow)
}

func (ft *Table) expire(expireBefore int64) {
	var expiredFlows []*Flow
	flowTableSzBefore := ft.table.Len()
	keys := ft.table.Keys()
	for _, k := range keys {
		fl, _ := ft.table.Peek(k)
		f := fl.(*Flow)
		if f.Last < expireBefore {
			duration := time.Duration(f.Last - f.Start)
			if f.XXX_state.updateVersion > ft.updateVersion {
				ft.updateMetric(f, ft.lastUpdate, f.Last)
			}

			logging.GetLogger().Debugf("Expire flow %s Duration %v", f.UUID, duration)
			if f.FinishType == FlowFinishType_NOT_FINISHED {
				f.FinishType = FlowFinishType_TIMEOUT
				expiredFlows = append(expiredFlows, f)
			}

			// need to use the key as the key could be not equal to the UUID
			ft.table.Remove(k)
		}
	}

	/* Advise Clients */
	ft.expireHandler.callback(&FlowArray{Flows: expiredFlows})

	flowTableSz := ft.table.Len()
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
	f.LastUpdateMetric.RTT = f.Metric.RTT

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
	for _, k := range ft.table.Keys() {
		fl, _ := ft.table.Peek(k)
		f := fl.(*Flow)
		if f.XXX_state.updateVersion > ft.updateVersion {
			ft.updateMetric(f, updateFrom, updateTime)
			updatedFlows = append(updatedFlows, f)
		} else if updateTime-f.Last > ft.appTimeout[f.Application] && ft.appTimeout[f.Application] > 0 {
			updatedFlows = append(updatedFlows, f)
			f.FinishType = FlowFinishType_TIMEOUT
			ft.table.Remove(k)
		} else {
			f.LastUpdateMetric = &FlowMetric{Start: updateFrom, Last: updateTime}
		}

		f.XXX_state.lastMetric = f.Metric.Copy()

		if f.FinishType != FlowFinishType_NOT_FINISHED && updateTime-f.Last >= HoldTimeoutMilliseconds {
			ft.table.Remove(k)
		}
	}

	if len(updatedFlows) != 0 {
		/* Advise Clients */
		ft.updateHandler.callback(&FlowArray{Flows: updatedFlows})
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

func (ft *Table) onQuery(tq *TableQuery) []byte {
	switch tq.Type {
	case "SearchQuery":
		fs := ft.getFlows(tq.Query)

		// marshal early to avoid race outside of query loop
		b, err := fs.Marshal()
		if err != nil {
			return nil
		}

		return b
	}

	return nil
}

// Query a flow table
func (ft *Table) Query(query *TableQuery) []byte {
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

func (ft *Table) processFlowOP(op *Operation) {
	switch op.Type {
	case ReplaceOperation:
		fl := op.Flow

		prev := ft.replaceFlow(op.Key, fl)
		if prev != nil {
			fl.LastUpdateMetric = prev.LastUpdateMetric

			fl.XXX_state = prev.XXX_state
			fl.XXX_state.updateVersion = ft.updateVersion + 1
		}
	case UpdateOperation:
		flo, found := ft.table.Get(op.Key)
		if !found {
			return
		}

		fl := flo.(*Flow)
		fl.Metric.ABBytes += op.Flow.Metric.ABBytes
		fl.Metric.BABytes += op.Flow.Metric.BABytes
		fl.Metric.ABPackets += op.Flow.Metric.ABPackets
		fl.Metric.BAPackets += op.Flow.Metric.BAPackets

		fl.Last = op.Flow.Last
		if fl.Transport != nil && fl.Transport.Protocol == FlowProtocol_TCP && fl.TCPMetric != nil {
			fl.TCPMetric = &TCPMetric{
				ABSynStart: updateTCPFlagTime(fl.TCPMetric.ABSynStart, op.Flow.TCPMetric.ABSynStart),
				BASynStart: updateTCPFlagTime(fl.TCPMetric.BASynStart, op.Flow.TCPMetric.BASynStart),
				ABFinStart: updateTCPFlagTime(fl.TCPMetric.ABFinStart, op.Flow.TCPMetric.ABFinStart),
				BAFinStart: updateTCPFlagTime(fl.TCPMetric.BAFinStart, op.Flow.TCPMetric.BAFinStart),
				ABRstStart: updateTCPFlagTime(fl.TCPMetric.ABRstStart, op.Flow.TCPMetric.ABRstStart),
				BARstStart: updateTCPFlagTime(fl.TCPMetric.BARstStart, op.Flow.TCPMetric.BARstStart),
			}
		}

		// TODO(safchain) remove this should be provided by the sender
		// with a good time resolution
		if fl.Metric.RTT == 0 && fl.Metric.ABPackets > 0 && fl.Metric.BAPackets > 0 {
			fl.Metric.RTT = fl.Last - fl.Start
		}
	}
}

func (ft *Table) processEBPFFlow(ebpfFlow *EBPFFlow, nfl *Flow) {
	key := kernFlowKey(ebpfFlow.kernFlow)
	f, found := ft.table.Get(key)
	if !found {
		keys, flows := ft.newFlowFromEBPF(ebpfFlow, key)
		for i := range keys {
			ft.table.Add(keys[i], flows[i])
		}
		return
	}
	ft.updateFlowFromEBPF(ebpfFlow, nfl)
	fl := f.(*Flow)
	fl.Metric.ABBytes += nfl.Metric.ABBytes
	fl.Metric.BABytes += nfl.Metric.BABytes
	fl.Metric.ABPackets += nfl.Metric.ABPackets
	fl.Metric.BAPackets += nfl.Metric.BAPackets

	fl.Last = nfl.Last
	if fl.Transport != nil && fl.Transport.Protocol == FlowProtocol_TCP && fl.TCPMetric != nil {
		fl.TCPMetric = &TCPMetric{
			ABSynStart: updateTCPFlagTime(fl.TCPMetric.ABSynStart, nfl.TCPMetric.ABSynStart),
			BASynStart: updateTCPFlagTime(fl.TCPMetric.BASynStart, nfl.TCPMetric.BASynStart),
			ABFinStart: updateTCPFlagTime(fl.TCPMetric.ABFinStart, nfl.TCPMetric.ABFinStart),
			BAFinStart: updateTCPFlagTime(fl.TCPMetric.BAFinStart, nfl.TCPMetric.BAFinStart),
			ABRstStart: updateTCPFlagTime(fl.TCPMetric.ABRstStart, nfl.TCPMetric.ABRstStart),
			BARstStart: updateTCPFlagTime(fl.TCPMetric.BARstStart, nfl.TCPMetric.BARstStart),
		}
	}
	// TODO(safchain) remove this should be provided by the sender
	// with a good time resolution
	if fl.Metric.RTT == 0 && fl.Metric.ABPackets > 0 && fl.Metric.BAPackets > 0 {
		fl.Metric.RTT = fl.Last - fl.Start
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

	ph := Flow{} // placeholder to avoid memory allocation upon update
	ph.TCPMetric = &TCPMetric{}
	ph.Metric = &FlowMetric{}
	ft.query = make(chan *TableQuery, 100)
	ft.reply = make(chan []byte, 100)

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
		case op := <-ft.flowChanOperation:
			ft.processFlowOP(op)
		case ebpfFlow := <-ft.flowEBPFChan:
			ft.processEBPFFlow(ebpfFlow, &ph)
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
func (ft *Table) Start() (chan *PacketSequence, chan *EBPFFlow, chan *Operation) {
	go ft.Run()
	return ft.packetSeqChan, ft.flowEBPFChan, ft.flowChanOperation
}

// Stop the flow table
func (ft *Table) Stop() {
	ft.lockState.Lock()
	defer ft.lockState.Unlock()

	if atomic.CompareAndSwapInt64(&ft.state, common.RunningState, common.StoppingState) {
		ft.quit <- true
		ft.wg.Wait()
		ph := Flow{}
		ph.TCPMetric = &TCPMetric{}
		ph.Metric = &FlowMetric{}

		close(ft.query)
		close(ft.reply)

		for len(ft.packetSeqChan) != 0 {
			ps := <-ft.packetSeqChan
			ft.processPacketSeq(ps)
		}

		for len(ft.flowChanOperation) != 0 {
			op := <-ft.flowChanOperation
			ft.processFlowOP(op)
		}

		for len(ft.flowEBPFChan) != 0 {
			ebpfFlow := <-ft.flowEBPFChan
			ft.processEBPFFlow(ebpfFlow, &ph)
		}

		close(ft.packetSeqChan)
		close(ft.flowChanOperation)
		close(ft.flowEBPFChan)
	}

	ft.expireNow()
}
