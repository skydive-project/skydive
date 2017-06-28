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
	"testing"
	"time"

	"github.com/skydive-project/skydive/common"
)

type fakeServerSubscriptionHandler struct {
	DefaultWSServerEventHandler
	t        *testing.T
	server   *WSServer
	received map[string]bool
}

type fakeClientSubscriptionHandler struct {
	DefaultWSClientEventHandler
	t        *testing.T
	received map[string]bool
}

func (f *fakeServerSubscriptionHandler) OnRegisterClient(c *WSClient) {
	c.SendWSMessage(NewWSMessage("SrvValidNS", "SrvValidNSUnicast1", "AAA", "001"))
	c.SendWSMessage(NewWSMessage("SrvNotValidNS", "SrvNotValidNSUnicast2", "AAA", "001"))
	c.SendWSMessage(NewWSMessage("SrvValidNS", "SrvValidNSUnicast3", "AAA", "001"))

	f.server.BroadcastWSMessage(NewWSMessage("SrvValidNS", "SrvValidNSBroadcast1", "AAA", "001"))
	f.server.BroadcastWSMessage(NewWSMessage("SrvNotValidNS", "SrvNotValidNSBroacast2", "AAA", "001"))
	f.server.BroadcastWSMessage(NewWSMessage("SrvValidNS", "SrvValidNSBroadcast3", "AAA", "001"))
}

func (f *fakeServerSubscriptionHandler) OnMessage(c *WSClient, m WSMessage) {
	f.received[m.Type] = true
}

func (f *fakeClientSubscriptionHandler) OnConnected(c *WSAsyncClient) {
	c.SendWSMessage(NewWSMessage("ClientValidNS", "ClientValidNS1", "AAA", "001"))
	c.SendWSMessage(NewWSMessage("ClientNotValidNS", "ClientNotValidNS2", "AAA", "001"))
	c.SendWSMessage(NewWSMessage("ClientValidNS", "ClientValidNS3", "AAA", "001"))
}

func (f *fakeClientSubscriptionHandler) OnWSMessage(c *WSAsyncClient, m WSMessage) {
	f.received[m.Type] = true
}

func TestSubscription(t *testing.T) {
	httpserver := NewServer("myhost", common.AnalyzerService, "localhost", 59999, NewNoAuthenticationBackend())

	go httpserver.ListenAndServe()
	defer httpserver.Stop()

	wsserver := NewWSServer(httpserver, 10*time.Second, 100, time.Second, "/wstest")

	serverHandler := &fakeServerSubscriptionHandler{t: t, server: wsserver, received: make(map[string]bool)}
	wsserver.AddEventHandler(serverHandler, []string{"ClientValidNS"})

	go wsserver.ListenAndServe()
	defer wsserver.Stop()

	wsclient := NewWSMessageAsyncClient("myhost", common.AgentService, "localhost", 59999, "/wstest", nil)

	wspool := NewWSMessageAsyncClientPool()
	wspool.AddWSMessageAsyncClient(wsclient)

	clientHandler := &fakeClientSubscriptionHandler{t: t, received: make(map[string]bool)}
	wspool.AddMessageHandler(clientHandler, []string{"SrvValidNS"})
	wspool.AddEventHandler(clientHandler)

	wsclient.Connect()
	defer wsclient.Disconnect()

	err := common.Retry(func() error {
		if len(serverHandler.received) != 2 {
			return fmt.Errorf("Should have received 2 messages: %v", serverHandler.received)
		}

		if len(clientHandler.received) != 4 {
			return fmt.Errorf("Should have received 2 messages: %v", clientHandler.received)
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
