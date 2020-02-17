/*
 * Copyright (C) 2019 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
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

package targets

import (
	"bytes"
	"encoding/binary"
	"math"
	"net"
	"sync"
	"time"

	"github.com/google/gopacket"

	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/logging"
)

// NetFlowV5Target defines a NetFlow v5 target
type NetFlowV5Target struct {
	sync.RWMutex
	target    string
	conn      net.Conn
	table     *flow.Table
	sysBoot   time.Time
	sysBootMs int64
	seqNum    uint32
}

// SendPacket implements the Target interface
func (nf *NetFlowV5Target) SendPacket(packet gopacket.Packet, bpf *flow.BPF) {
	nf.table.FeedWithGoPacket(packet, bpf)
}

func (nf *NetFlowV5Target) sendFlowBulk(flows []*flow.Flow, now time.Time) {
	records := new(bytes.Buffer)

	var nFlows uint16
	for _, f := range flows {
		if f.Network == nil {
			continue
		}

		// Source IP Address
		ipA, err := common.IPStrToUint32(f.Network.A)
		if err != nil {
			continue
		}

		// Destination IP Address
		ipB, err := common.IPStrToUint32(f.Network.B)
		if err != nil {
			continue
		}

		binary.Write(records, binary.BigEndian, ipA)
		binary.Write(records, binary.BigEndian, ipB)

		// IP Address of the next hop router
		binary.Write(records, binary.BigEndian, uint32(0))

		// Index of input interface
		binary.Write(records, binary.BigEndian, uint16(0))

		// Index of output interface
		binary.Write(records, binary.BigEndian, uint16(0))

		// Number of packets in the flow
		binary.Write(records, binary.BigEndian, uint32(f.Metric.ABPackets+f.Metric.BAPackets))

		// Total number of Layer 3 bytes in the packets of the flow
		etherSize := int64(0)
		if f.Link != nil {
			etherSize = int64(14)
		}
		binary.Write(records, binary.BigEndian, uint32(f.Metric.ABBytes+f.Metric.BABytes-etherSize))

		// SysUptime at start of flow in ms since last boot
		ms := f.Metric.Start - nf.sysBootMs
		binary.Write(records, binary.BigEndian, uint32(ms))

		// SysUptime at end of flow in ms since last boot
		ms = f.Metric.Last - nf.sysBootMs
		binary.Write(records, binary.BigEndian, uint32(ms))

		srcPort, dstPort := uint16(0), uint16(0)
		if f.Transport != nil {
			srcPort, dstPort = uint16(f.Transport.A), uint16(f.Transport.B)
		}

		// TCP/UDP source port number or equivalent
		binary.Write(records, binary.BigEndian, uint16(srcPort))
		// TCP/UDP destination port number or equivalent
		binary.Write(records, binary.BigEndian, uint16(dstPort))

		// Unused (zero) bytes
		binary.Write(records, binary.BigEndian, uint8(0))

		// Cumulative OR of TCP flags
		binary.Write(records, binary.BigEndian, uint8(0))

		// IP protocol type (for example, TCP = 6; UDP = 17)
		if f.Network.Protocol == flow.FlowProtocol_TCP {
			binary.Write(records, binary.BigEndian, uint8(6))
		} else {
			binary.Write(records, binary.BigEndian, uint8(17))
		}

		// IP type of service (ToS)
		binary.Write(records, binary.BigEndian, uint8(0))

		// Autonomous system number of the source, either origin or peer
		binary.Write(records, binary.BigEndian, uint16(0))

		// Autonomous system number of the destination, either origin or peer
		binary.Write(records, binary.BigEndian, uint16(0))

		// Source address prefix mask bits
		binary.Write(records, binary.BigEndian, uint8(math.MaxUint8))

		// Destination address prefix mask bits
		binary.Write(records, binary.BigEndian, uint8(math.MaxUint8))

		// Unused (zero) bytes
		binary.Write(records, binary.BigEndian, uint16(0))

		nFlows++
	}

	if nFlows == 0 {
		return
	}

	header := new(bytes.Buffer)

	// Version
	binary.Write(header, binary.BigEndian, uint16(5))

	// Count
	binary.Write(header, binary.BigEndian, nFlows)

	// SysUpTimeMSecs
	ms := now.Sub(nf.sysBoot)
	binary.Write(header, binary.BigEndian, uint32(ms/time.Millisecond))

	// UNIXSecs
	binary.Write(header, binary.BigEndian, uint32(now.Unix()))

	// UNIXNSecs
	binary.Write(header, binary.BigEndian, uint32(now.Nanosecond()))

	// SeqNum
	nf.seqNum += uint32(nFlows)
	binary.Write(header, binary.BigEndian, nf.seqNum)

	// EngType
	binary.Write(header, binary.BigEndian, uint8(0))

	// EngID
	binary.Write(header, binary.BigEndian, uint8(1))

	// SmpInt
	binary.Write(header, binary.BigEndian, uint16(0))

	nf.conn.Write(append(header.Bytes(), records.Bytes()...))
}

// SendFlows implements the flow Sender interface
func (nf *NetFlowV5Target) SendFlows(flows []*flow.Flow) {
	now := time.Now()

	const bulkSize int = 20

	for i := 0; i < len(flows); i += bulkSize {
		e := i + bulkSize

		if e > len(flows) {
			e = len(flows)
		}

		nf.sendFlowBulk(flows[i:e], now)
	}
}

// SendStats implements the flow Sender interface
func (nf *NetFlowV5Target) SendStats(stats flow.Stats) {
}

// Start start the target
func (nf *NetFlowV5Target) Start() {
	conn, err := net.Dial("udp", nf.target)
	if err != nil {
		logging.GetLogger().Errorf("NetFlow socket error: %s", err)
		return
	}

	nf.Lock()
	defer nf.Unlock()

	nf.conn = conn
	nf.table.Start(nil)
}

// Stop stops the target
func (nf *NetFlowV5Target) Stop() {
	nf.RLock()
	defer nf.RUnlock()

	nf.table.Stop()

	if nf.conn != nil {
		nf.conn.Close()
	}
}

// NewNetFlowV5Target returns a new NetFlow v5 target
func NewNetFlowV5Target(g *graph.Graph, n *graph.Node, capture *types.Capture, uuids flow.UUIDs) (*NetFlowV5Target, error) {
	now := time.Now()

	updateEvery := time.Duration(config.GetInt("flow.update")) * time.Second
	expireAfter := time.Duration(config.GetInt("flow.expire")) * time.Second

	nf := &NetFlowV5Target{
		target:    capture.Target,
		sysBoot:   now,
		sysBootMs: common.UnixMillis(now),
	}

	nf.table = flow.NewTable(updateEvery, expireAfter, nf, uuids, tableOptsFromCapture(capture))

	return nf, nil
}
