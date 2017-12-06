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
	"net/http"
	"sync"
	"time"
)

// TableAllocator aims to create/allocate a new flow table
type TableAllocator struct {
	sync.RWMutex
	update   time.Duration
	expire   time.Duration
	tables   map[*Table]bool
	pipeline *EnhancerPipeline
}

// Expire returns the expire parameter used by allocated tables
func (a *TableAllocator) Expire() time.Duration {
	return a.expire
}

// Update returns the update parameter used by allocated tables
func (a *TableAllocator) Update() time.Duration {
	return a.update
}

func (a *TableAllocator) aggregateReplies(query *TableQuery, replies []*TableReply) *TableReply {
	reply := &TableReply{
		status: http.StatusOK,
		Obj:    make([][]byte, 0),
	}

	for _, r := range replies {
		if r.status >= http.StatusBadRequest {
			// FIX, 207 => http.StatusMultiStatus when moving to >= 1.7
			reply.status = 207
			continue
		}
		reply.Obj = append(reply.Obj, r.Obj...)
	}

	return reply
}

// QueryTable search/query within the flow table
func (a *TableAllocator) QueryTable(query *TableQuery) *TableReply {
	a.RLock()
	defer a.RUnlock()

	var replies []*TableReply
	for table := range a.tables {
		reply := table.Query(query)
		if reply != nil {
			replies = append(replies, reply)
		}
	}

	return a.aggregateReplies(query, replies)
}

// Alloc instanciate/allocate a new table
func (a *TableAllocator) Alloc(flowCallBack ExpireUpdateFunc, nodeTID string, opts TableOpts) *Table {
	a.Lock()
	defer a.Unlock()

	updateHandler := NewFlowHandler(flowCallBack, a.update)
	expireHandler := NewFlowHandler(flowCallBack, a.expire)
	t := NewTable(updateHandler, expireHandler, a.pipeline, nodeTID, opts)
	a.tables[t] = true

	return t
}

// Release release/destroy a flow table
func (a *TableAllocator) Release(t *Table) {
	a.Lock()
	delete(a.tables, t)
	a.Unlock()
}

// NewTableAllocator creates a new flow table
func NewTableAllocator(update, expire time.Duration, pipeline *EnhancerPipeline) *TableAllocator {
	return &TableAllocator{
		update:   update,
		expire:   expire,
		tables:   make(map[*Table]bool),
		pipeline: pipeline,
	}
}
