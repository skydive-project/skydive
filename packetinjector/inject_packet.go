/*
 * Copyright (C) 2016 Red Hat, Inc.
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
	"errors"
	"net"
	"strings"
	"sync"
	"syscall"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology"
)

var (
	options = gopacket.SerializeOptions{
		ComputeChecksums: true,
		FixLengths:       true,
	}
)

// PacketInjectionParams describes the packet parameters to be injected
type PacketInjectionParams struct {
	UUID             string
	SrcNodeID        graph.Identifier `valid:"nonzero"`
	SrcIP            net.IP
	SrcMAC           net.HardwareAddr
	SrcPort          uint16
	DstIP            net.IP
	DstMAC           net.HardwareAddr
	DstPort          uint16
	Type             string `valid:"regexp=^(icmp4|icmp6|tcp4|tcp6|udp4|udp6)$"`
	Count            uint64 `valid:"min=1"`
	ID               uint64
	Interval         uint64
	Increment        bool
	IncrementPayload int64
	Payload          string
	Pcap             []byte
	TTL              uint8
}

// Channels channels struct
type Channels struct {
	sync.Mutex
	Pipes map[string](chan bool)
}

// Packet is defined as a gopacket and it byte representation
type Packet struct {
	data     []byte
	gopacket gopacket.Packet
}

// PacketForger describes an objects that feeds a channel with packets
type PacketForger interface {
	PacketSource() chan *Packet
}

// PacketInjector injects packets coming from a packet forget intoto a raw socket
type PacketInjector struct {
	IfName    string
	RawSocket *common.RawSocket
}

// Write injects data into the raw socket
func (p *PacketInjector) Write(data []byte) {
	logging.GetLogger().Debugf("Injecting packet (%d bytes) on interface %s", len(data), p.IfName)
	if _, err := p.RawSocket.Write(data); err != nil {
		if err == syscall.ENXIO {
			logging.GetLogger().Warningf("Write error on interface %s: %s", p.IfName, err)
		} else {
			logging.GetLogger().Errorf("Write error on interface %s: %s", p.IfName, err)
		}
	}
}

// Close the packet injector and its socket
func (p *PacketInjector) Close() {
	p.RawSocket.Close()
}

// NewPacketInjector returns a new packet injector into one interface
func NewPacketInjector(g *graph.Graph, srcNode *graph.Node) (*PacketInjector, error) {
	encapType, _ := srcNode.GetFieldString("EncapType")

	protocol := common.AllPackets
	layerType, _ := flow.GetFirstLayerType(encapType)
	switch layerType {
	case flow.LayerTypeRawIP, layers.LayerTypeIPv4, layers.LayerTypeIPv6:
		protocol = common.OnlyIPPackets
	}

	ifName, err := srcNode.GetFieldString("Name")
	if err != nil {
		return nil, errors.New("Source node has no name")
	}

	_, nsPath, err := topology.NamespaceFromNode(g, srcNode)
	if err != nil {
		return nil, err
	}

	var rawSocket *common.RawSocket
	if nsPath != "" {
		rawSocket, err = common.NewRawSocketInNs(nsPath, ifName, protocol)
	} else {
		rawSocket, err = common.NewRawSocket(ifName, protocol)
	}
	if err != nil {
		return nil, err
	}

	return &PacketInjector{
		RawSocket: rawSocket,
		IfName:    ifName,
	}, nil
}

// InjectPackets inject some packets based on the graph
func InjectPackets(pp *PacketInjectionParams, g *graph.Graph, chnl *Channels) (string, error) {
	g.RLock()

	srcNode := g.GetNode(pp.SrcNodeID)
	if srcNode == nil {
		g.RUnlock()
		return "", errors.New("Unable to find source node")
	}

	tid, err := srcNode.GetFieldString("TID")
	if err != nil {
		return "", errors.New("Source node has no TID")
	}

	injector, err := NewPacketInjector(g, srcNode)
	if err != nil {
		return "", err
	}

	var forgePackets PacketForger
	if len(pp.Pcap) > 0 {
		forgePackets, err = NewPcapPacketGenerator(pp.Pcap)
	} else {
		forgePackets, err = NewForgedPacketGenerator(pp, srcNode)
	}

	g.RUnlock()

	if err != nil {
		return "", err
	}
	trackID, injected := DoInjections(pp, chnl, injector, forgePackets, tid)
	go func(injected chan string) {
		for range injected {
		}
	}(injected)
	return trackID, nil
}

// DoInjections inject the packets
func DoInjections(pp *PacketInjectionParams, chnl *Channels, injector *PacketInjector, forgePackets PacketForger, tid string) (string, chan string) {
	p := make(chan bool)
	injected := make(chan string)
	chnl.Lock()
	chnl.Pipes[pp.UUID] = p
	chnl.Unlock()

	packetSource := forgePackets.PacketSource()
	packet := <-packetSource

	go func(c chan bool, injected chan string) {
		defer func() {
			injector.Close()
			chnl.Lock()
			delete(chnl.Pipes, pp.UUID)
			chnl.Unlock()
		}()

		injector.Write(packet.data)
		injected <- "New packet injected"

		for {
			select {
			case <-c:
				logging.GetLogger().Infof("Injection stopped on interface %s", injector.IfName)
				close(injected)
				return
			case packet := <-packetSource:
				if packet == nil {
					close(injected)
					return
				}
				injector.Write(packet.data)
				injected <- "New packet injected"
			}
		}
	}(p, injected)

	if packet.gopacket != nil && tid != "" {
		f := flow.NewFlowFromGoPacket(packet.gopacket, tid, flow.UUIDs{}, flow.Opts{})
		return f.TrackingID, injected
	}

	return "", injected
}

// GetIP returns a firstIP of the CIDR
func GetIP(cidr string) net.IP {
	if len(cidr) <= 0 {
		return nil
	}
	ips := strings.Split(cidr, ",")
	//TODO(masco): currently taking first IP, need to implement to select a proper IP
	ip, _, err := net.ParseCIDR(ips[0])
	if err != nil {
		return nil
	}
	return ip
}
