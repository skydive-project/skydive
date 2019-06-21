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
	updateEvery time.Duration
	expireAfter time.Duration
	sender      Sender
	tables      map[*Table]bool
}

// ExpireAfter returns the expiration duration
func (a *TableAllocator) ExpireAfter() time.Duration {
	return a.expireAfter
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
func (a *TableAllocator) Alloc(uuids UUIDs, opts TableOpts) *Table {
	a.Lock()
	defer a.Unlock()

	t := NewTable(a.updateEvery, a.expireAfter, a.sender, uuids, opts)
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
func NewTableAllocator(updateEvery, expireAfter time.Duration, sender Sender) *TableAllocator {
	return &TableAllocator{
		updateEvery: updateEvery,
		expireAfter: expireAfter,
		sender:      sender,
		tables:      make(map[*Table]bool),
	}
}
