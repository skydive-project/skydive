/*
 * Copyright (C) 2017 Red Hat, Inc.
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

package websocket

import (
	"errors"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/avast/retry-go"
	"github.com/safchain/insanelock"

	"github.com/skydive-project/skydive/common"
	shttp "github.com/skydive-project/skydive/http"
)

type fakeServerSubscriptionHandler struct {
	insanelock.RWMutex
	DefaultSpeakerEventHandler
	t         *testing.T
	server    *Server
	received  int
	connected int
}

type fakeClientSubscriptionHandler struct {
	insanelock.RWMutex
	DefaultSpeakerEventHandler
	t         *testing.T
	received  int
	connected int
}

func (f *fakeServerSubscriptionHandler) OnConnected(c Speaker) {
	f.Lock()
	f.connected++
	f.Unlock()

	go retry.Do(func() error {
		f.RLock()
		defer f.RUnlock()
		if f.received == 0 {
			return errors.New("Client not ready")
		}
		c.SendMessage(RawMessage{})

		return nil
	}, retry.Delay(10*time.Millisecond))
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

const (
	defaultHostID = "myhost"
	host          = "localhost"
	port          = 59999
	path          = "wstest"
)

type testServer struct {
	hostID     string
	httpServer *shttp.Server
	wsServer   *Server
	handler    *fakeServerSubscriptionHandler
	t          *testing.T
}

func newTestServer(t *testing.T, hostID ...string) *testServer {
	s := &testServer{t: t}
	s.hostID = defaultHostID
	if len(hostID) > 0 {
		s.hostID = hostID[0]
	}
	return s
}

func (s *testServer) start() {
	httpServer := shttp.NewServer(s.hostID, common.AnalyzerService, host, port, nil, nil)

	httpServer.Start()
	s.httpServer = httpServer

	serverOpts := ServerOpts{
		WriteCompression: true,
		QueueSize:        100,
		PingDelay:        2 * time.Second,
		PongTimeout:      5 * time.Second,
	}

	wsServer := NewServer(httpServer, "/"+path, shttp.NewNoAuthenticationBackend(), serverOpts)

	handler := &fakeServerSubscriptionHandler{t: s.t, server: wsServer}
	wsServer.AddEventHandler(handler)
	s.handler = handler

	wsServer.Start()
	s.wsServer = wsServer
}

func (s *testServer) stop() {
	if s.wsServer != nil {
		s.wsServer.Stop()
	}
	if s.httpServer != nil {
		s.httpServer.Stop()
	}
}

func (s *testServer) test(numClients int) error {
	// serverHandler should be notified once:
	// - by server (Server)
	if s.handler.connected != numClients {
		return fmt.Errorf("Server should have received %d OnConnected event: %v", numClients, s.handler.connected)
	}

	// serverHandler should be notified by:
	// - by server (Server)
	// 2 times, as there are 2 messages sent by the client
	if s.handler.received != 2*numClients {
		return fmt.Errorf("Server should have received %d OnMessage event: %v", 2*numClients, s.handler.received)
	}

	return nil
}

type testClient struct {
	hostID   string
	wsClient *Client
	handler  *fakeClientSubscriptionHandler
	t        *testing.T
}

func newTestClient(t *testing.T, hostID ...string) *testClient {
	c := &testClient{t: t}
	c.hostID = defaultHostID
	if len(hostID) > 0 {
		c.hostID = hostID[0]
	}
	return c
}

func (c *testClient) start() {
	u, _ := url.Parse(fmt.Sprintf("ws://%s:%d/%s", host, port, path))

	opts := ClientOpts{
		QueueSize:        1000,
		WriteCompression: true,
	}

	wsClient := NewClient(c.hostID, common.AgentService, u, opts)
	wsPool := NewClientPool("TestSubscription", PoolOpts{})

	wsPool.AddClient(wsClient)

	handler := &fakeClientSubscriptionHandler{t: c.t}
	wsClient.AddEventHandler(handler)
	wsPool.AddEventHandler(handler)
	c.handler = handler

	wsClient.Start()
	c.wsClient = wsClient
}

func (c *testClient) stop() {
	if c.wsClient != nil {
		c.wsClient.Stop()
	}
}

func (c *testClient) test() error {
	if c.handler.connected != 2 {
		// clientHandler should be notified twice:
		// - by client (Client)
		// - by pool (ClientPool)
		return fmt.Errorf("Client should have received 2 OnConnected events: %v", c.handler.connected)
	}

	if c.handler.received != 2 {
		// clientHandler should be notified twice:
		// - by client (Client)
		// - by pool (ClientPool)
		// only one time, because only one message should be sent by the server
		return fmt.Errorf("Client should have received 2 OnMessage events: %v", c.handler.received)
	}

	return nil
}

func TestSubscription(t *testing.T) {
	server := newTestServer(t)
	server.start()
	defer server.stop()

	client := newTestClient(t)
	client.start()
	defer client.stop()

	err := retry.Do(func() error {
		client.handler.Lock()
		defer client.handler.Unlock()

		server.handler.Lock()
		defer server.handler.Unlock()

		if err := client.test(); err != nil {
			return err
		}

		if err := server.test(1); err != nil {
			return err
		}

		return nil
	}, retry.Delay(10*time.Millisecond))

	if err != nil {
		t.Error(err.Error())
	}
}

func TestSubscriptionMultiClient(t *testing.T) {
	const numClients = 10

	server := newTestServer(t)
	server.start()
	defer server.stop()

	client := [numClients]*testClient{}

	for i := 0; i < numClients; i++ {
		client[i] = newTestClient(t, "" /* avoid bad handshake due to conflicting host_id */)
		client[i].start()
		defer client[i].stop()
	}

	err := retry.Do(func() error {
		for i := 0; i < numClients; i++ {
			client[i].handler.Lock()
			defer client[i].handler.Unlock()
		}

		server.handler.Lock()
		defer server.handler.Unlock()

		for i := 0; i < numClients; i++ {
			if err := client[i].test(); err != nil {
				return err
			}
		}

		if err := server.test(numClients); err != nil {
			return err
		}

		return nil
	}, retry.Delay(10*time.Millisecond))

	if err != nil {
		t.Error(err.Error())
	}
}
