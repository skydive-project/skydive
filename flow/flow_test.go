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
	"encoding/json"
	"reflect"
	"testing"

	v "github.com/gima/govalid/v1"
)

func TestFlowJSON(t *testing.T) {
	f := Flow{
		UUID:       "uuid-1",
		LayersPath: "layerpath-1",
		Statistics: &FlowStatistics{
			Start: 1111,
			Last:  222,
			Endpoints: []*FlowEndpointsStatistics{
				{
					Type: FlowEndpointType_ETHERNET,
					AB: &FlowEndpointStatistics{
						Value:   "value-1",
						Bytes:   33,
						Packets: 34,
					},
					BA: &FlowEndpointStatistics{
						Value:   "value-2",
						Bytes:   44,
						Packets: 55,
					},
				},
			},
		},
		ProbeGraphPath: "probepath-1",
		IfSrcGraphPath: "srcgraphpath-1",
		IfDstGraphPath: "dstgraphpath-1",
	}

	j, err := json.Marshal(f)
	if err != nil {
		t.Error(err.Error())
	}

	schema := v.Object(
		v.ObjKV("UUID", v.String()),
		v.ObjKV("LayersPath", v.String()),
		v.ObjKV("ProbeGraphPath", v.String()),
		v.ObjKV("IfSrcGraphPath", v.String()),
		v.ObjKV("IfDstGraphPath", v.String()),
		v.ObjKV("Statistics", v.Object(
			v.ObjKV("Start", v.Number()),
			v.ObjKV("Last", v.Number()),
			v.ObjKV("Endpoints", v.Array(v.ArrEach(v.Object(
				v.ObjKV("Type", v.String()),
				v.ObjKV("AB", v.Object(
					v.ObjKV("Value", v.String()),
					v.ObjKV("Packets", v.Number()),
					v.ObjKV("Bytes", v.Number()),
				)),
				v.ObjKV("AB", v.Object(
					v.ObjKV("Value", v.String()),
					v.ObjKV("Packets", v.Number()),
					v.ObjKV("Bytes", v.Number()),
				)),
			)))),
		)),
	)

	var data interface{}
	if err := json.Unmarshal(j, &data); err != nil {
		t.Fatal("JSON parsing failed. Err =", err)
	}

	if path, err := schema.Validate(data); err != nil {
		t.Fatalf("Validation failed at %s. Error (%s)", path, err)
	}

	var e Flow
	if err := json.Unmarshal(j, &e); err != nil {
		t.Fatal("JSON parsing failed. Err =", err)
	}

	if !reflect.DeepEqual(f, e) {
		t.Fatal("Unmarshalled flow not equal to the original")
	}
}
