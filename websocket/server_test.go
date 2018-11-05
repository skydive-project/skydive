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

package websocket

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/skydive-project/skydive/common"
	shttp "github.com/skydive-project/skydive/http"
)

type fakeServerSubscriptionHandler struct {
	common.RWMutex
	DefaultSpeakerEventHandler
	t         *testing.T
	server    *Server
	received  int
	connected int
}

type fakeClientSubscriptionHandler struct {
	common.RWMutex
	DefaultSpeakerEventHandler
	t         *testing.T
	received  int
	connected int
}

func (f *fakeServerSubscriptionHandler) OnConnected(c Speaker) {
	f.Lock()
	f.connected++
	f.Unlock()

	fnc := func() error {
		f.RLock()
		defer f.RUnlock()
		if f.received == 0 {
			return errors.New("Client not ready")
		}
		c.SendMessage(RawMessage{})

		return nil
	}
	go common.Retry(fnc, 5, time.Second)
}

func (f *fakeServerSubscriptionHandler) OnMessage(c Speaker, m Message) {
	f.Lock()
	f.received++
	f.Unlock()
}

func (f *fakeClientSubscriptionHandler) OnConnected(c Speaker) {
	f.Lock()
	f.connected++
	f.Unlock()
	c.SendMessage(RawMessage{})
}

func (f *fakeClientSubscriptionHandler) OnMessage(c Speaker, m Message) {
	f.Lock()
	f.received++
	f.Unlock()
}

func TestSubscription(t *testing.T) {
	httpServer := shttp.NewServer("myhost", common.AnalyzerService, "localhost", 59999, nil)

	go httpServer.ListenAndServe()
	defer httpServer.Stop()

	wsServer := NewServer(httpServer, "/wstest", shttp.NewNoAuthenticationBackend(), true, 100, 2*time.Second, 5*time.Second)

	serverHandler := &fakeServerSubscriptionHandler{t: t, server: wsServer, connected: 0, received: 0}
	wsServer.AddEventHandler(serverHandler)

	wsServer.Start()
	defer wsServer.Stop()

	u, _ := url.Parse("ws://localhost:59999/wstest")

	wsClient := NewClient("myhost", common.AgentService, u, nil, http.Header{}, 1000, true, nil)
	wsPool := NewClientPool("TestSubscription")

	wsPool.AddClient(wsClient)

	clientHandler := &fakeClientSubscriptionHandler{t: t, received: 0}
	wsClient.AddEventHandler(clientHandler)
	wsPool.AddEventHandler(clientHandler)
	wsClient.Connect()
	defer wsClient.Disconnect()

	err := common.Retry(func() error {
		clientHandler.Lock()
		defer clientHandler.Unlock()
		serverHandler.Lock()
		defer serverHandler.Unlock()

		if clientHandler.connected != 2 {
			// clientHandler should be notified twice:
			// - by client (Client)
			// - by pool (ClientPool)
			return fmt.Errorf("Client should have received 2 OnConnected events: %v", clientHandler.connected)
		}

		if clientHandler.received != 2 {
			// clientHandler should be notified twice:
			// - by client (Client)
			// - by pool (ClientPool)
			// only one time, because only one message should be sent by the server
			return fmt.Errorf("Client should have received 2 OnMessage events: %v", clientHandler.received)
		}

		// serverHandler should be notified once:
		// - by server (Server)
		if serverHandler.connected != 1 {
			return fmt.Errorf("Server should have received 1 OnConnected event: %v", serverHandler.connected)
		}

		// serverHandler should be notified by:
		// - by server (Server)
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
