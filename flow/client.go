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

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/filters"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology"
)

// TableClient describes a mechanism to Query a flow table via flowSet in JSON
type TableClient struct {
	shttp.DefaultWSServerEventHandler
	WSServer       *shttp.WSServer
	replyChanMutex sync.RWMutex
	replyChan      map[string]chan *json.RawMessage
}

// OnMessage event
func (f *TableClient) OnMessage(c *shttp.WSClient, m shttp.WSMessage) {
	f.replyChanMutex.RLock()
	defer f.replyChanMutex.RUnlock()

	ch, ok := f.replyChan[m.UUID]
	if !ok {
		logging.GetLogger().Errorf("Unable to send reply, chan not found for %s, available: %v", m.UUID, f.replyChan)
		return
	}

	if m.Status >= http.StatusBadRequest {
		ch <- nil
	} else {
		ch <- m.Obj
	}
}

func (f *TableClient) lookupFlows(flowset chan *FlowSet, host string, flowSearchQuery filters.SearchQuery) {
	obj, _ := proto.Marshal(&flowSearchQuery)
	tq := TableQuery{
		Type: "SearchQuery",
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

		if raw == nil || json.Unmarshal([]byte(*raw), &reply) != nil {
			logging.GetLogger().Errorf("Error returned while reading TableReply from: %s", host)
			break
		}

		fs := NewFlowSet()
		context := MergeContext{
			Sort:      flowSearchQuery.Sort,
			SortBy:    flowSearchQuery.SortBy,
			SortOrder: common.SortOrder(flowSearchQuery.SortOrder),
			Dedup:     flowSearchQuery.Dedup,
			DedupBy:   flowSearchQuery.DedupBy,
		}
		for _, b := range reply.Obj {
			var fsr FlowSearchReply
			if err := proto.Unmarshal(b, &fsr); err != nil {
				logging.GetLogger().Errorf("Unable to decode flow search reply from: %s", host)
				continue
			}
			fs.Merge(fsr.FlowSet, context)
		}
		flowset <- fs

		return
	case <-time.After(time.Second * 10):
		logging.GetLogger().Errorf("Timeout while reading TableReply from: %s", host)
	}

	flowset <- NewFlowSet()
}

// LookupFlows query flow table based on a filter search query
func (f *TableClient) LookupFlows(flowSearchQuery filters.SearchQuery) (*FlowSet, error) {
	clients := f.WSServer.GetClientsByType(common.AgentService)
	ch := make(chan *FlowSet, len(clients))

	for _, client := range clients {
		go f.lookupFlows(ch, client.Host, flowSearchQuery)
	}

	flowset := NewFlowSet()

	// for sort order we assume that the SortOrder of a flowSearchQuery comes from
	// an already validated entry.
	context := MergeContext{
		Sort:      flowSearchQuery.Sort,
		SortBy:    flowSearchQuery.SortBy,
		SortOrder: common.SortOrder(flowSearchQuery.SortOrder),
		Dedup:     flowSearchQuery.Dedup,
		DedupBy:   flowSearchQuery.DedupBy,
	}
	for i := 0; i != len(clients); i++ {
		fs := <-ch
		flowset.Merge(fs, context)
	}

	return flowset, nil
}

// LookupFlowsByNodes query flow table based on multiple nodes
func (f *TableClient) LookupFlowsByNodes(hnmap topology.HostNodeTIDMap, flowSearchQuery filters.SearchQuery) (*FlowSet, error) {
	ch := make(chan *FlowSet, len(hnmap))

	// We conserve the original filter to reuse it for each host
	searchQuery := flowSearchQuery.Filter
	for host, tids := range hnmap {
		flowSearchQuery.Filter = filters.NewAndFilter(NewFilterForNodeTIDs(tids), searchQuery)
		go f.lookupFlows(ch, host, flowSearchQuery)
	}

	flowset := NewFlowSet()

	// for sort order we assume that the SortOrder of a flowSearchQuery comes from
	// an already validated entry.
	context := MergeContext{
		Sort:      flowSearchQuery.Sort,
		SortBy:    flowSearchQuery.SortBy,
		SortOrder: common.SortOrder(flowSearchQuery.SortOrder),
		Dedup:     flowSearchQuery.Dedup,
		DedupBy:   flowSearchQuery.DedupBy,
	}
	for i := 0; i != len(hnmap); i++ {
		fs := <-ch
		flowset.Merge(fs, context)
	}

	return flowset, nil
}

// NewTableClient creates a new table client based on websocket
func NewTableClient(w *shttp.WSServer) *TableClient {
	tc := &TableClient{
		WSServer:  w,
		replyChan: make(map[string]chan *json.RawMessage),
	}
	w.AddEventHandler(tc, []string{Namespace})

	return tc
}
