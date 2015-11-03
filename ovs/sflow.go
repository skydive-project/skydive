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

type SFlowAgent struct {
	ID         string
	Interface  string
	Target     string
	HeaderSize uint32
	Sampling   uint32
	Polling    uint32
}

type OvsSFlowAgentsHandler struct {
	agents []SFlowAgent
}

func newInsertSFlowAgentOP(agent SFlowAgent) (*libovsdb.Operation, error) {
	sFlowRow := make(map[string]interface{})
	sFlowRow["agent"] = agent.Interface
	sFlowRow["targets"] = agent.Target
	sFlowRow["header"] = agent.HeaderSize
	sFlowRow["sampling"] = agent.Sampling
	sFlowRow["polling"] = agent.Polling

	extIds := make(map[string]string)
	extIds["agent-id"] = agent.ID
	ovsMap, err := libovsdb.NewOvsMap(extIds)
	if err != nil {
		return nil, err
	}
	sFlowRow["external_ids"] = ovsMap

	insertOp := libovsdb.Operation{
		Op:       "insert",
		Table:    "sFlow",
		Row:      sFlowRow,
		UUIDName: agent.ID,
	}

	return &insertOp, nil
}

func compareAgentID(row *map[string]interface{}, agent SFlowAgent) (bool, error) {
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

			if value, ok := oMap.GoMap["agent-id"]; ok {
				if value == agent.ID {
					return true, nil
				}
			}
		}
	}

	return false, nil
}

func (o *OvsSFlowAgentsHandler) retrieveSFlowAgentUUID(monitor *OvsMonitor, agent SFlowAgent) (string, error) {
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
				if targets != agent.Target {
					continue
				}
			}

			if polling, ok := row["polling"]; ok {
				if uint32(polling.(float64)) != agent.Polling {
					continue
				}
			}

			if sampling, ok := row["sampling"]; ok {
				if uint32(sampling.(float64)) != agent.Sampling {
					continue
				}
			}

			if ok, _ := compareAgentID(&row, agent); ok {
				return uuid, nil
			}
		}
	}

	return "", nil
}

func (o *OvsSFlowAgentsHandler) registerSFLowAgent(monitor *OvsMonitor, agent SFlowAgent, bridgeUUID string) error {
	agentUUID, err := o.retrieveSFlowAgentUUID(monitor, agent)
	if err != nil {
		return err
	}

	operations := []libovsdb.Operation{}

	var uuid libovsdb.UUID
	if agentUUID != "" {
		uuid = libovsdb.UUID{agentUUID}

		logging.GetLogger().Info("Using already registered sFlow agent \"%s(%s)\"", agent.ID, uuid)
	} else {
		insertOp, err := newInsertSFlowAgentOP(agent)
		if err != nil {
			return err
		}
		uuid = libovsdb.UUID{insertOp.UUIDName}
		logging.GetLogger().Info("Registering new sFlow agent \"%s(%s)\"", agent.ID, uuid)

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

func (o *OvsSFlowAgentsHandler) registerAgent(monitor *OvsMonitor, agent SFlowAgent, bridgeUUID string) error {
	err := o.registerSFLowAgent(monitor, agent, bridgeUUID)
	if err != nil {
		return err
	}
	return nil
}

func (o *OvsSFlowAgentsHandler) registerAgents(monitor *OvsMonitor, bridgeUUID string) {
	for _, agent := range o.agents {
		err := o.registerAgent(monitor, agent, bridgeUUID)
		if err != nil {
			logging.GetLogger().Error("Error while registering agent %s", err)
		}
	}
}

func (o *OvsSFlowAgentsHandler) OnOvsBridgeAdd(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	o.registerAgents(monitor, uuid)
}

func (o *OvsSFlowAgentsHandler) OnOvsBridgeDel(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate) {

}

func (o *OvsSFlowAgentsHandler) OnOvsInterfaceAdd(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate) {

}

func (o *OvsSFlowAgentsHandler) OnOvsInterfaceDel(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate) {

}

func (o *OvsSFlowAgentsHandler) OnOvsPortAdd(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate) {

}

func (o *OvsSFlowAgentsHandler) OnOvsPortDel(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate) {

}

func NewOvsSFlowAgentsHandler(agents []SFlowAgent) *OvsSFlowAgentsHandler {
	return &OvsSFlowAgentsHandler{agents: agents}
}
