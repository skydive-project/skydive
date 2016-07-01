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
	"sync"
	"time"

	"github.com/nu7hatch/gouuid"

	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
)

type TableClient struct {
	shttp.DefaultWSServerEventHandler
	WSServer       *shttp.WSServer
	replyChanMutex sync.RWMutex
	replyChan      map[string]chan *json.RawMessage
}

type HostNodeIDMap map[string][]string

func (f *TableClient) OnMessage(c *shttp.WSClient, m shttp.WSMessage) {
	if m.Namespace != Namespace {
		return
	}

	f.replyChanMutex.RLock()
	defer f.replyChanMutex.RUnlock()

	ch, ok := f.replyChan[m.UUID]
	if !ok {
		logging.GetLogger().Errorf("Unable to send reply, chan not found for %s, available: %v", m.UUID, f.replyChan)
		return
	}

	ch <- m.Obj
}

func (f *TableClient) lookupFlowsByNodes(flowset chan *FlowSet, host string, uuids []string) {
	terms := make([]Term, len(uuids)*3)
	for i, uuid := range uuids {
		terms[i*3] = Term{Key: "ProbeNodeUUID", Value: uuid}
		terms[i*3+1] = Term{Key: "IfSrcNodeUUID", Value: uuid}
		terms[i*3+2] = Term{Key: "IfDstNodeUUID", Value: uuid}
	}
	tq := TableQuery{
		Obj: Filters{Term: TermFilter{
			Op:    OR,
			Terms: terms,
		}},
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

func (f *TableClient) LookupFlowsByNodes(hnmap HostNodeIDMap) (*FlowSet, error) {
	ch := make(chan *FlowSet, len(hnmap))

	for host, uuids := range hnmap {
		go f.lookupFlowsByNodes(ch, host, uuids)
	}

	flowset := NewFlowSet()
	for i := 0; i != len(hnmap); i++ {
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
