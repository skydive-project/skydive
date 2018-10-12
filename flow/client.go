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
	"github.com/golang/protobuf/proto"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/filters"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology"
	ws "github.com/skydive-project/skydive/websocket"
)

// TableClient describes a mechanism to query a flow table
type TableClient interface {
	LookupFlows(flowSearchQuery filters.SearchQuery) (*FlowSet, error)
	LookupFlowsByNodes(hnmap topology.HostNodeTIDMap, flowSearchQuery filters.SearchQuery) (*FlowSet, error)
}

// WSTableClient implements a flow table client using WebSocket
type WSTableClient struct {
	structServer *ws.StructServer
}

func (f *WSTableClient) lookupFlows(flowset chan *FlowSet, host string, flowSearchQuery filters.SearchQuery) {
	obj, _ := proto.Marshal(&flowSearchQuery)
	tq := TableQuery{Type: "SearchQuery", Obj: obj}
	msg := ws.NewStructMessage(Namespace, "TableQuery", tq)

	resp, err := f.structServer.Request(host, msg, ws.DefaultRequestTimeout)
	if err != nil {
		logging.GetLogger().Errorf("Unable to send message to agent %s: %s", host, err.Error())
		flowset <- NewFlowSet()
		return
	}

	var reply TableReply
	if resp == nil || resp.UnmarshalObj(&reply) != nil {
		logging.GetLogger().Errorf("Error returned while reading TableReply from: %s", host)
		flowset <- NewFlowSet()
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
}

// LookupFlows query flow table based on a filter search query
func (f *WSTableClient) LookupFlows(flowSearchQuery filters.SearchQuery) (*FlowSet, error) {
	speakers := f.structServer.GetSpeakersByType(common.AgentService)
	ch := make(chan *FlowSet, len(speakers))

	for _, c := range speakers {
		go f.lookupFlows(ch, c.GetRemoteHost(), flowSearchQuery)
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
	for i := 0; i != len(speakers); i++ {
		fs := <-ch
		flowset.Merge(fs, context)
	}

	return flowset, nil
}

// LookupFlowsByNodes query flow table based on multiple nodes
func (f *WSTableClient) LookupFlowsByNodes(hnmap topology.HostNodeTIDMap, flowSearchQuery filters.SearchQuery) (*FlowSet, error) {
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

// NewWSTableClient creates a new table client based on websocket
func NewWSTableClient(w *ws.StructServer) *WSTableClient {
	return &WSTableClient{structServer: w}
}
