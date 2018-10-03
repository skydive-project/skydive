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
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/logging"
)

// PcapWriter provides helpers on top of gopacket pcap to write pcap files.
type PcapWriter struct {
	writer *pcapgo.Writer
}

// PcapTableFeeder replaies a pcap file
type PcapTableFeeder struct {
	sync.WaitGroup
	state       int64
	replay      bool
	r           io.ReadCloser
	handleRead  *pcapgo.Reader
	packetsChan chan *PacketSequence
	bpfFilter   string
}

// Start a pcap injector
func (p *PcapTableFeeder) Start() {
	if atomic.CompareAndSwapInt64(&p.state, common.StoppedState, common.RunningState) {
		p.Add(1)
		go p.feedFlowTable()
	}
}

// Stop a pcap injector
func (p *PcapTableFeeder) Stop() {
	if atomic.CompareAndSwapInt64(&p.state, common.RunningState, common.StoppingState) {
		atomic.StoreInt64(&p.state, common.StoppingState)
		p.r.Close()
		p.Wait()
		atomic.StoreInt64(&p.state, common.StoppedState)
	}
}

func (p *PcapTableFeeder) feedFlowTable() {
	var (
		lastTS   time.Time
		lastSend time.Time
		pkt      = 1
	)

	defer p.Done()

	var bpf *BPF
	if b, err := NewBPF(p.handleRead.LinkType(), MaxCaptureLength, p.bpfFilter); err == nil {
		bpf = b
	} else {
		logging.GetLogger().Error(err.Error())
	}

	atomic.StoreInt64(&p.state, common.RunningState)
	for atomic.LoadInt64(&p.state) == common.RunningState {
		logging.GetLogger().Debugf("Reading one pcap packet")
		data, ci, err := p.handleRead.ReadPacketData()
		if err != nil {
			if atomic.LoadInt64(&p.state) == common.RunningState && err != io.EOF {
				logging.GetLogger().Warningf("Failed to read packet: %s\n", err)
			}
			p.r.Close()
			return
		}

		packet := gopacket.NewPacket(data, p.handleRead.LinkType(), gopacket.DecodeOptions{NoCopy: true})
		packet.Metadata().CaptureInfo = ci
		if p.replay {
			intervalInCapture := ci.Timestamp.Sub(lastTS)
			elapsedTime := time.Since(lastSend)

			if (intervalInCapture > elapsedTime) && !lastSend.IsZero() {
				time.Sleep(intervalInCapture - elapsedTime)
			}

			lastSend = time.Now()
			lastTS = ci.Timestamp

			packet.Metadata().CaptureInfo.Timestamp = lastSend
		}

		ps := PacketSeqFromGoPacket(packet, 0, bpf, nil)
		if ps == nil {
			logging.GetLogger().Warningf("Failed to parse packet")
		} else if len(ps.Packets) > 0 {
			logging.GetLogger().Debugf("Sending %d packets to chan (%d)", len(ps.Packets), pkt)
			p.packetsChan <- ps
			logging.GetLogger().Debugf("Sent %d packets to chan (%d)", len(ps.Packets), pkt)
		}
		pkt++
	}
}

// NewPcapTableFeeder reads a pcap from a file reader and inject it in a flow table
func NewPcapTableFeeder(r io.ReadCloser, packetsChan chan *PacketSequence, replay bool, bpfFilter string) (*PcapTableFeeder, error) {
	handle, err := pcapgo.NewReader(r)
	if err != nil {
		return nil, err
	}

	return &PcapTableFeeder{
		replay:      replay,
		r:           r,
		handleRead:  handle,
		state:       common.StoppedState,
		packetsChan: packetsChan,
		bpfFilter:   bpfFilter,
	}, nil
}

// WriteRawPacket writes a RawPacket
func (p *PcapWriter) WriteRawPacket(r *RawPacket) error {
	ci := gopacket.CaptureInfo{
		Length:         int(MaxCaptureLength),
		CaptureLength:  len(r.Data),
		InterfaceIndex: 1,
		Timestamp:      time.Unix(0, r.Timestamp*int64(time.Millisecond)),
	}

	p.writer.WritePacket(ci, r.Data)

	return nil
}

// WriteRawPackets writes a RawPackets iterating over the RawPackets and using
// WriteRawPacket for each.
func (p *PcapWriter) WriteRawPackets(fr *RawPackets) error {
	if fr.LinkType != layers.LinkTypeEthernet {
		return errors.New("Support only Ethernet link type for the moment")
	}

	for _, r := range fr.RawPackets {
		if err := p.WriteRawPacket(r); err != nil {
			return err
		}
	}

	return nil
}

// NewPcapWriter returns a new PcapWriter based on the given io.Writer.
// Due to the current limitation of the gopacket pcap implementation only
// RawPacket with Ethernet link type are supported.
func NewPcapWriter(w io.Writer) *PcapWriter {
	writer := pcapgo.NewWriter(w)

	writer.WriteFileHeader(MaxCaptureLength, layers.LinkTypeEthernet)

	return &PcapWriter{
		writer: writer,
	}
}
