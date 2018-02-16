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
	"encoding/json"
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

type PacketInjectorReply struct {
	TrackingID string
	Error      string
}

type PacketInjectorClient struct {
	*etcd.MasterElector
	pool      shttp.WSJSONSpeakerPool
	watcher   apiServer.StoppableWatcher
	graph     *graph.Graph
	piHandler *apiServer.PacketInjectorAPI
}

func (pc *PacketInjectorClient) StopInjection(host string, uuid string) error {
	msg := shttp.NewWSJSONMessage(Namespace, "PIStopRequest", uuid)

	resp, err := pc.pool.Request(host, msg, shttp.DefaultRequestTimeout)
	if err != nil {
		return fmt.Errorf("Unable to send message to agent %s: %s", host, err.Error())
	}

	var reply PacketInjectorReply
	if err := json.Unmarshal([]byte(*resp.Obj), &reply); err != nil {
		return fmt.Errorf("Failed to parse response from %s: %s", host, err.Error())
	}

	if resp.Status != http.StatusOK {
		return errors.New(reply.Error)
	}

	return nil
}

func (pc *PacketInjectorClient) InjectPacket(host string, pp *PacketParams) (string, error) {
	msg := shttp.NewWSJSONMessage(Namespace, "PIRequest", pp)

	resp, err := pc.pool.Request(host, msg, shttp.DefaultRequestTimeout)
	if err != nil {
		return "", fmt.Errorf("Unable to send message to agent %s: %s", host, err.Error())
	}

	var reply PacketInjectorReply
	if err := json.Unmarshal([]byte(*resp.Obj), &reply); err != nil {
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

func (pc *PacketInjectorClient) requestToParams(ppr *types.PacketParamsReq) (string, *PacketParams, error) {
	pc.graph.RLock()
	defer pc.graph.RUnlock()

	srcNode := pc.getNode(ppr.Src)
	dstNode := pc.getNode(ppr.Dst)

	if srcNode == nil {
		return "", nil, errors.New("Not able to find a source node")
	}

	ipField := "IPV4"
	if ppr.Type == "icmp6" || ppr.Type == "tcp6" || ppr.Type == "udp6" {
		ipField = "IPV6"
	}

	if ppr.SrcIP == "" {
		ips, _ := srcNode.GetFieldStringList(ipField)
		if len(ips) == 0 {
			return "", nil, errors.New("No source IP in node and user input")
		}
		ppr.SrcIP = ips[0]
	} else {
		ppr.SrcIP = pc.normalizeIP(ppr.SrcIP, ipField)
	}

	if ppr.DstIP == "" {
		if dstNode != nil {
			ips, _ := dstNode.GetFieldStringList(ipField)
			if len(ips) == 0 {
				return "", nil, errors.New("No dest IP in node and user input")
			}
			ppr.DstIP = ips[0]
		} else {
			return "", nil, errors.New("Not able to find a dest node and dest IP also empty")
		}
	} else {
		ppr.DstIP = pc.normalizeIP(ppr.DstIP, ipField)
	}

	if ppr.SrcMAC == "" {
		if srcNode != nil {
			mac, _ := srcNode.GetFieldString("MAC")
			if mac == "" {
				return "", nil, errors.New("No source MAC in node and user input")
			}
			ppr.SrcMAC = mac
		} else {
			return "", nil, errors.New("Not able to find a source node and source MAC also empty")
		}
	}

	if ppr.DstMAC == "" {
		if dstNode != nil {
			mac, _ := dstNode.GetFieldString("MAC")
			if mac == "" {
				return "", nil, errors.New("No dest MAC in node and user input")
			}
			ppr.DstMAC = mac
		} else {
			return "", nil, errors.New("Not able to find a dest node and dest MAC also empty")
		}
	}

	if ppr.Type == "tcp4" || ppr.Type == "tcp6" {
		if ppr.SrcPort == 0 {
			ppr.SrcPort = rand.Int63n(max-min) + min
		}
		if ppr.DstPort == 0 {
			ppr.DstPort = rand.Int63n(max-min) + min
		}
	}

	pp := &PacketParams{
		UUID:      ppr.UUID,
		SrcNodeID: srcNode.ID,
		SrcIP:     ppr.SrcIP,
		SrcMAC:    ppr.SrcMAC,
		SrcPort:   ppr.SrcPort,
		DstIP:     ppr.DstIP,
		DstMAC:    ppr.DstMAC,
		DstPort:   ppr.DstPort,
		Type:      ppr.Type,
		Payload:   ppr.Payload,
		Count:     ppr.Count,
		Interval:  ppr.Interval,
		ID:        ppr.ICMPID,
	}

	if errs := validator.Validate(pp); errs != nil {
		return "", nil, errors.New("All the parms not set properly")
	}

	return srcNode.Host(), pp, nil
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
	ppr := resource.(*types.PacketParamsReq)
	switch action {
	case "create", "set":
		host, pp, err := pc.requestToParams(ppr)
		if err != nil {
			pc.piHandler.TrackingId <- ""
			logging.GetLogger().Errorf("Not able to parse request: %s", err.Error())
			pc.piHandler.BasicAPIHandler.Delete(ppr.UUID)
			return
		}
		trackingID, err := pc.InjectPacket(host, pp)
		if err != nil {
			pc.piHandler.TrackingId <- ""
			logging.GetLogger().Errorf("Not able to inject on host %s :: %s", host, err.Error())
			pc.piHandler.BasicAPIHandler.Delete(ppr.UUID)
			return
		}
		pc.piHandler.TrackingId <- trackingID
		ppr.TrackingID = trackingID
		ppr.StartTime = time.Now()
		pc.piHandler.BasicAPIHandler.Update(ppr.UUID, ppr)

		go pc.expirePI(ppr.UUID, time.Duration(ppr.Count*ppr.Interval)*time.Millisecond)
	case "expire", "delete":
		pc.graph.RLock()
		srcNode := pc.getNode(ppr.Src)
		pc.graph.RUnlock()
		if srcNode == nil {
			return
		}
		pc.StopInjection(srcNode.Host(), ppr.UUID)
	}
}

//Start the PI client
func (pc *PacketInjectorClient) Start() {
	pc.MasterElector.StartAndWait()
	pc.watcher = pc.piHandler.AsyncWatch(pc.onAPIWatcherEvent)
}

//Stop the PI client
func (pc *PacketInjectorClient) Stop() {
	pc.watcher.Stop()
	pc.MasterElector.Stop()
}

func (pc *PacketInjectorClient) setTimeouts() {
	injections := pc.piHandler.Index()
	for _, v := range injections {
		ppr := v.(*types.PacketParamsReq)
		validity := ppr.StartTime.Add(time.Duration(ppr.Count*ppr.Interval) * time.Millisecond)
		if validity.After(time.Now()) {
			elapsedTime := time.Now().Sub(ppr.StartTime)
			totalTime := time.Duration(ppr.Count*ppr.Interval) * time.Millisecond
			go pc.expirePI(ppr.UUID, totalTime-elapsedTime)
		} else {
			pc.piHandler.BasicAPIHandler.Delete(ppr.UUID)
		}
	}
}

func NewPacketInjectorClient(pool shttp.WSJSONSpeakerPool, etcdClient *etcd.Client, piHandler *apiServer.PacketInjectorAPI, g *graph.Graph) *PacketInjectorClient {
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
