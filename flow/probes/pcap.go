// +build linux

/*
 * Copyright (C) 2018 Red Hat, Inc.
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
	"fmt"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
	"github.com/skydive-project/skydive/topology/graph"
)

// PcapPacketProbe describes a libpcap based packet probe
type PcapPacketProbe struct {
	handle       *pcap.Handle
	packetSource *gopacket.PacketSource
}

// Close the probe
func (p *PcapPacketProbe) Close() {
	p.handle.Close()
}

// Stats returns statistics about captured packets
func (p *PcapPacketProbe) Stats() (graph.Metadata, error) {
	stats, err := p.handle.Stats()
	if err != nil {
		return nil, err
	}
	return graph.Metadata{
		"PacketsReceived":  stats.PacketsReceived,
		"PacketsDropped":   stats.PacketsDropped,
		"PacketsIfDropped": stats.PacketsIfDropped,
	}, nil
}

// SetBPFFilter applies a BPF filter to the probe
func (p *PcapPacketProbe) SetBPFFilter(bpf string) error {
	return p.handle.SetBPFFilter(bpf)
}

// PacketSource returns the Gopacket packet source for the probe
func (p *PcapPacketProbe) PacketSource() *gopacket.PacketSource {
	return p.packetSource
}

// NewPcapPacketProbe returns a new libpcap capture probe
func NewPcapPacketProbe(ifName string, headerSize int) (*PcapPacketProbe, error) {
	handle, err := pcap.OpenLive(ifName, int32(headerSize), true, time.Second)
	if err != nil {
		return nil, fmt.Errorf("Error while opening device %s: %s", ifName, err)
	}

	return &PcapPacketProbe{
		handle:       handle,
		packetSource: gopacket.NewPacketSource(handle, handle.LinkType()),
	}, nil
}
