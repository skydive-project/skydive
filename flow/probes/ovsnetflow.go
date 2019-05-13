/*
 * Copyright (C) 2019 Red Hat, Inc.
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
	"github.com/skydive-project/skydive/netflow"
	"github.com/skydive-project/skydive/ovs/ovsdb"
	"github.com/skydive-project/skydive/probe"
	ovsprobe "github.com/skydive-project/skydive/topology/probes/ovsdb"
)

// OvsNetFlowProbe describes a NetFlow probe from OVS switch
type OvsNetFlowProbe struct {
	ID        string
	Target    string
	EngineID  int
	flowTable *flow.Table
}

// OvsNetFlowProbesHandler describes a flow probe in running in the graph
type OvsNetFlowProbesHandler struct {
	probes       map[string]OvsNetFlowProbe
	probesLock   common.RWMutex
	Graph        *graph.Graph
	Node         *graph.Node
	fpta         *FlowProbeTableAllocator
	ovsClient    *ovsdb.OvsClient
	allocator    *netflow.AgentAllocator
	eventHandler FlowProbeEventHandler
	engineID     int
}

func newInsertNetFlowProbeOP(probe OvsNetFlowProbe) (*libovsdb.Operation, error) {
	netFlowRow := make(map[string]interface{})
	netFlowRow["add_id_to_interface"] = true
	netFlowRow["targets"] = probe.Target
	netFlowRow["engine_id"] = probe.EngineID
	netFlowRow["engine_type"] = 10

	extIds := make(map[string]string)
	extIds["skydive-probe-id"] = ovsProbeID(probe.ID)
	ovsMap, err := libovsdb.NewOvsMap(extIds)
	if err != nil {
		return nil, err
	}
	netFlowRow["external_ids"] = ovsMap

	return &libovsdb.Operation{
		Op:       "insert",
		Table:    "NetFlow",
		Row:      netFlowRow,
		UUIDName: ovsNamedUUID(probe.ID),
	}, nil
}

func (o *OvsNetFlowProbesHandler) registerNetFlowProbeOnBridge(probe OvsNetFlowProbe, bridgeUUID string) error {
	o.probesLock.Lock()
	o.probes[bridgeUUID] = probe
	o.probesLock.Unlock()

	probeUUID, err := ovsRetrieveSkydiveProbeRowUUID(o.ovsClient, "NetFlow", ovsProbeID(bridgeUUID))
	if err != nil {
		return err
	}

	operations := []libovsdb.Operation{}

	var uuid libovsdb.UUID
	if probeUUID != "" {
		uuid = libovsdb.UUID{GoUUID: probeUUID}

		logging.GetLogger().Infof("Using already registered OVS NetFlow probe \"%s\"", probe.ID)
	} else {
		insertOp, err := newInsertNetFlowProbeOP(probe)
		if err != nil {
			return err
		}
		o.engineID++

		uuid = libovsdb.UUID{GoUUID: insertOp.UUIDName}
		logging.GetLogger().Infof("Registering new OVS NetFlow probe \"%s\"", probe.ID)

		operations = append(operations, *insertOp)
	}

	bridgeRow := make(map[string]interface{})
	bridgeRow["netflow"] = uuid

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

// UnregisterNetFlowProbeFromBridge unregisters a flow probe from the bridge selected by UUID
func (o *OvsNetFlowProbesHandler) UnregisterNetFlowProbeFromBridge(bridgeUUID string) error {
	o.allocator.Release(bridgeUUID)

	o.probesLock.RLock()
	probe, ok := o.probes[bridgeUUID]
	if !ok {
		o.probesLock.RUnlock()
		return fmt.Errorf("probe didn't exist on bridgeUUID %s", bridgeUUID)
	}

	if probe.flowTable != nil {
		o.fpta.Release(probe.flowTable)
	}
	o.probesLock.RUnlock()

	probeUUID, err := ovsRetrieveSkydiveProbeRowUUID(o.ovsClient, "NetFlow", ovsProbeID(bridgeUUID))
	if err != nil {
		return err
	} else if probeUUID == "" {
		return nil
	}

	operations := []libovsdb.Operation{}

	bridgeRow := make(map[string]interface{})
	bridgeRow["netflow"] = libovsdb.OvsSet{GoSet: make([]interface{}, 0)}

	condition := libovsdb.NewCondition("_uuid", "==", libovsdb.UUID{GoUUID: bridgeUUID})
	updateOp := libovsdb.Operation{
		Op:    "update",
		Table: "Bridge",
		Row:   bridgeRow,
		Where: []interface{}{condition},
	}

	operations = append(operations, updateOp)
	_, err = o.ovsClient.Exec(operations...)
	return err
}

func (o *OvsNetFlowProbesHandler) registerProbeOnBridge(bridgeUUID string, tid string, capture *types.Capture) error {
	probe := OvsNetFlowProbe{
		ID:       bridgeUUID,
		EngineID: 1,
	}

	if capture.Target == "" {
		opts := tableOptsFromCapture(capture)
		probe.flowTable = o.fpta.Alloc(tid, opts)

		address := config.GetString("agent.flow.netflow.bind_address")
		if address == "" {
			address = "127.0.0.1"
		}

		addr := common.ServiceAddress{Addr: address, Port: 0}
		agent, err := o.allocator.Alloc(bridgeUUID, probe.flowTable, &addr, tid)
		if err != nil && err != netflow.ErrAgentAlreadyAllocated {
			return err
		}

		probe.Target = agent.GetTarget()
	} else {
		probe.Target = capture.Target
	}

	return o.registerNetFlowProbeOnBridge(probe, bridgeUUID)
}

func (o *OvsNetFlowProbesHandler) registerProbe(n *graph.Node, capture *types.Capture, e FlowProbeEventHandler) error {
	tid, _ := n.GetFieldString("TID")
	if tid == "" {
		return fmt.Errorf("No TID for node %v", n)
	}

	if isOvsBridge(n) {
		if uuid, _ := n.GetFieldString("UUID"); uuid != "" {
			if err := o.registerProbeOnBridge(uuid, tid, capture); err != nil {
				return err
			}
			go e.OnStarted()
		}
	}
	return nil
}

// RegisterProbe registers a probe on a graph node
func (o *OvsNetFlowProbesHandler) RegisterProbe(n *graph.Node, capture *types.Capture, e FlowProbeEventHandler) error {
	err := o.registerProbe(n, capture, e)
	if err != nil {
		go e.OnError(err)
	}
	return err
}

// UnregisterProbe at the graph node
func (o *OvsNetFlowProbesHandler) UnregisterProbe(n *graph.Node, e FlowProbeEventHandler) error {
	if isOvsBridge(n) {
		if uuid, _ := n.GetFieldString("UUID"); uuid != "" {
			if err := o.UnregisterNetFlowProbeFromBridge(uuid); err != nil {
				return err
			}
			go e.OnStopped()
		}
	}
	return nil
}

// Start the probe
func (o *OvsNetFlowProbesHandler) Start() {
}

// Stop the probe
func (o *OvsNetFlowProbesHandler) Stop() {
	o.allocator.ReleaseAll()
}

// NewOvsNetFlowProbesHandler creates a new OVS NetFlow porbes
func NewOvsNetFlowProbesHandler(g *graph.Graph, fpta *FlowProbeTableAllocator, tb *probe.Bundle) (*OvsNetFlowProbesHandler, error) {
	probe := tb.GetProbe("ovsdb")
	if probe == nil {
		return nil, errors.New("Agent.ovsnetflow probe depends on agent.ovsdb topology probe: agent.ovsnetflow probe can't start properly")
	}
	p := probe.(*ovsprobe.Probe)

	allocator, err := netflow.NewAgentAllocator()
	if err != nil {
		return nil, err
	}

	return &OvsNetFlowProbesHandler{
		probes:    make(map[string]OvsNetFlowProbe),
		Graph:     g,
		fpta:      fpta,
		ovsClient: p.OvsMon.OvsClient,
		allocator: allocator,
	}, nil
}
