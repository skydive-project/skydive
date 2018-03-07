/*
 * Copyright (C) 2017 Red Hat, Inc.
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

package graph

import (
	"encoding/json"
	"net/http"
	"testing"

	shttp "github.com/skydive-project/skydive/http"
)

func TestNullNodesEdges(t *testing.T) {
	nodesNull := []byte(`{"Nodes": null}`)

	raw := json.RawMessage(nodesNull)

	msg := &shttp.WSStructMessage{
		Protocol:  shttp.JsonProtocol,
		Namespace: Namespace,
		Type:      SyncMsgType,
		UUID:      "aaa",
		Status:    http.StatusOK,
		JsonObj:   &raw,
	}

	if _, _, err := UnmarshalWSMessage(msg); err != nil {
		t.Error("Should not raise error if Nodes is null")
	}

	edgesNull := []byte(`{"Nodes": [{"ID": "aaa"}], "Edges": null}`)

	raw = json.RawMessage(edgesNull)

	msg = &shttp.WSStructMessage{
		Protocol:  shttp.JsonProtocol,
		Namespace: Namespace,
		Type:      SyncMsgType,
		UUID:      "bbb",
		Status:    http.StatusOK,
		JsonObj:   &raw,
	}

	if _, _, err := UnmarshalWSMessage(msg); err != nil {
		t.Errorf("Should not raise error if Nodes is null: %s", err.Error())
	}
}

func TestID(t *testing.T) {
	idMissing := []byte(`{"Nodes": [{"AAA": 123}]}`)

	raw := json.RawMessage(idMissing)

	msg := &shttp.WSStructMessage{
		Protocol:  shttp.JsonProtocol,
		Namespace: Namespace,
		Type:      SyncMsgType,
		UUID:      "aaa",
		Status:    http.StatusOK,
		JsonObj:   &raw,
	}

	if _, _, err := UnmarshalWSMessage(msg); err == nil {
		t.Error("Should raise an error")
	}

	idWrongType := []byte(`{"Nodes": [{"ID": 123}]}`)

	raw = json.RawMessage(idWrongType)

	msg = &shttp.WSStructMessage{
		Protocol:  shttp.JsonProtocol,
		Namespace: Namespace,
		Type:      SyncMsgType,
		UUID:      "aaa",
		Status:    http.StatusOK,
		JsonObj:   &raw,
	}

	if _, _, err := UnmarshalWSMessage(msg); err == nil {
		t.Error("Should raise an error")
	}
}

func TestHost(t *testing.T) {
	hostWrongType := []byte(`{"Nodes": [{"Host": 123}]}`)

	raw := json.RawMessage(hostWrongType)

	msg := &shttp.WSStructMessage{
		Protocol:  shttp.JsonProtocol,
		Namespace: Namespace,
		Type:      SyncMsgType,
		UUID:      "aaa",
		Status:    http.StatusOK,
		JsonObj:   &raw,
	}

	if _, _, err := UnmarshalWSMessage(msg); err == nil {
		t.Error("Should raise an error")
	}
}
