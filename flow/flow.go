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

package flow

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"github.com/nu7hatch/gouuid"

	"github.com/redhat-cip/skydive/logging"
)

func LayerFlow(l gopacket.Layer) gopacket.Flow {
	switch l.(type) {
	case gopacket.LinkLayer:
		return l.(gopacket.LinkLayer).LinkFlow()
	case gopacket.NetworkLayer:
		return l.(gopacket.NetworkLayer).NetworkFlow()
	case gopacket.TransportLayer:
		return l.(gopacket.TransportLayer).TransportFlow()
	}
	panic("Unknown gopacket.Layer " + reflect.TypeOf(l).String())
}

type FlowKey struct {
	net, transport uint64
}

func (key FlowKey) fillFromGoPacket(p *gopacket.Packet) FlowKey {
	key.net = LayerFlow((*p).NetworkLayer()).FastHash()
	key.transport = LayerFlow((*p).TransportLayer()).FastHash()
	return key
}

func (key FlowKey) String() string {
	return fmt.Sprint("%x-%x", key.net, key.transport)
}

var flowTable = make(map[FlowKey]*Flow)

func AsyncFlowTableUpdate() {
	ticker := time.NewTicker(5 * time.Minute)
	defer func() {
		ticker.Stop()
	}()
	for {
		select {
		case now := <-ticker.C:
			flowTableSzBefore := len(flowTable)
			expire := now.Unix() - int64((5 * time.Minute).Seconds())

			for key, f := range flowTable {
				fs := f.GetStatistics()
				if fs.Last < expire {
					duration := time.Duration(fs.Last - fs.Start)
					logging.GetLogger().Debug("%v Expire flow %s Duration %v", now, f.UUID, duration)
					/* send a special event to the analyzer */
					delete(flowTable, key)
				}
			}
			logging.GetLogger().Debug("%v Expire flow table size, removed %v now %v", now, flowTableSzBefore-len(flowTable), len(flowTable))
		}
	}
}

func (flow *Flow) fillFromGoPacket(packet *gopacket.Packet) error {
	/* Continue if no ethernet layer */
	ethernetLayer := (*packet).Layer(layers.LayerTypeEthernet)
	_, ok := ethernetLayer.(*layers.Ethernet)
	if !ok {
		return errors.New("Unable to decode the ethernet layer")
	}

	/* FlowTable */
	key := (FlowKey{}).fillFromGoPacket(packet)
	f, found := flowTable[key]
	if found == false {
		flowTable[key] = flow
	} else if flow.UUID != f.UUID {
		return errors.New(fmt.Sprint("FlowTable key (%s) Collision on flow.UUID (%s) and f.UUID (%s)", key.String(), flow.UUID, f.UUID))
	}

	fs := f.GetStatistics()
	if found == false {
		fs.Start = (*packet).Metadata().Timestamp.Unix()
		fs.newEthernetEndpointStatistics(packet)
		fs.newIPV4EndpointStatistics(packet)
		fs.newTransportEndpointStatistics(packet)
	}
	fs.Last = (*packet).Metadata().Timestamp.Unix()
	fs.updateEthernetFromGoPacket(packet)
	fs.updateIPV4FromGoPacket(packet)
	fs.updateTransportFromGoPacket(packet)

	hasher := sha1.New()

	path := ""
	for i, layer := range (*packet).Layers() {
		if i > 0 {
			path += "/"
		}
		path += layer.LayerType().String()
	}
	flow.LayersPath = path
	hasher.Write([]byte(flow.LayersPath))

	flow.DebugKeyNet = key.net
	flow.DebugKeyTransport = key.transport

	/* Generate an flow UUID */
	for _, ep := range fs.GetEndpoints() {
		hasher.Write([]byte(ep.AB.Value))
		hasher.Write([]byte(ep.BA.Value))
	}
	flow.UUID = hex.EncodeToString(hasher.Sum(nil))
	return nil
}

func FromData(data []byte) (*Flow, error) {
	flow := new(Flow)

	err := proto.Unmarshal(data, flow)
	if err != nil {
		return nil, err
	}

	return flow, nil
}

func (flow *Flow) GetData() ([]byte, error) {
	data, err := proto.Marshal(flow)
	if err != nil {
		return []byte{}, err
	}

	return data, nil
}

func New(packet *gopacket.Packet, probePath *string) *Flow {
	u, _ := uuid.NewV4()
	t := time.Now().Unix()

	flow := &Flow{
		UUID:           u.String(),
		DebugTimestamp: t,
		ProbeGraphPath: *probePath,
	}

	if packet != nil {
		flow.fillFromGoPacket(packet)
	}

	return flow
}

func FLowsFromSFlowSample(sample *layers.SFlowFlowSample, probePath *string) []*Flow {
	flows := []*Flow{}

	for _, rec := range sample.Records {

		/* FIX(safchain): just keeping the raw packet for now */
		record, ok := rec.(layers.SFlowRawPacketFlowRecord)
		if !ok {
			continue
		}

		flow := New(&record.Header, probePath)
		flows = append(flows, flow)
	}

	return flows
}
