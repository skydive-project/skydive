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

type TableAllocator struct {
	sync.RWMutex
	update time.Duration
	expire time.Duration
	tables map[*Table]bool
}

func (a *TableAllocator) Flush() {
	a.RLock()
	defer a.RUnlock()

	for table := range a.tables {
		table.Flush()
	}
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

func (a *TableAllocator) Alloc(flowCallBack ExpireUpdateFunc) *Table {
	a.Lock()
	defer a.Unlock()

	updateHandler := NewFlowHandler(flowCallBack, a.update)
	expireHandler := NewFlowHandler(flowCallBack, a.expire)
	t := NewTable(updateHandler, expireHandler)
	a.tables[t] = true

	return t
}

func (a *TableAllocator) Release(t *Table) {
	a.Lock()
	delete(a.tables, t)
	a.Unlock()
}

func NewTableAllocator(update, expire time.Duration) *TableAllocator {
	return &TableAllocator{
		update: update,
		expire: expire,
		tables: make(map[*Table]bool),
	}
}
