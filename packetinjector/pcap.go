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

package packetinjector

import (
	"bytes"
	"io"

	"github.com/google/gopacket/pcapgo"
	"github.com/skydive-project/skydive/logging"
)

// PcapPacketGenerator reads packets from a pcap file and inject it into a channel
type PcapPacketGenerator struct {
	pcapReader *pcapgo.Reader
}

// PacketSource returns a channel when pcap packets are pushed
func (p *PcapPacketGenerator) PacketSource() chan *Packet {
	ch := make(chan *Packet)

	go func() {
		for {
			data, _, err := p.pcapReader.ReadPacketData()
			if err != nil && err != io.EOF {
				logging.GetLogger().Warningf("Failed to read packet: %s\n", err)
				return
			}
			logging.GetLogger().Debugf("Read %d bytes of pcap", len(data))
			if len(data) == 0 {
				close(ch)
				return
			}
			ch <- &Packet{data: data}
		}
	}()

	return ch
}

// NewPcapPacketGenerator returns a new pcap packet generator
func NewPcapPacketGenerator(pcap []byte) (*PcapPacketGenerator, error) {
	buffer := bytes.NewReader(pcap)
	pcapReader, err := pcapgo.NewReader(buffer)
	if err != nil {
		return nil, err
	}
	return &PcapPacketGenerator{pcapReader: pcapReader}, nil
}
