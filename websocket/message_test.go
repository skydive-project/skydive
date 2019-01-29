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
	"net/url"
	"testing"
	"time"

	proto "github.com/gogo/protobuf/proto"
	"github.com/skydive-project/skydive/common"
	shttp "github.com/skydive-project/skydive/http"
)

type fakeMessageServerSubscriptionHandler struct {
	common.RWMutex
	DefaultSpeakerEventHandler
	t             *testing.T
	server        *StructServer
	received      map[string]bool
	receivedCount int
}

type fakeMessageClientSubscriptionHandler struct {
	common.RWMutex
	DefaultSpeakerEventHandler
	t             *testing.T
	received      map[string]bool
	receivedCount int
	connected     int
}

func (f *fakeMessageServerSubscriptionHandler) OnConnected(c Speaker) {
	// wait first message received to be sure that the client can consume messages
	fnc := func() error {
		f.RLock()
		defer f.RUnlock()
		if f.receivedCount == 0 {
			return errors.New("Client not ready")
		}
		c.SendMessage(NewStructMessage("SrvValidNS", "SrvValidNSUnicast666", "AAA", "001"))
		c.SendMessage(NewStructMessage("SrvNotValidNS", "SrvNotValidNSUnicast2", "AAA", "001"))
		c.SendMessage(NewStructMessage("SrvValidNS", "SrvValidNSUnicast3", "AAA", "001"))

		f.server.BroadcastMessage(NewStructMessage("SrvValidNS", "SrvValidNSBroadcast1", "AAA", "001"))
		f.server.BroadcastMessage(NewStructMessage("SrvNotValidNS", "SrvNotValidNSBroacast2", "AAA", "001"))
		f.server.BroadcastMessage(NewStructMessage("SrvValidNS", "SrvValidNSBroadcast3", "AAA", "001"))

		return nil
	}
	go common.Retry(fnc, 5, time.Second)
}

func (f *fakeMessageServerSubscriptionHandler) OnStructMessage(c Speaker, m *StructMessage) {
	f.Lock()
	f.received[m.Type] = true
	f.receivedCount++
	f.Unlock()
}

func (f *fakeMessageClientSubscriptionHandler) OnConnected(c Speaker) {
	f.Lock()
	f.connected++
	f.Unlock()

	c.SendMessage(NewStructMessage("ClientValidNS", "ClientValidNS1", "AAA", "001"))
	c.SendMessage(NewStructMessage("ClientNotValidNS", "ClientNotValidNS2", "AAA", "001"))
	c.SendMessage(NewStructMessage("ClientValidNS", "ClientValidNS3", "AAA", "001"))
}

func (f *fakeMessageClientSubscriptionHandler) OnStructMessage(c Speaker, m *StructMessage) {
	f.Lock()
	f.received[m.Type] = true
	f.receivedCount++
	f.Unlock()
}

func TestMessageSubscription(t *testing.T) {
	httpserver := shttp.NewServer("myhost", common.AnalyzerService, "localhost", 59999, nil)

	httpserver.ListenAndServe()
	defer httpserver.Stop()

	wsserver := NewStructServer(NewServer(httpserver, "/wstest", shttp.NewNoAuthenticationBackend(), true, 100, 2*time.Second, 5*time.Second))

	serverHandler := &fakeMessageServerSubscriptionHandler{t: t, server: wsserver, received: make(map[string]bool)}
	wsserver.AddEventHandler(serverHandler)
	wsserver.AddStructMessageHandler(serverHandler, []string{"ClientValidNS"})

	wsserver.Start()
	defer wsserver.Stop()

	u, _ := url.Parse("ws://localhost:59999/wstest")

	opts := ClientOpts{
		QueueSize:        1000,
		WriteCompression: true,
	}

	wsclient := NewClient("myhost", common.AgentService, u, opts)

	wspool := NewStructClientPool("TestMessageSubscription")
	wspool.AddClient(wsclient)

	clientHandler := &fakeMessageClientSubscriptionHandler{t: t, received: make(map[string]bool)}
	wspool.AddEventHandler(clientHandler)

	wspool.AddStructMessageHandler(clientHandler, []string{"SrvValidNS"})

	wsclient.Start()
	defer wsclient.Stop()

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
