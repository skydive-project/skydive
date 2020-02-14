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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/avast/retry-go"
	proto "github.com/gogo/protobuf/proto"
	"github.com/safchain/insanelock"

	"github.com/skydive-project/skydive/common"
	shttp "github.com/skydive-project/skydive/http"
)

func newServerOpts() ServerOpts {
	return ServerOpts{
		WriteCompression: true,
		QueueSize:        100,
		PingDelay:        2 * time.Second,
		PongTimeout:      5 * time.Second,
	}
}

func newClientOpts(ns ...string) ClientOpts {
	clientOpts := ClientOpts{
		QueueSize:        1000,
		WriteCompression: true,
		Headers:          http.Header{},
	}

	if len(ns) > 0 {
		clientOpts.Headers["X-Websocket-Namespace"] = ns
	}

	return clientOpts
}

func newStructMessage(ns, tp string) *StructMessage {
	return NewStructMessage(ns, tp, "AAA", "001")
}

func newHTTPServer() *shttp.Server {
	httpserver := shttp.NewServer(defaultHostID, common.AnalyzerService, host, port, nil, nil)
	httpserver.Start()
	return httpserver
}

func newWsClient(hostID string, ns ...string) *Client {
	u, _ := url.Parse(fmt.Sprintf("ws://%s:%d/%s", host, port, path))
	wsclient := NewClient(hostID, common.AgentService, u, newClientOpts(ns...))
	wsclient.Start()
	return wsclient
}

func newWsServer(httpserver *shttp.Server) *StructServer {
	wsserver := NewStructServer(NewServer(httpserver, "/"+path, newServerOpts()))
	wsserver.Start()
	return wsserver
}

type fakeMessageServerSubscriptionHandler struct {
	insanelock.RWMutex
	DefaultSpeakerEventHandler
	t             *testing.T
	server        *StructServer
	received      map[string]bool
	receivedCount int
}

type fakeMessageServerSubscriptionHandler2 struct {
	insanelock.RWMutex
	DefaultSpeakerEventHandler
	t             *testing.T
	server        *StructServer
	received      map[string]bool
	receivedCount int
}

type fakeMessageClientSubscriptionHandler struct {
	insanelock.RWMutex
	DefaultSpeakerEventHandler
	t        *testing.T
	received map[string]bool
}

type fakeMessageClientSubscriptionHandler2 struct {
	insanelock.RWMutex
	DefaultSpeakerEventHandler
	t        *testing.T
	received map[string]bool
}

func (f *fakeMessageServerSubscriptionHandler) OnConnected(c Speaker) {
	// wait first message received to be sure that the client can consume messages
	go retry.Do(func() error {
		f.RLock()
		defer f.RUnlock()
		if f.receivedCount == 0 {
			return errors.New("Client not ready")
		}
		c.SendMessage(newStructMessage("SrvValidNS", "SrvValidNSUnicast1"))
		c.SendMessage(newStructMessage("SrvNotValidNS", "SrvNotValidNSUnicast2"))
		c.SendMessage(newStructMessage("SrvValidNS", "SrvValidNSUnicast3"))

		f.server.BroadcastMessage(newStructMessage("SrvValidNS", "SrvValidNSBroadcast1"))
		f.server.BroadcastMessage(newStructMessage("SrvNotValidNS", "SrvNotValidNSBroacast2"))
		f.server.BroadcastMessage(newStructMessage("SrvValidNS", "SrvValidNSBroadcast3"))

		return nil
	}, retry.Delay(10*time.Millisecond))
}

func (f *fakeMessageServerSubscriptionHandler) OnStructMessage(c Speaker, m *StructMessage) {
	f.Lock()
	f.received[m.Type] = true
	f.receivedCount++
	f.Unlock()
}

func (f *fakeMessageServerSubscriptionHandler2) OnConnected(c Speaker) {
	// wait first message received to be sure that the client can consume messages
	go retry.Do(func() error {
		f.RLock()
		defer f.RUnlock()

		if f.receivedCount == 0 {
			return errors.New("Client not ready")
		}

		c.SendMessage(newStructMessage("flows/1", "SrvFlowUnicast1"))
		c.SendMessage(newStructMessage("flows/2", "SrvFlowUnicast2"))

		f.server.BroadcastMessage(newStructMessage("flows/1", "SrvFlowBroadcast1"))
		f.server.BroadcastMessage(newStructMessage("flows/2", "SrvFlowBroadcast2"))

		return nil
	}, retry.Delay(10*time.Millisecond))
}

func (f *fakeMessageServerSubscriptionHandler2) OnStructMessage(c Speaker, m *StructMessage) {
	f.Lock()
	defer f.Unlock()

	f.received[m.Type] = true
	f.receivedCount++
}

func (f *fakeMessageClientSubscriptionHandler) OnConnected(c Speaker) {
	c.SendMessage(newStructMessage("ClientValidNS", "ClientValidNS1"))
	c.SendMessage(newStructMessage("ClientNotValidNS", "ClientNotValidNS2"))
	c.SendMessage(newStructMessage("ClientValidNS", "ClientValidNS3"))
}

func (f *fakeMessageClientSubscriptionHandler) OnStructMessage(c Speaker, m *StructMessage) {
	f.Lock()
	f.received[m.Type] = true
	f.Unlock()
}

func (f *fakeMessageClientSubscriptionHandler2) OnConnected(c Speaker) {
	c.SendMessage(newStructMessage("ClientValidNS", "ClientValidNS1"))
}

func (f *fakeMessageClientSubscriptionHandler2) OnMessage(c Speaker, m Message) {
	// m is a rawmessage at this point
	bytes, _ := m.Bytes(RawProtocol)

	var structMsg StructMessage
	if err := structMsg.unmarshalByProtocol(bytes, c.GetClientProtocol()); err != nil {
		return
	}

	f.Lock()
	f.received[structMsg.Namespace] = true
	f.Unlock()
}

func TestMessageSubscription1(t *testing.T) {
	httpserver := newHTTPServer()
	defer httpserver.Stop()

	wsserver := newWsServer(httpserver)
	defer wsserver.Stop()

	serverHandler := &fakeMessageServerSubscriptionHandler{t: t, server: wsserver, received: make(map[string]bool)}
	wsserver.AddEventHandler(serverHandler)
	wsserver.AddStructMessageHandler(serverHandler, []string{"ClientValidNS"})

	wsclient := newWsClient(defaultHostID)
	defer wsclient.Stop()

	wspool := NewStructClientPool("TestMessageSubscription", PoolOpts{})
	wspool.AddClient(wsclient)

	clientHandler := &fakeMessageClientSubscriptionHandler{t: t, received: make(map[string]bool)}
	wspool.AddEventHandler(clientHandler)

	wspool.AddStructMessageHandler(clientHandler, []string{"SrvValidNS"})

	err := retry.Do(func() error {
		clientHandler.Lock()
		defer clientHandler.Unlock()
		serverHandler.Lock()
		defer serverHandler.Unlock()

		if len(serverHandler.received) != 2 {
			return fmt.Errorf("Server should have received 2 message types: %v", serverHandler.received)
		}

		if len(clientHandler.received) != 4 {
			return fmt.Errorf("Client should have received 4 message types: %v", clientHandler.received)
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
	}, retry.Delay(10*time.Millisecond))

	if err != nil {
		t.Error(err)
	}
}

func TestMessageSubscription2(t *testing.T) {
	httpserver := newHTTPServer()
	defer httpserver.Stop()

	wsserver := newWsServer(httpserver)
	defer wsserver.Stop()

	serverHandler := &fakeMessageServerSubscriptionHandler{t: t, server: wsserver, received: make(map[string]bool)}
	wsserver.AddEventHandler(serverHandler)
	wsserver.AddStructMessageHandler(serverHandler, []string{"ClientValidNS"})

	wsclient := newWsClient(defaultHostID, "SrvValidNS")
	defer wsclient.Stop()

	wspool := NewStructClientPool("TestMessageSubscription", PoolOpts{})
	wspool.AddClient(wsclient)

	clientHandler := &fakeMessageClientSubscriptionHandler2{t: t, received: make(map[string]bool)}
	wspool.AddEventHandler(clientHandler)

	err := retry.Do(func() error {
		clientHandler.Lock()
		defer clientHandler.Unlock()

		if len(clientHandler.received) != 1 {
			return fmt.Errorf("Client should have received 1 message namespace: %v", clientHandler.received)
		}

		if _, ok := clientHandler.received["SrvNotValidNS"]; ok {
			return fmt.Errorf("Received message from wrong namespace: %v", serverHandler.received)
		}

		return nil
	}, retry.Delay(10*time.Millisecond))

	if err != nil {
		t.Error(err)
	}
}

func TestMessageSubscription3(t *testing.T) {
	httpserver := newHTTPServer()
	defer httpserver.Stop()

	wsserver := newWsServer(httpserver)
	defer wsserver.Stop()

	serverHandler := &fakeMessageServerSubscriptionHandler2{t: t, server: wsserver, received: make(map[string]bool)}
	wsserver.AddEventHandler(serverHandler)
	wsserver.AddStructMessageHandler(serverHandler, []string{"ClientValidNS"})

	wspool1 := NewStructClientPool("TestMessageSubscription", PoolOpts{})
	clientHandler1 := &fakeMessageClientSubscriptionHandler2{t: t, received: make(map[string]bool)}
	wspool1.AddEventHandler(clientHandler1)
	wsclient1 := newWsClient(defaultHostID+"1", "flows/1")
	defer wsclient1.Stop()
	wspool1.AddClient(wsclient1)

	wspool2 := NewStructClientPool("TestMessageSubscription", PoolOpts{})
	clientHandler2 := &fakeMessageClientSubscriptionHandler2{t: t, received: make(map[string]bool)}
	wspool2.AddEventHandler(clientHandler2)
	wsclient2 := newWsClient(defaultHostID+"2", "flows/2")
	defer wsclient2.Stop()
	wspool2.AddClient(wsclient2)

	err := retry.Do(func() error {
		clientHandler1.Lock()
		defer clientHandler1.Unlock()

		if len(clientHandler1.received) != 1 {
			return fmt.Errorf("Client should have received 1 message namespace: %v", clientHandler1.received)
		}

		if _, ok := clientHandler1.received["flows/2"]; ok {
			return fmt.Errorf("Received message from wrong namespace: %v", serverHandler.received)
		}

		return nil
	}, retry.Delay(10*time.Millisecond))

	if err != nil {
		t.Error(err)
	}

	err = retry.Do(func() error {
		clientHandler2.Lock()
		defer clientHandler2.Unlock()

		if len(clientHandler2.received) != 1 {
			return fmt.Errorf("Client should have received 1 message namespace: %v", clientHandler2.received)
		}

		if _, ok := clientHandler2.received["flows/1"]; ok {
			return fmt.Errorf("Received message from wrong namespace: %v", serverHandler.received)
		}

		return nil
	}, retry.Delay(10*time.Millisecond))

	if err != nil {
		t.Error(err)
	}
}

type fakeJSONObj struct {
	Desc string
}

func TestMessageJsonProtocol(t *testing.T) {
	obj := fakeJSONObj{
		Desc: "json",
	}

	msg := NewStructMessage("test", "test", &obj)

	// first test a full json
	b, err := msg.Bytes(JSONProtocol)
	if err != nil {
		t.Error(err)
	}

	var i map[string]interface{}
	if err = json.Unmarshal(b, &i); err != nil {
		t.Error(err)
	}

	if i["Obj"].(map[string]interface{})["Desc"].(string) != "json" {
		t.Error("wrong json format")
	}

	if err = msg.unmarshalByProtocol(b, JSONProtocol); err != nil {
		t.Error(err)
	}

	if err = json.Unmarshal(msg.Obj, &obj); err != nil {
		t.Error(err)
	}

	if obj.Desc != "json" {
		t.Error("unexpected value")
	}
}

type fakeProtobufObj struct {
	Desc string `protobuf:"bytes,1,opt,name=Desc,proto3"`
}

func (f *fakeProtobufObj) Reset()         {}
func (f *fakeProtobufObj) String() string { return "" }
func (f *fakeProtobufObj) ProtoMessage()  {}

func (f *fakeProtobufObj) Marshal() ([]byte, error) {
	return []byte{0x11, 0x07, 0x74, 0x65, 0x73, 0x74, 0x69, 0x6e, 0x67}, nil
}

func (f *fakeProtobufObj) Unmarshal(b []byte) error {
	f.Desc = "protobuf unmarshal"
	return nil
}

func TestMessageProtobufProtocol(t *testing.T) {
	// test fallback first
	jsonObj := fakeJSONObj{
		Desc: "json",
	}

	msg := NewStructMessage("test", "test", &jsonObj)

	// first test a json object
	b, err := msg.Bytes(ProtobufProtocol)
	if err != nil {
		t.Error(err)
	}

	if err = msg.unmarshalByProtocol(b, ProtobufProtocol); err != nil {
		t.Error(err)
	}

	if err = json.Unmarshal(msg.Obj, &jsonObj); err != nil {
		t.Error(err)
	}

	if jsonObj.Desc != "json" {
		t.Error("unexpected value")
	}

	// test with full protobuf
	pbObj := fakeProtobufObj{
		Desc: "protobuf",
	}

	msg = NewStructMessage("test", "test", &pbObj)

	b, err = msg.Bytes(ProtobufProtocol)
	if err != nil {
		t.Error(err)
	}

	if err = msg.unmarshalByProtocol(b, ProtobufProtocol); err != nil {
		t.Error(err)
	}

	if err = proto.Unmarshal(msg.Obj, &pbObj); err != nil {
		t.Error(err)
	}

	if pbObj.Desc != "protobuf unmarshal" {
		t.Error("Unexpected desc")
	}
}
