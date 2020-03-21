/*
 * Copyright (C) 2015 Red Hat, Inc.
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

package sflow

import (
	"errors"
	"fmt"
	"math"
	"net"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/safchain/insanelock"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/service"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/portallocator"
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
	insanelock.RWMutex
	UUID       string
	FlowTable  *flow.Table
	Conn       *net.UDPConn
	Addr       string
	Port       int
	BPFFilter  string
	HeaderSize uint32
	Graph      *graph.Graph
	Node       *graph.Node
}

// AgentAllocator describes an SFlow agent allocator to manage multiple SFlow agent probe
type AgentAllocator struct {
	insanelock.RWMutex
	portAllocator *portallocator.PortAllocator
	agents        []*Agent
}

// GetTarget returns the current used connection
func (sfa *Agent) GetTarget() string {
	return fmt.Sprintf("%s:%d", sfa.Addr, sfa.Port)
}

func (sfa *Agent) feedFlowTable() {
	var bpf *flow.BPF

	if b, err := flow.NewBPF(layers.LinkTypeEthernet, sfa.HeaderSize, sfa.BPFFilter); err == nil {
		bpf = b
	} else {
		logging.GetLogger().Error(err)
	}

	defer func() {
		sfa.Graph.Lock()
		sfa.Graph.DelMetadata(sfa.Node, "SFlow")
		sfa.Graph.Unlock()
	}()

	var buf [maxDgramSize]byte
	for {
		n, ra, err := sfa.Conn.ReadFromUDP(buf[:])
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

		for _, sample := range sflowPacket.FlowSamples {
			// iterate over a set of Packets as a sample contains multiple
			// records each generating Packets.
			sfa.FlowTable.FeedWithSFlowSample(&sample, bpf)
		}

		sfa.Graph.Lock()

		var prevMetric *SFMetric

		var sf *SFlow
		if s, err := sfa.Node.GetField("SFlow"); err == nil {
			sf = s.(*SFlow)
			prevMetric = sf.Metric.Sub(&SFMetric{}).(*SFMetric)

			// copy to not update the one that is in metadata
			// otherwise the AddMetadata won't send the notification
			sf = &SFlow{
				IfMetrics:        sf.IfMetrics,
				Metric:           sf.Metric,
				LastUpdateMetric: sf.LastUpdateMetric,
			}
		} else {
			prevMetric = &SFMetric{}
			sf = &SFlow{
				IfMetrics: make(map[int64]*IfMetric),
				Metric:    &SFMetric{},
			}
		}

		maxUint64 := func(n uint64, o int64) int64 {
			if n == 0 || n == math.MaxUint64 {
				return o
			}
			return int64(n)
		}
		maxUint32 := func(n uint32, o int64) int64 {
			if n == 0 || n == math.MaxUint32 {
				return o
			}
			return int64(n)
		}

		for _, sample := range sflowPacket.CounterSamples {
			for _, record := range sample.GetRecords() {
				switch record.(type) {
				case layers.SFlowGenericInterfaceCounters:
					gen := record.(layers.SFlowGenericInterfaceCounters)

					ifIndex := int64(gen.IfIndex)

					ifMetric, ok := sf.IfMetrics[ifIndex]
					if !ok {
						ifMetric = &IfMetric{}
						sf.IfMetrics[ifIndex] = ifMetric
					}

					ifMetric.IfInOctets = maxUint64(gen.IfInOctets, ifMetric.IfInOctets)
					ifMetric.IfInUcastPkts = maxUint32(gen.IfInUcastPkts, ifMetric.IfInUcastPkts)
					ifMetric.IfInMulticastPkts = maxUint32(gen.IfInMulticastPkts, ifMetric.IfInMulticastPkts)
					ifMetric.IfInBroadcastPkts = maxUint32(gen.IfInBroadcastPkts, ifMetric.IfInBroadcastPkts)
					ifMetric.IfInDiscards = maxUint32(gen.IfInDiscards, ifMetric.IfInDiscards)
					ifMetric.IfInErrors = maxUint32(gen.IfInErrors, ifMetric.IfInErrors)
					ifMetric.IfInUnknownProtos = maxUint32(gen.IfInUnknownProtos, ifMetric.IfInUnknownProtos)
					ifMetric.IfOutOctets = maxUint64(gen.IfOutOctets, ifMetric.IfOutOctets)
					ifMetric.IfOutUcastPkts = maxUint32(gen.IfOutUcastPkts, ifMetric.IfOutUcastPkts)
					ifMetric.IfOutMulticastPkts = maxUint32(gen.IfOutMulticastPkts, ifMetric.IfOutMulticastPkts)
					ifMetric.IfOutBroadcastPkts = maxUint32(gen.IfOutBroadcastPkts, ifMetric.IfOutBroadcastPkts)
					ifMetric.IfOutDiscards = maxUint32(gen.IfOutDiscards, ifMetric.IfOutDiscards)
					ifMetric.IfOutErrors = maxUint32(gen.IfOutErrors, ifMetric.IfOutErrors)

				case layers.SFlowOVSDPCounters:
					// add for those that are gauge and not counters
					ovsdp := record.(layers.SFlowOVSDPCounters)
					sf.Metric.OvsDpNHit += maxUint32(ovsdp.NHit, prevMetric.OvsDpNHit)
					sf.Metric.OvsDpNMissed += maxUint32(ovsdp.NMissed, prevMetric.OvsDpNMissed)
					sf.Metric.OvsDpNLost += maxUint32(ovsdp.NLost, prevMetric.OvsDpNLost)
					sf.Metric.OvsDpNMaskHit += maxUint32(ovsdp.NMaskHit, prevMetric.OvsDpNMaskHit)
					sf.Metric.OvsDpNFlows += maxUint32(ovsdp.NFlows, prevMetric.OvsDpNFlows)
					sf.Metric.OvsDpNMasks += maxUint32(ovsdp.NMasks, prevMetric.OvsDpNMasks)

				case layers.SFlowAppresourcesCounters:
					app := record.(layers.SFlowAppresourcesCounters)
					// add for those that are gauge and not counters
					sf.Metric.OvsAppFdOpen += maxUint32(app.FdOpen, prevMetric.OvsAppFdOpen)
					sf.Metric.OvsAppFdMax = maxUint32(app.FdMax, prevMetric.OvsAppFdMax)
					sf.Metric.OvsAppConnOpen += maxUint32(app.ConnOpen, prevMetric.OvsAppConnOpen)
					sf.Metric.OvsAppConnMax = maxUint32(app.ConnMax, prevMetric.OvsAppConnMax)
					sf.Metric.OvsAppMemUsed += maxUint64(app.MemUsed, prevMetric.OvsAppMemUsed)
					sf.Metric.OvsAppMemMax = maxUint64(app.MemMax, prevMetric.OvsAppMemMax)

				case layers.SFlowVLANCounters:
					vlan := record.(layers.SFlowVLANCounters)
					sf.Metric.VlanOctets = maxUint64(vlan.Octets, prevMetric.VlanOctets)
					sf.Metric.VlanUcastPkts = maxUint32(vlan.UcastPkts, prevMetric.VlanUcastPkts)
					sf.Metric.VlanMulticastPkts = maxUint32(vlan.MulticastPkts, prevMetric.VlanMulticastPkts)
					sf.Metric.VlanBroadcastPkts = maxUint32(vlan.BroadcastPkts, prevMetric.VlanBroadcastPkts)
					sf.Metric.VlanDiscards = maxUint32(vlan.Discards, prevMetric.VlanDiscards)

				case layers.SFlowEthernetCounters:
					eth := record.(layers.SFlowEthernetCounters)
					sf.Metric.EthAlignmentErrors = maxUint32(eth.AlignmentErrors, prevMetric.EthAlignmentErrors)
					sf.Metric.EthFCSErrors = maxUint32(eth.FCSErrors, prevMetric.EthFCSErrors)
					sf.Metric.EthSingleCollisionFrames = maxUint32(eth.SingleCollisionFrames, prevMetric.EthSingleCollisionFrames)
					sf.Metric.EthMultipleCollisionFrames = maxUint32(eth.MultipleCollisionFrames, prevMetric.EthMultipleCollisionFrames)
					sf.Metric.EthSQETestErrors = maxUint32(eth.SQETestErrors, prevMetric.EthSQETestErrors)
					sf.Metric.EthDeferredTransmissions = maxUint32(eth.DeferredTransmissions, prevMetric.EthDeferredTransmissions)
					sf.Metric.EthLateCollisions = maxUint32(eth.LateCollisions, prevMetric.EthLateCollisions)
					sf.Metric.EthExcessiveCollisions = maxUint32(eth.ExcessiveCollisions, prevMetric.EthExcessiveCollisions)
					sf.Metric.EthInternalMacReceiveErrors = maxUint32(eth.InternalMacReceiveErrors, prevMetric.EthInternalMacReceiveErrors)
					sf.Metric.EthInternalMacTransmitErrors = maxUint32(eth.InternalMacTransmitErrors, prevMetric.EthInternalMacTransmitErrors)
					sf.Metric.EthCarrierSenseErrors = maxUint32(eth.CarrierSenseErrors, prevMetric.EthCarrierSenseErrors)
					sf.Metric.EthFrameTooLongs = maxUint32(eth.FrameTooLongs, prevMetric.EthFrameTooLongs)
					sf.Metric.EthSymbolErrors = maxUint32(eth.SymbolErrors, prevMetric.EthSymbolErrors)
				}
			}
		}

		// sum all the intf metrics
		sf.Metric.IfMetric = IfMetric{}
		for _, m := range sf.IfMetrics {
			sf.Metric.IfInOctets += m.IfInOctets
			sf.Metric.IfInUcastPkts += m.IfInUcastPkts
			sf.Metric.IfInMulticastPkts += m.IfInMulticastPkts
			sf.Metric.IfInBroadcastPkts += m.IfInBroadcastPkts
			sf.Metric.IfInDiscards += m.IfInDiscards
			sf.Metric.IfInErrors += m.IfInErrors
			sf.Metric.IfInUnknownProtos += m.IfInUnknownProtos
			sf.Metric.IfOutOctets += m.IfOutOctets
			sf.Metric.IfOutUcastPkts += m.IfOutUcastPkts
			sf.Metric.IfOutMulticastPkts += m.IfOutMulticastPkts
			sf.Metric.IfOutBroadcastPkts += m.IfOutBroadcastPkts
			sf.Metric.IfOutDiscards += m.IfOutDiscards
			sf.Metric.IfOutErrors += m.IfOutErrors
		}

		if sf.Metric.IsZero() {
			sfa.Graph.Unlock()
			continue
		}

		now := int64(common.UnixMillis(time.Now()))

		if lastUpdateMetric := sf.Metric.Sub(prevMetric).(*SFMetric); !lastUpdateMetric.IsZero() {
			lastUpdateMetric.Start = sf.Metric.Last
			lastUpdateMetric.Last = now

			sf.LastUpdateMetric = lastUpdateMetric
		}
		sf.Metric.Last = now

		if err := sfa.Graph.AddMetadata(sfa.Node, "SFlow", sf); err != nil {
			logging.GetLogger().Errorf("Unable to add sflow metadata from: %s, %s", ra, err)
		}

		sfa.Graph.Unlock()
	}
}

func (sfa *Agent) start() error {
	sfa.FlowTable.Start(nil)
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
func NewAgent(u string, conn *net.UDPConn, addr string, port int, ft *flow.Table, bpfFilter string, headerSize uint32, n *graph.Node, g *graph.Graph) *Agent {
	if headerSize == 0 {
		headerSize = flow.DefaultCaptureLength
	}

	return &Agent{
		UUID:       u,
		Conn:       conn,
		Addr:       addr,
		Port:       port,
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
func (a *AgentAllocator) Alloc(uuid string, ft *flow.Table, bpfFilter string, headerSize uint32, addr *service.Address, n *graph.Node, g *graph.Graph) (*Agent, error) {
	a.Lock()
	defer a.Unlock()

	// check if there is an already allocated agent for this uuid
	for _, agent := range a.agents {
		if uuid == agent.UUID {
			return agent, ErrAgentAlreadyAllocated
		}
	}

	var port int
	var conn *net.UDPConn
	var err error

	if addr.Port <= 0 {
		fnc := func(p int) error {
			conn, err = net.ListenUDP("udp", &net.UDPAddr{
				Port: p,
				IP:   net.ParseIP(addr.Addr),
			})
			return err
		}

		if port, err = a.portAllocator.Allocate(fnc); err != nil {
			logging.GetLogger().Errorf("failed to allocate sflow port: %s", err)
			return nil, err
		}
	} else {
		conn, err = net.ListenUDP("udp", &net.UDPAddr{
			Port: addr.Port,
			IP:   net.ParseIP(addr.Addr),
		})
		if err != nil {
			logging.GetLogger().Errorf("Unable to listen on port %d: %s", addr.Port, err)
			return nil, err
		}
		port = addr.Port
	}

	s := NewAgent(uuid, conn, addr.Addr, port, ft, bpfFilter, headerSize, n, g)

	a.agents = append(a.agents, s)

	s.Start()
	return s, nil
}

// NewAgentAllocator creates a new sFlow agent allocator
func NewAgentAllocator() (*AgentAllocator, error) {
	min := config.GetInt("agent.flow.sflow.port_min")
	max := config.GetInt("agent.flow.sflow.port_max")

	portAllocator, err := portallocator.New(min, max)
	if err != nil {
		return nil, err
	}

	return &AgentAllocator{portAllocator: portAllocator}, nil
}
