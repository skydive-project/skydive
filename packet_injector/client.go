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
	"net/http"
	"sync"
	"time"

	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
)

type PacketInjectorClient struct {
	shttp.DefaultWSServerEventHandler
	WSServer       *shttp.WSServer
	replyChanMutex sync.RWMutex
	replyChan      map[string]chan error
}

func (pc *PacketInjectorClient) OnMessage(c *shttp.WSClient, m shttp.WSMessage) {
	pc.replyChanMutex.RLock()
	defer pc.replyChanMutex.RUnlock()

	ch, ok := pc.replyChan[m.UUID]
	if !ok {
		logging.GetLogger().Errorf("Unable to send reply, chan not found for %s, available: %v", m.UUID, pc.replyChan)
		return
	}

	if m.Status >= http.StatusBadRequest {
		var reply string
		if err := json.Unmarshal([]byte(*m.Obj), &reply); err != nil {
			ch <- fmt.Errorf("Error while reading reply from: %s", c.Host)
		} else {
			ch <- errors.New(reply)
		}
	} else {
		ch <- nil
	}
}

func (pc *PacketInjectorClient) injectPacket(host string, pp *PacketParams) error {
	msg := shttp.NewWSMessage(Namespace, "PIRequest", pp)

	ch := make(chan error)
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
		return fmt.Errorf("Unable to send message to agent: %s", host)
	}

	var err error
	select {
	case err = <-ch:
	case <-time.After(time.Second * 10):
		err = fmt.Errorf("Timeout while reading PIReply from: %s", host)
	}
	return err
}

func (pc *PacketInjectorClient) InjectPacket(host string, pp *PacketParams) error {
	if err := pc.injectPacket(host, pp); err != nil {
		logging.GetLogger().Errorf(err.Error())
		return err
	}

	return nil
}

func NewPacketInjectorClient(w *shttp.WSServer) *PacketInjectorClient {
	pic := &PacketInjectorClient{
		WSServer:  w,
		replyChan: make(map[string]chan error),
	}
	w.AddEventHandler(pic, []string{Namespace})

	return pic
}
