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
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

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
	UUID                string
	Addr                string
	Port                int
	FlowTable           *flow.Table
	FlowProbeNodeSetter flow.FlowProbeNodeSetter
}

type SFlowAgentAllocator struct {
	sync.RWMutex
	FlowProbeNodeSetter flow.FlowProbeNodeSetter
	Addr                string
	MinPort             int
	MaxPort             int
	allocated           map[int]*SFlowAgent
}

func (sfa *SFlowAgent) GetTarget() string {
	target := []string{sfa.Addr, strconv.FormatInt(int64(sfa.Port), 10)}
	return strings.Join(target, ":")
}

func (sfa *SFlowAgent) feedFlowTable(conn *net.UDPConn) {
	var buf [maxDgramSize]byte
	_, _, err := conn.ReadFromUDP(buf[:])
	if err != nil {
		conn.SetDeadline(time.Now().Add(1 * time.Second))
		return
	}

	// TODO use gopacket.NoCopy ? instead of gopacket.Default
	p := gopacket.NewPacket(buf[:], layers.LayerTypeSFlow, gopacket.Default)
	sflowLayer := p.Layer(layers.LayerTypeSFlow)
	sflowPacket, ok := sflowLayer.(*layers.SFlowDatagram)
	if !ok {
		return
	}

	if sflowPacket.SampleCount > 0 {
		for _, sample := range sflowPacket.FlowSamples {
			flows := flow.FlowsFromSFlowSample(sfa.FlowTable, &sample, sfa.FlowProbeNodeSetter)
			logging.GetLogger().Debugf("%d flows captured", len(flows))
		}
	}
}

func (sfa *SFlowAgent) start() error {
	addr := net.UDPAddr{
		Port: sfa.Port,
		IP:   net.ParseIP(sfa.Addr),
	}
	conn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		logging.GetLogger().Errorf("Unable to listen on port %d: %s", sfa.Port, err.Error())
		return err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(1 * time.Second))

	feedFlowTable := func() {
		sfa.feedFlowTable(conn)
	}
	sfa.FlowTable.RegisterDefault(feedFlowTable)
	sfa.FlowTable.Start()

	return nil
}

func (sfa *SFlowAgent) Start() {
	go sfa.start()
}

func (sfa *SFlowAgent) Stop() {
	sfa.FlowTable.Stop()
}

func (sfa *SFlowAgent) Flush() {
	logging.GetLogger().Critical("Flush() MUST be called for testing purpose only, not in production")
	sfa.FlowTable.Flush()
}

func (sfa *SFlowAgent) SetFlowProbeNodeSetter(p flow.FlowProbeNodeSetter) {
	sfa.FlowProbeNodeSetter = p
}

func NewSFlowAgent(u string, a string, p int, ft *flow.Table) *SFlowAgent {
	return &SFlowAgent{
		UUID:      u,
		Addr:      a,
		Port:      p,
		FlowTable: ft,
	}
}

func NewSFlowAgentFromConfig(u string, ft *flow.Table) (*SFlowAgent, error) {
	addr, port, err := config.GetHostPortAttributes("sflow", "listen")
	if err != nil {
		return nil, err
	}

	return NewSFlowAgent(u, addr, port, ft), nil
}

func (a *SFlowAgentAllocator) Agents() []*SFlowAgent {
	a.Lock()
	defer a.Unlock()

	agents := make([]*SFlowAgent, 0)

	for _, agent := range a.allocated {
		agents = append(agents, agent)
	}

	return agents
}

func (a *SFlowAgentAllocator) Release(uuid string) {
	a.Lock()
	defer a.Unlock()

	for i, agent := range a.allocated {
		if uuid == agent.UUID {
			agent.Stop()

			delete(a.allocated, i)
		}
	}
}

func (a *SFlowAgentAllocator) ReleaseAll() {
	a.Lock()
	defer a.Unlock()

	for i, agent := range a.allocated {
		agent.Stop()

		delete(a.allocated, i)
	}
}

func (a *SFlowAgentAllocator) Alloc(uuid string, p flow.FlowProbeNodeSetter, ft *flow.Table) (*SFlowAgent, error) {
	address := config.GetConfig().GetString("sflow.bind_address")
	if address == "" {
		address = "127.0.0.1"
	}

	min := config.GetConfig().GetInt("sflow.port_min")
	if min == 0 {
		min = 6345
	}

	max := config.GetConfig().GetInt("sflow.port_max")
	if max == 0 {
		max = 6355
	}

	a.Lock()
	defer a.Unlock()

	// check if there is an already allocated agent for this uuid
	for _, agent := range a.allocated {
		if uuid == agent.UUID {
			return agent, AgentAlreadyAllocated
		}
	}

	for i := min; i != max+1; i++ {
		if _, ok := a.allocated[i]; !ok {
			s := NewSFlowAgent(uuid, address, i, ft)
			s.SetFlowProbeNodeSetter(p)

			a.allocated[i] = s

			s.Start()

			return s, nil
		}
	}

	return nil, errors.New("sflow port exhausted")
}

func NewSFlowAgentAllocator() *SFlowAgentAllocator {
	return &SFlowAgentAllocator{
		allocated: make(map[int]*SFlowAgent),
	}
}
