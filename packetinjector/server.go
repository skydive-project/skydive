// +build packetinject

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
	"syscall"
	"time"

	"github.com/google/gopacket/layers"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/graffiti/api/rest"
	"github.com/skydive-project/skydive/graffiti/getter"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/graffiti/ondemand"
	"github.com/skydive-project/skydive/rawsocket"
	"github.com/skydive-project/skydive/topology"
)

func (o *onDemandPacketInjectServer) CreateTask(srcNode *graph.Node, resource rest.Resource) (ondemand.Task, error) {
	logging.GetLogger().Debugf("Registering packet injection %s on %s", resource.GetID(), srcNode.ID)

	pp := resource.(*PacketInjectionRequest)

	encapType, _ := srcNode.GetFieldString("EncapType")

	protocol := rawsocket.AllPackets
	layerType, _ := flow.GetFirstLayerType(encapType)
	switch layerType {
	case flow.LayerTypeRawIP, layers.LayerTypeIPv4, layers.LayerTypeIPv6:
		protocol = rawsocket.OnlyIPPackets
	}

	ifName, err := srcNode.GetFieldString("Name")
	if err != nil {
		return nil, errors.New("Source node has no name")
	}

	_, nsPath, err := topology.NamespaceFromNode(o.graph, srcNode)
	if err != nil {
		return nil, err
	}

	var rawSocket *rawsocket.RawSocket
	if nsPath != "" {
		rawSocket, err = rawsocket.NewRawSocketInNs(nsPath, ifName, protocol)
	} else {
		rawSocket, err = rawsocket.NewRawSocket(ifName, protocol)
	}
	if err != nil {
		return nil, err
	}

	var packetForger PacketForger
	if len(pp.Pcap) > 0 {
		packetForger, err = NewPcapPacketGenerator(pp)
	} else {
		encapType, _ := srcNode.GetFieldString("EncapType")
		packetForger, err = NewForgedPacketGenerator(pp, encapType)
	}

	metadata := &InjectionMetadata{
		PacketInjectionRequest: *pp,
		ID:                     pp.UUID,
		State:                  "active",
	}

	if o.graph.UpdateMetadata(srcNode, "PacketInjections", func(obj interface{}) bool {
		captures := obj.(*Injections)
		*captures = append(*captures, metadata)
		return true
	}) == getter.ErrFieldNotFound {
		o.graph.AddMetadata(srcNode, "PacketInjections", &Injections{metadata})
	}

	packetSource := packetForger.PacketSource()
	packetCount := int64(0)
	cancel := make(chan bool, 1)

	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()

	InjectLoop:
		for {
			select {
			case packet := <-packetSource:
				if packet == nil {
					break InjectLoop
				}

				if packet.gopacket != nil {
					logging.GetLogger().Debugf("Injecting packet %+v (%d bytes) on interface %s", packet.gopacket, len(packet.data), ifName)
				} else {
					logging.GetLogger().Debugf("Injecting packet (%d bytes) on interface %s", len(packet.data), ifName)
				}

				if _, err := rawSocket.Write(packet.data); err != nil {
					if err == syscall.ENXIO {
						logging.GetLogger().Warningf("Write error on interface %s: %s", ifName, err)
					} else {
						logging.GetLogger().Errorf("Write error on interface %s: %s", ifName, err)
					}
				}
				packetCount++
			case <-cancel:
				break InjectLoop
			case <-ticker.C:
				o.graph.Lock()
				o.graph.UpdateMetadata(srcNode, "PacketInjections", func(obj interface{}) bool {
					metadata.PacketCount = packetCount
					return true
				})
				o.graph.Unlock()
			}
		}

		o.graph.Lock()
		o.graph.UpdateMetadata(srcNode, "PacketInjections", func(obj interface{}) bool {
			injections := obj.(*Injections)
			for i, injection := range *injections {
				if injection.UUID == pp.UUID {
					if len(*injections) <= 1 {
						o.graph.DelMetadata(srcNode, "PacketInjections")
						return false
					}
					*injections = append((*injections)[:i], (*injections)[i+1:]...)
					return true
				}
			}
			return false
		})
		o.graph.Unlock()

		packetForger.Close()
		rawSocket.Close()
	}()

	return cancel, nil
}

func (o *onDemandPacketInjectServer) RemoveTask(n *graph.Node, resource rest.Resource, task ondemand.Task) error {
	logging.GetLogger().Debugf("Unregister packet injection %s on %s", n.ID, resource.GetID())

	cancel := task.(chan bool)
	cancel <- true
	return nil
}
