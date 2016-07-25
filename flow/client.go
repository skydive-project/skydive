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
	"errors"
	"sync"
	"time"

	"github.com/nu7hatch/gouuid"

	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"
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

func (f *TableClient) lookupFlowsByNode(flowset chan *FlowSet, host string, uuids []string) {
	tq := TableQuery{
		Obj: FlowSearchQuery{
			NodeUUIDs: uuids,
		},
	}
	b, _ := json.Marshal(tq)
	raw := json.RawMessage(b)

	u, _ := uuid.NewV4()

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

	ok := f.WSServer.SendWSMessageTo(msg, host)
	if !ok {
		logging.GetLogger().Errorf("Unable to send message to agent: %s", host)
		flowset <- NewFlowSet()
		return
	}

	select {
	case raw := <-ch:
		reply := TableReply{}
		err := json.Unmarshal([]byte(*raw), &reply)
		if err != nil {
			logging.GetLogger().Errorf("Error returned while reading TableReply from: %s", host)
			break
		}

		if reply.Status != 200 {
			logging.GetLogger().Errorf("Error %d TableReply from: %s", reply.Status, host)
			break
		}

		fsr := []FlowSearchReply{}
		err = json.Unmarshal(reply.Obj, &fsr)
		if err != nil {
			logging.GetLogger().Errorf("Error returned while reading TableReply from: %s", host)
			break
		}

		fs := NewFlowSet()
		for _, reply := range fsr {
			fs.Merge(reply.FlowSet)
		}
		flowset <- fs

		return
	case <-time.After(time.Second * 10):
		logging.GetLogger().Errorf("Timeout while reading TableReply from: %s", host)
	}

	flowset <- NewFlowSet()
}

func (f *TableClient) LookupFlowsByNode(nodes ...*graph.Node) (*FlowSet, error) {
	if len(nodes) == 0 {
		return nil, errors.New("No node provided")
	}

	uuids := make(map[string][]string)
	for _, node := range nodes {
		uuids[node.Host()] = append(uuids[node.Host()], string(node.ID))
	}

	ch := make(chan *FlowSet)

	for host, uuids := range uuids {
		go f.lookupFlowsByNode(ch, host, uuids)
	}

	flowset := NewFlowSet()
	for i := 0; i != len(uuids); i++ {
		fs := <-ch
		flowset.Merge(fs)
	}

	return flowset, nil
}

func NewTableClient(w *shttp.WSServer) *TableClient {
	tc := &TableClient{
		WSServer:  w,
		replyChan: make(map[string]chan *json.RawMessage),
	}
	w.AddEventHandler(tc)

	return tc
}
