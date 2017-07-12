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
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/pcapgo"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/logging"
)

// PcapInject replay a pcap file
type PcapInject struct {
	sync.WaitGroup
	state       int64
	replay      bool
	r           io.ReadCloser
	handleRead  *pcapgo.Reader
	packetsChan chan *Packets
	bpfFilter   string
}

// Start a pcap injector
func (p *PcapInject) Start() {
	if atomic.CompareAndSwapInt64(&p.state, common.StoppedState, common.RunningState) {
		p.Add(1)
		go p.FeedFlowTable()
	}
}

// Stop a pcap injector
func (p *PcapInject) Stop() {
	if atomic.CompareAndSwapInt64(&p.state, common.RunningState, common.StoppingState) {
		atomic.StoreInt64(&p.state, common.StoppingState)
		p.r.Close()
		p.Wait()
		atomic.StoreInt64(&p.state, common.StoppedState)
	}
}

// FeedFlowTable mechanism, inject pcap in a flow table
func (p *PcapInject) FeedFlowTable() {
	var (
		lastTS    time.Time
		lastSend  time.Time
		pkt       = 1
		timestamp int64
	)

	defer p.Done()

	var bpf *BPF
	if b, err := NewBPF(p.handleRead.LinkType(), CaptureLength, p.bpfFilter); err == nil {
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

		packet := gopacket.NewPacket(data, p.handleRead.LinkType(), gopacket.NoCopy)
		if p.replay {
			timestamp = -1
			intervalInCapture := ci.Timestamp.Sub(lastTS)
			elapsedTime := time.Since(lastSend)

			if (intervalInCapture > elapsedTime) && !lastSend.IsZero() {
				time.Sleep(intervalInCapture - elapsedTime)
			}

			lastSend = time.Now()
			lastTS = ci.Timestamp
		} else {
			timestamp = common.UnixMillis(ci.Timestamp)
		}

		flowPackets := PacketsFromGoPacket(&packet, 0, timestamp, bpf)
		if flowPackets == nil {
			logging.GetLogger().Warningf("Failed to parse packet")
		} else if len(flowPackets.Packets) > 0 {
			logging.GetLogger().Debugf("Sending %d packets to chan (%d)", len(flowPackets.Packets), pkt)
			p.packetsChan <- flowPackets
			logging.GetLogger().Debugf("Sent %d packets to chan (%d)", len(flowPackets.Packets), pkt)
		}
		pkt++
	}
}

// NewPcapInject reads a pcap from a file reader and inject it in a flow table
func NewPcapInject(r io.ReadCloser, packetsChan chan *Packets, replay bool, bpfFilter string) (*PcapInject, error) {
	handle, err := pcapgo.NewReader(r)
	if err != nil {
		return nil, err
	}

	return &PcapInject{
		replay:      replay,
		r:           r,
		handleRead:  handle,
		state:       common.StoppedState,
		packetsChan: packetsChan,
		bpfFilter:   bpfFilter,
	}, nil
}
