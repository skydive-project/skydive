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
	"time"

	"github.com/google/gopacket/pcapgo"
	"github.com/skydive-project/skydive/graffiti/logging"
)

// PcapPacketGenerator reads packets from a pcap file and inject it into a channel
type PcapPacketGenerator struct {
	*PacketInjectionRequest
	close chan bool
}

// Close the packet generator
func (p *PcapPacketGenerator) Close() {
	p.close <- true
}

func packetNumberFromPcap(b []byte) (uint64, error) {
	buffer := bytes.NewReader(b)
	pcapReader, err := pcapgo.NewReader(buffer)
	if err != nil {
		return 0, err
	}

	var count uint64

	for {
		data, _, _ := pcapReader.ReadPacketData()
		if len(data) == 0 {
			break
		}
		count++
	}
	return count, nil
}

// PacketSource returns a channel when pcap packets are pushed
func (p *PcapPacketGenerator) PacketSource() chan *Packet {
	ch := make(chan *Packet)

	go func() {
		defer close(ch)

		var pcapReader *pcapgo.Reader
		var err error

		if p.Count == 0 {
			if p.Count, err = packetNumberFromPcap(p.Pcap); err != nil {
				logging.GetLogger().Errorf("Error while reading pcap file: %s", err)
				return
			}
		}

		for i := uint64(0); i < p.Count; i++ {
			if pcapReader == nil {
				buffer := bytes.NewReader(p.Pcap)
				if pcapReader, err = pcapgo.NewReader(buffer); err != nil {
					logging.GetLogger().Errorf("Error while reading pcap file: %s", err)
					return
				}
			}

			data, _, err := pcapReader.ReadPacketData()
			if err != nil && err != io.EOF {
				logging.GetLogger().Warningf("Failed to read packet: %s", err)
				return
			}
			logging.GetLogger().Debugf("Read %d bytes of pcap", len(data))
			if len(data) == 0 {
				pcapReader = nil
				continue
			}
			select {
			case ch <- &Packet{data: data}:
			case <-p.close:
				return
			}

			if i != p.Count-1 && p.Interval != 0 {
				select {
				case <-p.close:
					return
				case <-time.After(time.Millisecond * time.Duration(p.Interval)):
				}
			}
		}
	}()

	return ch
}

// NewPcapPacketGenerator returns a new pcap packet generator
func NewPcapPacketGenerator(pp *PacketInjectionRequest) (*PcapPacketGenerator, error) {
	return &PcapPacketGenerator{
		PacketInjectionRequest: pp,
		close: make(chan bool, 1),
	}, nil
}
