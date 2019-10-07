// +build linux

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

package gopacket

import (
	"fmt"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/afpacket"
	"github.com/google/gopacket/layers"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/flow/probes"
	"golang.org/x/net/bpf"
)

// AFPacketHandle describes a AF network kernel packets
type AFPacketHandle struct {
	tpacket *afpacket.TPacket
}

// ReadPacketData reads one packet
func (h *AFPacketHandle) ReadPacketData() ([]byte, gopacket.CaptureInfo, error) {
	return h.tpacket.ReadPacketData()
}

// TPacket returns the afpacket TPacket instance
func (h *AFPacketHandle) TPacket() *afpacket.TPacket {
	return h.tpacket
}

// Close the AF packet handle
func (h *AFPacketHandle) Close() {
	h.tpacket.Close()
}

// NewAFPacketHandle creates a new network AF packet probe
func NewAFPacketHandle(ifName string, snaplen int32) (*AFPacketHandle, error) {
	tpacket, err := afpacket.NewTPacket(
		afpacket.OptInterface(ifName),
		afpacket.OptFrameSize(snaplen),
		afpacket.OptPollTimeout(1*time.Second),
		afpacket.OptAddVLANHeader(true),
	)

	if err != nil {
		return nil, err
	}

	return &AFPacketHandle{tpacket: tpacket}, err
}

// AfpacketPacketProbe describes an afpacket based packet probe
type AfpacketPacketProbe struct {
	handle       *AFPacketHandle
	packetSource *gopacket.PacketSource
	layerType    gopacket.LayerType
	linkType     layers.LinkType
	headerSize   uint32
}

// Close the probe
func (a *AfpacketPacketProbe) Close() {
	a.handle.Close()
}

// Stats returns statistics about captured packets
func (a *AfpacketPacketProbe) Stats() (*probes.CaptureStats, error) {
	_, v3, e := a.handle.tpacket.SocketStats()
	if e != nil {
		return nil, fmt.Errorf("Cannot get afpacket capture stats")
	}
	return &probes.CaptureStats{
		PacketsReceived: int64(v3.Packets()),
		PacketsDropped:  int64(v3.Drops()),
	}, nil
}

// SetBPFFilter applies a BPF filter to the probe
func (a *AfpacketPacketProbe) SetBPFFilter(filter string) error {
	var rawBPF []bpf.RawInstruction
	rawBPF, err := flow.BPFFilterToRaw(a.linkType, a.headerSize, filter)
	if err != nil {
		return err
	}
	return a.handle.tpacket.SetBPF(rawBPF)
}

// PacketSource returns the Gopacket packet source for the probe
func (a *AfpacketPacketProbe) PacketSource() *gopacket.PacketSource {
	return a.packetSource
}

// NewAfpacketPacketProbe returns a new afpacket capture probe
func NewAfpacketPacketProbe(ifName string, headerSize int, layerType gopacket.LayerType, linkType layers.LinkType) (*AfpacketPacketProbe, error) {
	handle, err := NewAFPacketHandle(ifName, int32(headerSize))
	if err != nil {
		return nil, fmt.Errorf("Error while opening device %s: %s", ifName, err)
	}

	return &AfpacketPacketProbe{
		handle:       handle,
		packetSource: gopacket.NewPacketSource(handle, layerType),
		layerType:    layerType,
		linkType:     linkType,
		headerSize:   uint32(headerSize),
	}, nil
}
