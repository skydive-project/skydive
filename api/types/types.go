/*
 * Copyright (C) 2018 Red Hat, Inc.
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

package types

import (
	"time"

	"github.com/nu7hatch/gouuid"
	shttp "github.com/skydive-project/skydive/http"
)

// Resource used as interface resources for each API
type Resource interface {
	ID() string
	SetID(string)
}

// Alert is a set of parameters, the Alert Action will Trigger according to its Expression.
type Alert struct {
	Resource
	UUID        string
	Name        string `json:",omitempty"`
	Description string `json:",omitempty"`
	Expression  string `json:",omitempty" valid:"nonzero"`
	Action      string `json:",omitempty" valid:"regexp=^(|http://|https://|file://).*$"`
	Trigger     string `json:",omitempty" valid:"regexp=^(graph|duration:.+|)$"`
	CreateTime  time.Time
}

// ID returns the alert ID
func (a *Alert) ID() string {
	return a.UUID
}

// SetID set ID
func (a *Alert) SetID(i string) {
	a.UUID = i
}

// NewAlert creates a New empty Alert, only UUID and CreateTime are set.
func NewAlert() *Alert {
	id, _ := uuid.NewV4()

	return &Alert{
		UUID:       id.String(),
		CreateTime: time.Now().UTC(),
	}
}

// AnalyzerStatus describes the status of an analyzer
type AnalyzerStatus struct {
	Agents      map[string]shttp.WSConnStatus
	Peers       PeersStatus
	Publishers  map[string]shttp.WSConnStatus
	Subscribers map[string]shttp.WSConnStatus
	Alerts      ElectionStatus
	Captures    ElectionStatus
	Probes      []string
}

// Capture describes a capture API
type Capture struct {
	UUID           string
	GremlinQuery   string `json:"GremlinQuery,omitempty" valid:"isGremlinExpr"`
	BPFFilter      string `json:"BPFFilter,omitempty" valid:"isBPFFilter"`
	Name           string `json:"Name,omitempty"`
	Description    string `json:"Description,omitempty"`
	Type           string `json:"Type,omitempty"`
	Count          int    `json:"Count"`
	PCAPSocket     string `json:"PCAPSocket,omitempty"`
	Port           int    `json:"Port,omitempty"`
	RawPacketLimit int    `json:"RawPacketLimit,omitempty" valid:"isValidRawPacketLimit"`
	HeaderSize     int    `json:"HeaderSize,omitempty" valid:"isValidCaptureHeaderSize"`
	ExtraTCPMetric bool   `json:"ExtraTCPMetric"`
	SocketInfo     bool   `json:"SocketInfo"`
}

// ID returns the capture Identifier
func (c *Capture) ID() string {
	return c.UUID
}

// SetID set a new identifier for this capture
func (c *Capture) SetID(i string) {
	c.UUID = i
}

// NewCapture creates a new capture
func NewCapture(query string, bpfFilter string) *Capture {
	id, _ := uuid.NewV4()

	return &Capture{
		UUID:         id.String(),
		GremlinQuery: query,
		BPFFilter:    bpfFilter,
	}
}

// ElectionStatus describes the status of an election
type ElectionStatus struct {
	IsMaster bool
}

// PacketParamsReq packet injector API parameters
type PacketParamsReq struct {
	UUID       string
	Src        string
	Dst        string
	SrcIP      string
	DstIP      string
	SrcMAC     string
	DstMAC     string
	SrcPort    int64
	DstPort    int64
	Type       string
	Payload    string
	TrackingID string
	ICMPID     int64
	Count      int64
	Interval   int64
	StartTime  time.Time
}

// ID returns the packet injector request identifier
func (ppr *PacketParamsReq) ID() string {
	return ppr.UUID
}

// SetID set a new identifier for this injector
func (ppr *PacketParamsReq) SetID(id string) {
	ppr.UUID = id
}

type PeersStatus struct {
	Incomers map[string]shttp.WSConnStatus
	Outgoers map[string]shttp.WSConnStatus
}

// TopologyParam topology API parameter
type TopologyParam struct {
	GremlinQuery string `json:"GremlinQuery,omitempty" valid:"isGremlinExpr"`
}

// UserMetadata describes a user metadata
type UserMetadata struct {
	UUID         string
	GremlinQuery string `valid:"isGremlinExpr"`
	Key          string `valid:"nonzero"`
	Value        string `valid:"nonzero"`
}

// ID returns the user metadata identifier
func (m *UserMetadata) ID() string {
	return m.UUID
}

// SetID set a new identifier for this user metadata
func (m *UserMetadata) SetID(id string) {
	m.UUID = id
}

// NewUserMetadata creates a new user metadata
func NewUserMetadata(query string, key string, value string) *UserMetadata {
	id, _ := uuid.NewV4()

	return &UserMetadata{
		UUID:         id.String(),
		GremlinQuery: query,
		Key:          key,
		Value:        value,
	}
}
