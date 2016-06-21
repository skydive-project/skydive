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

package api

import (
	"github.com/nu7hatch/gouuid"
)

type Capture struct {
	UUID         string
	GremlinQuery string `json:"GremlinQuery,omitempty" valid:"isGremlinExpr"`
	BPFFilter    string `json:"BPFFilter,omitempty"`
	Name         string `json:"Name,omitempty"`
	Description  string `json:"Description,omitempty"`
}

type CaptureHandler struct {
}

func NewCapture(query string, bpfFilter string) *Capture {
	id, _ := uuid.NewV4()

	return &Capture{
		UUID:         id.String(),
		GremlinQuery: query,
		BPFFilter:    bpfFilter,
	}
}

func (c *CaptureHandler) New() ApiResource {
	id, _ := uuid.NewV4()

	return &Capture{
		UUID: id.String(),
	}
}

func (c *CaptureHandler) Name() string {
	return "capture"
}

func (c *Capture) ID() string {
	return c.UUID
}

func (c *Capture) SetID(i string) {
	c.UUID = i
}
