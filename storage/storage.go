/*
 * Copyright (C) 2015 Red Hat, Inc.
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

package storage

import (
	"github.com/skydive-project/skydive/flow"
)

type RangeFilter struct {
	Gt  interface{} `json:"gt,omitempty"`
	Lt  interface{} `json:"lt,omitempty"`
	Gte interface{} `json:"gte,omitempty"`
	Lte interface{} `json:"lte,omitempty"`
}

type TermFilterOp int

const (
	OR TermFilterOp = iota
	AND
)

type TermFilter struct {
	Op    TermFilterOp
	Terms map[string]interface{}
}

type Filters struct {
	Term  TermFilter
	Range map[string]interface{}
}

type Storage interface {
	Start()
	StoreFlows(flows []*flow.Flow) error
	SearchFlows(filters *Filters) ([]*flow.Flow, error)
	Stop()
}

func NewFilters() *Filters {
	return &Filters{
		Term: TermFilter{
			Op:    AND,
			Terms: make(map[string]interface{}),
		},
		Range: make(map[string]interface{}),
	}
}
