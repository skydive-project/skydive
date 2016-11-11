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

	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"
)

const (
	Namespace = "Packet_Injection"
)

type PacketInjectorServer struct {
	shttp.DefaultWSClientEventHandler
	WSAsyncClient *shttp.WSAsyncClient
	Graph         *graph.Graph
}

func (pis *PacketInjectorServer) injectPacket(msg shttp.WSMessage) {
	params := struct {
		SrcNode interface{}
		DstNode interface{}
		Type    string
		Payload string
		Count   int
	}{}
	if err := json.Unmarshal([]byte(*msg.Obj), &params); err != nil {
		logging.GetLogger().Errorf("Unable to decode packet inject param message %v", msg)
		return
	}

	var srcNode graph.Node
	if err := srcNode.Decode(params.SrcNode); err != nil {
		logging.GetLogger().Errorf("Unable to decode source node %s", err.Error())
		return
	}

	var dstNode graph.Node
	if err := dstNode.Decode(params.DstNode); err != nil {
		logging.GetLogger().Errorf("Unable to decode destination node %s", err.Error())
		return
	}

	pip := PacketParams{
		SrcNode: &srcNode,
		DstNode: &dstNode,
		Type:    params.Type,
		Payload: params.Payload,
		Count:   params.Count,
	}

	if err := InjectPacket(&pip, pis.Graph); err != nil {
		logging.GetLogger().Errorf("Failed to inject packet: %s", err.Error())
	}
}

func (pis *PacketInjectorServer) OnMessage(msg shttp.WSMessage) {
	if msg.Namespace != Namespace {
		return
	}

	switch msg.Type {
	case "InjectPacket":
		pis.injectPacket(msg)
	}
}

func NewServer(client *shttp.WSAsyncClient, graph *graph.Graph) *PacketInjectorServer {
	s := &PacketInjectorServer{
		WSAsyncClient: client,
		Graph:         graph,
	}
	client.AddEventHandler(s)

	return s
}
