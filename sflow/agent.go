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

package sflow

import (
	"errors"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/logging"
)

const (
	maxDgramSize = 1500
)

var (
	AgentAlreadyAllocated error = errors.New("agent already allocated for this uuid")
)

type SFlowAgent struct {
	sync.RWMutex
	UUID      string
	Addr      string
	Port      int
	FlowTable *flow.Table
	Conn      *net.UDPConn
	BPFFilter string
}

type SFlowAgentAllocator struct {
	sync.RWMutex
	portAllocator *common.PortAllocator
	Addr          string
}

func (sfa *SFlowAgent) GetTarget() string {
	target := []string{sfa.Addr, strconv.FormatInt(int64(sfa.Port), 10)}
	return strings.Join(target, ":")
}

func (sfa *SFlowAgent) feedFlowTable(packetsChan chan *flow.FlowPackets) {
	var bpf *flow.BPF

	if b, err := flow.NewBPF(layers.LinkTypeEthernet, flow.CaptureLength, sfa.BPFFilter); err == nil {
		bpf = b
	} else {
		logging.GetLogger().Error(err.Error())
	}

	var buf [maxDgramSize]byte
	for {
		_, _, err := sfa.Conn.ReadFromUDP(buf[:])
		if err != nil {
			return
		}

		// TODO use gopacket.NoCopy ? instead of gopacket.Default
		p := gopacket.NewPacket(buf[:], layers.LayerTypeSFlow, gopacket.Default)
		sflowLayer := p.Layer(layers.LayerTypeSFlow)
		sflowPacket, ok := sflowLayer.(*layers.SFlowDatagram)
		if !ok {
			continue
		}

		if sflowPacket.SampleCount > 0 {
			logging.GetLogger().Debugf("%d sample captured", sflowPacket.SampleCount)
			for _, sample := range sflowPacket.FlowSamples {
				// iterate over a set of FlowPackets as a sample contains multiple
				// records each generating FlowPackets.
				for _, flowPackets := range flow.FlowPacketsFromSFlowSample(&sample, -1, bpf) {
					packetsChan <- flowPackets
				}
			}
		}
	}
}

func (sfa *SFlowAgent) start() error {
	sfa.Lock()
	addr := net.UDPAddr{
		Port: sfa.Port,
		IP:   net.ParseIP(sfa.Addr),
	}
	conn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		logging.GetLogger().Errorf("Unable to listen on port %d: %s", sfa.Port, err.Error())
		sfa.Unlock()
		return err
	}
	sfa.Conn = conn
	sfa.Unlock()

	packetsChan := sfa.FlowTable.Start()
	defer sfa.FlowTable.Stop()

	sfa.feedFlowTable(packetsChan)

	return nil
}

func (sfa *SFlowAgent) Start() {
	go sfa.start()
}

func (sfa *SFlowAgent) Stop() {
	sfa.Lock()
	defer sfa.Unlock()

	if sfa.Conn != nil {
		sfa.Conn.Close()
	}
}

func NewSFlowAgent(u string, a string, p int, ft *flow.Table, bpfFilter string) *SFlowAgent {
	return &SFlowAgent{
		UUID:      u,
		Addr:      a,
		Port:      p,
		FlowTable: ft,
		BPFFilter: bpfFilter,
	}
}

func (a *SFlowAgentAllocator) Release(uuid string) {
	a.Lock()
	defer a.Unlock()

	for i, obj := range a.portAllocator.PortMap {
		agent := obj.(*SFlowAgent)
		if uuid == agent.UUID {
			agent.Stop()
			a.portAllocator.Release(i)
		}
	}
}

func (a *SFlowAgentAllocator) ReleaseAll() {
	a.Lock()
	for _, agent := range a.portAllocator.PortMap {
		agent.(*SFlowAgent).Stop()
	}
	defer a.Unlock()

	a.portAllocator.ReleaseAll()
}

func (a *SFlowAgentAllocator) Alloc(uuid string, ft *flow.Table, bpfFilter string) (agent *SFlowAgent, _ error) {
	address := config.GetConfig().GetString("sflow.bind_address")
	if address == "" {
		address = "127.0.0.1"
	}

	a.Lock()
	defer a.Unlock()

	// check if there is an already allocated agent for this uuid
	a.portAllocator.RLock()
	for _, obj := range a.portAllocator.PortMap {
		if uuid == obj.(*SFlowAgent).UUID {
			agent = obj.(*SFlowAgent)
		}
	}
	a.portAllocator.RUnlock()
	if agent != nil {
		return agent, AgentAlreadyAllocated
	}

	port, err := a.portAllocator.Allocate()
	if port <= 0 {
		return nil, errors.New("failed to allocate sflow port: " + err.Error())
	}

	s := NewSFlowAgent(uuid, address, port, ft, bpfFilter)
	a.portAllocator.Set(port, s)
	s.Start()
	return s, nil
}

func NewSFlowAgentAllocator() (*SFlowAgentAllocator, error) {
	min := config.GetConfig().GetInt("sflow.port_min")
	max := config.GetConfig().GetInt("sflow.port_max")

	portAllocator, err := common.NewPortAllocator(min, max)
	if err != nil {
		return nil, err
	}

	return &SFlowAgentAllocator{portAllocator: portAllocator}, nil
}
