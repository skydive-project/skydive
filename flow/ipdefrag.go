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

package flow

import (
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/ip4defrag"
	"github.com/google/gopacket/layers"
	"github.com/skydive-project/skydive/common"
)

type ipv4Key struct {
	ipv4 gopacket.Flow
	id   uint16
}

// IPDefraggerMetric defines the structure keeping metrics of fragments
type IPDefraggerMetric struct {
	metric *IPMetric
	last   time.Time
}

// IPDefragger defines an IPv4 defragmenter
type IPDefragger struct {
	common.RWMutex
	defragger *ip4defrag.IPv4Defragmenter
	dfmetrics map[ipv4Key]*IPDefraggerMetric
}

func newIPv4(ip *layers.IPv4) ipv4Key {
	return ipv4Key{
		ipv4: ip.NetworkFlow(),
		id:   ip.Id,
	}
}

// NewIPDefragger returns a new IPv4 defragger
func NewIPDefragger() *IPDefragger {
	return &IPDefragger{
		defragger: ip4defrag.NewIPv4Defragmenter(),
		dfmetrics: make(map[ipv4Key]*IPDefraggerMetric, 0),
	}
}

// Defrag tries to defragment
func (d *IPDefragger) Defrag(packet gopacket.Packet) (*IPMetric, bool) {
	ipv4Layer := packet.Layer(layers.LayerTypeIPv4)
	if ipv4Packet, ok := ipv4Layer.(*layers.IPv4); ok {
		key := newIPv4(ipv4Packet)

		d.Lock()
		dfm := d.dfmetrics[key]
		if dfm == nil {
			dfm = &IPDefraggerMetric{metric: &IPMetric{}}
			d.dfmetrics[key] = dfm
		}
		d.Unlock()

		t := packet.Metadata().Timestamp

		new, err := d.defragger.DefragIPv4WithTimestamp(ipv4Packet, t)
		if err != nil {
			dfm.metric.FragmentErrors++
			return dfm.metric, false
		} else if new == nil {
			dfm.metric.Fragments++
			return dfm.metric, false
		}

		if new != ipv4Packet {
			nextDecoder := new.NextLayerType()
			nextDecoder.Decode(new.Payload, packet.(gopacket.PacketBuilder))
		}

		d.Lock()
		delete(d.dfmetrics, key)
		d.Unlock()

		return dfm.metric, true
	}
	return nil, true
}

// FlushOlderThan frees resources for fragment older than the give time
func (d *IPDefragger) FlushOlderThan(t time.Time) {
	d.Lock()
	defer d.Unlock()
	for k, v := range d.dfmetrics {
		if v.last.Before(t) {
			delete(d.dfmetrics, k)
		}
	}
}

// FlushAll frees all the fragment resources
func (d *IPDefragger) FlushAll() {
	d.Lock()
	defer d.Unlock()
	for k := range d.dfmetrics {
		delete(d.dfmetrics, k)
	}
}
