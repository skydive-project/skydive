/*
 * Copyright (C) 2016 Red Hat, Inc.
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

package packet_injector

import (
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"

	apiServer "github.com/skydive-project/skydive/api/server"
	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/etcd"
	ge "github.com/skydive-project/skydive/gremlin/traversal"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"
	"github.com/skydive-project/skydive/validator"
)

const (
	min = 1024
	max = 65535
)

// PacketInjectorReply describes the reply to a packet injection request
type PacketInjectorReply struct {
	TrackingID string
	Error      string
}

// PacketInjectorClient describes a packet injector client
type PacketInjectorClient struct {
	*etcd.MasterElector
	pool      shttp.WSStructSpeakerPool
	watcher   apiServer.StoppableWatcher
	graph     *graph.Graph
	piHandler *apiServer.PacketInjectorAPI
}

// StopInjection cancels a running packet injection
func (pc *PacketInjectorClient) StopInjection(host string, uuid string) error {
	msg := shttp.NewWSStructMessage(Namespace, "PIStopRequest", uuid)

	resp, err := pc.pool.Request(host, msg, shttp.DefaultRequestTimeout)
	if err != nil {
		return fmt.Errorf("Unable to send message to agent %s: %s", host, err.Error())
	}

	var reply PacketInjectorReply
	if err := resp.UnmarshalObj(&reply); err != nil {
		return fmt.Errorf("Failed to parse response from %s: %s", host, err.Error())
	}

	if resp.Status != http.StatusOK {
		return errors.New(reply.Error)
	}

	return nil
}

// InjectPackets issues a packet injection request and returns the expected
// tracking id
func (pc *PacketInjectorClient) InjectPackets(host string, pp *PacketInjectionParams) (string, error) {
	msg := shttp.NewWSStructMessage(Namespace, "PIRequest", pp)

	resp, err := pc.pool.Request(host, msg, shttp.DefaultRequestTimeout)
	if err != nil {
		return "", fmt.Errorf("Unable to send message to agent %s: %s", host, err.Error())
	}

	var reply PacketInjectorReply
	if err := resp.UnmarshalObj(&reply); err != nil {
		return "", fmt.Errorf("Failed to parse response from %s: %s", host, err.Error())
	}

	if resp.Status != http.StatusOK {
		return "", errors.New(reply.Error)
	}

	return reply.TrackingID, nil
}

func (pc *PacketInjectorClient) normalizeIP(ip, ipFamily string) string {
	if strings.Contains(ip, "/") {
		return ip
	}
	if ipFamily == "IPV4" {
		return ip + "/32"
	}
	return ip + "/64"
}

func (pc *PacketInjectorClient) getNode(gremlinQuery string) *graph.Node {
	res, err := ge.TopologyGremlinQuery(pc.graph, gremlinQuery)
	if err != nil {
		return nil
	}

	for _, value := range res.Values() {
		switch value.(type) {
		case *graph.Node:
			return value.(*graph.Node)
		default:
			return nil
		}
	}
	return nil
}

func (pc *PacketInjectorClient) requestToParams(pi *types.PacketInjection) (string, *PacketInjectionParams, error) {
	pc.graph.RLock()
	defer pc.graph.RUnlock()

	srcNode := pc.getNode(pi.Src)
	dstNode := pc.getNode(pi.Dst)

	if srcNode == nil {
		return "", nil, errors.New("Not able to find a source node")
	}

	ipField := "IPV4"
	if pi.Type == "icmp6" || pi.Type == "tcp6" || pi.Type == "udp6" {
		ipField = "IPV6"
	}

	if pi.SrcIP == "" {
		ips, _ := srcNode.GetFieldStringList(ipField)
		if len(ips) == 0 {
			return "", nil, errors.New("No source IP in node and user input")
		}
		pi.SrcIP = ips[0]
	} else {
		pi.SrcIP = pc.normalizeIP(pi.SrcIP, ipField)
	}

	if pi.DstIP == "" {
		if dstNode != nil {
			ips, _ := dstNode.GetFieldStringList(ipField)
			if len(ips) == 0 {
				return "", nil, errors.New("No dest IP in node and user input")
			}
			pi.DstIP = ips[0]
		} else {
			return "", nil, errors.New("Not able to find a dest node and dest IP also empty")
		}
	} else {
		pi.DstIP = pc.normalizeIP(pi.DstIP, ipField)
	}

	if pi.SrcMAC == "" {
		if srcNode != nil {
			mac, _ := srcNode.GetFieldString("MAC")
			if mac == "" {
				return "", nil, errors.New("No source MAC in node and user input")
			}
			pi.SrcMAC = mac
		} else {
			return "", nil, errors.New("Not able to find a source node and source MAC also empty")
		}
	}

	if pi.DstMAC == "" {
		if dstNode != nil {
			mac, _ := dstNode.GetFieldString("MAC")
			if mac == "" {
				return "", nil, errors.New("No dest MAC in node and user input")
			}
			pi.DstMAC = mac
		} else {
			return "", nil, errors.New("Not able to find a dest node and dest MAC also empty")
		}
	}

	if pi.Type == "tcp4" || pi.Type == "tcp6" {
		if pi.SrcPort == 0 {
			pi.SrcPort = rand.Int63n(max-min) + min
		}
		if pi.DstPort == 0 {
			pi.DstPort = rand.Int63n(max-min) + min
		}
	}

	pip := &PacketInjectionParams{
		UUID:      pi.UUID,
		SrcNodeID: srcNode.ID,
		SrcIP:     pi.SrcIP,
		SrcMAC:    pi.SrcMAC,
		SrcPort:   pi.SrcPort,
		DstIP:     pi.DstIP,
		DstMAC:    pi.DstMAC,
		DstPort:   pi.DstPort,
		Type:      pi.Type,
		Payload:   pi.Payload,
		Count:     pi.Count,
		Interval:  pi.Interval,
		ID:        pi.ICMPID,
		Increment: pi.Increment,
	}

	if errs := validator.Validate(pip); errs != nil {
		return "", nil, errors.New("All the parms not set properly")
	}

	return srcNode.Host(), pip, nil
}

func (pc *PacketInjectorClient) expirePI(id string, expireTime time.Duration) {
	time.Sleep(expireTime)
	pc.piHandler.BasicAPIHandler.Delete(id)
}

// OnStartAsMaster event
func (pc *PacketInjectorClient) OnStartAsMaster() {
}

// OnStartAsSlave event
func (pc *PacketInjectorClient) OnStartAsSlave() {
}

// OnSwitchToMaster event
func (pc *PacketInjectorClient) OnSwitchToMaster() {
	pc.setTimeouts()
}

// OnSwitchToSlave event
func (pc *PacketInjectorClient) OnSwitchToSlave() {
}

func (pc *PacketInjectorClient) onAPIWatcherEvent(action string, id string, resource types.Resource) {
	logging.GetLogger().Debugf("New watcher event %s for %s", action, id)
	pi := resource.(*types.PacketInjection)
	switch action {
	case "create", "set":
		host, pip, err := pc.requestToParams(pi)
		if err != nil {
			pc.piHandler.TrackingID <- ""
			logging.GetLogger().Errorf("Not able to parse request: %s", err.Error())
			pc.piHandler.BasicAPIHandler.Delete(pi.UUID)
			return
		}
		trackingID, err := pc.InjectPackets(host, pip)
		if err != nil {
			pc.piHandler.TrackingID <- ""
			logging.GetLogger().Errorf("Not able to inject on host %s :: %s", host, err.Error())
			pc.piHandler.BasicAPIHandler.Delete(pi.UUID)
			return
		}
		pc.piHandler.TrackingID <- trackingID
		pi.TrackingID = trackingID
		pi.StartTime = time.Now()
		pc.piHandler.BasicAPIHandler.Update(pi.UUID, pi)

		go pc.expirePI(pi.UUID, time.Duration(pi.Count*pi.Interval)*time.Millisecond)
	case "expire", "delete":
		pc.graph.RLock()
		srcNode := pc.getNode(pi.Src)
		pc.graph.RUnlock()
		if srcNode == nil {
			return
		}
		pc.StopInjection(srcNode.Host(), pi.UUID)
	}
}

// Start the packet injector client
func (pc *PacketInjectorClient) Start() {
	pc.MasterElector.StartAndWait()
	pc.watcher = pc.piHandler.AsyncWatch(pc.onAPIWatcherEvent)
}

// Stop the packet injector client
func (pc *PacketInjectorClient) Stop() {
	pc.watcher.Stop()
	pc.MasterElector.Stop()
}

func (pc *PacketInjectorClient) setTimeouts() {
	injections := pc.piHandler.Index()
	for _, v := range injections {
		pi := v.(*types.PacketInjection)
		validity := pi.StartTime.Add(time.Duration(pi.Count*pi.Interval) * time.Millisecond)
		if validity.After(time.Now()) {
			elapsedTime := time.Now().Sub(pi.StartTime)
			totalTime := time.Duration(pi.Count*pi.Interval) * time.Millisecond
			go pc.expirePI(pi.UUID, totalTime-elapsedTime)
		} else {
			pc.piHandler.BasicAPIHandler.Delete(pi.UUID)
		}
	}
}

// NewPacketInjectorClient returns a new packet injector client
func NewPacketInjectorClient(pool shttp.WSStructSpeakerPool, etcdClient *etcd.Client, piHandler *apiServer.PacketInjectorAPI, g *graph.Graph) *PacketInjectorClient {
	elector := etcd.NewMasterElectorFromConfig(common.AnalyzerService, "pi-client", etcdClient)

	pic := &PacketInjectorClient{
		MasterElector: elector,
		pool:          pool,
		piHandler:     piHandler,
		graph:         g,
	}

	elector.AddEventListener(pic)

	pic.setTimeouts()
	return pic
}
