/*
 * Copyright (C) 2018 Red Hat, Inc.
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
	"strconv"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/tcpassembly"

	"github.com/skydive-project/skydive/graffiti/logging"
)

// TCPAssembler defines a tcp reassembler
type TCPAssembler struct {
	assembler *tcpassembly.Assembler
	flows     map[uint64]*Flow
}

// TCPAssemblerStream will handle the actual tcp stream decoding
type TCPAssemblerStream struct {
	flow         *Flow
	network      gopacket.Flow
	transport    gopacket.Flow
	bytes        int64
	packets      int64
	outOfOrder   int64
	skipped      int64
	skippedBytes int64
	start        time.Time
	end          time.Time
	sawStart     bool
	sawEnd       bool
}

// NewTCPAssembler returns a new TCPAssembler
func NewTCPAssembler() *TCPAssembler {
	ta := &TCPAssembler{
		flows: make(map[uint64]*Flow, 0),
	}
	ta.assembler = tcpassembly.NewAssembler(tcpassembly.NewStreamPool(ta))

	return ta
}

func tcpKey(network, transport gopacket.Flow) uint64 {
	return network.FastHash() ^ transport.FastHash()
}

// FlushOlderThan frees the resources older than the given time
func (t *TCPAssembler) FlushOlderThan(tm time.Time) {
	t.assembler.FlushOlderThan(tm)
}

// FlushAll frees all the resources
func (t *TCPAssembler) FlushAll() {
	t.assembler.FlushAll()
}

// RegisterFlow registers a new flow to be tracked
func (t *TCPAssembler) RegisterFlow(flow *Flow, packet gopacket.Packet) {
	key := tcpKey(packet.NetworkLayer().NetworkFlow(), packet.TransportLayer().TransportFlow())
	t.flows[key] = flow

	t.Assemble(packet)
}

// Assemble add a new packet to be reassembled
func (t *TCPAssembler) Assemble(packet gopacket.Packet) {
	t.assembler.AssembleWithTimestamp(packet.NetworkLayer().NetworkFlow(), packet.TransportLayer().(*layers.TCP), packet.Metadata().Timestamp)
}

// New creates a new stream.  It's called whenever the assembler sees a stream
// it isn't currently following.
func (t *TCPAssembler) New(network, transport gopacket.Flow) tcpassembly.Stream {
	key := tcpKey(network, transport)
	f := t.flows[key]
	if f == nil {
		logging.GetLogger().Errorf("TCP Reassembly, unable to find flow: %s, %s", network.String(), transport.String())
	}

	return &TCPAssemblerStream{flow: f, network: network, transport: transport}
}

// Reassembled is called whenever new packet data is available for reading.
// Reassembly objects contain stream data in received order.
func (s *TCPAssemblerStream) Reassembled(reassemblies []tcpassembly.Reassembly) {
	for _, reassembly := range reassemblies {
		if s.start.IsZero() {
			s.start = reassembly.Seen
			s.end = s.start
		}
		if reassembly.Seen.Before(s.end) {
			s.outOfOrder++
		} else {
			s.end = reassembly.Seen
		}
		s.bytes += int64(len(reassembly.Bytes))
		s.packets++
		if reassembly.Skip != 0 {
			s.skipped++
			s.skippedBytes += int64(reassembly.Skip)
		}
		s.sawStart = s.sawStart || reassembly.Start
		s.sawEnd = s.sawEnd || reassembly.End
	}
}

// ReassemblyComplete is called when the TCP assembler believes a stream has finished.
func (s *TCPAssemblerStream) ReassemblyComplete() {
	f := s.flow
	if f == nil {
		return
	}

	m := f.TCPMetric
	if m == nil {
		m = &TCPMetric{}
		s.flow.TCPMetric = m
	}

	port, _ := strconv.ParseInt(s.transport.Src().String(), 10, 64)
	if f.Network.A == s.network.Src().String() && f.Transport.A == port {
		m.ABSegmentOutOfOrder = s.outOfOrder
		m.ABSegmentSkipped = s.skipped
		m.ABSegmentSkippedBytes = s.skippedBytes
		m.ABBytes = s.bytes
		m.ABPackets = s.packets
		if s.sawStart {
			m.ABSawStart = 1
		}
		if s.sawEnd {
			m.ABSawEnd = 1
		}
	} else {
		m.BASegmentOutOfOrder = s.outOfOrder
		m.BASegmentSkipped = s.skipped
		m.BASegmentSkippedBytes = s.skippedBytes
		m.BABytes = s.bytes
		m.BAPackets = s.packets
		if s.sawStart {
			m.BASawStart = 1
		}
		if s.sawEnd {
			m.BASawEnd = 1
		}
	}
	f.TCPMetric = m
}
