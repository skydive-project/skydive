/*
 * Copyright (C) 2015 Red Hat, Inc.
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

package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"

	"github.com/redhat-cip/skydive/logging"
)

var (
	NbPackets    uint
	PacketsBytes uint

	NbSFlowMsg        uint
	SFlowMsgBytes     uint
	NbSFlowMsgDropped uint

	SFlowSeq       uint
	SFlowSampleSeq uint
)

const (
	MinPPS = 100
)

type Throttle struct {
	MaxHitPerSecond uint
	hits            uint
	last            time.Time
	delta           time.Duration
}

func (t *Throttle) StartHook(hits uint) {
	t.hits += hits

	if t.hits < (t.MaxHitPerSecond / MinPPS) {
		return
	}

	now := time.Now()
	if !t.last.IsZero() {
		t.delta = now.Sub(t.last)
	}
	t.last = now
}

func (t *Throttle) wait() {
	sleeptime := time.Duration((int64(t.hits) * time.Second.Nanoseconds()) / (int64(t.MaxHitPerSecond)))
	derror := sleeptime - t.delta

	time.Sleep(sleeptime + derror)
	t.hits = 0
}

func (t *Throttle) EndHook() {
	if t.hits < (t.MaxHitPerSecond / MinPPS) {
		return
	}
	t.wait()
}

func SFlowRawPacketFlowRecordSerialize(rec *layers.SFlowRawPacketFlowRecord, payload *[]byte) []byte {
	nbBytes := uint32(len(*payload))
	rec.FrameLength = nbBytes
	rec.HeaderLength = nbBytes
	rec.FlowDataLength = rec.HeaderLength + 16

	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, ((uint32(rec.EnterpriseID) << 12) | (uint32(rec.Format))))
	binary.Write(buf, binary.BigEndian, rec.FlowDataLength)
	binary.Write(buf, binary.BigEndian, rec.HeaderProtocol)
	binary.Write(buf, binary.BigEndian, rec.FrameLength)
	binary.Write(buf, binary.BigEndian, rec.PayloadRemoved)
	binary.Write(buf, binary.BigEndian, rec.HeaderLength)
	buf.Write(*payload)
	// The raw packet need to be Aligned on 4 bytes
	headerLenWithPadding := uint32(rec.HeaderLength + ((4 - rec.HeaderLength) % 4))
	npad := headerLenWithPadding - nbBytes
	for ; npad > 0; npad-- {
		buf.Write([]byte{0})
	}
	return buf.Bytes()
}

func SFlowFlowSampleSerialize(sf *layers.SFlowFlowSample, packets *[][]byte) []byte {
	bufRec := new(bytes.Buffer)
	for _, record := range sf.Records {
		rec := record.(layers.SFlowRawPacketFlowRecord)
		for _, payload := range *packets {
			bufRec.Write(SFlowRawPacketFlowRecordSerialize(&rec, &payload))
		}
	}
	sf.SampleLength = uint32(bufRec.Len()) + 32
	sf.RecordCount = uint32(len(*packets))

	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, ((uint32(sf.EnterpriseID) << 12) | (uint32(sf.Format))))
	binary.Write(buf, binary.BigEndian, sf.SampleLength)
	binary.Write(buf, binary.BigEndian, sf.SequenceNumber)
	binary.Write(buf, binary.BigEndian, ((uint32(sf.SourceIDClass) << 30) | (uint32(sf.SourceIDIndex))))
	binary.Write(buf, binary.BigEndian, sf.SamplingRate)
	binary.Write(buf, binary.BigEndian, sf.SamplePool)
	binary.Write(buf, binary.BigEndian, sf.Dropped)
	binary.Write(buf, binary.BigEndian, sf.InputInterface)
	binary.Write(buf, binary.BigEndian, sf.OutputInterface)
	binary.Write(buf, binary.BigEndian, sf.RecordCount)
	buf.Write(bufRec.Bytes())
	return buf.Bytes()
}

func SFlowDatagramSerialize(sfd *layers.SFlowDatagram, packets *[][]byte) []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, sfd.DatagramVersion)
	binary.Write(buf, binary.BigEndian, uint32(layers.SFlowIPv4))
	binary.Write(buf, binary.BigEndian, sfd.AgentAddress)
	binary.Write(buf, binary.BigEndian, sfd.SubAgentID)
	binary.Write(buf, binary.BigEndian, sfd.SequenceNumber)
	binary.Write(buf, binary.BigEndian, sfd.AgentUptime)
	binary.Write(buf, binary.BigEndian, sfd.SampleCount)
	for _, fs := range sfd.FlowSamples {
		buf.Write(SFlowFlowSampleSerialize(&fs, packets))
	}
	return buf.Bytes()
}

func SFlowPackets(packets *[][]byte) []byte {
	NbSFlowMsg++

	sfraw := layers.SFlowRawPacketFlowRecord{
		SFlowBaseFlowRecord: layers.SFlowBaseFlowRecord{
			EnterpriseID: layers.SFlowStandard,
			Format:       layers.SFlowTypeRawPacketFlow,
			//				FlowDataLength uint32
		},
		HeaderProtocol: layers.SFlowProtoEthernet,
		//	FrameLength    uint32
		PayloadRemoved: 0,
		//	HeaderLength   uint32
		//	Header         gopacket.NewPacket
	}

	SFlowSampleSeq++
	sf := layers.SFlowFlowSample{
		EnterpriseID: layers.SFlowStandard,
		Format:       layers.SFlowTypeFlowSample,
		//	SampleLength    uint32
		SequenceNumber:  uint32(SFlowSampleSeq),
		SourceIDClass:   layers.SFlowTypeSingleInterface,
		SourceIDIndex:   layers.SFlowSourceValue(47),
		SamplingRate:    300,
		SamplePool:      0x12345,
		Dropped:         0,
		InputInterface:  48,
		OutputInterface: 47,
		//		RecordCount:     1,
		//		Records:
	}
	sf.Records = append(sf.Records, sfraw)
	sf.RecordCount = uint32(len(sf.Records))

	SFlowSeq++
	sflowLayer := &layers.SFlowDatagram{
		DatagramVersion: 5,
		AgentAddress:    net.IP{127, 0, 0, 3},
		SubAgentID:      0,
		SequenceNumber:  uint32(SFlowSeq),
		AgentUptime:     2294190,
		//		SampleCount:     count,
		//		FlowSamples:     sflowsamples,
		//		CounterSamples  []SFlowCounterSample
	}
	sflowLayer.FlowSamples = append(sflowLayer.FlowSamples, sf)
	sflowLayer.SampleCount = uint32(len(sflowLayer.FlowSamples))

	rawBytes := SFlowDatagramSerialize(sflowLayer, packets)
	return rawBytes
}

func sendPackets(conn *net.UDPConn, packets *[][]byte) {
	sflowPacketData := SFlowPackets(packets)
	*packets = (*packets)[:0]

	NbSFlowMsg++
	SFlowMsgBytes += uint(len(sflowPacketData))
	_, err := conn.Write(sflowPacketData)
	if err != nil {
		if (NbSFlowMsgDropped % 1000) == 0 {
			logging.GetLogger().Critical("PCAP2SFlow Agent connection issue : %s", err.Error())
		}
		NbSFlowMsgDropped++
	}
}

func NewUDPConnection(addr string, port int) (*net.UDPConn, error) {
	srv, err := net.ResolveUDPAddr("udp", addr+":"+strconv.FormatInt(int64(port), 10))
	if err != nil {
		return nil, err
	}

	connection, err := net.DialUDP("udp", nil, srv)
	if err != nil {
		return nil, err
	}

	return connection, nil
}

func AsyncProgressInfo() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	oldNbPackets := NbPackets
	for {
		<-ticker.C
		dpkts := NbPackets - oldNbPackets
		oldNbPackets = NbPackets
		logging.GetLogger().Debug("%d packets replayed, pps %d, NbSFlowMsgDropped %d", NbPackets, dpkts, NbSFlowMsgDropped)
	}
}

func usage() {
	fmt.Printf("\nUsage: %s -trace <trace.pcap> [-pps <1000>] [-pktspersflow <5>] <sflow_agent[:port]>\n", filepath.Base(os.Args[0]))
}

func main() {
	pcaptrace := flag.String("trace", "", "PCAP trace file to read")
	pps := flag.Uint("pps", 1000, "Packets per second")
	PktsPerFlow := flag.Uint("pktspersflow", 5, "Number of Packets per SFlow Datagram")
	flag.CommandLine.Usage = usage
	flag.Parse()

	if *pcaptrace == "" {
		usage()
		os.Exit(1)
	}
	if *pps < MinPPS {
		logging.GetLogger().Fatal("Minimal packet per seconds is ", MinPPS)
	}

	sflowAgent := strings.Split(os.Args[len(os.Args)-1], ":")
	AgentAddr := sflowAgent[0]
	AgentPort := 6345
	if len(sflowAgent) == 2 {
		var err error
		AgentPort, err = strconv.Atoi(sflowAgent[1])
		if err != nil {
			logging.GetLogger().Fatal("Can't parse UDP port ", err)
		}
	}

	conn, err := NewUDPConnection(AgentAddr, AgentPort)
	if err != nil {
		logging.GetLogger().Fatal("UDP connection error ", err)
	}
	conn.SetWriteBuffer(256 * 1024)

	f, err := os.Open(*pcaptrace)
	if err != nil {
		logging.GetLogger().Fatal("PCAP OpenOffline error (", *pcaptrace, ")", err)
	}
	defer f.Close()
	handleRead, err := pcapgo.NewReader(f)
	if err != nil {
		logging.GetLogger().Fatal("PCAP OpenOffline error (handle to read packet)", err)
	}

	go AsyncProgressInfo()

	throttle := Throttle{MaxHitPerSecond: *pps}

	var packets [][]byte
	for {
		data, _, err := handleRead.ReadPacketData()
		if err != nil && err != io.EOF {
			logging.GetLogger().Debug("Capture file has been cut in the middle of a packet", err.Error())
			break
		} else if err == io.EOF {
			logging.GetLogger().Debug("End of capture file")
			break
		} else {
			NbPackets++
			PacketsBytes += uint(len(data))

			dataCopy := make([]byte, len(data))
			copy(dataCopy, data)
			packets = append(packets, dataCopy)

			if (NbPackets % *PktsPerFlow) != 0 {
				continue
			}

			throttle.StartHook(*PktsPerFlow)
			sendPackets(conn, &packets)
			throttle.EndHook()
		}
	}
	if len(packets) > 0 {
		sendPackets(conn, &packets)
	}

	logging.GetLogger().Info("PCAP Trace replay finished")
	return
}
