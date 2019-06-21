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
	"math"

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
	graph        *graph.Graph
	fta          *flow.TableAllocator
	ovsClient    *ovsdb.OvsClient
	allocator    *sflow.AgentAllocator
	eventHandler ProbeEventHandler
}

func newInsertSFlowProbeOP(probe *OvsSFlowProbe) (*libovsdb.Operation, error) {
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

func (o *OvsSFlowProbesHandler) registerSFlowProbeOnBridge(probe *OvsSFlowProbe, bridgeUUID string) error {
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
func (o *OvsSFlowProbesHandler) UnregisterSFlowProbeFromBridge(probe *OvsSFlowProbe) error {
	bridgeUUID := probe.ID
	o.allocator.Release(bridgeUUID)

	if probe.flowTable != nil {
		o.fta.Release(probe.flowTable)
	}

	probeUUID, err := ovsRetrieveSkydiveProbeRowUUID(o.ovsClient, "sFlow", ovsProbeID(bridgeUUID))
	if err != nil {
		return err
	}

	if probeUUID == "" {
		return fmt.Errorf("No active SFlow probe found on bridge %s", bridgeUUID)
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
	return err
}

func (o *OvsSFlowProbesHandler) registerProbeOnBridge(bridgeUUID string, tid string, capture *types.Capture, n *graph.Node) (*OvsSFlowProbe, error) {
	headerSize := flow.DefaultCaptureLength
	if capture.HeaderSize != 0 {
		headerSize = uint32(capture.HeaderSize)
	}

	if capture.SamplingRate < 1 {
		capture.SamplingRate = math.MaxUint32
	}

	probe := &OvsSFlowProbe{
		ID:         bridgeUUID,
		Interface:  "lo",
		HeaderSize: headerSize,
		Sampling:   capture.SamplingRate,
		Polling:    capture.PollingInterval,
	}

	if capture.Target == "" {
		uuids := flow.UUIDs{NodeTID: tid, CaptureID: capture.UUID}
		opts := tableOptsFromCapture(capture)
		probe.flowTable = o.fta.Alloc(uuids, opts)

		address := config.GetString("agent.flow.sflow.bind_address")
		if address == "" {
			address = "127.0.0.1"
		}

		addr := common.ServiceAddress{Addr: address}
		bfpFilter := NormalizeBPFFilter(capture)

		agent, err := o.allocator.Alloc(bridgeUUID, probe.flowTable, bfpFilter, headerSize, &addr, n, o.graph)
		if err != nil && err != sflow.ErrAgentAlreadyAllocated {
			return nil, err
		}

		probe.Target = agent.GetTarget()
	} else {
		probe.Target = capture.Target
	}

	if err := o.registerSFlowProbeOnBridge(probe, bridgeUUID); err != nil {
		return nil, err
	}

	return probe, nil
}

// RegisterProbe registers a probe on a graph node
func (o *OvsSFlowProbesHandler) RegisterProbe(n *graph.Node, capture *types.Capture, e ProbeEventHandler) (Probe, error) {
	tid, _ := n.GetFieldString("TID")
	if tid == "" {
		return nil, fmt.Errorf("No TID for node %v", n)
	}

	uuid, _ := n.GetFieldString("UUID")
	if uuid == "" {
		return nil, fmt.Errorf("Node %s has no attribute 'UUID'", n.ID)
	}

	probe, err := o.registerProbeOnBridge(uuid, tid, capture, n)
	if err != nil {
		return nil, err
	}

	go e.OnStarted(&CaptureMetadata{})

	return probe, nil
}

// UnregisterProbe at the graph node
func (o *OvsSFlowProbesHandler) UnregisterProbe(n *graph.Node, e ProbeEventHandler, fp Probe) error {
	if err := o.UnregisterSFlowProbeFromBridge(fp.(*OvsSFlowProbe)); err != nil {
		return err
	}

	go e.OnStopped()

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
func NewOvsSFlowProbesHandler(g *graph.Graph, fta *flow.TableAllocator, tb *probe.Bundle) (*OvsSFlowProbesHandler, error) {
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
		graph:     g,
		fta:       fta,
		ovsClient: p.OvsMon.OvsClient,
		allocator: allocator,
	}, nil
}
