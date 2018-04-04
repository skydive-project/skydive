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

type fakeServerSubscriptionHandler struct {
	sync.RWMutex
	DefaultWSSpeakerEventHandler
	t         *testing.T
	server    *WSServer
	received  int
	connected int
}

type fakeClientSubscriptionHandler struct {
	sync.RWMutex
	DefaultWSSpeakerEventHandler
	t         *testing.T
	received  int
	connected int
}

func (f *fakeServerSubscriptionHandler) OnConnected(c WSSpeaker) {
	f.Lock()
	f.connected++
	f.Unlock()
	f.t.Log("Server has a new client")
	c.SendMessage(WSRawMessage{})
}

func (f *fakeServerSubscriptionHandler) OnMessage(c WSSpeaker, m WSMessage) {
	f.Lock()
	f.received++
	f.Unlock()
	f.t.Log("Server received a new message")
}

func (f *fakeClientSubscriptionHandler) OnConnected(c WSSpeaker) {
	f.Lock()
	f.connected++
	f.Unlock()
	f.t.Log("Client sent a new message")
	c.SendMessage(WSRawMessage{})
}

func (f *fakeClientSubscriptionHandler) OnMessage(c WSSpeaker, m WSMessage) {
	f.Lock()
	f.received++
	f.Unlock()
	f.t.Log("Client received a new message")
}

func TestSubscription(t *testing.T) {
	httpserver := NewServer("myhost", common.AnalyzerService, "localhost", 59999, NewNoAuthenticationBackend(), "")

	go httpserver.ListenAndServe()
	defer httpserver.Stop()

	wsserver := NewWSServer(httpserver, "/wstest")

	serverHandler := &fakeServerSubscriptionHandler{t: t, server: wsserver, connected: 0, received: 0}
	wsserver.AddEventHandler(serverHandler)

	wsserver.Start()
	defer wsserver.Stop()

	wsclient := NewWSClient("myhost", common.AgentService, config.GetURL("ws", "localhost", 59999, "/wstest"), nil, http.Header{}, 1000)
	wspool := NewWSClientPool("TestSubscription")

	wspool.AddClient(wsclient)

	clientHandler := &fakeClientSubscriptionHandler{t: t, received: 0}
	wsclient.AddEventHandler(clientHandler)
	wspool.AddEventHandler(clientHandler)
	wsclient.Connect()
	defer wsclient.Disconnect()

	err := common.Retry(func() error {
		clientHandler.Lock()
		defer clientHandler.Unlock()
		serverHandler.Lock()
		defer serverHandler.Unlock()

		if clientHandler.connected != 2 {
			// clientHandler should be notified twice:
			// - by wsclient (WSAsyncClient)
			// - by wspool (WSClientPool)
			return fmt.Errorf("Client should have received 2 OnConnected events: %v", clientHandler.connected)
		}

		if clientHandler.received != 2 {
			// clientHandler should be notified twice:
			// - by wsclient (WSAsyncClient)
			// - by wspool (WSClientPool)
			// only one time, because only one message should be sent by the server
			return fmt.Errorf("Client should have received 2 OnMessage events: %v", clientHandler.received)
		}

		// serverHandler should be notified once:
		// - by wsserver (WSServer)
		if serverHandler.connected != 1 {
			return fmt.Errorf("Server should have received 1 OnConnected event: %v", serverHandler.connected)
		}

		// serverHandler should be notified by:
		// - by wsserver (WSServer)
		// 2 times, as there are 2 messages sent by the client
		if serverHandler.received != 2 {
			return fmt.Errorf("Server should have received 2 OnMessage event: %v", serverHandler.received)
		}

		return nil
	}, 5, time.Second)

	if err != nil {
		t.Error(err.Error())
	}
}
