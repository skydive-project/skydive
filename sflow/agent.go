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

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/logging"
)

const (
	maxDgramSize = 65535
)

var (
	// ErrAgentAlreadyAllocated error agent already allocated for this uuid
	ErrAgentAlreadyAllocated = errors.New("agent already allocated for this uuid")
)

// Agent describes SFlow agent probe
type Agent struct {
	common.RWMutex
	UUID       string
	Addr       string
	Port       int
	FlowTable  *flow.Table
	Conn       *net.UDPConn
	BPFFilter  string
	HeaderSize uint32
}

// AgentAllocator describes an SFlow agent allocator to manage multiple SFlow agent probe
type AgentAllocator struct {
	common.RWMutex
	portAllocator *common.PortAllocator
	agents        []*Agent
}

// GetTarget returns the current used connection
func (sfa *Agent) GetTarget() string {
	target := []string{sfa.Addr, strconv.FormatInt(int64(sfa.Port), 10)}
	return strings.Join(target, ":")
}

func (sfa *Agent) feedFlowTable() {
	var bpf *flow.BPF

	if b, err := flow.NewBPF(layers.LinkTypeEthernet, sfa.HeaderSize, sfa.BPFFilter); err == nil {
		bpf = b
	} else {
		logging.GetLogger().Error(err.Error())
	}

	var buf [maxDgramSize]byte
	for {
		n, _, err := sfa.Conn.ReadFromUDP(buf[:])
		if err != nil {
			return
		}

		// TODO use gopacket.NoCopy ? instead of gopacket.Default
		p := gopacket.NewPacket(buf[:n], layers.LayerTypeSFlow, gopacket.DecodeOptions{NoCopy: true})
		sflowLayer := p.Layer(layers.LayerTypeSFlow)
		sflowPacket, ok := sflowLayer.(*layers.SFlowDatagram)
		if !ok {
			logging.GetLogger().Errorf("Unable to decode sFlow packet: %s", p)
			continue
		}

		if sflowPacket.SampleCount > 0 {
			logging.GetLogger().Debugf("%d sample captured", sflowPacket.SampleCount)
			for _, sample := range sflowPacket.FlowSamples {
				// iterate over a set of Packets as a sample contains multiple
				// records each generating Packets.
				sfa.FlowTable.FeedWithSFlowSample(&sample, bpf)
			}
		}
	}
}

func (sfa *Agent) start() error {
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

	sfa.FlowTable.Start()
	defer sfa.FlowTable.Stop()

	sfa.feedFlowTable()

	return nil
}

// Start the SFlow probe agent
func (sfa *Agent) Start() {
	go sfa.start()
}

// Stop the SFlow probe agent
func (sfa *Agent) Stop() {
	sfa.Lock()
	defer sfa.Unlock()

	if sfa.Conn != nil {
		sfa.Conn.Close()
	}
}

// NewAgent creates a new sFlow agent which will populate the given flowtable
func NewAgent(u string, a *common.ServiceAddress, ft *flow.Table, bpfFilter string, headerSize uint32) *Agent {
	if headerSize == 0 {
		headerSize = flow.DefaultCaptureLength
	}

	return &Agent{
		UUID:       u,
		Addr:       a.Addr,
		Port:       a.Port,
		FlowTable:  ft,
		BPFFilter:  bpfFilter,
		HeaderSize: headerSize,
	}
}

func (a *AgentAllocator) release(uuid string) {
	for i, agent := range a.agents {
		if uuid == agent.UUID {
			agent.Stop()
			a.portAllocator.Release(agent.Port)
			a.agents = append(a.agents[:i], a.agents[i+1:]...)

			break
		}
	}
}

// Release a sFlow agent
func (a *AgentAllocator) Release(uuid string) {
	a.Lock()
	defer a.Unlock()

	a.release(uuid)
}

// ReleaseAll sFlow agents
func (a *AgentAllocator) ReleaseAll() {
	a.Lock()
	defer a.Unlock()

	for _, agent := range a.agents {
		a.release(agent.UUID)
	}
}

// Alloc allocates a new sFlow agent
func (a *AgentAllocator) Alloc(uuid string, ft *flow.Table, bpfFilter string, headerSize uint32, addr *common.ServiceAddress) (agent *Agent, _ error) {
	a.Lock()
	defer a.Unlock()

	// check if there is an already allocated agent for this uuid
	for _, agent := range a.agents {
		if uuid == agent.UUID {
			return agent, ErrAgentAlreadyAllocated
		}
	}

	// get port, if port is not given by user.
	var err error
	if addr.Port <= 0 {
		if addr.Port, err = a.portAllocator.Allocate(); addr.Port <= 0 {
			return nil, errors.New("failed to allocate sflow port: " + err.Error())
		}
	}
	s := NewAgent(uuid, addr, ft, bpfFilter, headerSize)

	a.agents = append(a.agents, s)

	s.Start()
	return s, nil
}

// NewAgentAllocator creates a new sFlow agent allocator
func NewAgentAllocator() (*AgentAllocator, error) {
	min := config.GetInt("sflow.port_min")
	max := config.GetInt("sflow.port_max")

	portAllocator, err := common.NewPortAllocator(min, max)
	if err != nil {
		return nil, err
	}

	return &AgentAllocator{portAllocator: portAllocator}, nil
}
