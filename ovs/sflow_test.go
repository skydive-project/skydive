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

func getSelectPreviousRegisteredResults(sensor SFlowSensor) []libovsdb.OperationResult {
	selectResult := make([]libovsdb.OperationResult, 1)

	rows := make(map[string]interface{})
	rows["_uuid"] = []interface{}{"_uuid", "sensor-uuid"}

	rows["sampling"] = float64(1)
	rows["polling"] = float64(0)

	sensorID := []interface{}{"sensor-id", sensor.ID}
	extMap := []interface{}{"map", []interface{}{sensorID}}
	rows["external_ids"] = extMap

	selectResult[0].Rows = make([]map[string]interface{}, 1)
	selectResult[0].Rows[0] = rows

	return selectResult
}

func TestRegisterNewSensors(t *testing.T) {
	sensor := SFlowSensor{
		ID:         "SensorID",
		Interface:  "eth0",
		Target:     "1.1.1.1:111",
		HeaderSize: 256,
		Sampling:   1,
		Polling:    0,
	}

	fakeClient := &FakeOvsClient{}
	monitor := NewOvsMonitor("127.0.0.1", 8888)
	monitor.OvsClient = fakeClient

	handler := NewOvsSFlowSensorsHandler([]SFlowSensor{sensor})

	fakeClient.CurrentResult = 0
	fakeClient.Results = make([]FakeOvsResult, 2)

	expected := `[{"op":"insert","table":"sFlow","row":{"agent":"eth0","external_ids":["map",[["sensor-id","SensorID"]]],"header":256,"polling":0,"sampling":1,"targets":"1.1.1.1:111"},"uuid-name":"SensorID"},`
	expected += `{"op":"update","table":"Bridge","row":{"sflow":["named-uuid","SensorID"]},"where":[["_uuid","==",["named-uuid","bridge1-uuid"]]]}]`

	handler.registerSensors(monitor, "bridge1-uuid")

	operations, _ := json.Marshal(fakeClient.Operations)
	if string(operations) != expected {
		t.Error("Expected: ", expected, " Got: ", string(operations))
	}
}

func TestReusingRegisteredSensors(t *testing.T) {
	sensor := SFlowSensor{
		ID:         "SensorID",
		Interface:  "eth0",
		Target:     "1.1.1.1:111",
		HeaderSize: 256,
		Sampling:   1,
		Polling:    0,
	}

	fakeClient := &FakeOvsClient{}
	monitor := NewOvsMonitor("127.0.0.1", 8888)
	monitor.OvsClient = fakeClient

	handler := NewOvsSFlowSensorsHandler([]SFlowSensor{sensor})

	fakeClient.CurrentResult = 0
	fakeClient.Results = make([]FakeOvsResult, 2)
	fakeClient.Results[0].Result = getSelectPreviousRegisteredResults(sensor)

	expected := `[{"op":"update","table":"Bridge","row":{"sflow":["named-uuid","sensor-uuid"]},"where":[["_uuid","==",["named-uuid","bridge1-uuid"]]]}]`

	handler.registerSensors(monitor, "bridge1-uuid")

	operations, _ := json.Marshal(fakeClient.Operations)
	if string(operations) != expected {
		t.Error("Expected: ", expected, " Got: ", string(operations))
	}
}

func TestUpdateRegisteredSensors(t *testing.T) {
	sensor := SFlowSensor{
		ID:         "SensorID",
		Interface:  "eth0",
		Target:     "1.1.1.1:111",
		HeaderSize: 256,
		Sampling:   2,
		Polling:    0,
	}

	fakeClient := &FakeOvsClient{}
	monitor := NewOvsMonitor("127.0.0.1", 8888)
	monitor.OvsClient = fakeClient

	handler := NewOvsSFlowSensorsHandler([]SFlowSensor{sensor})

	fakeClient.CurrentResult = 0
	fakeClient.Results = make([]FakeOvsResult, 2)
	fakeClient.Results[0].Result = getSelectPreviousRegisteredResults(sensor)

	expected := `[{"op":"insert","table":"sFlow","row":{"agent":"eth0","external_ids":["map",[["sensor-id","SensorID"]]],"header":256,"polling":0,"sampling":2,"targets":"1.1.1.1:111"},"uuid-name":"SensorID"},`
	expected += `{"op":"update","table":"Bridge","row":{"sflow":["named-uuid","SensorID"]},"where":[["_uuid","==",["named-uuid","bridge1-uuid"]]]}]`

	handler.registerSensors(monitor, "bridge1-uuid")

	operations, _ := json.Marshal(fakeClient.Operations)
	if string(operations) != expected {
		t.Error("Expected: ", expected, " Got: ", string(operations))
	}
}

func TestNewBridgeAdded(t *testing.T) {
	sensor := SFlowSensor{
		ID:         "SensorID",
		Interface:  "eth0",
		Target:     "1.1.1.1:111",
		HeaderSize: 256,
		Sampling:   2,
		Polling:    0,
	}

	fakeClient := &FakeOvsClient{}
	monitor := NewOvsMonitor("127.0.0.1", 8888)
	monitor.OvsClient = fakeClient

	handler := NewOvsSFlowSensorsHandler([]SFlowSensor{sensor})

	fakeClient.CurrentResult = 0
	fakeClient.Results = make([]FakeOvsResult, 4)

	handler.registerSensors(monitor, "bridge1-uuid")

	expected := `[{"op":"insert","table":"sFlow","row":{"agent":"eth0","external_ids":["map",[["sensor-id","SensorID"]]],"header":256,"polling":0,"sampling":2,"targets":"1.1.1.1:111"},"uuid-name":"SensorID"},`
	expected += `{"op":"update","table":"Bridge","row":{"sflow":["named-uuid","SensorID"]},"where":[["_uuid","==",["named-uuid","bridge2-uuid"]]]}]`

	handler.registerSensors(monitor, "bridge2-uuid")

	operations, _ := json.Marshal(fakeClient.Operations)
	if string(operations) != expected {
		t.Error("Expected: ", expected, " Got: ", string(operations))
	}
}
