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

package probes

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"

	"github.com/pmylund/go-cache"

	"github.com/redhat-cip/skydive/analyzer"
	"github.com/redhat-cip/skydive/config"
	"github.com/redhat-cip/skydive/flow"
	"github.com/redhat-cip/skydive/flow/mappings"
	"github.com/redhat-cip/skydive/logging"
	"github.com/redhat-cip/skydive/topology/graph"
)

type PcapProbe struct {
	Filename string

	Graph               *graph.Graph
	AnalyzerClient      *analyzer.Client
	FlowMappingPipeline *mappings.FlowMappingPipeline

	cache            *cache.Cache
	cacheUpdaterChan chan uint32
	flowtable        *flow.FlowTable
}

func (probe *PcapProbe) GetTarget() string {
	return "PcapProbe:1234"
}

func (probe *PcapProbe) cacheUpdater() {
	logging.GetLogger().Debug("Start PcapProbe cache updater")

	var index uint32
	for {
		index = <-probe.cacheUpdaterChan

		logging.GetLogger().Debug("PcapProbe request received: %d", index)

		probe.Graph.Lock()

		intfs := probe.Graph.LookupNodes(graph.Metadatas{"IfIndex": index})

		// lookup for the interface that is a part of an ovs bridge
		for _, intf := range intfs {
			ancestors, ok := probe.Graph.GetAncestorsTo(intf, graph.Metadatas{"Type": "ovsbridge"})
			if ok {
				bridge := ancestors[2]
				ancestors, ok = probe.Graph.GetAncestorsTo(bridge, graph.Metadatas{"Type": "host"})

				var path string
				for i := len(ancestors) - 1; i >= 0; i-- {
					if len(path) > 0 {
						path += "/"
					}
					path += ancestors[i].Metadatas()["Name"].(string)
				}
				probe.cache.Set(strconv.FormatUint(uint64(index), 10), path, cache.DefaultExpiration)
				break
			}
		}
		probe.Graph.Unlock()
	}
}

func (probe *PcapProbe) getProbePath(index uint32) *string {
	p, f := probe.cache.Get(strconv.FormatUint(uint64(index), 10))
	if f {
		path := p.(string)
		return &path
	}

	probe.cacheUpdaterChan <- index

	return nil
}

func (probe *PcapProbe) flowExpire(f *flow.Flow) {
	/* send a special event to the analyzer */
}

var nbpackets int = 0
var nbsflowmsg int = 0
var sflowSeq uint32 = 0

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
	// Add padding
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

func (probe *PcapProbe) sflowPackets(packets *[][]byte) []byte {
	nbsflowmsg++
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

	sflowSeq++

	sf := layers.SFlowFlowSample{
		EnterpriseID: layers.SFlowStandard,
		Format:       layers.SFlowTypeFlowSample,
		//	SampleLength    uint32
		SequenceNumber:  uint32(sflowSeq),
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

	sflowLayer := &layers.SFlowDatagram{
		DatagramVersion: 5,
		AgentAddress:    net.IP{127, 0, 0, 3},
		SubAgentID:      0,
		SequenceNumber:  sflowSeq,
		AgentUptime:     2294190,
		//		SampleCount:     count,
		//		FlowSamples:     sflowsamples,
		//				CounterSamples  []SFlowCounterSample
	}
	sflowLayer.FlowSamples = append(sflowLayer.FlowSamples, sf)
	sflowLayer.SampleCount = uint32(len(sflowLayer.FlowSamples))

	rawBytes := SFlowDatagramSerialize(sflowLayer, packets)
	return rawBytes
}

func genEthIPUdp(payload []byte) []byte {
	ethernetLayer := &layers.Ethernet{
		SrcMAC: net.HardwareAddr{0x00, 0x01, 0xFF, 0xAA, 0xFA, 0xAA},
		DstMAC: net.HardwareAddr{0x00, 0x01, 0xBD, 0xBD, 0xBD, 0xBD},
	}
	ethernetLayer.EthernetType = layers.EthernetTypeIPv4
	ipLayer := &layers.IPv4{
		SrcIP: net.IP{127, 0, 0, 1},
		DstIP: net.IP{127, 0, 0, 2},
	}
	ipLayer.IHL = 0x45
	ipLayer.Protocol = layers.IPProtocolUDP
	udpLayer := &layers.UDP{
		SrcPort: layers.UDPPort(54321),
		DstPort: layers.UDPPort(6343),
	}
	udpLayer.Length = uint16(8 + len(payload))
	ipLayer.Length = 20 + udpLayer.Length
	ipLayer.TTL = 64
	ipLayer.Id = uint16(0xbbea)

	// And create the packet with the layers
	buffer := gopacket.NewSerializeBuffer()
	options := gopacket.SerializeOptions{}
	gopacket.SerializeLayers(buffer, options,
		ethernetLayer,
		ipLayer,
		udpLayer,
		gopacket.Payload(payload),
	)

	return buffer.Bytes()
}

func writePcap(packet []byte) {
	logging.GetLogger().Debug("Writing PCAP file")
	f, _ := os.Create("/tmp/file.pcap")
	w := pcapgo.NewWriter(f)
	w.WriteFileHeader(65536, layers.LinkTypeEthernet) // new file, must do this.
	w.WritePacket(
		gopacket.CaptureInfo{Timestamp: time.Now(),
			CaptureLength: len(packet),
			Length:        len(packet),
		}, packet)
	f.Close()
}

func (probe *PcapProbe) AsyncProgressInfo() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			logging.GetLogger().Debug("%d packets replayed | %s", nbpackets, probe.flowtable.String())
		}
	}
}

func (probe *PcapProbe) Start() error {
	f, err := os.Open(probe.Filename)
	if err != nil {
		logging.GetLogger().Fatal("PCAP OpenOffline error (", probe.Filename, ")", err)
	}
	handleRead, err := pcapgo.NewReader(f)
	if err != nil {
		logging.GetLogger().Fatal("PCAP OpenOffline error (handle to read packet)", err)
	}
	defer f.Close()

	// start index/mac cache updater
	go probe.cacheUpdater()

	go probe.flowtable.AsyncExpire(probe.flowExpire, 5*time.Minute)
	go probe.AnalyzerClient.AsyncFlowsUpdate(probe.flowtable, 30*time.Second)

	go probe.AsyncProgressInfo()

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
			nbpackets++
			dataCopy := make([]byte, len(data))
			copy(dataCopy, data)
			packets = append(packets, dataCopy)

			if (nbpackets % 5) != 0 {
				continue
			}

			sflowPacketData := probe.sflowPackets(&packets)
			packets = packets[:0]

			p := gopacket.NewPacket(sflowPacketData[:], layers.LayerTypeSFlow, gopacket.Default)
			sflowLayer := p.Layer(layers.LayerTypeSFlow)
			sflowPacket, ok := sflowLayer.(*layers.SFlowDatagram)
			if !ok {
				logging.GetLogger().Critical("Can't cast gopacket as a SFlowDatagram", p)
				continue
			}

			if sflowPacket.SampleCount > 0 {
				for _, sample := range sflowPacket.FlowSamples {
					replayIntf := "replay0"
					flows := flow.FLowsFromSFlowSample(probe.flowtable, &sample, &replayIntf) // probe.getProbePath(sample.InputInterface))
					logging.GetLogger().Debug("%d flows captured", len(flows))

					if probe.FlowMappingPipeline != nil {
						probe.FlowMappingPipeline.Enhance(flows)
					}
				}
			}
		}
	}

	logging.GetLogger().Info("PCAP Trace replay is finished, press Return to quit")
	var b []byte = make([]byte, 1)
	os.Stdin.Read(b)
	os.Exit(0)

	return nil
}

func (probe *PcapProbe) SetAnalyzerClient(a *analyzer.Client) {
	probe.AnalyzerClient = a
}

func (probe *PcapProbe) SetMappingPipeline(p *mappings.FlowMappingPipeline) {
	probe.FlowMappingPipeline = p
}

func NewPcapProbe(pcapfilename string, g *graph.Graph) (*PcapProbe, error) {
	probe := &PcapProbe{
		Filename: pcapfilename,
		Graph:    g,
	}

	if probe.Filename == "" {
		probe.Filename = config.GetConfig().Section("agent").Key("pcaptrace").String()
	}

	expire, err := config.GetConfig().Section("cache").Key("expire").Int()
	if err != nil {
		return nil, err
	}
	cleanup, err := config.GetConfig().Section("cache").Key("cleanup").Int()
	if err != nil {
		return nil, err
	}

	probe.cache = cache.New(time.Duration(expire)*time.Second, time.Duration(cleanup)*time.Second)
	probe.cacheUpdaterChan = make(chan uint32, 200)

	probe.flowtable = flow.NewFlowTable()

	return probe, nil
}
