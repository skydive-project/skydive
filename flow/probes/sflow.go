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

	"github.com/socketplane/libovsdb"

	"github.com/redhat-cip/skydive/analyzer"
	"github.com/redhat-cip/skydive/flow/mappings"
	"github.com/redhat-cip/skydive/logging"
	"github.com/redhat-cip/skydive/ovs"
	"github.com/redhat-cip/skydive/sflow"
	"github.com/redhat-cip/skydive/topology/graph"
	"github.com/redhat-cip/skydive/topology/probes"
)

type OvsSFlowProbe struct {
	ID         string
	Interface  string
	Target     string
	HeaderSize uint32
	Sampling   uint32
	Polling    uint32
}

type OvsSFlowProbesHandler struct {
	ovsmon *ovsdb.OvsMonitor
	agent  *sflow.SFlowAgent
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

func compareProbeID(row *map[string]interface{}, probe OvsSFlowProbe) (bool, error) {
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
				if value == probe.ID {
					return true, nil
				}
			}
		}
	}

	return false, nil
}

func (o *OvsSFlowProbesHandler) retrieveSFlowProbeUUID(monitor *ovsdb.OvsMonitor, probe OvsSFlowProbe) (string, error) {
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
				if targets != probe.Target {
					continue
				}
			}

			if polling, ok := row["polling"]; ok {
				if uint32(polling.(float64)) != probe.Polling {
					continue
				}
			}

			if sampling, ok := row["sampling"]; ok {
				if uint32(sampling.(float64)) != probe.Sampling {
					continue
				}
			}

			if ok, _ := compareProbeID(&row, probe); ok {
				return uuid, nil
			}
		}
	}

	return "", nil
}

func (o *OvsSFlowProbesHandler) registerSFlowProbe(monitor *ovsdb.OvsMonitor, probe OvsSFlowProbe, bridgeUUID string) error {
	probeUUID, err := o.retrieveSFlowProbeUUID(monitor, probe)
	if err != nil {
		return err
	}

	operations := []libovsdb.Operation{}

	var uuid libovsdb.UUID
	if probeUUID != "" {
		uuid = libovsdb.UUID{probeUUID}

		logging.GetLogger().Infof("Using already registered OVS sFlow probe \"%s(%s)\"", probe.ID, uuid)
	} else {
		insertOp, err := newInsertSFlowProbeOP(probe)
		if err != nil {
			return err
		}
		uuid = libovsdb.UUID{insertOp.UUIDName}
		logging.GetLogger().Infof("Registering new OVS sFlow probe \"%s(%s)\"", probe.ID, uuid)

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

func (o *OvsSFlowProbesHandler) registerProbe(monitor *ovsdb.OvsMonitor, bridgeUUID string) error {
	// TODO(safchain) add config parameter
	probe := OvsSFlowProbe{
		ID:         "SkydiveSFlowProbe",
		Interface:  "eth0",
		Target:     o.agent.GetTarget(),
		HeaderSize: 256,
		Sampling:   1,
		Polling:    0,
	}

	err := o.registerSFlowProbe(monitor, probe, bridgeUUID)
	if err != nil {
		return err
	}
	return nil
}

func (o *OvsSFlowProbesHandler) OnOvsBridgeUpdate(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
}

func (o *OvsSFlowProbesHandler) OnOvsBridgeAdd(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	o.registerProbe(monitor, uuid)
}

func (o *OvsSFlowProbesHandler) OnOvsBridgeDel(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
}

func (o *OvsSFlowProbesHandler) OnOvsInterfaceUpdate(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
}

func (o *OvsSFlowProbesHandler) OnOvsInterfaceAdd(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
}

func (o *OvsSFlowProbesHandler) OnOvsInterfaceDel(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
}

func (o *OvsSFlowProbesHandler) OnOvsPortUpdate(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
}

func (o *OvsSFlowProbesHandler) OnOvsPortAdd(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
}

func (o *OvsSFlowProbesHandler) OnOvsPortDel(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
}

func (o *OvsSFlowProbesHandler) Start() {
	o.ovsmon.AddMonitorHandler(o)
	o.agent.Start()
}

func (o *OvsSFlowProbesHandler) Stop() {
	// TODO(safchain) call RemoveMonitorHandler when implemented
	o.agent.Stop()
}

func NewOvsSFlowProbesHandler(p *probes.OvsdbProbe, agent *sflow.SFlowAgent) *OvsSFlowProbesHandler {
	o := &OvsSFlowProbesHandler{
		ovsmon: p.OvsMon,
		agent:  agent,
	}

	return o
}

func NewOvsSFlowProbesHandlerFromConfig(tb *probes.TopologyProbeBundle, g *graph.Graph, p *mappings.FlowMappingPipeline, a *analyzer.Client) *OvsSFlowProbesHandler {
	probe := tb.GetProbe("ovsdb")
	if probe == nil {
		return nil
	}

	agent, err := sflow.NewSFlowAgentFromConfig(g)
	if err != nil {
		logging.GetLogger().Errorf("Unable to start a sflow agent: %s", err.Error())
		return nil
	}
	agent.SetMappingPipeline(p)

	if a != nil {
		agent.SetAnalyzerClient(a)
	}

	return NewOvsSFlowProbesHandler(probe.(*probes.OvsdbProbe), agent)
}
