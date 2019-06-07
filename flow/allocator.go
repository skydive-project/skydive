/*
 * Copyright (C) 2016 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy ofthe License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specificlanguage governing permissions and
 * limitations under the License.
 *
 */

package flow

import (
	"net/http"
	"time"

	"github.com/skydive-project/skydive/common"
)

// TableAllocator aims to create/allocate a new flow table
type TableAllocator struct {
	common.RWMutex
	update time.Duration
	expire time.Duration
	tables map[*Table]bool
}

// Expire returns the expire parameter used by allocated tables
func (a *TableAllocator) Expire() time.Duration {
	return a.expire
}

// Update returns the update parameter used by allocated tables
func (a *TableAllocator) Update() time.Duration {
	return a.update
}

// QueryTable search/query within the flow table
func (a *TableAllocator) QueryTable(tq *TableQuery) *TableReply {
	a.RLock()
	defer a.RUnlock()

	reply := &TableReply{
		Status: int32(http.StatusOK),
	}

	for table := range a.tables {
		if b := table.Query(tq); b != nil {
			reply.FlowSetBytes = append(reply.FlowSetBytes, b)
		}
	}

	return reply
}

// Alloc instantiate/allocate a new table
func (a *TableAllocator) Alloc(flowCallBack ExpireUpdateFunc, nodeTID string, opts TableOpts) *Table {
	a.Lock()
	defer a.Unlock()

	updateHandler := NewFlowHandler(flowCallBack, a.update)
	expireHandler := NewFlowHandler(flowCallBack, a.expire)
	t := NewTable(updateHandler, expireHandler, nodeTID, opts)
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
func NewTableAllocator(update, expire time.Duration) *TableAllocator {
	return &TableAllocator{
		update: update,
		expire: expire,
		tables: make(map[*Table]bool),
	}
}
