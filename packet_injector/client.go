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
	"errors"
	"fmt"
	"sync"

	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
)

type PacketInjectorClient struct {
	shttp.DefaultWSServerEventHandler
	WSServer       *shttp.WSServer
	replyChanMutex sync.RWMutex
	replyChan      map[string]chan *json.RawMessage
}

func (pc *PacketInjectorClient) OnMessage(c *shttp.WSClient, m shttp.WSMessage) {
	if m.Namespace != Namespace {
		return
	}

	pc.replyChanMutex.RLock()
	defer pc.replyChanMutex.RUnlock()

	ch, ok := pc.replyChan[m.UUID]
	if !ok {
		logging.GetLogger().Errorf("Unable to send reply, chan not found for %s, available: %v", m.UUID, pc.replyChan)
		return
	}

	ch <- m.Obj
}

func (pc *PacketInjectorClient) injectPacket(host string, pp *PacketParams, status chan *string) {
	msg := shttp.NewWSMessage(Namespace, "InjectPacket", pp)

	ch := make(chan *json.RawMessage)
	defer close(ch)

	pc.replyChanMutex.Lock()
	pc.replyChan[msg.UUID] = ch
	pc.replyChanMutex.Unlock()

	defer func() {
		pc.replyChanMutex.Lock()
		delete(pc.replyChan, msg.UUID)
		pc.replyChanMutex.Unlock()
	}()

	if !pc.WSServer.SendWSMessageTo(msg, host) {
		e := fmt.Sprintf("Unable to send message to agent: %s", host)
		logging.GetLogger().Errorf(e)
		status <- &e
		return
	}

	data := <-ch
	var reply string
	if err := json.Unmarshal([]byte(*data), &reply); err != nil {
		e := fmt.Sprintf("Error while reading reply from: %s", host)
		logging.GetLogger().Errorf(e)
		status <- &e
		return
	}

	status <- &reply
}

func (pc *PacketInjectorClient) InjectPacket(host string, pp *PacketParams) error {
	ch := make(chan *string, 1)

	go pc.injectPacket(host, pp, ch)
	reply := <-ch
	if *reply != "" {
		return errors.New(*reply)
	} else {
		return nil
	}
}

func NewPacketInjectorClient(w *shttp.WSServer) *PacketInjectorClient {
	pic := &PacketInjectorClient{
		WSServer:  w,
		replyChan: make(map[string]chan *json.RawMessage),
	}
	w.AddEventHandler(pic)

	return pic
}
