/*
 * Copyright (C) 2015 Red Hat, Inc.
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

package probes

import (
	"errors"
	"fmt"

	"github.com/socketplane/libovsdb"

	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/ovs/ovsdb"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/sflow"
	ovsprobe "github.com/skydive-project/skydive/topology/probes/ovsdb"
)

// OvsSFlowProbe describes a SFlow probe from OVS switch
type OvsSFlowProbe struct {
	ID         string
	Interface  string
	Target     string
	HeaderSize uint32
	Sampling   uint32
	Polling    uint32
	flowTable  *flow.Table
}

// OvsSFlowProbesHandler describes a flow probe in running in the graph
type OvsSFlowProbesHandler struct {
	probes       map[string]OvsSFlowProbe
	probesLock   common.RWMutex
	Graph        *graph.Graph
	Node         *graph.Node
	fpta         *FlowProbeTableAllocator
	ovsClient    *ovsdb.OvsClient
	allocator    *sflow.AgentAllocator
	eventHandler FlowProbeEventHandler
}

func newInsertSFlowProbeOP(probe OvsSFlowProbe) (*libovsdb.Operation, error) {
	sFlowRow := make(map[string]interface{})
	sFlowRow["agent"] = probe.Interface
	sFlowRow["targets"] = probe.Target
	sFlowRow["header"] = probe.HeaderSize
	sFlowRow["sampling"] = probe.Sampling
	sFlowRow["polling"] = probe.Polling

	extIds := make(map[string]string)
	extIds["skydive-probe-id"] = ovsProbeID(probe.ID)
	ovsMap, err := libovsdb.NewOvsMap(extIds)
	if err != nil {
		return nil, err
	}
	sFlowRow["external_ids"] = ovsMap

	insertOp := libovsdb.Operation{
		Op:       "insert",
		Table:    "sFlow",
		Row:      sFlowRow,
		UUIDName: ovsNamedUUID(probe.ID),
	}

	return &insertOp, nil
}

func (o *OvsSFlowProbesHandler) registerSFlowProbeOnBridge(probe OvsSFlowProbe, bridgeUUID string) error {
	o.probesLock.Lock()
	o.probes[bridgeUUID] = probe
	o.probesLock.Unlock()

	probeUUID, err := ovsRetrieveSkydiveProbeRowUUID(o.ovsClient, "sFlow", ovsProbeID(bridgeUUID))
	if err != nil {
		return err
	}

	operations := []libovsdb.Operation{}

	var uuid libovsdb.UUID
	if probeUUID != "" {
		uuid = libovsdb.UUID{GoUUID: probeUUID}

		logging.GetLogger().Infof("Using already registered OVS SFlow probe \"%s\"", probe.ID)
	} else {
		insertOp, err := newInsertSFlowProbeOP(probe)
		if err != nil {
			return err
		}
		uuid = libovsdb.UUID{GoUUID: insertOp.UUIDName}
		logging.GetLogger().Infof("Registering new OVS SFlow probe \"%s\"", probe.ID)

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

// UnregisterSFlowProbeFromBridge unregisters a flow probe from the bridge selected by UUID
func (o *OvsSFlowProbesHandler) UnregisterSFlowProbeFromBridge(bridgeUUID string) error {
	o.allocator.Release(bridgeUUID)

	o.probesLock.RLock()
	probe, ok := o.probes[bridgeUUID]
	if !ok {
		o.probesLock.RUnlock()
		return fmt.Errorf("probe didn't exist on bridgeUUID %s", bridgeUUID)
	}
	o.fpta.Release(probe.flowTable)
	o.probesLock.RUnlock()

	probeUUID, err := ovsRetrieveSkydiveProbeRowUUID(o.ovsClient, "sFlow", ovsProbeID(bridgeUUID))
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

// RegisterProbeOnBridge registers a new probe on the OVS bridge
func (o *OvsSFlowProbesHandler) RegisterProbeOnBridge(bridgeUUID string, tid string, capture *types.Capture, n *graph.Node) error {
	headerSize := flow.DefaultCaptureLength
	if capture.HeaderSize != 0 {
		headerSize = uint32(capture.HeaderSize)
	}

	opts := tableOptsFromCapture(capture)
	ft := o.fpta.Alloc(tid, opts)

	probe := OvsSFlowProbe{
		ID:         bridgeUUID,
		Interface:  "lo",
		HeaderSize: headerSize,
		Sampling:   1,
		Polling:    10,
		flowTable:  ft,
	}

	address := config.GetString("sflow.bind_address")
	if address == "" {
		address = "127.0.0.1"
	}

	addr := common.ServiceAddress{Addr: address, Port: 0}
	agent, err := o.allocator.Alloc(bridgeUUID, probe.flowTable, capture.BPFFilter, headerSize, &addr, n, o.Graph)
	if err != nil && err != sflow.ErrAgentAlreadyAllocated {
		return err
	}

	probe.Target = agent.GetTarget()

	return o.registerSFlowProbeOnBridge(probe, bridgeUUID)
}

func isOvsBridge(n *graph.Node) bool {
	if uuid, _ := n.GetFieldString("UUID"); uuid == "" {
		return false
	}
	if tp, _ := n.GetFieldString("Type"); tp == "ovsbridge" {
		return true
	}

	return false
}

func (o *OvsSFlowProbesHandler) registerProbe(n *graph.Node, capture *types.Capture, e FlowProbeEventHandler) error {
	tid, _ := n.GetFieldString("TID")
	if tid == "" {
		return fmt.Errorf("No TID for node %v", n)
	}

	if isOvsBridge(n) {
		if uuid, _ := n.GetFieldString("UUID"); uuid != "" {
			if err := o.RegisterProbeOnBridge(uuid, tid, capture, n); err != nil {
				return err
			}
			go e.OnStarted()
		}
	}
	return nil
}

// RegisterProbe registers a probe on a graph node
func (o *OvsSFlowProbesHandler) RegisterProbe(n *graph.Node, capture *types.Capture, e FlowProbeEventHandler) error {
	err := o.registerProbe(n, capture, e)
	if err != nil {
		go e.OnError(err)
	}
	return err
}

// UnregisterProbe at the graph node
func (o *OvsSFlowProbesHandler) UnregisterProbe(n *graph.Node, e FlowProbeEventHandler) error {
	if isOvsBridge(n) {
		if uuid, _ := n.GetFieldString("UUID"); uuid != "" {
			if err := o.UnregisterSFlowProbeFromBridge(uuid); err != nil {
				return err
			}
			go e.OnStopped()
		}
	}
	return nil
}

// Start the probe
func (o *OvsSFlowProbesHandler) Start() {
}

// Stop the probe
func (o *OvsSFlowProbesHandler) Stop() {
	o.allocator.ReleaseAll()
}

// NewOvsSFlowProbesHandler creates a new OVS SFlow porbes
func NewOvsSFlowProbesHandler(g *graph.Graph, fpta *FlowProbeTableAllocator, tb *probe.Bundle) (*OvsSFlowProbesHandler, error) {
	probe := tb.GetProbe("ovsdb")
	if probe == nil {
		return nil, errors.New("Agent.ovssflow probe depends on agent.ovsdb topology probe: agent.ovssflow probe can't start properly")
	}
	p := probe.(*ovsprobe.Probe)

	allocator, err := sflow.NewAgentAllocator()
	if err != nil {
		return nil, err
	}

	return &OvsSFlowProbesHandler{
		probes:    make(map[string]OvsSFlowProbe),
		Graph:     g,
		fpta:      fpta,
		ovsClient: p.OvsMon.OvsClient,
		allocator: allocator,
	}, nil
}
