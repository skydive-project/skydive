// +build linux

/*
 * Copyright (C) 2018 Red Hat, Inc.
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

package lldp

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/safchain/ethtool"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/flow/probes"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/graph"
	"golang.org/x/sys/unix"
)

/*
#include <linux/if_packet.h>  // packet_mreq
#include <linux/if_ether.h>  // ETH_ALEN
*/
import "C"

const lldpBPFFilter = `ether[0] & 1 = 1 and
  ether proto 0x88cc and
  !(ether src %s) and
  (ether dst 01:80:c2:00:00:0e or
   ether dst 01:80:c2:00:00:03 or
   ether dst 01:80:c2:00:00:00)`

// Capture 8192 bytes so that we have the full Ethernet frame
const lldpSnapLen = 8192

// Probe describes the probe that is in charge of listening for
// LLDP packets on interfaces and create the corresponding chassis and port nodes
type Probe struct {
	sync.RWMutex
	graph.DefaultGraphListener
	g             *graph.Graph
	hostNode      *graph.Node     // graph node of the running host
	interfaceMap  map[string]bool // map interface names to the capturing state
	state         int64           // state of the probe (running or stopped)
	wg            sync.WaitGroup  // capture goroutines wait group
	autoDiscovery bool            // capture LLDP traffic on all capable interfaces
}

type ifreq struct {
	ifrName   [ethtool.IFNAMSIZ]byte
	ifrHwaddr syscall.RawSockaddr
}

// addMulticastAddr adds a multicast address to an interface using an ioctl call
func addMulticastAddr(intf string, addr string) error {
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, 0)
	if err != nil {
		return err
	}
	defer syscall.Close(fd)

	var name [ethtool.IFNAMSIZ]byte
	copy(name[:], []byte(intf))

	mac, _ := net.ParseMAC(addr)

	var sockaddr syscall.RawSockaddr
	// RawSockaddr.Data is []int8 on amd64 and []uint8 on ppc64le
	switch data := interface{}(sockaddr.Data).(type) {
	case [14]int8:
		for i, n := range mac {
			data[i] = int8(n)
		}
	case [14]uint8:
		for i, n := range mac {
			data[i] = uint8(n)
		}
	}

	ifr := &ifreq{
		ifrName:   name,
		ifrHwaddr: sockaddr,
	}

	_, _, ep := unix.Syscall(unix.SYS_IOCTL, uintptr(fd),
		unix.SIOCADDMULTI, uintptr(unsafe.Pointer(ifr)))

	if ep != 0 {
		return syscall.Errno(ep)
	}
	return nil
}

func (p *Probe) handlePacket(n *graph.Node, packet gopacket.Packet) {
	lldpLayer := packet.Layer(layers.LayerTypeLinkLayerDiscovery)
	if lldpLayer != nil {
		lldpLayer := lldpLayer.(*layers.LinkLayerDiscovery)

		chassisMetadata := graph.Metadata{
			"LLDP": map[string]interface{}{
				"ChassisIDType": lldpLayer.ChassisID.Subtype.String(),
			},
			"Type":  "switch",
			"Probe": "lldp",
		}

		var chassisID string
		switch lldpLayer.ChassisID.Subtype {
		case layers.LLDPChassisIDSubTypeMACAddr:
			chassisID = net.HardwareAddr(lldpLayer.ChassisID.ID).String()
		default:
			chassisID = string(lldpLayer.ChassisID.ID)
		}
		common.SetField(chassisMetadata, "LLDP.ChassisID", chassisID)
		common.SetField(chassisMetadata, "Name", chassisID)

		portMetadata := graph.Metadata{
			"LLDP": map[string]interface{}{
				"PortIDType": lldpLayer.PortID.Subtype.String(),
			},
			"Type":  "switchport",
			"Probe": "lldp",
		}

		var portID string
		switch lldpLayer.PortID.Subtype {
		case layers.LLDPPortIDSubtypeMACAddr:
			portID = net.HardwareAddr(lldpLayer.PortID.ID).String()
		default:
			portID = string(lldpLayer.PortID.ID)
		}
		common.SetField(portMetadata, "LLDP.PortID", portID)
		common.SetField(portMetadata, "Name", portID)

		if lldpLayerInfo := packet.Layer(layers.LayerTypeLinkLayerDiscoveryInfo); lldpLayerInfo != nil {
			lldpLayerInfo := lldpLayerInfo.(*layers.LinkLayerDiscoveryInfo)

			if lldpLayerInfo.PortDescription != "" {
				common.SetField(portMetadata, "LLDP.Description", lldpLayerInfo.PortDescription)
				portMetadata["Name"] = lldpLayerInfo.PortDescription
			}

			if lldpLayerInfo.SysDescription != "" {
				common.SetField(chassisMetadata, "LLDP.Description", lldpLayerInfo.SysDescription)
			}

			if lldpLayerInfo.SysName != "" {
				common.SetField(chassisMetadata, "LLDP.SysName", lldpLayerInfo.SysName)
				chassisMetadata["Name"] = lldpLayerInfo.SysName
			}

			if lldp8201Q, err := lldpLayerInfo.Decode8021(); err == nil {
				if lldp8201Q.LinkAggregation.Supported {
					common.SetField(portMetadata, "LLDP.LinkAggregation", map[string]interface{}{
						"Enabled":   lldp8201Q.LinkAggregation.Enabled,
						"PortID":    int64(lldp8201Q.LinkAggregation.PortID),
						"Supported": lldp8201Q.LinkAggregation.Supported,
					})
				}

				if lldp8201Q.PVID != 0 {
					common.SetField(portMetadata, "LLDP.PVID", int64(lldp8201Q.PVID))
				}

				if lldp8201Q.VIDUsageDigest != 0 {
					common.SetField(portMetadata, "LLDP.VIDUsageDigest", int64(lldp8201Q.VIDUsageDigest))
				}

				if lldp8201Q.ManagementVID != 0 {
					common.SetField(portMetadata, "LLDP.ManagementVID", int64(lldp8201Q.ManagementVID))
				}

				if len(lldp8201Q.VLANNames) != 0 {
					vlanNames := make([]interface{}, len(lldp8201Q.VLANNames))
					for i, vlan := range lldp8201Q.VLANNames {
						vlanNames[i] = map[string]interface{}{
							"ID":   vlan.ID,
							"Name": vlan.Name,
						}
					}
					common.SetField(portMetadata, "LLDP.VLANNames", vlanNames)
				}

				if len(lldp8201Q.PPVIDs) != 0 {
					ppvids := make([]interface{}, len(lldp8201Q.PPVIDs))
					for i, ppvid := range lldp8201Q.PPVIDs {
						ppvids[i] = map[string]interface{}{
							"Enabled":   ppvid.Enabled,
							"ID":        int64(ppvid.ID),
							"Supported": ppvid.Supported,
						}
					}
					common.SetField(portMetadata, "LLDP.PPVIDs", ppvids)
				}
			}

			if lldp8023, err := lldpLayerInfo.Decode8023(); err == nil {
				if lldp8023.MTU != 0 {
					portMetadata["MTU"] = int64(lldp8023.MTU)
				}
			}
		}

		// TODO: Handle TTL (set port to down when timer expires ?)

		p.g.Lock()

		// Create a node for the sending chassis with a predictable ID
		chassis := p.getOrCreate(graph.GenID(chassisID, lldpLayer.ChassisID.Subtype.String()), chassisMetadata)

		// Create a port with a predicatable ID
		port := p.getOrCreate(graph.GenID(
			chassisID, lldpLayer.ChassisID.Subtype.String(),
			portID, lldpLayer.PortID.Subtype.String(),
		), portMetadata)

		if !topology.HaveOwnershipLink(p.g, chassis, port) {
			topology.AddOwnershipLink(p.g, chassis, port, nil)
			topology.AddLayer2Link(p.g, chassis, port, nil)
		}

		if !topology.HaveLayer2Link(p.g, port, n) {
			topology.AddLayer2Link(p.g, port, n, nil)
		}

		p.g.Unlock()
	}
}

func (p *Probe) startCapture(ifName, mac string, n *graph.Node) error {
	lldpPrefix := "01:80:c2:00:00"
	for _, lastByte := range []byte{0x00, 0x03, 0x0e} {
		// Add multicast address so that the kernel does not discard it
		if err := addMulticastAddr(ifName, fmt.Sprintf("%s:%02X", lldpPrefix, lastByte)); err != nil {
			return fmt.Errorf("Failed to add multicast address: %s", err)
		}
	}

	// Set BPF filter to only capture LLDP packets
	bpfFilter := fmt.Sprintf(lldpBPFFilter, mac)

	packetProbe, err := probes.NewGoPacketProbe(p.g, n, probes.AFPacket, bpfFilter, lldpSnapLen)
	if err != nil {
		return err
	}

	// Set capturing state to true
	p.interfaceMap[ifName] = true

	p.wg.Add(1)

	go func() {
		defer func() {
			logging.GetLogger().Infof("Stopping LLDP capture on %s", ifName)

			p.Lock()
			p.interfaceMap[ifName] = false
			p.Unlock()

			p.wg.Done()
		}()

		packetProbe.Run(func(packet gopacket.Packet) {
			p.handlePacket(n, packet)
		}, nil)
	}()

	return err
}

func (p *Probe) getOrCreate(id graph.Identifier, m graph.Metadata) *graph.Node {
	node := p.g.GetNode(id)
	if node == nil {
		node = p.g.NewNode(id, m)
	} else {
		tr := p.g.StartMetadataTransaction(node)
		for k, v := range m {
			tr.AddMetadata(k, v)
		}
		tr.Commit()
	}
	return node
}

// handleNode checks if LLDP capture should be started on the passed interface node, ie:
// - when no LLDP capture is running for this interface
// - when its first packet layer is Ethernet and it has a MAC address
// - when the interface is listed in the configuration file or we are in auto discovery mode
func (p *Probe) handleNode(n *graph.Node) {
	firstLayerType, _ := probes.GoPacketFirstLayerType(n)
	mac, _ := n.GetFieldString("MAC")
	name, _ := n.GetFieldString("Name")

	if name != "" && mac != "" && firstLayerType == layers.LayerTypeEthernet {
		if active, found := p.interfaceMap[name]; (found || p.autoDiscovery) && !active {
			logging.GetLogger().Infof("Starting LLDP capture on %s", name)
			if err := p.startCapture(name, mac, n); err != nil {
				logging.GetLogger().Error(err)
			}
		}
	}
}

// OnEdgeAdded is called when a new edge was created on the graph
func (p *Probe) OnEdgeAdded(e *graph.Edge) {
	p.Lock()
	defer p.Unlock()

	// Only consider nodes that are owned by the host node
	if e.GetParent() == p.hostNode.ID {
		if relationType, _ := e.GetFieldString("RelationType"); relationType == topology.OwnershipLink {
			n := p.g.GetNode(e.GetChild())
			p.handleNode(n)
		}
	}
}

// OnNodeUpdated is called when a new node was updated on the graph
func (p *Probe) OnNodeUpdated(n *graph.Node) {
	p.Lock()
	defer p.Unlock()

	// If the interface was modified from down to up
	if state, _ := n.GetFieldString("State"); state == "UP" {
		if p.g.AreLinked(p.hostNode, n, topology.OwnershipMetadata()) {
			p.handleNode(n)
		}
	}
}

// Start capturing LLDP packets
func (p *Probe) Start() {
	atomic.StoreInt64(&p.state, common.RunningState)
	p.g.AddEventListener(p)

	p.g.RLock()
	defer p.g.RUnlock()
	p.Lock()
	defer p.Unlock()

	// The nodes may have already been created
	children := p.g.LookupChildren(p.hostNode, nil, topology.OwnershipMetadata())
	for _, intfNode := range children {
		p.handleNode(intfNode)
	}
}

// Stop capturing LLDP packets
func (p *Probe) Stop() {
	p.g.RemoveEventListener(p)
	atomic.StoreInt64(&p.state, common.StoppingState)
	p.wg.Wait()
}

// NewProbe returns a new LLDP probe
func NewProbe(g *graph.Graph, hostNode *graph.Node, interfaces []string) (*Probe, error) {
	interfaceMap := make(map[string]bool)
	for _, intf := range interfaces {
		interfaceMap[intf] = false
	}

	return &Probe{
		g:             g,
		hostNode:      hostNode,
		interfaceMap:  interfaceMap,
		state:         common.StoppedState,
		autoDiscovery: len(interfaces) == 0,
	}, nil
}
