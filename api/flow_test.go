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
	"encoding/json"
	"testing"

	v "github.com/gima/govalid/v1"

	"github.com/skydive-project/skydive/flow"
)

func TestFlowTable_jsonFlowConversation(t *testing.T) {
	ft := flow.NewTestFlowTableComplex(t, nil, nil)
	fa := &FlowApi{
		FlowTable: ft,
	}

	statStr := fa.jsonFlowConversation("ethernet")
	if statStr == `{"nodes":[],"links":[]}` {
		t.Error("stat should not be empty")
	}

	var decoded interface{}
	if err := json.Unmarshal([]byte(statStr), &decoded); err != nil {
		t.Error("JSON parsing failed:", err)
	}

	schema := v.Object(
		v.ObjKV("nodes", v.Array(v.ArrEach(v.Object(
			v.ObjKV("name", v.String(v.StrMin(1))),
			v.ObjKV("group", v.Number(v.NumMin(0), v.NumMax(20))),
		)))),
		v.ObjKV("links", v.Array(v.ArrEach(v.Object(
			v.ObjKV("source", v.Number(v.NumMin(0), v.NumMax(20))),
			v.ObjKV("target", v.Number(v.NumMin(0), v.NumMax(20))),
			v.ObjKV("value", v.Number(v.NumMin(0), v.NumMax(9999))),
		)))),
	)

	if path, err := schema.Validate(decoded); err != nil {
		t.Errorf("Failed (%s). Path: %s", err, path)
	}
}

func test_jsonFlowDiscovery(t *testing.T, DiscoType discoType) {
	ft := flow.NewTestFlowTableComplex(t, nil, nil)
	fa := &FlowApi{
		FlowTable: ft,
	}
	disco := fa.jsonFlowDiscovery(DiscoType)

	if disco == `{"name":"root","children":[]}` {
		t.Error("disco should not be empty")
	}

	var decoded interface{}
	if err := json.Unmarshal([]byte(disco), &decoded); err != nil {
		t.Error("JSON parsing failed:", err)
	}

	schema := v.Object(
		v.ObjKV("name", v.String(v.StrMin(1))), // root
		v.ObjKV("children", v.Array(v.ArrEach(v.Object(
			v.ObjKV("name", v.String(v.StrMin(1))), // Ethernet
			v.ObjKV("children", v.Array(v.ArrEach(v.Object(
				v.ObjKV("name", v.String(v.StrMin(1))), // IPv4
				v.ObjKV("children", v.Array(v.ArrEach(v.Object(
					v.ObjKV("name", v.String(v.StrMin(1))), // TCP or UDP
					v.ObjKV("children", v.Array(v.ArrEach(v.Object(
						v.ObjKV("name", v.String(v.StrMin(1))), // Payload
						v.ObjKV("size", v.Number(v.NumMin(0), v.NumMax(9999))),
					)))),
				)))),
			)))),
		)))),
	)

	if path, err := schema.Validate(decoded); err != nil {
		t.Errorf("Failed (%s). Path: %s", err, path)
	}
}

func TestFlowTable_jsonFlowDiscovery(t *testing.T) {
	test_jsonFlowDiscovery(t, bytes)
	t.Log("jsonFlowDiscovery BYTES : ok")
	test_jsonFlowDiscovery(t, packets)
	t.Log("jsonFlowDiscovery PACKETS : ok")
}
