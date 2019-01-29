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
	"math"
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
			var Countersamples []layers.SFlowCounterSample
			var gen layers.SFlowGenericInterfaceCounters
			var ovsdp layers.SFlowOVSDPCounters
			var app layers.SFlowAppresourcesCounters
			var vlan layers.SFlowVLANCounters

			for _, sample := range sflowPacket.CounterSamples {
				records := sample.GetRecords()
				var counter layers.SFlowCounterSample
				counter.EnterpriseID = sample.EnterpriseID
				counter.Format = sample.Format
				counter.SampleLength = sample.SampleLength
				counter.SourceIDClass = sample.SourceIDClass
				counter.SourceIDIndex = sample.SourceIDIndex
				counter.RecordCount = sample.RecordCount
				counter.SequenceNumber = sample.SequenceNumber
				maxuint64 := func(key uint64) uint64 {
					if key == math.MaxUint64 {
						key = 0
					}
					return key
				}
				maxuint32 := func(key uint32) uint32 {
					if key == math.MaxUint32 {
						key = 0
					}
					return key
				}

				for _, record := range records {
					switch record.(type) {
					case layers.SFlowGenericInterfaceCounters:
						gen1 := record.(layers.SFlowGenericInterfaceCounters)
						gen1.IfInOctets = maxuint64(gen1.IfInOctets)
						gen.IfInOctets += gen1.IfInOctets
						gen1.IfInUcastPkts = maxuint32(gen1.IfInUcastPkts)
						gen.IfInUcastPkts += gen1.IfInUcastPkts
						gen1.IfInMulticastPkts = maxuint32(gen1.IfInMulticastPkts)
						gen.IfInMulticastPkts += gen1.IfInMulticastPkts
						gen1.IfInBroadcastPkts = maxuint32(gen1.IfInBroadcastPkts)
						gen.IfInBroadcastPkts += gen1.IfInBroadcastPkts
						gen1.IfInDiscards = maxuint32(gen1.IfInDiscards)
						gen.IfInDiscards += gen1.IfInDiscards
						gen1.IfInErrors = maxuint32(gen1.IfInErrors)
						gen.IfInErrors += gen1.IfInErrors
						gen1.IfInUnknownProtos = maxuint32(gen1.IfInUnknownProtos)
						gen.IfInUnknownProtos += gen1.IfInUnknownProtos
						gen1.IfOutOctets = maxuint64(gen1.IfOutOctets)
						gen.IfOutOctets += gen1.IfOutOctets
						gen1.IfOutUcastPkts = maxuint32(gen1.IfOutUcastPkts)
						gen.IfOutUcastPkts += gen1.IfOutUcastPkts
						gen1.IfOutMulticastPkts = maxuint32(gen1.IfOutMulticastPkts)
						gen.IfOutMulticastPkts += gen1.IfOutMulticastPkts
						gen1.IfOutBroadcastPkts = maxuint32(gen1.IfOutBroadcastPkts)
						gen.IfOutBroadcastPkts += gen1.IfOutBroadcastPkts
						gen1.IfOutDiscards = maxuint32(gen1.IfOutDiscards)
						gen.IfOutDiscards += gen1.IfOutDiscards
						gen1.IfOutErrors = maxuint32(gen1.IfOutErrors)
						gen.IfOutErrors += gen1.IfOutErrors
						counter.Records = append(counter.Records, gen1)

					case layers.SFlowOVSDPCounters:
						ovsdp1 := record.(layers.SFlowOVSDPCounters)
						ovsdp1.NHit = maxuint32(ovsdp1.NHit)
						ovsdp.NHit += ovsdp1.NHit
						ovsdp1.NMissed = maxuint32(ovsdp1.NMissed)
						ovsdp.NMissed += ovsdp1.NMissed
						ovsdp1.NLost = maxuint32(ovsdp1.NLost)
						ovsdp.NLost += ovsdp1.NLost
						ovsdp1.NMaskHit = maxuint32(ovsdp1.NMaskHit)
						ovsdp.NMaskHit += ovsdp1.NMaskHit
						ovsdp1.NFlows = maxuint32(ovsdp1.NFlows)
						ovsdp.NFlows += ovsdp1.NFlows
						ovsdp1.NMasks = maxuint32(ovsdp1.NMasks)
						ovsdp.NMasks += ovsdp1.NMasks
						counter.Records = append(counter.Records, ovsdp1)

					case layers.SFlowAppresourcesCounters:
						app1 := record.(layers.SFlowAppresourcesCounters)
						app1.FdOpen = maxuint32(app1.FdOpen)
						app.FdOpen += app1.FdOpen
						app1.FdMax = maxuint32(app1.FdMax)
						app.FdMax += app1.FdMax
						app1.ConnOpen = maxuint32(app1.ConnOpen)
						app.ConnOpen += app1.ConnOpen
						app1.ConnMax = maxuint32(app1.ConnMax)
						app.ConnMax += app1.ConnMax
						app1.MemUsed = maxuint64(app1.MemUsed)
						app.MemUsed += app1.MemUsed
						app1.MemMax = maxuint64(app1.MemMax)
						app.MemMax += app1.MemMax
						counter.Records = append(counter.Records, app1)

					case layers.SFlowVLANCounters:
						vlan1 := record.(layers.SFlowVLANCounters)
						vlan1.Octets = maxuint64(vlan1.Octets)
						vlan.Octets += vlan1.Octets
						vlan1.UcastPkts = maxuint32(vlan1.UcastPkts)
						vlan.UcastPkts += vlan1.UcastPkts
						vlan1.MulticastPkts = maxuint32(vlan1.MulticastPkts)
						vlan.MulticastPkts += vlan1.MulticastPkts
						vlan1.BroadcastPkts = maxuint32(vlan1.BroadcastPkts)
						vlan.BroadcastPkts += vlan1.BroadcastPkts
						vlan1.Discards = maxuint32(vlan1.Discards)
						vlan.Discards += vlan1.Discards
						counter.Records = append(counter.Records, vlan1)

					case layers.SFlowOpenflowPortCounters:
						ofpc1 := record.(layers.SFlowOpenflowPortCounters)
						ofpc1.DatapathID = maxuint64(ofpc1.DatapathID)
						ofpc1.PortNo = maxuint32(ofpc1.PortNo)
						counter.Records = append(counter.Records, ofpc1)

					case layers.SFlowEthernetCounters:
						eth1 := record.(layers.SFlowEthernetCounters)
						eth1.AlignmentErrors = maxuint32(eth1.AlignmentErrors)
						eth1.FCSErrors = maxuint32(eth1.FCSErrors)
						eth1.SingleCollisionFrames = maxuint32(eth1.SingleCollisionFrames)
						eth1.MultipleCollisionFrames = maxuint32(eth1.MultipleCollisionFrames)
						eth1.SQETestErrors = maxuint32(eth1.SQETestErrors)
						eth1.DeferredTransmissions = maxuint32(eth1.DeferredTransmissions)
						eth1.LateCollisions = maxuint32(eth1.LateCollisions)
						eth1.ExcessiveCollisions = maxuint32(eth1.ExcessiveCollisions)
						eth1.InternalMacReceiveErrors = maxuint32(eth1.InternalMacReceiveErrors)
						eth1.InternalMacTransmitErrors = maxuint32(eth1.InternalMacTransmitErrors)
						eth1.CarrierSenseErrors = maxuint32(eth1.CarrierSenseErrors)
						eth1.FrameTooLongs = maxuint32(eth1.FrameTooLongs)
						eth1.SymbolErrors = maxuint32(eth1.SymbolErrors)
						counter.Records = append(counter.Records, eth1)

					case layers.SFlowPORTNAME, layers.SFlowLACPCounters:
						counter.RecordCount--

					}
				}
				Countersamples = append(Countersamples, counter)
			}

			sfa.Graph.Lock()
			tr := sfa.Graph.StartMetadataTransaction(sfa.Node)

			currMetric := &SFMetric{
				IfInOctets:         int64(gen.IfInOctets),
				IfInUcastPkts:      int64(gen.IfInUcastPkts),
				IfInMulticastPkts:  int64(gen.IfInMulticastPkts),
				IfInBroadcastPkts:  int64(gen.IfInBroadcastPkts),
				IfInDiscards:       int64(gen.IfInDiscards),
				IfInErrors:         int64(gen.IfInErrors),
				IfInUnknownProtos:  int64(gen.IfInUnknownProtos),
				IfOutOctets:        int64(gen.IfOutOctets),
				IfOutUcastPkts:     int64(gen.IfOutUcastPkts),
				IfOutMulticastPkts: int64(gen.IfOutMulticastPkts),
				IfOutBroadcastPkts: int64(gen.IfOutBroadcastPkts),
				IfOutDiscards:      int64(gen.IfOutDiscards),
				IfOutErrors:        int64(gen.IfOutErrors),
				OvsdpNHit:          int64(ovsdp.NHit),
				OvsdpNMissed:       int64(ovsdp.NMissed),
				OvsdpNLost:         int64(ovsdp.NLost),
				OvsdpNMaskHit:      int64(ovsdp.NMaskHit),
				OvsdpNFlows:        int64(ovsdp.NFlows),
				OvsdpNMasks:        int64(ovsdp.NMasks),
				OvsAppFdOpen:       int64(app.FdOpen),
				OvsAppFdMax:        int64(app.FdMax),
				OvsAppConnOpen:     int64(app.ConnOpen),
				OvsAppConnMax:      int64(app.ConnMax),
				OvsAppMemUsed:      int64(app.MemUsed),
				OvsAppMemMax:       int64(app.MemMax),
				VlanOctets:         int64(vlan.Octets),
				VlanUcastPkts:      int64(vlan.UcastPkts),
				VlanMulticastPkts:  int64(vlan.MulticastPkts),
				VlanBroadcastPkts:  int64(vlan.BroadcastPkts),
				VlanDiscards:       int64(vlan.Discards),
			}
			now := int64(common.UnixMillis(time.Now()))

			currMetric.Last = now

			var prevMetric, lastUpdateMetric, totalMetric *SFMetric

			if metric, err := sfa.Node.GetField("SFlow.Metric"); err == nil {
				prevMetric = metric.(*SFMetric)
				lastUpdateMetric = currMetric
				totalMetric = currMetric.Add(prevMetric).(*SFMetric)
				totalMetric.Last = now
			} else {
				totalMetric = currMetric
			}

			// nothing changed since last update
			if lastUpdateMetric != nil && !lastUpdateMetric.IsZero() {
				lastUpdateMetric.Start = prevMetric.Last
				lastUpdateMetric.Last = now
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
