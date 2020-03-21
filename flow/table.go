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
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/hashicorp/golang-lru/simplelru"
	"github.com/safchain/insanelock"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/filters"
	"github.com/skydive-project/skydive/graffiti/service"
	"github.com/skydive-project/skydive/logging"
)

// HoldTimeoutMilliseconds is the number of milliseconds for holding ended flows before they get deleted from the flow table
const HoldTimeoutMilliseconds = int64(time.Duration(15) * time.Second / time.Millisecond)

// TableOpts defines flow table options
type TableOpts struct {
	RawPacketLimit int64
	ExtraTCPMetric bool
	IPDefrag       bool
	ReassembleTCP  bool
	LayerKeyMode   LayerKeyMode
	ExtraLayers    ExtraLayers
}

// UUIDs describes UUIDs that can be applied to flows table wise
type UUIDs struct {
	NodeTID   string
	CaptureID string
}

// ExtFlowType type of an external flow
type ExtFlowType int

const (
	// OperationExtFlowType classic flow passed as Operation
	OperationExtFlowType = iota

	// EBPFExtFlowType type of eBPF flow
	EBPFExtFlowType
)

// ExtFlow structure use to send external flow to the flow table
type ExtFlow struct {
	Type ExtFlowType
	Obj  interface{}
}

// Table store the flow table and related metrics mechanism
type Table struct {
	Opts              TableOpts
	packetSeqChan     chan *PacketSequence
	extFlowChan       chan *ExtFlow
	statsChan         chan Stats
	expiredExtKeyChan chan interface{} // used when flow expired, the extKey will be sent over this chan
	table             *simplelru.LRU
	flush             chan bool
	flushDone         chan bool
	query             chan *TableQuery
	reply             chan []byte
	state             service.State
	lockState         insanelock.RWMutex
	wg                sync.WaitGroup
	quit              chan bool
	updateEvery       time.Duration
	expireAfter       time.Duration
	sender            Sender
	lastUpdate        int64
	updateVersion     int64
	lastExpire        int64
	ipDefragger       *IPDefragger
	tcpAssembler      *TCPAssembler
	opts              Opts
	appPortMap        *ApplicationPortMap
	appTimeout        map[string]int64
	stats             Stats
	uuids             UUIDs
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
	Key  uint64
	Flow *Flow
	Type OperationType
}

// Sender defines a flows sender interface
type Sender interface {
	SendFlows(flows []*Flow)
	SendStats(stats Stats)
}

func updateTCPFlagTime(prevFlagTime int64, currFlagTime int64) int64 {
	if prevFlagTime != 0 {
		return prevFlagTime
	}
	return currFlagTime
}

// NewTable creates a new flow table
func NewTable(updateEvery, expireAfter time.Duration, sender Sender, uuids UUIDs, opts ...TableOpts) *Table {
	appTimeout := make(map[string]int64)
	for key := range config.GetConfig().GetStringMap("flow.application_timeout") {
		// convert seconds to milleseconds
		appTimeout[strings.ToUpper(key)] = int64(1000 * config.GetConfig().GetInt("flow.application_timeout."+key))
	}
	LRU, _ := simplelru.NewLRU(config.GetConfig().GetInt("flow.max_entries"), nil)
	t := &Table{
		packetSeqChan: make(chan *PacketSequence, 1000),
		extFlowChan:   make(chan *ExtFlow, 1000),
		statsChan:     make(chan Stats, 10),
		table:         LRU,
		flush:         make(chan bool),
		flushDone:     make(chan bool),
		state:         service.StoppedState,
		quit:          make(chan bool),
		updateEvery:   updateEvery,
		expireAfter:   expireAfter,
		sender:        sender,
		uuids:         uuids,
		appPortMap:    NewApplicationPortMapFromConfig(),
		appTimeout:    appTimeout,
	}
	if len(opts) > 0 {
		t.Opts = opts[0]
	}

	t.opts = Opts{
		TCPMetric:    t.Opts.ExtraTCPMetric,
		IPDefrag:     t.Opts.IPDefrag,
		LayerKeyMode: t.Opts.LayerKeyMode,
		AppPortMap:   t.appPortMap,
		ExtraLayers:  t.Opts.ExtraLayers,
	}

	if t.Opts.IPDefrag {
		t.ipDefragger = NewIPDefragger()
	}

	if t.Opts.ReassembleTCP {
		t.tcpAssembler = NewTCPAssembler()
	}

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

func (ft *Table) getOrCreateFlow(key uint64) (*Flow, bool) {
	if flow, found := ft.table.Get(key); found {
		return flow.(*Flow), false
	}

	new := NewFlow()
	if ft.table.Add(key, new) {
		ft.stats.FlowDropped++
	}
	return new, true
}

func (ft *Table) replaceFlow(key uint64, f *Flow) *Flow {
	prev, _ := ft.table.Get(key)
	if ft.table.Add(key, f) {
		ft.stats.FlowDropped++
	}
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

	ft.sender.SendFlows(expiredFlows)

	if ft.expiredExtKeyChan != nil {
		for _, f := range expiredFlows {
			if f.XXX_state.extKey != nil {
				ft.expiredExtKeyChan <- f.XXX_state.extKey
			}
		}
	}

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
		start = f.Start
	}

	*f.LastUpdateMetric = *f.Metric
	f.LastUpdateMetric.ABPackets -= f.XXX_state.lastMetric.ABPackets
	f.LastUpdateMetric.ABBytes -= f.XXX_state.lastMetric.ABBytes
	f.LastUpdateMetric.BAPackets -= f.XXX_state.lastMetric.BAPackets
	f.LastUpdateMetric.BABytes -= f.XXX_state.lastMetric.BABytes
	f.LastUpdateMetric.Start = start
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
		} else if f.LastUpdateMetric != nil {
			f.LastUpdateMetric.ABBytes = 0
			f.LastUpdateMetric.ABPackets = 0
			f.LastUpdateMetric.BABytes = 0
			f.LastUpdateMetric.BAPackets = 0
			f.LastUpdateMetric.Start = updateFrom
			f.LastUpdateMetric.Last = updateTime
		} else {
			f.LastUpdateMetric = &FlowMetric{Start: updateFrom, Last: updateTime}
		}

		f.XXX_state.lastMetric = *f.Metric

		if f.FinishType != FlowFinishType_NOT_FINISHED && updateTime-f.Last >= HoldTimeoutMilliseconds {
			ft.table.Remove(k)
		}
	}

	if len(updatedFlows) != 0 {
		/* Advise Clients */
		ft.sender.SendFlows(updatedFlows)
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

	if ft.state.Load() == service.RunningState {
		ft.query <- query

		timer := time.NewTicker(1 * time.Second)
		defer timer.Stop()

		for {
			select {
			case r := <-ft.reply:
				return r
			case <-timer.C:
				if ft.state.Load() != service.RunningState {
					return nil
				}
			}
		}
	}
	return nil
}

func (ft *Table) packetToFlow(packet *Packet, parentUUID string) *Flow {
	key, l2Key, l3Key := packet.Keys(parentUUID, &ft.uuids, &ft.opts)
	flow, new := ft.getOrCreateFlow(key)
	if new {
		if ft.Opts.ReassembleTCP {
			if layer := packet.GoPacket.TransportLayer(); layer != nil && layer.LayerType() == layers.LayerTypeTCP {
				ft.tcpAssembler.RegisterFlow(flow, packet.GoPacket)
			}
		}

		flow.initFromPacket(key, l2Key, l3Key, packet, parentUUID, &ft.uuids, &ft.opts)
	} else {
		if ft.Opts.ReassembleTCP {
			if layer := packet.GoPacket.TransportLayer(); layer != nil && layer.LayerType() == layers.LayerTypeTCP {
				ft.tcpAssembler.Assemble(packet.GoPacket)
			}
		}

		flow.Update(packet, &ft.opts)
	}

	/* we need to reset state here to avoid re-using underlayer in tunnel */
	flow.XXX_state.ipv4 = nil
	flow.XXX_state.ipv6 = nil

	// notify that the flow has been updated between two table updates
	flow.XXX_state.updateVersion = ft.updateVersion + 1

	if ft.Opts.RawPacketLimit != 0 && flow.RawPacketsCaptured < ft.Opts.RawPacketLimit {
		flow.RawPacketsCaptured++
		linkType, _ := flow.LinkType()
		data := &RawPacket{
			Timestamp: common.UnixMillis(packet.GoPacket.Metadata().CaptureInfo.Timestamp),
			Index:     flow.RawPacketsCaptured,
			Data:      packet.Data,
			LinkType:  linkType,
		}
		flow.LastRawPackets = append(flow.LastRawPackets, data)
	}

	return flow
}

func (ft *Table) processPacketSeq(ps *PacketSequence) {
	var parentUUID string
	logging.GetLogger().Debugf("%d Packets received for capture node %s", len(ps.Packets), ft.uuids.NodeTID)
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

func (ft *Table) processEBPFFlow(ebpfFlow *EBPFFlow) {
	key := kernFlowKey(ebpfFlow.KernFlow)
	f, found := ft.table.Get(key)
	if !found {
		keys, flows, err := ft.newFlowFromEBPF(ebpfFlow, key)
		if err != nil {
			logging.GetLogger().Debugf("eBPF flow parsing error: %s", err)
			return
		}

		for i := range keys {
			if ft.table.Add(keys[i], flows[i]) {
				ft.stats.FlowDropped++
			}
		}
		return
	}
	if ft.updateFlowFromEBPF(ebpfFlow, f.(*Flow)) {
		// notify that the flow has been updated between two table updates
		f.(*Flow).XXX_state.updateVersion = ft.updateVersion + 1
	}
}

func (ft *Table) processExtFlow(extFlow *ExtFlow) {
	switch extFlow.Type {
	case OperationExtFlowType:
		ft.processFlowOP(extFlow.Obj.(*Operation))
	case EBPFExtFlowType:
		ft.processEBPFFlow(extFlow.Obj.(*EBPFFlow))
	}
}

// State returns the state of the flow table, stopped, running...
func (ft *Table) State() service.State {
	return ft.state.Load()
}

// Run background jobs, like update/expire entries event
func (ft *Table) Run() {
	ft.wg.Add(1)
	defer ft.wg.Done()

	updateTicker := time.NewTicker(ft.updateEvery)
	defer updateTicker.Stop()

	expireTicker := time.NewTicker(ft.expireAfter)
	defer expireTicker.Stop()

	// ticker used internally to track fragment and tcp connections
	const ctDuration = 30 * time.Second

	ctTicker := time.NewTicker(ctDuration)
	defer ctTicker.Stop()

	nowTicker := time.NewTicker(time.Second * 1)
	defer nowTicker.Stop()

	statsTicker := time.NewTicker(time.Second * 10)
	defer statsTicker.Stop()

	ft.query = make(chan *TableQuery, 100)
	ft.reply = make(chan []byte, 100)

	ft.state.Store(service.RunningState)
	for {
		select {
		case <-ft.quit:
			return
		case now := <-expireTicker.C:
			ft.expireAt(now)
		case now := <-updateTicker.C:
			ft.updateAt(now)
		case <-ft.flush:
			if ft.Opts.ReassembleTCP {
				ft.tcpAssembler.FlushAll()
			}

			ft.expireNow()
			ft.flushDone <- true
		case query, ok := <-ft.query:
			if ok {
				ft.reply <- ft.onQuery(query)
			}
		case ps := <-ft.packetSeqChan:
			ft.processPacketSeq(ps)
		case extFlow := <-ft.extFlowChan:
			ft.processExtFlow(extFlow)
		case now := <-ctTicker.C:
			t := now.Add(-ctDuration)
			if ft.tcpAssembler != nil {
				ft.tcpAssembler.FlushOlderThan(t)
			}
			if ft.ipDefragger != nil {
				ft.ipDefragger.FlushOlderThan(t)
			}
		case s := <-ft.statsChan:
			ft.stats.KernelFlowDropped += s.KernelFlowDropped
			ft.stats.PacketsReceived += s.PacketsReceived
			ft.stats.PacketsDropped += s.PacketsDropped
		case <-statsTicker.C:
			ft.stats.CaptureID = ft.uuids.CaptureID
			ft.stats.FlowCount = int64(ft.table.Len())

			ft.sender.SendStats(ft.stats)

			logging.GetLogger().Debugf("Flow table stats: %+v", ft.stats)
			ft.stats = Stats{}
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
func (ft *Table) Start(extFlowExp chan interface{}) (chan *PacketSequence, chan *ExtFlow, chan Stats) {
	ft.expiredExtKeyChan = extFlowExp
	go ft.Run()
	return ft.packetSeqChan, ft.extFlowChan, ft.statsChan
}

// Stop the flow table
func (ft *Table) Stop() {
	ft.lockState.Lock()
	defer ft.lockState.Unlock()

	if ft.state.CompareAndSwap(service.RunningState, service.StoppingState) {
		ft.quit <- true
		ft.wg.Wait()

		close(ft.query)
		close(ft.reply)

		for len(ft.packetSeqChan) != 0 {
			ps := <-ft.packetSeqChan
			ft.processPacketSeq(ps)
		}

		for len(ft.extFlowChan) != 0 {
			extFlow := <-ft.extFlowChan
			ft.processExtFlow(extFlow)
		}

		close(ft.packetSeqChan)
		close(ft.extFlowChan)
	}

	ft.expireNow()
}
