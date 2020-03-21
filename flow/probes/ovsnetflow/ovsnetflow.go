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

package ovsnetflow

import (
	"errors"
	"fmt"

	"github.com/safchain/insanelock"
	"github.com/socketplane/libovsdb"

	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/flow/probes"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/service"
	"github.com/skydive-project/skydive/netflow"
	"github.com/skydive-project/skydive/ovs/ovsdb"
	"github.com/skydive-project/skydive/probe"
	ovsprobe "github.com/skydive-project/skydive/topology/probes/ovsdb"
)

// Probe describes a NetFlow probe from OVS switch
type Probe struct {
	ID        string
	Target    string
	EngineID  int
	flowTable *flow.Table
}

// ProbesHandler describes a flow probe in running in the graph
type ProbesHandler struct {
	Ctx          probes.Context
	probes       map[string]Probe
	probesLock   insanelock.RWMutex
	Node         *graph.Node
	ovsClient    *ovsdb.OvsClient
	allocator    *netflow.AgentAllocator
	eventHandler probes.ProbeEventHandler
	engineID     int
}

func newInsertNetFlowProbeOP(probe *Probe) (*libovsdb.Operation, error) {
	netFlowRow := make(map[string]interface{})
	netFlowRow["add_id_to_interface"] = true
	netFlowRow["targets"] = probe.Target
	netFlowRow["engine_id"] = probe.EngineID
	netFlowRow["engine_type"] = 10

	extIds := make(map[string]string)
	extIds["skydive-probe-id"] = ovsdb.ProbeID(probe.ID)
	ovsMap, err := libovsdb.NewOvsMap(extIds)
	if err != nil {
		return nil, err
	}
	netFlowRow["external_ids"] = ovsMap

	return &libovsdb.Operation{
		Op:       "insert",
		Table:    "NetFlow",
		Row:      netFlowRow,
		UUIDName: ovsdb.NamedUUID(probe.ID),
	}, nil
}

func (o *ProbesHandler) registerNetFlowProbeOnBridge(probe *Probe) error {
	probeUUID, err := o.ovsClient.RetrieveSkydiveProbeRowUUID("NetFlow", ovsdb.ProbeID(probe.ID))
	if err != nil {
		return err
	}

	operations := []libovsdb.Operation{}

	var uuid libovsdb.UUID
	if probeUUID != "" {
		uuid = libovsdb.UUID{GoUUID: probeUUID}

		o.Ctx.Logger.Infof("Using already registered OVS NetFlow probe \"%s\"", probe.ID)
	} else {
		insertOp, err := newInsertNetFlowProbeOP(probe)
		if err != nil {
			return err
		}
		o.engineID++

		uuid = libovsdb.UUID{GoUUID: insertOp.UUIDName}
		o.Ctx.Logger.Infof("Registering new OVS NetFlow probe \"%s\"", probe.ID)

		operations = append(operations, *insertOp)
	}

	bridgeRow := make(map[string]interface{})
	bridgeRow["netflow"] = uuid

	condition := libovsdb.NewCondition("_uuid", "==", libovsdb.UUID{GoUUID: probe.ID})
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
func (o *ProbesHandler) UnregisterNetFlowProbeFromBridge(bridgeUUID string) error {
	o.allocator.Release(bridgeUUID)

	o.probesLock.RLock()
	probe, ok := o.probes[bridgeUUID]
	if !ok {
		o.probesLock.RUnlock()
		return fmt.Errorf("probe didn't exist on bridgeUUID %s", bridgeUUID)
	}

	if probe.flowTable != nil {
		o.Ctx.FTA.Release(probe.flowTable)
	}
	o.probesLock.RUnlock()

	probeUUID, err := o.ovsClient.RetrieveSkydiveProbeRowUUID("NetFlow", ovsdb.ProbeID(probe.ID))
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

func (o *ProbesHandler) registerProbeOnBridge(bridgeUUID string, tid string, capture *types.Capture) (*Probe, error) {
	probe := &Probe{
		ID:       bridgeUUID,
		EngineID: 1,
	}

	if capture.Target == "" {
		uuids := flow.UUIDs{NodeTID: tid, CaptureID: capture.UUID}
		opts := probes.TableOptsFromCapture(capture)
		probe.flowTable = o.Ctx.FTA.Alloc(uuids, opts)

		address := o.Ctx.Config.GetString("agent.flow.netflow.bind_address")
		if address == "" {
			address = "127.0.0.1"
		}

		addr := service.Address{Addr: address, Port: 0}
		agent, err := o.allocator.Alloc(bridgeUUID, probe.flowTable, &addr, uuids)
		if err != nil && err != netflow.ErrAgentAlreadyAllocated {
			return nil, err
		}

		probe.Target = agent.GetTarget()
	} else {
		probe.Target = capture.Target
	}

	if err := o.registerNetFlowProbeOnBridge(probe); err != nil {
		return nil, err
	}

	return probe, nil
}

// RegisterProbe registers a probe on a graph node
func (o *ProbesHandler) RegisterProbe(n *graph.Node, capture *types.Capture, e probes.ProbeEventHandler) (probes.Probe, error) {
	tid, _ := n.GetFieldString("TID")
	if tid == "" {
		return nil, fmt.Errorf("No TID for node %v", n)
	}

	uuid, _ := n.GetFieldString("UUID")
	if uuid == "" {
		return nil, fmt.Errorf("Node %s has no attribute 'UUID'", n.ID)
	}

	probe, err := o.registerProbeOnBridge(uuid, tid, capture)
	if err != nil {
		return nil, err
	}

	go e.OnStarted(&probes.CaptureMetadata{})

	return probe, nil
}

// UnregisterProbe at the graph node
func (o *ProbesHandler) UnregisterProbe(n *graph.Node, e probes.ProbeEventHandler, p probes.Probe) error {
	probe := p.(*Probe)

	if err := o.UnregisterNetFlowProbeFromBridge(probe.ID); err != nil {
		return err
	}

	go e.OnStopped()

	return nil
}

// Start the probe
func (o *ProbesHandler) Start() error {
	return nil
}

// Stop the probe
func (o *ProbesHandler) Stop() {
	o.allocator.ReleaseAll()
}

// CaptureTypes supported
func (o *ProbesHandler) CaptureTypes() []string {
	return []string{"ovsnetflow"}
}

// NewProbe returns a new ovsnetflow probe
func NewProbe(ctx probes.Context, bundle *probe.Bundle) (probes.FlowProbeHandler, error) {
	handler := ctx.TB.GetHandler("ovsdb")
	if handler == nil {
		return nil, errors.New("ovsnetflow probe depends on ovsdb topology probe, probe can't start properly")
	}
	p := handler.(*ovsprobe.Probe)

	allocator, err := netflow.NewAgentAllocator()
	if err != nil {
		return nil, err
	}

	return &ProbesHandler{
		Ctx:       ctx,
		probes:    make(map[string]Probe),
		ovsClient: p.OvsMon.OvsClient,
		allocator: allocator,
	}, nil
}
