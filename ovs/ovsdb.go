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
	"errors"
	"reflect"

	"github.com/socketplane/libovsdb"

	"github.com/redhat-cip/skydive/agents"
	"github.com/redhat-cip/skydive/logging"
)

type Agent interface {
}

type SFlowAgent struct {
	Id         string
	Interface  string
	Agent      agents.SFlowAgent
	HeaderSize uint32
	Sampling   uint32
	Polling    uint32
}

type IpFixAgent struct {
	Id string
}

var ovsAgents []Agent

var ovsDb *libovsdb.OvsdbClient
var bridgeCache = map[string]string{}

type Notifier struct {
}

func (n Notifier) Update(context interface{}, tableUpdates libovsdb.TableUpdates) {
	registerAgents(&tableUpdates)
}

func (n Notifier) Locked([]interface{}) {
}

func (n Notifier) Stolen([]interface{}) {
}

func (n Notifier) Echo([]interface{}) {
}

func (n Notifier) Disconnected(*libovsdb.OvsdbClient) {
	/* TODO(safchain) handle connection lost */
}

func execOps(operations ...libovsdb.Operation) ([]libovsdb.OperationResult, error) {
	result, err := ovsDb.Transact("Open_vSwitch", operations...)
	if err != nil {
		return nil, errors.New(
			"Replies number should be atleast equal to number of Operations ")
	}

	if len(result) < len(operations) {
		return nil, errors.New(
			"Replies number should be atleast equal to number of Operations ")
	}

	for i, o := range result {
		if o.Error != "" && i < len(operations) {
			return nil, errors.New(
				"Transaction Failed due to an error :" +
					o.Error + " details:" + o.Details)
		} else if o.Error != "" {
			return nil, errors.New(
				"Transaction Failed due to an error :" + o.Error)
		}
	}
	return result, nil
}

func NewInsertSFlowAgentOP(agent SFlowAgent) (*libovsdb.Operation, error) {
	sFlowRow := make(map[string]interface{})
	sFlowRow["agent"] = agent.Interface
	sFlowRow["targets"] = agent.Agent.GetTarget()
	sFlowRow["header"] = agent.HeaderSize
	sFlowRow["sampling"] = agent.Sampling
	sFlowRow["polling"] = agent.Polling

	extIds := make(map[string]string)
	extIds["agent-id"] = agent.Id
	ovsMap, err := libovsdb.NewOvsMap(extIds)
	if err != nil {
		return nil, err
	}
	sFlowRow["external_ids"] = ovsMap

	namedUuid := agent.Id
	insertOp := libovsdb.Operation{
		Op:       "insert",
		Table:    "sFlow",
		Row:      sFlowRow,
		UUIDName: namedUuid,
	}

	return &insertOp, nil
}

func compareAgentId(row *map[string]interface{}, agent SFlowAgent) (bool, error) {
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
				if value == agent.Id {
					return true, nil
				}
			}
		}
	}

	return false, nil
}

func retrieveSFlowAgentUuid(agent SFlowAgent) (string, error) {
	/* FIX(safchain) don't find a way to send a null condition */
	condition := libovsdb.NewCondition("_uuid", "!=", libovsdb.UUID{"abc"})
	selectOp := libovsdb.Operation{
		Op:    "select",
		Table: "sFlow",
		Where: []interface{}{condition},
	}

	operations := []libovsdb.Operation{selectOp}
	result, err := execOps(operations...)
	if err != nil {
		return "", err
	}

	for _, o := range result {
		for _, row := range o.Rows {
			u := row["_uuid"].([]interface{})[1]
			uuid := u.(string)

			if targets, ok := row["targets"]; ok {
				if targets != agent.Agent.GetTarget() {
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

			if ok, _ := compareAgentId(&row, agent); ok {
				return uuid, nil
			}
		}
	}

	return "", nil
}

func registerSFLowAgent(agent SFlowAgent, bridgeUuid string) error {
	agentUuid, err := retrieveSFlowAgentUuid(agent)
	if err != nil {
		return err
	}

	operations := []libovsdb.Operation{}

	var uuid libovsdb.UUID
	if agentUuid != "" {
		uuid = libovsdb.UUID{agentUuid}

		logging.GetLogger().Info("Using already registered sFlow agent \"%s(%s)\"", agent.Id, uuid)
	} else {
		insertOp, err := NewInsertSFlowAgentOP(agent)
		if err != nil {
			return err
		}
		uuid = libovsdb.UUID{insertOp.UUIDName}
		logging.GetLogger().Info("Registering new sFlow agent \"%s(%s)\"", agent.Id, uuid)

		operations = append(operations, *insertOp)
	}

	bridgeRow := make(map[string]interface{})
	bridgeRow["sflow"] = uuid

	condition := libovsdb.NewCondition("_uuid", "==", libovsdb.UUID{bridgeUuid})
	updateOp := libovsdb.Operation{
		Op:    "update",
		Table: "Bridge",
		Row:   bridgeRow,
		Where: []interface{}{condition},
	}

	operations = append(operations, updateOp)
	_, err = execOps(operations...)
	if err != nil {
		return err
	}
	return nil
}

func registerAgent(agent Agent, bridgeUuid string) error {
	switch t := agent.(type) {
	case SFlowAgent:
		sflowAgent := agent.(SFlowAgent)

		err := registerSFLowAgent(sflowAgent, bridgeUuid)
		if err != nil {
			return err
		}
	default:
		return errors.New("Agent type unknown " + reflect.TypeOf(t).String())
	}

	return nil
}

func registerAgents(updates *libovsdb.TableUpdates) {
	empty := libovsdb.Row{}

	for _, tableUpdate := range updates.Updates {
		for bridgeUuid, row := range tableUpdate.Rows {
			if !reflect.DeepEqual(row.New, empty) {
				if _, ok := bridgeCache[bridgeUuid]; ok {
					continue
				}
				bridgeCache[bridgeUuid] = bridgeUuid

				logging.GetLogger().Info("New bridge \"%s(%s)\" added, registering agents",
					row.New.Fields["name"], bridgeUuid)

				for _, agent := range ovsAgents {
					err := registerAgent(agent, bridgeUuid)
					if err != nil {
						logging.GetLogger().Error("Error while registering agent %s", err)
					}
				}
			} else {
				delete(bridgeCache, bridgeUuid)

				/* NOTE: got delete, ovs will release the agent if not anymore referenced */
				logging.GetLogger().Info("Bridge \"%s(%s)\" got deleted",
					row.Old.Fields["name"], bridgeUuid)
			}
		}
	}
}

func registerBridgeHandler() (*libovsdb.TableUpdates, error) {
	schema, ok := ovsDb.Schema["Open_vSwitch"]
	if !ok {
		return nil, errors.New("invalid Database Schema")
	}

	requests := make(map[string]libovsdb.MonitorRequest)
	var columns []string
	for column, _ := range schema.Tables["Bridge"].Columns {
		columns = append(columns, column)
	}
	requests["Bridge"] = libovsdb.MonitorRequest{
		Columns: columns,
		Select: libovsdb.MonitorSelect{
			Initial: true,
			Insert:  true,
			Delete:  true,
			Modify:  true,
		},
	}
	return ovsDb.Monitor("Open_vSwitch", "", requests)
}

func StartBridgesMonitor(addr string, port int, agts []Agent) error {
	var err error
	ovsDb, err = libovsdb.Connect(addr, 6400)
	if err != nil {
		return err
	}

	ovsAgents = agts

	var notifier Notifier
	ovsDb.Register(notifier)

	updates, err := registerBridgeHandler()
	if err != nil {
		return err
	}

	registerAgents(updates)

	return nil
}
