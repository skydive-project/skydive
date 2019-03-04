/*
 * Copyright (C) 2017 Red Hat, Inc.
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

package graph

import (
	"net/http"
	"testing"

	ws "github.com/skydive-project/skydive/websocket"
)

func TestNullNodesEdges(t *testing.T) {
	nodesNull := []byte(`{"Nodes": null}`)

	msg := &ws.StructMessage{
		Namespace: Namespace,
		Type:      SyncMsgType,
		UUID:      "aaa",
		Status:    http.StatusOK,
		Obj:       nodesNull,
	}

	if _, _, err := UnmarshalMessage(msg); err != nil {
		t.Errorf("Should not raise error if Nodes is null: %s", err)
	}

	edgesNull := []byte(`{"Nodes": [{"ID": "aaa"}], "Edges": null}`)

	msg = &ws.StructMessage{
		Namespace: Namespace,
		Type:      SyncMsgType,
		UUID:      "bbb",
		Status:    http.StatusOK,
		Obj:       edgesNull,
	}

	if _, _, err := UnmarshalMessage(msg); err != nil {
		t.Errorf("Should not raise error if Edges is null: %s", err)
	}
}

func TestID(t *testing.T) {
	idMissing := []byte(`{"Nodes": [{"AAA": 123}]}`)

	msg := &ws.StructMessage{
		Namespace: Namespace,
		Type:      SyncMsgType,
		UUID:      "aaa",
		Status:    http.StatusOK,
		Obj:       idMissing,
	}

	if _, _, err := UnmarshalMessage(msg); err == nil {
		t.Error("Should raise an error")
	}

	idWrongType := []byte(`{"Nodes": [{"ID": 123}]}`)

	msg = &ws.StructMessage{
		Namespace: Namespace,
		Type:      SyncMsgType,
		UUID:      "aaa",
		Status:    http.StatusOK,
		Obj:       idWrongType,
	}

	if _, _, err := UnmarshalMessage(msg); err == nil {
		t.Error("Should raise an error")
	}
}

func TestHost(t *testing.T) {
	hostWrongType := []byte(`{"Nodes": [{"Host": 123}]}`)

	msg := &ws.StructMessage{
		Namespace: Namespace,
		Type:      SyncMsgType,
		UUID:      "aaa",
		Status:    http.StatusOK,
		Obj:       hostWrongType,
	}

	if _, _, err := UnmarshalMessage(msg); err == nil {
		t.Error("Should raise an error")
	}
}
