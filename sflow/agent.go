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
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/graffiti/graph"
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
	Graph      *graph.Graph
	Node       *graph.Node
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
			for _, sample := range sflowPacket.FlowSamples {
				// iterate over a set of Packets as a sample contains multiple
				// records each generating Packets.
				sfa.FlowTable.FeedWithSFlowSample(&sample, bpf)
			}

			// SFlow Counter Samples
			Countersamples := sflowPacket.CounterSamples

			var gen layers.SFlowGenericInterfaceCounters
			var ovsdp layers.SFlowOVSDPCounters
			var app layers.SFlowAppresourcesCounters
			var vlan layers.SFlowVLANCounters

			for _, sample := range sflowPacket.CounterSamples {
				records := sample.GetRecords()

				for _, record := range records {
					switch record.(type) {
					case layers.SFlowGenericInterfaceCounters:
						gen = record.(layers.SFlowGenericInterfaceCounters) //Adding Generic Interface Counters
					case layers.SFlowOVSDPCounters:
						ovsdp = record.(layers.SFlowOVSDPCounters) //Adding OVSDP Counters
					case layers.SFlowAppresourcesCounters:
						app = record.(layers.SFlowAppresourcesCounters) //Adding OVSAppresources Counters
					case layers.SFlowVLANCounters:
						vlan = record.(layers.SFlowVLANCounters) //Adding Vlan Counters
					}
				}
			}

			sfa.Graph.Lock()
			tr := sfa.Graph.StartMetadataTransaction(sfa.Node)

			Uint64ToInt64 := func(key uint64) int64 {
				if key == 18446744073709551615 {
					key = 0
				}
				return int64(float64(key))
			}
			Uint32ToInt64 := func(key uint32) int64 {
				if key == 4294967295 {
					key = 0
				}
				return int64(float64(key))
			}
			currMetric := &SFMetric{
				IfInOctets:         Uint64ToInt64(gen.IfInOctets),
				IfInUcastPkts:      Uint32ToInt64(gen.IfInUcastPkts),
				IfInMulticastPkts:  Uint32ToInt64(gen.IfInMulticastPkts),
				IfInBroadcastPkts:  Uint32ToInt64(gen.IfInBroadcastPkts),
				IfInDiscards:       Uint32ToInt64(gen.IfInDiscards),
				IfInErrors:         Uint32ToInt64(gen.IfInErrors),
				IfInUnknownProtos:  Uint32ToInt64(gen.IfInUnknownProtos),
				IfOutOctets:        Uint64ToInt64(gen.IfOutOctets),
				IfOutUcastPkts:     Uint32ToInt64(gen.IfOutUcastPkts),
				IfOutMulticastPkts: Uint32ToInt64(gen.IfOutMulticastPkts),
				IfOutBroadcastPkts: Uint32ToInt64(gen.IfOutBroadcastPkts),
				IfOutDiscards:      Uint32ToInt64(gen.IfOutDiscards),
				IfOutErrors:        Uint32ToInt64(gen.IfOutErrors),
				OvsdpNHit:          Uint32ToInt64(ovsdp.NHit),
				OvsdpNMissed:       Uint32ToInt64(ovsdp.NMissed),
				OvsdpNLost:         Uint32ToInt64(ovsdp.NLost),
				OvsdpNMaskHit:      Uint32ToInt64(ovsdp.NMaskHit),
				OvsdpNFlows:        Uint32ToInt64(ovsdp.NFlows),
				OvsdpNMasks:        Uint32ToInt64(ovsdp.NMasks),
				OvsAppFdOpen:       Uint32ToInt64(app.FdOpen),
				OvsAppFdMax:        Uint32ToInt64(app.FdMax),
				OvsAppConnOpen:     Uint32ToInt64(app.ConnOpen),
				OvsAppConnMax:      Uint32ToInt64(app.ConnMax),
				OvsAppMemUsed:      Uint64ToInt64(app.MemUsed),
				OvsAppMemMax:       Uint64ToInt64(app.MemMax),
				VlanOctets:         Uint64ToInt64(vlan.Octets),
				VlanUcastPkts:      Uint32ToInt64(vlan.UcastPkts),
				VlanMulticastPkts:  Uint32ToInt64(vlan.MulticastPkts),
				VlanBroadcastPkts:  Uint32ToInt64(vlan.BroadcastPkts),
				VlanDiscards:       Uint32ToInt64(vlan.Discards),
			}
			now := time.Now()

			currMetric.Last = int64(common.UnixMillis(now))

			var prevMetric, lastUpdateMetric, totalMetric *SFMetric

			if metric, err := sfa.Node.GetField("SFlow.Metric"); err == nil {
				prevMetric = metric.(*SFMetric)
				lastUpdateMetric = currMetric
				totalMetric = currMetric.Add(prevMetric).(*SFMetric)
				totalMetric.Last = int64(common.UnixMillis(now))
			} else {
				totalMetric = currMetric
			}

			// nothing changed since last update
			if lastUpdateMetric != nil && !lastUpdateMetric.IsZero() {
				lastUpdateMetric.Start = prevMetric.Last
				lastUpdateMetric.Last = int64(common.UnixMillis(now))
			} else {
				lastUpdateMetric = currMetric
			}

			sfl := &SFlow{
				Counters:         Countersamples,
				Metric:           totalMetric,
				LastUpdateMetric: lastUpdateMetric,
			}

			tr.AddMetadata("SFlow", sfl)
			tr.Commit()
			sfa.Graph.Unlock()
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

	sfa.Graph.DelMetadata(sfa.Node, "SFlow")
	if sfa.Conn != nil {
		sfa.Conn.Close()
	}
}

// NewAgent creates a new sFlow agent which will populate the given flowtable
func NewAgent(u string, a *common.ServiceAddress, ft *flow.Table, bpfFilter string, headerSize uint32, n *graph.Node, g *graph.Graph) *Agent {
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
		Graph:      g,
		Node:       n,
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
func (a *AgentAllocator) Alloc(uuid string, ft *flow.Table, bpfFilter string, headerSize uint32, addr *common.ServiceAddress, n *graph.Node, g *graph.Graph) (agent *Agent, _ error) {
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
	s := NewAgent(uuid, addr, ft, bpfFilter, headerSize, n, g)

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
