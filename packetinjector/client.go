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

package packetinjector

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
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"
	"github.com/skydive-project/skydive/validator"
	ws "github.com/skydive-project/skydive/websocket"
)

const (
	min = 1024
	max = 65535
)

// Reply describes the reply to a packet injection request
type Reply struct {
	TrackingID string
	Error      string
}

// Client describes a packet injector client
type Client struct {
	*etcd.MasterElector
	pool      ws.StructSpeakerPool
	watcher   apiServer.StoppableWatcher
	graph     *graph.Graph
	piHandler *apiServer.PacketInjectorAPI
}

// StopInjection cancels a running packet injection
func (pc *Client) StopInjection(host string, uuid string) error {
	msg := ws.NewStructMessage(Namespace, "PIStopRequest", uuid)

	resp, err := pc.pool.Request(host, msg, ws.DefaultRequestTimeout)
	if err != nil {
		return fmt.Errorf("Unable to send message to agent %s: %s", host, err.Error())
	}

	var reply Reply
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
func (pc *Client) InjectPackets(host string, pp *PacketInjectionParams) (string, error) {
	msg := ws.NewStructMessage(Namespace, "PIRequest", pp)

	resp, err := pc.pool.Request(host, msg, ws.DefaultRequestTimeout)
	if err != nil {
		return "", fmt.Errorf("Unable to send message to agent %s: %s", host, err.Error())
	}

	var reply Reply
	if err := resp.UnmarshalObj(&reply); err != nil {
		return "", fmt.Errorf("Failed to parse response from %s: %s", host, err.Error())
	}

	if resp.Status != http.StatusOK {
		return "", errors.New(reply.Error)
	}

	return reply.TrackingID, nil
}

func (pc *Client) normalizeIP(ip, ipFamily string) string {
	if strings.Contains(ip, "/") {
		return ip
	}
	if ipFamily == "IPV4" {
		return ip + "/32"
	}
	return ip + "/64"
}

func (pc *Client) getNode(gremlinQuery string) *graph.Node {
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

func (pc *Client) requestToParams(pi *types.PacketInjection) (string, *PacketInjectionParams, error) {
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
		return "", nil, fmt.Errorf("All the params were not set properly: %s", errs)
	}

	return srcNode.Host(), pip, nil
}

func (pc *Client) expirePI(id string, expireTime time.Duration) {
	time.Sleep(expireTime)
	pc.piHandler.BasicAPIHandler.Delete(id)
}

// OnStartAsMaster event
func (pc *Client) OnStartAsMaster() {
}

// OnStartAsSlave event
func (pc *Client) OnStartAsSlave() {
}

// OnSwitchToMaster event
func (pc *Client) OnSwitchToMaster() {
	pc.setTimeouts()
}

// OnSwitchToSlave event
func (pc *Client) OnSwitchToSlave() {
}

func (pc *Client) onAPIWatcherEvent(action string, id string, resource types.Resource) {
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
func (pc *Client) Start() {
	pc.MasterElector.StartAndWait()
	pc.watcher = pc.piHandler.AsyncWatch(pc.onAPIWatcherEvent)
}

// Stop the packet injector client
func (pc *Client) Stop() {
	pc.watcher.Stop()
	pc.MasterElector.Stop()
}

func (pc *Client) setTimeouts() {
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

// NewClient returns a new packet injector client
func NewClient(pool ws.StructSpeakerPool, etcdClient *etcd.Client, piHandler *apiServer.PacketInjectorAPI, g *graph.Graph) *Client {
	elector := etcd.NewMasterElectorFromConfig(common.AnalyzerService, "pi-client", etcdClient)

	pic := &Client{
		MasterElector: elector,
		pool:          pool,
		piHandler:     piHandler,
		graph:         g,
	}

	elector.AddEventListener(pic)

	pic.setTimeouts()
	return pic
}
