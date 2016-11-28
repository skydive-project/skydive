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

package probes

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/socketplane/libovsdb"

	"github.com/skydive-project/skydive/api"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/ovs"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/sflow"
	"github.com/skydive-project/skydive/topology/graph"
	"github.com/skydive-project/skydive/topology/probes"
)

type OvsSFlowProbe struct {
	ID         string
	Interface  string
	Target     string
	HeaderSize uint32
	Sampling   uint32
	Polling    uint32
	NodeTID    string
}

type OvsSFlowProbesHandler struct {
	FlowProbe
	Graph     *graph.Graph
	ovsClient *ovsdb.OvsClient
	allocator *sflow.SFlowAgentAllocator
}

func probeID(i string) string {
	return "SkydiveSFlowProbe_" + strings.Replace(i, "-", "_", -1)
}

func newInsertSFlowProbeOP(probe OvsSFlowProbe) (*libovsdb.Operation, error) {
	sFlowRow := make(map[string]interface{})
	sFlowRow["agent"] = probe.Interface
	sFlowRow["targets"] = probe.Target
	sFlowRow["header"] = probe.HeaderSize
	sFlowRow["sampling"] = probe.Sampling
	sFlowRow["polling"] = probe.Polling

	extIds := make(map[string]string)
	extIds["probe-id"] = probe.ID
	ovsMap, err := libovsdb.NewOvsMap(extIds)
	if err != nil {
		return nil, err
	}
	sFlowRow["external_ids"] = ovsMap

	insertOp := libovsdb.Operation{
		Op:       "insert",
		Table:    "sFlow",
		Row:      sFlowRow,
		UUIDName: probe.ID,
	}

	return &insertOp, nil
}

func compareProbeID(row *map[string]interface{}, id string) (bool, error) {
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

			if value, ok := oMap.GoMap["probe-id"]; ok {
				if value.(string) == id {
					return true, nil
				}
			}
		}
	}

	return false, nil
}

func (o *OvsSFlowProbesHandler) retrieveSFlowProbeUUID(id string) (string, error) {
	/* FIX(safchain) don't find a way to send a null condition */
	condition := libovsdb.NewCondition("_uuid", "!=", libovsdb.UUID{GoUUID: "abc"})
	selectOp := libovsdb.Operation{
		Op:    "select",
		Table: "sFlow",
		Where: []interface{}{condition},
	}

	operations := []libovsdb.Operation{selectOp}
	result, err := o.ovsClient.Exec(operations...)
	if err != nil {
		return "", err
	}

	for _, o := range result {
		for _, row := range o.Rows {
			u := row["_uuid"].([]interface{})[1]
			uuid := u.(string)

			if ok, _ := compareProbeID(&row, id); ok {
				return uuid, nil
			}
		}
	}

	return "", nil
}

func (o *OvsSFlowProbesHandler) registerSFlowProbeOnBridge(probe OvsSFlowProbe, bridgeUUID string) error {
	probeUUID, err := o.retrieveSFlowProbeUUID(probeID(bridgeUUID))
	if err != nil {
		return err
	}

	operations := []libovsdb.Operation{}

	var uuid libovsdb.UUID
	if probeUUID != "" {
		uuid = libovsdb.UUID{GoUUID: probeUUID}

		logging.GetLogger().Infof("Using already registered OVS SFlow probe \"%s(%s)\"", probe.ID, uuid)
	} else {
		insertOp, err := newInsertSFlowProbeOP(probe)
		if err != nil {
			return err
		}
		uuid = libovsdb.UUID{GoUUID: insertOp.UUIDName}
		logging.GetLogger().Infof("Registering new OVS SFlow probe \"%s(%s)\"", probe.ID, uuid)

		operations = append(operations, *insertOp)
	}

	bridgeRow := make(map[string]interface{})
	bridgeRow["sflow"] = uuid

	condition := libovsdb.NewCondition("_uuid", "==", libovsdb.UUID{GoUUID: bridgeUUID})
	updateOp := libovsdb.Operation{
		Op:    "update",
		Table: "Bridge",
		Row:   bridgeRow,
		Where: []interface{}{condition},
	}

	operations = append(operations, updateOp)
	_, err = o.ovsClient.Exec(operations...)
	if err != nil {
		return err
	}
	return nil
}

func (o *OvsSFlowProbesHandler) UnregisterSFlowProbeFromBridge(bridgeUUID string) error {
	o.allocator.Release(bridgeUUID)

	probeUUID, err := o.retrieveSFlowProbeUUID(probeID(bridgeUUID))
	if err != nil {
		return err
	}
	if probeUUID == "" {
		return nil
	}

	operations := []libovsdb.Operation{}

	bridgeRow := make(map[string]interface{})
	bridgeRow["sflow"] = libovsdb.OvsSet{GoSet: make([]interface{}, 0)}

	condition := libovsdb.NewCondition("_uuid", "==", libovsdb.UUID{GoUUID: bridgeUUID})
	updateOp := libovsdb.Operation{
		Op:    "update",
		Table: "Bridge",
		Row:   bridgeRow,
		Where: []interface{}{condition},
	}

	operations = append(operations, updateOp)
	_, err = o.ovsClient.Exec(operations...)
	if err != nil {
		return err
	}
	return nil
}

func (o *OvsSFlowProbesHandler) RegisterProbeOnBridge(bridgeUUID string, tid string, ft *flow.Table) error {
	probe := OvsSFlowProbe{
		ID:         probeID(bridgeUUID),
		Interface:  "lo",
		HeaderSize: 256,
		Sampling:   1,
		Polling:    0,
		NodeTID:    tid,
	}

	agent, err := o.allocator.Alloc(bridgeUUID, ft)
	if err != nil && err != sflow.AgentAlreadyAllocated {
		return err
	}

	probe.Target = agent.GetTarget()

	err = o.registerSFlowProbeOnBridge(probe, bridgeUUID)
	if err != nil {
		return err
	}
	return nil
}

func isOvsBridge(n *graph.Node) bool {
	return n.Metadata()["UUID"] != "" && n.Metadata()["Type"] == "ovsbridge"
}

func (o *OvsSFlowProbesHandler) RegisterProbe(n *graph.Node, capture *api.Capture, ft *flow.Table) error {
	tid, ok := n.Metadata()["TID"]
	if !ok {
		return fmt.Errorf("No TID for node %v", n)
	}

	if isOvsBridge(n) {
		err := o.RegisterProbeOnBridge(n.Metadata()["UUID"].(string), tid.(string), ft)
		if err != nil {
			return err
		}
	}
	return nil
}

func (o *OvsSFlowProbesHandler) unregisterProbe(bridgeUUID string) error {
	err := o.UnregisterSFlowProbeFromBridge(bridgeUUID)
	if err != nil {
		return err
	}
	return nil
}

func (o *OvsSFlowProbesHandler) UnregisterProbe(n *graph.Node) error {
	if isOvsBridge(n) {
		err := o.unregisterProbe(n.Metadata()["UUID"].(string))
		if err != nil {
			return err
		}
	}
	return nil
}

func (o *OvsSFlowProbesHandler) Start() {
}

func (o *OvsSFlowProbesHandler) Stop() {
	o.allocator.ReleaseAll()
}

func NewOvsSFlowProbesHandler(tb *probe.ProbeBundle, g *graph.Graph) *OvsSFlowProbesHandler {
	probe := tb.GetProbe("ovsdb")
	if probe == nil {
		logging.GetLogger().Error("Agent.ovssflow probe depends on agent.ovsdb topology probe: agent.ovssflow probe can't start properly")
		return nil
	}
	p := probe.(*probes.OvsdbProbe)

	o := &OvsSFlowProbesHandler{
		Graph:     g,
		ovsClient: p.OvsMon.OvsClient,
		allocator: sflow.NewSFlowAgentAllocator(),
	}

	return o
}
