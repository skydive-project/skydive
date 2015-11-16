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

	"github.com/socketplane/libovsdb"

	"github.com/redhat-cip/skydive/logging"
)

type SFlowSensor struct {
	ID         string
	Interface  string
	Target     string
	HeaderSize uint32
	Sampling   uint32
	Polling    uint32
}

type OvsSFlowSensorsHandler struct {
	sensors []SFlowSensor
}

func newInsertSFlowSensorOP(sensor SFlowSensor) (*libovsdb.Operation, error) {
	sFlowRow := make(map[string]interface{})
	sFlowRow["agent"] = sensor.Interface
	sFlowRow["targets"] = sensor.Target
	sFlowRow["header"] = sensor.HeaderSize
	sFlowRow["sampling"] = sensor.Sampling
	sFlowRow["polling"] = sensor.Polling

	extIds := make(map[string]string)
	extIds["sensor-id"] = sensor.ID
	ovsMap, err := libovsdb.NewOvsMap(extIds)
	if err != nil {
		return nil, err
	}
	sFlowRow["external_ids"] = ovsMap

	insertOp := libovsdb.Operation{
		Op:       "insert",
		Table:    "sFlow",
		Row:      sFlowRow,
		UUIDName: sensor.ID,
	}

	return &insertOp, nil
}

func compareSensorID(row *map[string]interface{}, sensor SFlowSensor) (bool, error) {
	extIds := (*row)["external_ids"]
	switch extIds.(type) {
	case []interface{}:
		sl := extIds.([]interface{})
		bSliced, err := json.Marshal(sl)
		if err != nil {
			return false, err
		}

		switch sl[0] {
		case "map":
			var oMap libovsdb.OvsMap
			err = json.Unmarshal(bSliced, &oMap)
			if err != nil {
				return false, err
			}

			if value, ok := oMap.GoMap["sensor-id"]; ok {
				if value == sensor.ID {
					return true, nil
				}
			}
		}
	}

	return false, nil
}

func (o *OvsSFlowSensorsHandler) retrieveSFlowSensorUUID(monitor *OvsMonitor, sensor SFlowSensor) (string, error) {
	/* FIX(safchain) don't find a way to send a null condition */
	condition := libovsdb.NewCondition("_uuid", "!=", libovsdb.UUID{"abc"})
	selectOp := libovsdb.Operation{
		Op:    "select",
		Table: "sFlow",
		Where: []interface{}{condition},
	}

	operations := []libovsdb.Operation{selectOp}
	result, err := monitor.OvsClient.Exec(operations...)
	if err != nil {
		return "", err
	}

	for _, o := range result {
		for _, row := range o.Rows {
			u := row["_uuid"].([]interface{})[1]
			uuid := u.(string)

			if targets, ok := row["targets"]; ok {
				if targets != sensor.Target {
					continue
				}
			}

			if polling, ok := row["polling"]; ok {
				if uint32(polling.(float64)) != sensor.Polling {
					continue
				}
			}

			if sampling, ok := row["sampling"]; ok {
				if uint32(sampling.(float64)) != sensor.Sampling {
					continue
				}
			}

			if ok, _ := compareSensorID(&row, sensor); ok {
				return uuid, nil
			}
		}
	}

	return "", nil
}

func (o *OvsSFlowSensorsHandler) registerSFLowSensor(monitor *OvsMonitor, sensor SFlowSensor, bridgeUUID string) error {
	sensorUUID, err := o.retrieveSFlowSensorUUID(monitor, sensor)
	if err != nil {
		return err
	}

	operations := []libovsdb.Operation{}

	var uuid libovsdb.UUID
	if sensorUUID != "" {
		uuid = libovsdb.UUID{sensorUUID}

		logging.GetLogger().Info("Using already registered sFlow sensor \"%s(%s)\"", sensor.ID, uuid)
	} else {
		insertOp, err := newInsertSFlowSensorOP(sensor)
		if err != nil {
			return err
		}
		uuid = libovsdb.UUID{insertOp.UUIDName}
		logging.GetLogger().Info("Registering new sFlow sensor \"%s(%s)\"", sensor.ID, uuid)

		operations = append(operations, *insertOp)
	}

	bridgeRow := make(map[string]interface{})
	bridgeRow["sflow"] = uuid

	condition := libovsdb.NewCondition("_uuid", "==", libovsdb.UUID{bridgeUUID})
	updateOp := libovsdb.Operation{
		Op:    "update",
		Table: "Bridge",
		Row:   bridgeRow,
		Where: []interface{}{condition},
	}

	operations = append(operations, updateOp)
	_, err = monitor.OvsClient.Exec(operations...)
	if err != nil {
		return err
	}
	return nil
}

func (o *OvsSFlowSensorsHandler) registerSensor(monitor *OvsMonitor, sensor SFlowSensor, bridgeUUID string) error {
	err := o.registerSFLowSensor(monitor, sensor, bridgeUUID)
	if err != nil {
		return err
	}
	return nil
}

func (o *OvsSFlowSensorsHandler) registerSensors(monitor *OvsMonitor, bridgeUUID string) {
	for _, sensor := range o.sensors {
		err := o.registerSensor(monitor, sensor, bridgeUUID)
		if err != nil {
			logging.GetLogger().Error("Error while registering sensor %s", err.Error())
		}
	}
}

func (o *OvsSFlowSensorsHandler) OnOvsBridgeUpdate(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
}

func (o *OvsSFlowSensorsHandler) OnOvsBridgeAdd(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	o.registerSensors(monitor, uuid)
}

func (o *OvsSFlowSensorsHandler) OnOvsBridgeDel(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
}

func (o *OvsSFlowSensorsHandler) OnOvsInterfaceUpdate(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
}

func (o *OvsSFlowSensorsHandler) OnOvsInterfaceAdd(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
}

func (o *OvsSFlowSensorsHandler) OnOvsInterfaceDel(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
}

func (o *OvsSFlowSensorsHandler) OnOvsPortUpdate(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
}

func (o *OvsSFlowSensorsHandler) OnOvsPortAdd(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
}

func (o *OvsSFlowSensorsHandler) OnOvsPortDel(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
}

func NewOvsSFlowSensorsHandler(sensors []SFlowSensor) *OvsSFlowSensorsHandler {
	return &OvsSFlowSensorsHandler{sensors: sensors}
}
