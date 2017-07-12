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
	"bytes"
	"fmt"
	"net/http"

	"github.com/skydive-project/skydive/common"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"
)

const (
	// Namespace Packet_Injector
	Namespace = "Packet_Injector"
)

// PacketInjectorServer creates a packet injector server API
type PacketInjectorServer struct {
	shttp.DefaultWSClientEventHandler
	WSAsyncClientPool *shttp.WSAsyncClientPool
	Graph             *graph.Graph
}

func (pis *PacketInjectorServer) injectPacket(msg shttp.WSMessage) (string, error) {
	var params PacketParams
	if err := common.JSONDecode(bytes.NewBuffer([]byte(*msg.Obj)), &params); err != nil {
		return "", fmt.Errorf("Unable to decode packet inject param message %v", msg)
	}

	trackingID, err := InjectPacket(&params, pis.Graph)
	if err != nil {
		return "", fmt.Errorf("Failed to inject packet: %s", err.Error())
	}

	return trackingID, nil
}

// OnMessage event, websocket PIRequest message
func (pis *PacketInjectorServer) OnMessage(c *shttp.WSAsyncClient, msg shttp.WSMessage) {
	switch msg.Type {
	case "PIRequest":
		var reply *shttp.WSMessage
		trackingID, err := pis.injectPacket(msg)
		replyObj := &PacketInjectorReply{err: err, TrackingID: trackingID}
		if err != nil {
			logging.GetLogger().Error(err.Error())
			reply = msg.Reply(replyObj, "PIResult", http.StatusBadRequest)
		} else {
			reply = msg.Reply(replyObj, "PIResult", http.StatusOK)
		}

		c.SendWSMessage(reply)
	}
}

// NewServer creates a new packet injector server API based on websocket server
func NewServer(wspool *shttp.WSAsyncClientPool, graph *graph.Graph) *PacketInjectorServer {
	s := &PacketInjectorServer{
		WSAsyncClientPool: wspool,
		Graph:             graph,
	}
	wspool.AddEventHandler(s, []string{Namespace})

	return s
}
