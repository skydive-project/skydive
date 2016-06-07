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

package flow

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/nu7hatch/gouuid"

	shttp "github.com/redhat-cip/skydive/http"
	"github.com/redhat-cip/skydive/logging"
	"github.com/redhat-cip/skydive/topology/graph"
)

type TableClient struct {
	shttp.DefaultWSServerEventHandler
	WSServer       *shttp.WSServer
	replyChanMutex sync.RWMutex
	replyChan      map[string]chan *json.RawMessage
}

func (f *TableClient) OnMessage(c *shttp.WSClient, m shttp.WSMessage) {
	if m.Namespace != Namespace {
		return
	}

	f.replyChanMutex.RLock()
	defer f.replyChanMutex.RUnlock()

	ch, ok := f.replyChan[m.UUID]
	if !ok {
		logging.GetLogger().Errorf("Unable to send reply, chan not found for %s", m.UUID)
		return
	}

	ch <- m.Obj
}

func (f *TableClient) LookupFlowsByProbeNode(node *graph.Node) ([]*Flow, error) {
	u, _ := uuid.NewV4()

	tq := TableQuery{
		Obj: FlowSearchQuery{
			ProbeNodeUUID: string(node.ID),
		},
	}
	b, _ := json.Marshal(tq)
	raw := json.RawMessage(b)

	msg := shttp.WSMessage{
		Namespace: Namespace,
		Type:      "FlowSearchQuery",
		UUID:      u.String(),
		Obj:       &raw,
	}

	ch := make(chan *json.RawMessage)
	defer close(ch)

	f.replyChanMutex.Lock()
	f.replyChan[u.String()] = ch
	f.replyChanMutex.Unlock()

	defer func() {
		f.replyChanMutex.Lock()
		delete(f.replyChan, u.String())
		f.replyChanMutex.Unlock()
	}()

	ok := f.WSServer.SendWSMessageTo(msg, node.Host())
	if !ok {
		return nil, fmt.Errorf("Unable to send message to agent: %s", node.Host())
	}

	select {
	case raw := <-ch:
		reply := TableReply{
			Obj: &FlowSearchReply{},
		}

		err := json.Unmarshal([]byte(*raw), &reply)
		if err != nil {
			return nil, fmt.Errorf("Error returned while reading TableReply from: %s", node.Host())
		}

		if reply.Status != 200 {
			return nil, fmt.Errorf("Error %d TableReply from: %s", reply.Status, node.Host())
		}

		fr := reply.Obj.(*FlowSearchReply)

		return fr.Flows, nil
	case <-time.After(time.Second * 10):
		return nil, fmt.Errorf("Timeout while reading TableReply from: %s", node.Host())
	}
}

func NewTableClient(w *shttp.WSServer) *TableClient {
	tc := &TableClient{
		WSServer:  w,
		replyChan: make(map[string]chan *json.RawMessage),
	}
	w.AddEventHandler(tc)

	return tc
}
