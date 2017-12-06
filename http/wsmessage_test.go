/*
 * Copyright (C) 2017 Red Hat, Inc.
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

package http

import (
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
)

type fakeWSMessageServerSubscriptionHandler struct {
	sync.RWMutex
	DefaultWSSpeakerEventHandler
	t             *testing.T
	server        *WSJSONServer
	received      map[string]bool
	receivedCount int
}

type fakeWSMessageClientSubscriptionHandler struct {
	sync.RWMutex
	DefaultWSSpeakerEventHandler
	t             *testing.T
	received      map[string]bool
	receivedCount int
	connected     int
}

func (f *fakeWSMessageServerSubscriptionHandler) OnConnected(c WSSpeaker) {
	c.SendMessage(NewWSJSONMessage("SrvValidNS", "SrvValidNSUnicast666", "AAA", "001"))
	c.SendMessage(NewWSJSONMessage("SrvNotValidNS", "SrvNotValidNSUnicast2", "AAA", "001"))
	c.SendMessage(NewWSJSONMessage("SrvValidNS", "SrvValidNSUnicast3", "AAA", "001"))

	f.server.BroadcastMessage(NewWSJSONMessage("SrvValidNS", "SrvValidNSBroadcast1", "AAA", "001"))
	f.server.BroadcastMessage(NewWSJSONMessage("SrvNotValidNS", "SrvNotValidNSBroacast2", "AAA", "001"))
	f.server.BroadcastMessage(NewWSJSONMessage("SrvValidNS", "SrvValidNSBroadcast3", "AAA", "001"))
}

func (f *fakeWSMessageServerSubscriptionHandler) OnWSJSONMessage(c WSSpeaker, m *WSJSONMessage) {
	f.Lock()
	f.received[m.Type] = true
	f.receivedCount++
	f.Unlock()
}

func (f *fakeWSMessageClientSubscriptionHandler) OnConnected(c WSSpeaker) {
	f.Lock()
	f.connected++
	f.Unlock()

	c.SendMessage(NewWSJSONMessage("ClientValidNS", "ClientValidNS1", "AAA", "001"))
	c.SendMessage(NewWSJSONMessage("ClientNotValidNS", "ClientNotValidNS2", "AAA", "001"))
	c.SendMessage(NewWSJSONMessage("ClientValidNS", "ClientValidNS3", "AAA", "001"))
}

func (f *fakeWSMessageClientSubscriptionHandler) OnWSJSONMessage(c WSSpeaker, m *WSJSONMessage) {
	f.Lock()
	f.received[m.Type] = true
	f.receivedCount++
	f.Unlock()
}

func TestWSMessageSubscription(t *testing.T) {
	httpserver := NewServer("myhost", common.AnalyzerService, "localhost", 59999, NewNoAuthenticationBackend(), "")

	go httpserver.ListenAndServe()
	defer httpserver.Stop()

	wsserver := NewWSJSONServer(NewWSServer(httpserver, "/wstest"))

	serverHandler := &fakeWSMessageServerSubscriptionHandler{t: t, server: wsserver, received: make(map[string]bool)}
	wsserver.AddEventHandler(serverHandler)
	wsserver.AddJSONMessageHandler(serverHandler, []string{"ClientValidNS"})

	wsserver.Start()
	defer wsserver.Stop()

	wsclient := NewWSClient("myhost", common.AgentService, config.GetURL("ws", "localhost", 59999, "/wstest"), nil, http.Header{}, 1000)

	wspool := NewWSJSONClientPool()
	wspool.AddClient(wsclient)

	clientHandler := &fakeWSMessageClientSubscriptionHandler{t: t, received: make(map[string]bool)}
	wspool.AddEventHandler(clientHandler)

	wspool.AddJSONMessageHandler(clientHandler, []string{"SrvValidNS"})

	wsclient.Connect()
	defer wsclient.Disconnect()

	err := common.Retry(func() error {
		clientHandler.Lock()
		defer clientHandler.Unlock()
		serverHandler.Lock()
		defer serverHandler.Unlock()

		if len(serverHandler.received) != 2 {
			return fmt.Errorf("Server should have received 2 messages: %v", serverHandler.received)
		}

		if len(clientHandler.received) != 4 {
			return fmt.Errorf("Client should have received 4 messages: %v", clientHandler.received)
		}

		if _, ok := serverHandler.received["ClientNotValidNS2"]; ok {
			return fmt.Errorf("Received message from wrong namespace: %v", serverHandler.received)
		}

		if _, ok := clientHandler.received["SrvNotValidNSUnicast2"]; ok {
			return fmt.Errorf("Received message from wrong namespace: %v", serverHandler.received)
		}

		if _, ok := clientHandler.received["SrvNotValidNSBroacast2"]; ok {
			return fmt.Errorf("Received message from wrong namespace: %v", serverHandler.received)
		}

		return nil
	}, 5, time.Second)

	if err != nil {
		t.Error(err.Error())
	}
}
