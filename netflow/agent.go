/*
 * Copyright (C) 2019 Red Hat, Inc.
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

package netflow

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"time"

	netflow "github.com/VerizonDigital/vflow/netflow/v5"
	"github.com/google/gopacket/layers"
	"github.com/safchain/insanelock"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/graffiti/service"
	"github.com/skydive-project/skydive/portallocator"
)

const (
	maxDgramSize = 65535
)

var (
	// ErrAgentAlreadyAllocated error agent already allocated for this uuid
	ErrAgentAlreadyAllocated = errors.New("agent already allocated for this uuid")
)

// Agent describes NetFlow agent probe
type Agent struct {
	insanelock.RWMutex
	UUID      string
	Addr      string
	Port      int
	FlowTable *flow.Table
	Conn      *net.UDPConn
	UUIDs     flow.UUIDs
}

// AgentAllocator describes an NetFlow agent allocator to manage multiple NetFlow agent probe
type AgentAllocator struct {
	insanelock.RWMutex
	portAllocator *portallocator.PortAllocator
	agents        []*Agent
}

// GetTarget returns the current used connection
func (nfa *Agent) GetTarget() string {
	return fmt.Sprintf("%s:%d", nfa.Addr, nfa.Port)
}
func intToIP(nn uint32) net.IP {
	ip := make(net.IP, 4)
	binary.BigEndian.PutUint32(ip, nn)
	return ip
}

func (nfa *Agent) feedFlowTable(extFlowChan chan *flow.ExtFlow) {
	var buf [maxDgramSize]byte
	for {
		n, addr, err := nfa.Conn.ReadFromUDP(buf[:])
		if err != nil {
			return
		}

		decoder := netflow.NewDecoder(addr.IP, buf[0:n])
		msg, err := decoder.Decode()
		if err != nil {
			logging.GetLogger().Errorf("Unable to decode NetFlow message: %s", err)
			continue
		}

		bootTime := common.UnixMillis(time.Now()) - int64(msg.Header.SysUpTimeMSecs)

	LOOP:
		for _, nf := range msg.Flows {
			f := flow.NewFlow()
			f.Init(int64(nf.StartTime)+bootTime, "", &nfa.UUIDs)
			f.Last = int64(nf.EndTime) + bootTime

			// netflow v5 can't be ipv6
			f.Network = &flow.FlowLayer{
				Protocol: flow.FlowProtocol_IPV4,
				A:        intToIP(nf.SrcAddr).To4().String(),
				B:        intToIP(nf.DstAddr).To4().String(),
			}

			f.Transport = &flow.TransportLayer{
				A: int64(nf.SrcPort),
				B: int64(nf.DstPort),
			}

			switch layers.IPProtocol(nf.ProtType) {
			case layers.IPProtocolTCP:
				f.Transport.Protocol = flow.FlowProtocol_TCP
				f.Application = "TCP"
			case layers.IPProtocolUDP:
				f.Transport.Protocol = flow.FlowProtocol_UDP
				f.Application = "UDP"
			case layers.IPProtocolICMPv4:
				f.Transport.Protocol = flow.FlowProtocol_ICMPV4
				f.Application = "ICMPv4"
			case layers.IPProtocolIPv4:
				f.Transport.Protocol = flow.FlowProtocol_IPV4
				f.Application = "IPV4"
			case layers.IPProtocolSCTP:
				f.Transport.Protocol = flow.FlowProtocol_SCTP
				f.Application = "SCTP"
			default:
				goto LOOP
			}

			f.Metric = &flow.FlowMetric{
				ABBytes:   int64(nf.L3Octets / 2), // we consider that it is a strict two way protocol
				ABPackets: int64(nf.PktCount / 2),
				BABytes:   int64(nf.L3Octets / 2),
				BAPackets: int64(nf.PktCount / 2),
				Start:     f.Start,
				Last:      f.Last,
			}

			l2, l3 := f.SetUUIDs(123, flow.Opts{LayerKeyMode: flow.L3PreferredKeyMode})

			op := &flow.Operation{
				Type: flow.ReplaceOperation,
				Flow: f,
				Key:  l2 ^ l3,
			}

			extFlowChan <- &flow.ExtFlow{
				Type: flow.OperationExtFlowType,
				Obj:  op,
			}
		}
	}
}

func (nfa *Agent) start() error {
	_, extFlowChan, _ := nfa.FlowTable.Start(nil)
	defer nfa.FlowTable.Stop()

	nfa.feedFlowTable(extFlowChan)

	return nil
}

// Start the NetFlow probe agent
func (nfa *Agent) Start() {
	go nfa.start()
}

// Stop the NetFlow probe agent
func (nfa *Agent) Stop() {
	nfa.Lock()
	defer nfa.Unlock()

	if nfa.Conn != nil {
		nfa.Conn.Close()
	}
}

// NewAgent creates a new NetFlow agent which will populate the given flowtable
func NewAgent(u string, conn *net.UDPConn, addr string, port int, ft *flow.Table, uuids flow.UUIDs) *Agent {
	return &Agent{
		UUID:      u,
		Addr:      addr,
		Port:      port,
		Conn:      conn,
		FlowTable: ft,
		UUIDs:     uuids,
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

// Release a netflow agent
func (a *AgentAllocator) Release(uuid string) {
	a.Lock()
	defer a.Unlock()

	a.release(uuid)
}

// ReleaseAll netflow agents
func (a *AgentAllocator) ReleaseAll() {
	a.Lock()
	defer a.Unlock()

	for _, agent := range a.agents {
		a.release(agent.UUID)
	}
}

// Alloc allocates a new netflow agent
func (a *AgentAllocator) Alloc(uuid string, ft *flow.Table, addr *service.Address, uuids flow.UUIDs) (agent *Agent, _ error) {
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
			logging.GetLogger().Errorf("failed to allocate netflow port: %s", err)
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

	s := NewAgent(uuid, conn, addr.Addr, port, ft, uuids)
	a.agents = append(a.agents, s)

	s.Start()
	return s, nil
}

// NewAgentAllocator creates a new netflow agent allocator
func NewAgentAllocator() (*AgentAllocator, error) {
	min := config.GetInt("agent.flow.netflow.port_min")
	max := config.GetInt("agent.flow.netflow.port_max")

	portAllocator, err := portallocator.New(min, max)
	if err != nil {
		return nil, err
	}

	return &AgentAllocator{portAllocator: portAllocator}, nil
}
