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

package ovsdb

import (
	"encoding/json"
	"testing"

	"github.com/socketplane/libovsdb"
)

type FakeOvsResult struct {
	Result []libovsdb.OperationResult
}

type FakeOvsClient struct {
	Operations []libovsdb.Operation

	CurrentResult int
	Results       []FakeOvsResult
}

func (o *FakeOvsClient) Exec(operations ...libovsdb.Operation) ([]libovsdb.OperationResult, error) {
	o.Operations = operations

	result := o.Results[o.CurrentResult].Result
	o.CurrentResult++

	return result, nil
}

func getSelectPreviousRegisteredResults(agent SFlowAgent) []libovsdb.OperationResult {
	selectResult := make([]libovsdb.OperationResult, 1)

	rows := make(map[string]interface{})
	rows["_uuid"] = []interface{}{"_uuid", "agent-uuid"}

	rows["sampling"] = float64(1)
	rows["polling"] = float64(0)

	agentId := []interface{}{"agent-id", agent.Id}
	extMap := []interface{}{"map", []interface{}{agentId}}
	rows["external_ids"] = extMap

	selectResult[0].Rows = make([]map[string]interface{}, 1)
	selectResult[0].Rows[0] = rows

	return selectResult
}

func TestRegisterNewAgents(t *testing.T) {
	agent := SFlowAgent{
		Id:         "AgentId",
		Interface:  "eth0",
		Target:     "1.1.1.1:111",
		HeaderSize: 256,
		Sampling:   1,
		Polling:    0,
	}

	fakeClient := &FakeOvsClient{}
	monitor := NewOvsMonitor("127.0.0.1", 8888)
	monitor.OvsClient = fakeClient

	handler := NewOvsSFlowAgentsHandler([]SFlowAgent{agent})

	fakeClient.CurrentResult = 0
	fakeClient.Results = make([]FakeOvsResult, 2)

	expected := `[{"op":"insert","table":"sFlow","row":{"agent":"eth0","external_ids":["map",[["agent-id","AgentId"]]],"header":256,"polling":0,"sampling":1,"targets":"1.1.1.1:111"},"uuid-name":"AgentId"},`
	expected += `{"op":"update","table":"Bridge","row":{"sflow":["named-uuid","AgentId"]},"where":[["_uuid","==",["named-uuid","bridge1-uuid"]]]}]`

	handler.registerAgents(monitor, "bridge1-uuid")

	operations, _ := json.Marshal(fakeClient.Operations)
	if string(operations) != expected {
		t.Error("Expected: ", expected, " Got: ", string(operations))
	}
}

func TestReusingRegisteredAgents(t *testing.T) {
	agent := SFlowAgent{
		Id:         "AgentId",
		Interface:  "eth0",
		Target:     "1.1.1.1:111",
		HeaderSize: 256,
		Sampling:   1,
		Polling:    0,
	}

	fakeClient := &FakeOvsClient{}
	monitor := NewOvsMonitor("127.0.0.1", 8888)
	monitor.OvsClient = fakeClient

	handler := NewOvsSFlowAgentsHandler([]SFlowAgent{agent})

	fakeClient.CurrentResult = 0
	fakeClient.Results = make([]FakeOvsResult, 2)
	fakeClient.Results[0].Result = getSelectPreviousRegisteredResults(agent)

	expected := `[{"op":"update","table":"Bridge","row":{"sflow":["named-uuid","agent-uuid"]},"where":[["_uuid","==",["named-uuid","bridge1-uuid"]]]}]`

	handler.registerAgents(monitor, "bridge1-uuid")

	operations, _ := json.Marshal(fakeClient.Operations)
	if string(operations) != expected {
		t.Error("Expected: ", expected, " Got: ", string(operations))
	}
}

func TestUpdateRegisteredAgents(t *testing.T) {
	agent := SFlowAgent{
		Id:         "AgentId",
		Interface:  "eth0",
		Target:     "1.1.1.1:111",
		HeaderSize: 256,
		Sampling:   2,
		Polling:    0,
	}

	fakeClient := &FakeOvsClient{}
	monitor := NewOvsMonitor("127.0.0.1", 8888)
	monitor.OvsClient = fakeClient

	handler := NewOvsSFlowAgentsHandler([]SFlowAgent{agent})

	fakeClient.CurrentResult = 0
	fakeClient.Results = make([]FakeOvsResult, 2)
	fakeClient.Results[0].Result = getSelectPreviousRegisteredResults(agent)

	expected := `[{"op":"insert","table":"sFlow","row":{"agent":"eth0","external_ids":["map",[["agent-id","AgentId"]]],"header":256,"polling":0,"sampling":2,"targets":"1.1.1.1:111"},"uuid-name":"AgentId"},`
	expected += `{"op":"update","table":"Bridge","row":{"sflow":["named-uuid","AgentId"]},"where":[["_uuid","==",["named-uuid","bridge1-uuid"]]]}]`

	handler.registerAgents(monitor, "bridge1-uuid")

	operations, _ := json.Marshal(fakeClient.Operations)
	if string(operations) != expected {
		t.Error("Expected: ", expected, " Got: ", string(operations))
	}
}

func TestNewBridgeAdded(t *testing.T) {
	agent := SFlowAgent{
		Id:         "AgentId",
		Interface:  "eth0",
		Target:     "1.1.1.1:111",
		HeaderSize: 256,
		Sampling:   2,
		Polling:    0,
	}

	fakeClient := &FakeOvsClient{}
	monitor := NewOvsMonitor("127.0.0.1", 8888)
	monitor.OvsClient = fakeClient

	handler := NewOvsSFlowAgentsHandler([]SFlowAgent{agent})

	fakeClient.CurrentResult = 0
	fakeClient.Results = make([]FakeOvsResult, 4)

	handler.registerAgents(monitor, "bridge1-uuid")

	expected := `[{"op":"insert","table":"sFlow","row":{"agent":"eth0","external_ids":["map",[["agent-id","AgentId"]]],"header":256,"polling":0,"sampling":2,"targets":"1.1.1.1:111"},"uuid-name":"AgentId"},`
	expected += `{"op":"update","table":"Bridge","row":{"sflow":["named-uuid","AgentId"]},"where":[["_uuid","==",["named-uuid","bridge2-uuid"]]]}]`

	handler.registerAgents(monitor, "bridge2-uuid")

	operations, _ := json.Marshal(fakeClient.Operations)
	if string(operations) != expected {
		t.Error("Expected: ", expected, " Got: ", string(operations))
	}
}
