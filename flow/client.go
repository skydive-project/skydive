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
	"net/http"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"

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

func (f *TableClient) lookupFlows(flowset chan *FlowSet, host string, filter *Filter) {
	obj, _ := proto.Marshal(&FlowSearchQuery{Filter: filter})
	tq := TableQuery{
		Type: "FlowSearchQuery",
		Obj:  obj,
	}
	msg := shttp.NewWSMessage(Namespace, "TableQuery", tq)

	ch := make(chan *json.RawMessage)
	defer close(ch)

	f.replyChanMutex.Lock()
	f.replyChan[msg.UUID] = ch
	f.replyChanMutex.Unlock()

	defer func() {
		f.replyChanMutex.Lock()
		delete(f.replyChan, msg.UUID)
		f.replyChanMutex.Unlock()
	}()

	if !f.WSServer.SendWSMessageTo(msg, host) {
		logging.GetLogger().Errorf("Unable to send message to agent: %s", host)
		flowset <- NewFlowSet()
		return
	}

	select {
	case raw := <-ch:
		var reply TableReply
		if err := json.Unmarshal([]byte(*raw), &reply); err != nil {
			logging.GetLogger().Errorf("Error returned while reading TableReply from: %s", host)
			break
		}

		if reply.Status != http.StatusOK {
			logging.GetLogger().Errorf("Error %d TableReply from: %s", reply.Status, host)
			break
		}

		fs := NewFlowSet()
		for _, b := range reply.Obj {
			var fsr FlowSearchReply
			if err := proto.Unmarshal(b, &fsr); err != nil {
				logging.GetLogger().Errorf("Unable to decode flow search reply from: %s", host)
				continue
			}

			fs.Merge(fsr.FlowSet)
		}
		flowset <- fs

		return
	case <-time.After(time.Second * 10):
		logging.GetLogger().Errorf("Timeout while reading TableReply from: %s", host)
	}

	flowset <- NewFlowSet()
}

func (f *TableClient) LookupFlows(filter *Filter) (*FlowSet, error) {
	clients := f.WSServer.GetClientsByType("skydive-agent")
	ch := make(chan *FlowSet, len(clients))

	for _, client := range clients {
		hostname, _ := client.GetHostInfo()
		go f.lookupFlows(ch, hostname, filter)
	}

	flowset := NewFlowSet()
	for i := 0; i != len(clients); i++ {
		fs := <-ch
		flowset.Merge(fs)
	}

	return flowset, nil
}

func (f *TableClient) LookupFlowsByNodes(hnmap HostNodeIDMap, filter *Filter) (*FlowSet, error) {
	ch := make(chan *FlowSet, len(hnmap))

	for host, uuids := range hnmap {
		andFilter := &BoolFilter{
			Op: BoolFilterOp_AND,
			Filters: []*Filter{
				NewFilterForNodes(uuids),
			},
		}

		if filter != nil {
			andFilter.Filters = append(andFilter.Filters, filter)
		}

		queryFilter := &Filter{
			BoolFilter: andFilter,
		}

		go f.lookupFlows(ch, host, queryFilter)
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
