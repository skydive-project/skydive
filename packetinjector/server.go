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
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"syscall"
	"time"

	"github.com/google/gopacket/layers"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/ondemand"
	"github.com/skydive-project/skydive/topology"

	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/ondemand/server"
	ws "github.com/skydive-project/skydive/websocket"
)

const (
	// Namespace PacketInjector
	Namespace = "PacketInjector"
)

type onDemandPacketInjectServer struct {
	sync.RWMutex
	graph *graph.Graph
}

func (o *onDemandPacketInjectServer) CreateTask(srcNode *graph.Node, resource types.Resource) (ondemand.Task, error) {
	logging.GetLogger().Debugf("Registering packet injection %s on %s", resource.ID(), srcNode.ID)

	pp := resource.(*PacketInjectionRequest)

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

	_, nsPath, err := topology.NamespaceFromNode(o.graph, srcNode)
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

	var packetForger PacketForger
	if len(pp.Pcap) > 0 {
		packetForger, err = NewPcapPacketGenerator(pp.Pcap)
	} else {
		encapType, _ := srcNode.GetFieldString("EncapType")
		packetForger, err = NewForgedPacketGenerator(pp, encapType)
	}

	metadata := &InjectionMetadata{
		PacketInjectionRequest: *pp,
		ID:    pp.UUID,
		State: "active",
	}

	if o.graph.UpdateMetadata(srcNode, "PacketInjections", func(obj interface{}) bool {
		captures := obj.(*Injections)
		*captures = append(*captures, metadata)
		return true
	}) == common.ErrFieldNotFound {
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

				logging.GetLogger().Debugf("Injecting packet (%d bytes) on interface %s", len(packet.data), ifName)
				if _, err := rawSocket.Write(packet.data); err != nil {
					if err == syscall.ENXIO {
						logging.GetLogger().Warningf("Write error on interface %s: %s", ifName, err)
					} else {
						logging.GetLogger().Errorf("Write error on interface %s: %s", ifName, err)
					}
				}
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

func (o *onDemandPacketInjectServer) RemoveTask(n *graph.Node, resource types.Resource, task ondemand.Task) error {
	logging.GetLogger().Debugf("Unregister packet injection %s on %s", n.ID, resource.ID())

	o.Lock()
	defer o.Unlock()

	cancel := task.(chan bool)
	cancel <- true
	return nil
}

func (o *onDemandPacketInjectServer) ResourceName() string {
	return "PacketInjection"
}

func (o *onDemandPacketInjectServer) DecodeMessage(msg json.RawMessage) (types.Resource, error) {
	var params PacketInjectionRequest
	if err := json.Unmarshal(msg, &params); err != nil {
		return nil, fmt.Errorf("Unable to decode packet inject param message %v", msg)
	}
	return &params, nil
}

// NewOnDemandInjectionServer creates a new Ondemand probes server based on graph and websocket
func NewOnDemandInjectionServer(g *graph.Graph, pool *ws.StructClientPool) (*server.OnDemandServer, error) {
	return server.NewOnDemandServer(g, pool, &onDemandPacketInjectServer{
		graph: g,
	})
}
