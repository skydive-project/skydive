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

	"github.com/redhat-cip/skydive/agents"
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

func getTableUpdates(bridgeName string) *libovsdb.TableUpdates {
	tableUpdates := &libovsdb.TableUpdates{}
	tableUpdates.Updates = make(map[string]libovsdb.TableUpdate)

	rows := make(map[string]libovsdb.RowUpdate)
	tableUpdates.Updates["Bridge"] = libovsdb.TableUpdate{Rows: rows}

	rowFields := make(map[string]interface{})
	rowFields["name"] = bridgeName + "-name"
	row := libovsdb.Row{Fields: rowFields}

	bridgeUuid := bridgeName + "-uuid"
	rowUpdate := libovsdb.RowUpdate{Uuid: libovsdb.UUID{GoUuid: bridgeUuid}, New: row}
	rows[bridgeUuid] = rowUpdate

	return tableUpdates
}

func TestRegisterNewAgents(t *testing.T) {
	sflow := agents.NewSFlowAgent("1.1.1.1", 111, nil)
	agent := SFlowAgent{
		Id:         "AgentId",
		Interface:  "eth0",
		Agent:      sflow,
		HeaderSize: 256,
		Sampling:   1,
		Polling:    0,
	}

	fakeClient := &FakeOvsClient{}
	monitor := NewBridgesMonitor("127.0.0.1", 8888, []Agent{agent})
	monitor.ovsClient = fakeClient

	fakeClient.CurrentResult = 0
	fakeClient.Results = make([]FakeOvsResult, 2)

	expected := `[{"op":"insert","table":"sFlow","row":{"agent":"eth0","external_ids":["map",[["agent-id","AgentId"]]],"header":256,"polling":0,"sampling":1,"targets":"1.1.1.1:111"},"uuid-name":"AgentId"},`
	expected += `{"op":"update","table":"Bridge","row":{"sflow":["named-uuid","AgentId"]},"where":[["_uuid","==",["named-uuid","bridge1-uuid"]]]}]`

	tableUpdates := getTableUpdates("bridge1")
	monitor.registerAgents(tableUpdates)

	operations, _ := json.Marshal(fakeClient.Operations)
	if string(operations) != expected {
		t.Error("Expected: ", expected, " Got: ", string(operations))
	}
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

func TestReusingRegisteredAgents(t *testing.T) {
	sflow := agents.NewSFlowAgent("1.1.1.1", 111, nil)
	agent := SFlowAgent{
		Id:         "AgentId",
		Interface:  "eth0",
		Agent:      sflow,
		HeaderSize: 256,
		Sampling:   1,
		Polling:    0,
	}

	fakeClient := &FakeOvsClient{}
	monitor := NewBridgesMonitor("127.0.0.1", 8888, []Agent{agent})
	monitor.ovsClient = fakeClient

	fakeClient.CurrentResult = 0
	fakeClient.Results = make([]FakeOvsResult, 2)
	fakeClient.Results[0].Result = getSelectPreviousRegisteredResults(agent)

	expected := `[{"op":"update","table":"Bridge","row":{"sflow":["named-uuid","agent-uuid"]},"where":[["_uuid","==",["named-uuid","bridge1-uuid"]]]}]`

	tableUpdates := getTableUpdates("bridge1")
	monitor.registerAgents(tableUpdates)

	operations, _ := json.Marshal(fakeClient.Operations)
	if string(operations) != expected {
		t.Error("Expected: ", expected, " Got: ", string(operations))
	}
}

func TestUpdateRegisteredAgents(t *testing.T) {
	sflow := agents.NewSFlowAgent("1.1.1.1", 111, nil)
	agent := SFlowAgent{
		Id:         "AgentId",
		Interface:  "eth0",
		Agent:      sflow,
		HeaderSize: 256,
		Sampling:   2,
		Polling:    0,
	}

	fakeClient := &FakeOvsClient{}
	monitor := NewBridgesMonitor("127.0.0.1", 8888, []Agent{agent})
	monitor.ovsClient = fakeClient

	fakeClient.CurrentResult = 0
	fakeClient.Results = make([]FakeOvsResult, 2)
	fakeClient.Results[0].Result = getSelectPreviousRegisteredResults(agent)

	expected := `[{"op":"insert","table":"sFlow","row":{"agent":"eth0","external_ids":["map",[["agent-id","AgentId"]]],"header":256,"polling":0,"sampling":2,"targets":"1.1.1.1:111"},"uuid-name":"AgentId"},`
	expected += `{"op":"update","table":"Bridge","row":{"sflow":["named-uuid","AgentId"]},"where":[["_uuid","==",["named-uuid","bridge1-uuid"]]]}]`

	tableUpdates := getTableUpdates("bridge1")
	monitor.registerAgents(tableUpdates)

	operations, _ := json.Marshal(fakeClient.Operations)
	if string(operations) != expected {
		t.Error("Expected: ", expected, " Got: ", string(operations))
	}
}

func TestNewBridgeAdded(t *testing.T) {
	sflow := agents.NewSFlowAgent("1.1.1.1", 111, nil)
	agent := SFlowAgent{
		Id:         "AgentId",
		Interface:  "eth0",
		Agent:      sflow,
		HeaderSize: 256,
		Sampling:   2,
		Polling:    0,
	}

	fakeClient := &FakeOvsClient{}
	monitor := NewBridgesMonitor("127.0.0.1", 8888, []Agent{agent})
	monitor.ovsClient = fakeClient

	fakeClient.CurrentResult = 0
	fakeClient.Results = make([]FakeOvsResult, 4)

	tableUpdates := getTableUpdates("bridge1")
	monitor.registerAgents(tableUpdates)

	expected := `[{"op":"insert","table":"sFlow","row":{"agent":"eth0","external_ids":["map",[["agent-id","AgentId"]]],"header":256,"polling":0,"sampling":2,"targets":"1.1.1.1:111"},"uuid-name":"AgentId"},`
	expected += `{"op":"update","table":"Bridge","row":{"sflow":["named-uuid","AgentId"]},"where":[["_uuid","==",["named-uuid","bridge2-uuid"]]]}]`

	tableUpdates = getTableUpdates("bridge2")
	monitor.registerAgents(tableUpdates)

	operations, _ := json.Marshal(fakeClient.Operations)
	if string(operations) != expected {
		t.Error("Expected: ", expected, " Got: ", string(operations))
	}
}
