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

package tools

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"

	"github.com/skydive-project/skydive/logging"
)

var (
	sflowSeq       uint
	sflowSampleSeq uint
)

const (
	minPPS = 100
)

type throttle struct {
	maxHitPerSecond uint32
	hits            uint32
	last            time.Time
	delta           time.Duration
}

func (t *throttle) startHook(hits uint32) {
	t.hits += hits

	if t.hits < (t.maxHitPerSecond / minPPS) {
		return
	}

	now := time.Now()
	if !t.last.IsZero() {
		t.delta = now.Sub(t.last)
	}
	t.last = now
}

func (t *throttle) wait() {
	sleeptime := time.Duration((int64(t.hits) * time.Second.Nanoseconds()) / (int64(t.maxHitPerSecond)))
	derror := sleeptime - t.delta

	time.Sleep(sleeptime + derror)
	t.hits = 0
}

func (t *throttle) endHook() {
	if t.hits < (t.maxHitPerSecond / minPPS) {
		return
	}
	t.wait()
}

func sflowRawPacketFlowRecordSerialize(rec *layers.SFlowRawPacketFlowRecord, payload *[]byte) []byte {
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

func sflowFlowSampleSerialize(sf *layers.SFlowFlowSample, packets *[][]byte) []byte {
	bufRec := new(bytes.Buffer)
	for _, record := range sf.Records {
		rec := record.(layers.SFlowRawPacketFlowRecord)
		for _, payload := range *packets {
			bufRec.Write(sflowRawPacketFlowRecordSerialize(&rec, &payload))
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

func sflowDatagramSerialize(sfd *layers.SFlowDatagram, packets *[][]byte) []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, sfd.DatagramVersion)
	binary.Write(buf, binary.BigEndian, uint32(layers.SFlowIPv4))
	binary.Write(buf, binary.BigEndian, sfd.AgentAddress)
	binary.Write(buf, binary.BigEndian, sfd.SubAgentID)
	binary.Write(buf, binary.BigEndian, sfd.SequenceNumber)
	binary.Write(buf, binary.BigEndian, sfd.AgentUptime)
	binary.Write(buf, binary.BigEndian, sfd.SampleCount)
	for _, fs := range sfd.FlowSamples {
		buf.Write(sflowFlowSampleSerialize(&fs, packets))
	}
	return buf.Bytes()
}

func sflowPackets(packets *[][]byte, sflowSampleSeq *uint32, sflowSeq *uint32) []byte {
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

	*sflowSampleSeq++
	sf := layers.SFlowFlowSample{
		EnterpriseID: layers.SFlowStandard,
		Format:       layers.SFlowTypeFlowSample,
		//	SampleLength    uint32
		SequenceNumber:  uint32(*sflowSampleSeq),
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

	*sflowSeq++
	sflowLayer := &layers.SFlowDatagram{
		DatagramVersion: 5,
		AgentAddress:    net.IP{127, 0, 0, 3},
		SubAgentID:      0,
		SequenceNumber:  uint32(*sflowSeq),
		AgentUptime:     2294190,
		//		SampleCount:     count,
		//		FlowSamples:     sflowsamples,
		//		CounterSamples  []SFlowCounterSample
	}
	sflowLayer.FlowSamples = append(sflowLayer.FlowSamples, sf)
	sflowLayer.SampleCount = uint32(len(sflowLayer.FlowSamples))

	rawBytes := sflowDatagramSerialize(sflowLayer, packets)
	return rawBytes
}

func sendPackets(conn *net.UDPConn, packets *[][]byte, sflowSampleSeq *uint32, sflowSeq *uint32, droppedPackets *uint32) {
	sflowPacketData := sflowPackets(packets, sflowSampleSeq, sflowSeq)
	*packets = (*packets)[:0]

	_, err := conn.Write(sflowPacketData)
	if err != nil {
		if (atomic.LoadUint32(droppedPackets) % 1000) == 0 {
			logging.GetLogger().Criticalf("PCAP2SFlow Agent connection issue : %s", err.Error())
		}
		atomic.AddUint32(droppedPackets, 1)
	}
}

func newUDPConnection(addr string, port int) (*net.UDPConn, error) {
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

func PCAP2SFlowReplay(addr string, port int, file string, pps uint32, ppflow uint32) error {
	var nbPackets, packetsBytes, sflowSampleSeq, sflowSeq, droppedPackets uint32

	if pps < minPPS {
		return fmt.Errorf("Minimal packet per seconds is %d", minPPS)
	}

	conn, err := newUDPConnection(addr, port)
	if err != nil {
		return fmt.Errorf("UDP connection error: %s", err.Error())
	}
	conn.SetWriteBuffer(256 * 1024)

	f, err := os.Open(file)
	if err != nil {
		return fmt.Errorf("PCAP OpenOffline error(\"%s\"): %s", file, err.Error())
	}
	defer f.Close()

	handleRead, err := pcapgo.NewReader(f)
	if err != nil {
		return fmt.Errorf("PCAP OpenOffline error(handle to read packet): %s", err.Error())
	}

	var running atomic.Value
	running.Store(true)

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()

		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		oldNbPackets := atomic.LoadUint32(&nbPackets)
		for running.Load() == true {
			<-ticker.C

			nb := atomic.LoadUint32(&nbPackets)
			dpkts := nb - oldNbPackets
			oldNbPackets = nb
			dropped := atomic.LoadUint32(&droppedPackets)
			logging.GetLogger().Debugf("%d packets replayed, pps %d, nbSFlowMsgDropped %d", nb, dpkts, dropped)
		}
	}()

	throt := throttle{maxHitPerSecond: pps}

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
			atomic.AddUint32(&nbPackets, 1)
			atomic.AddUint32(&packetsBytes, uint32(len(data)))

			dataCopy := make([]byte, len(data))
			copy(dataCopy, data)
			packets = append(packets, dataCopy)

			if (atomic.LoadUint32(&nbPackets) % ppflow) != 0 {
				continue
			}

			throt.startHook(ppflow)
			sendPackets(conn, &packets, &sflowSampleSeq, &sflowSeq, &droppedPackets)
			throt.endHook()
		}
	}
	if len(packets) > 0 {
		sendPackets(conn, &packets, &sflowSampleSeq, &sflowSeq, &droppedPackets)
	}

	running.Store(false)
	wg.Wait()

	logging.GetLogger().Info("PCAP Trace replay finished")

	return nil
}
