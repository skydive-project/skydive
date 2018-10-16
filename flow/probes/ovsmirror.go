// +build linux

/*
 * Copyright (C) 2017 Red Hat, Inc.
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
	"errors"
	"fmt"

	"github.com/socketplane/libovsdb"
	"github.com/vishvananda/netlink"

	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/ovs"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/graph"
	op "github.com/skydive-project/skydive/topology/probes/ovsdb"
)

// ovsMirrorProbe describes a mirror probe from OVS switch
type ovsMirrorProbe struct {
	id         string
	graph      *graph.Graph
	node       *graph.Node
	mirrorNode *graph.Node
	capture    *types.Capture
	subProbe   FlowProbe
}

// OvsMirrorProbesHandler describes a flow probe in running in the graph
type OvsMirrorProbesHandler struct {
	ovsdb.DefaultOvsMonitorHandler
	probes      map[string]*ovsMirrorProbe
	probeBundle *probe.Bundle
	probesLock  common.RWMutex
	Graph       *graph.Graph
	ovsClient   *ovsdb.OvsClient
	intfIndexer *graph.MetadataIndexer
	portIndexer *graph.MetadataIndexer
	intfHandler *ovsMirrorInterfaceHandler
	portHandler *ovsMirrorPortHandler
}

type ovsMirrorInterfaceHandler struct {
	graph.DefaultGraphListener
	oph *OvsMirrorProbesHandler
}

type ovsMirrorPortHandler struct {
	graph.DefaultGraphListener
	oph *OvsMirrorProbesHandler
}

func newInsertInternalOP(probe *ovsMirrorProbe) (*libovsdb.Operation, error) {
	intfRow := make(map[string]interface{})
	intfRow["name"] = probe.mirrorName()
	intfRow["type"] = "internal"

	extIds := make(map[string]string)
	extIds["skydive-probe-id"] = probe.id
	ovsMap, err := libovsdb.NewOvsMap(extIds)
	if err != nil {
		return nil, err
	}
	intfRow["external_ids"] = ovsMap

	insertOp := libovsdb.Operation{Op: "insert", Table: "Interface", Row: intfRow, UUIDName: ovsNamedUUID("intf_" + probe.id)}

	return &insertOp, nil
}

func newDeleteInternalOP(probe *ovsMirrorProbe) *libovsdb.Operation {
	condition := libovsdb.NewCondition("name", "==", probe.mirrorName())
	return &libovsdb.Operation{
		Op:    "delete",
		Table: "Interface",
		Where: []interface{}{condition},
	}
}

func newInsertPortOP(probe *ovsMirrorProbe, intfInsertOp *libovsdb.Operation) (*libovsdb.Operation, error) {
	portRow := make(map[string]interface{})
	portRow["name"] = probe.mirrorName()
	portRow["interfaces"] = libovsdb.UUID{GoUUID: intfInsertOp.UUIDName}

	extIds := make(map[string]string)
	extIds["skydive-probe-id"] = probe.id
	ovsMap, err := libovsdb.NewOvsMap(extIds)
	if err != nil {
		return nil, err
	}
	portRow["external_ids"] = ovsMap

	insertOp := libovsdb.Operation{Op: "insert", Table: "Port", Row: portRow, UUIDName: ovsNamedUUID("port_" + probe.id)}

	return &insertOp, nil
}

func newDeletePortOP(probe *ovsMirrorProbe) *libovsdb.Operation {
	condition := libovsdb.NewCondition("name", "==", probe.mirrorName())
	return &libovsdb.Operation{
		Op:    "delete",
		Table: "Port",
		Where: []interface{}{condition},
	}
}

func newInsertMirrorOP(probe *ovsMirrorProbe, srcUUID string, dstInsertOp *libovsdb.Operation) (*libovsdb.Operation, error) {
	mirrorRow := make(map[string]interface{})
	mirrorRow["name"] = probe.mirrorName()
	srcSet, _ := libovsdb.NewOvsSet([]libovsdb.UUID{{GoUUID: srcUUID}})
	mirrorRow["select_src_port"] = srcSet
	dstSet, _ := libovsdb.NewOvsSet([]libovsdb.UUID{{GoUUID: srcUUID}})
	mirrorRow["select_dst_port"] = dstSet
	mirrorRow["output_port"] = libovsdb.UUID{GoUUID: dstInsertOp.UUIDName}

	extIds := make(map[string]string)
	extIds["skydive-probe-id"] = probe.id
	ovsMap, err := libovsdb.NewOvsMap(extIds)
	if err != nil {
		return nil, err
	}
	mirrorRow["external_ids"] = ovsMap

	insertOp := libovsdb.Operation{Op: "insert", Table: "Mirror", Row: mirrorRow, UUIDName: ovsNamedUUID("mirror_" + probe.id)}

	return &insertOp, nil
}

func newDeleteMirrorOP(probe *ovsMirrorProbe) *libovsdb.Operation {
	condition := libovsdb.NewCondition("name", "==", probe.mirrorName())
	return &libovsdb.Operation{
		Op:    "delete",
		Table: "Mirror",
		Where: []interface{}{condition},
	}
}

func (o *ovsMirrorProbe) mirrorName() string {
	return fmt.Sprintf("mir%s", o.id)[0:8]
}

func (o *OvsMirrorProbesHandler) retrieveBridgeUUID(portUUID string) (string, error) {
	condition := libovsdb.NewCondition("ports", "includes", libovsdb.UUID{GoUUID: portUUID})
	selectOp := libovsdb.Operation{
		Op:    "select",
		Table: "Bridge",
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
			return u.(string), nil
		}
	}

	return "", nil
}

func (o *OvsMirrorProbesHandler) registerProbeOnPort(probe *ovsMirrorProbe, portUUID string) error {
	o.probesLock.Lock()
	o.probes[portUUID] = probe
	o.probesLock.Unlock()

	bridgeUUID, err := o.retrieveBridgeUUID(portUUID)
	if err != nil {
		return err
	}

	operations := []libovsdb.Operation{}

	intfInsertOp, err := newInsertInternalOP(probe)
	if err != nil {
		return err
	}
	operations = append(operations, *intfInsertOp)

	portInsertOp, err := newInsertPortOP(probe, intfInsertOp)
	if err != nil {
		return err
	}
	operations = append(operations, *portInsertOp)

	mutateUUID := []libovsdb.UUID{{GoUUID: portInsertOp.UUIDName}}
	mutateSet, _ := libovsdb.NewOvsSet(mutateUUID)
	mutation := libovsdb.NewMutation("ports", "insert", mutateSet)

	condition := libovsdb.NewCondition("_uuid", "==", libovsdb.UUID{GoUUID: bridgeUUID})
	mutateOp := libovsdb.Operation{
		Op:        "mutate",
		Table:     "Bridge",
		Mutations: []interface{}{mutation},
		Where:     []interface{}{condition},
	}
	operations = append(operations, mutateOp)

	mirrorInsertOp, err := newInsertMirrorOP(probe, portUUID, portInsertOp)
	if err != nil {
		return err
	}
	operations = append(operations, *mirrorInsertOp)

	mutateMirrorsUUID := []libovsdb.UUID{{GoUUID: mirrorInsertOp.UUIDName}}
	mutateMirrosSet, _ := libovsdb.NewOvsSet(mutateMirrorsUUID)
	mutationMirrors := libovsdb.NewMutation("mirrors", "insert", mutateMirrosSet)

	condition = libovsdb.NewCondition("_uuid", "==", libovsdb.UUID{GoUUID: bridgeUUID})
	updateOp := libovsdb.Operation{
		Op:        "mutate",
		Table:     "Bridge",
		Mutations: []interface{}{mutationMirrors},
		Where:     []interface{}{condition},
	}
	operations = append(operations, updateOp)

	if _, err = o.ovsClient.Exec(operations...); err != nil {
		return err
	}

	return nil
}

func (o *OvsMirrorProbesHandler) unregisterProbeFromPort(portUUID string) error {
	o.probesLock.Lock()
	probe, ok := o.probes[portUUID]
	if !ok {
		o.probesLock.Unlock()
		return fmt.Errorf("probe didn't exist on probeUUID %s", portUUID)
	}
	delete(o.probes, portUUID)
	o.probesLock.Unlock()

	bridgeUUID, err := o.retrieveBridgeUUID(portUUID)
	if err != nil {
		return err
	}

	mirrorPortUUID, err := ovsRetrieveSkydiveProbeRowUUID(o.ovsClient, "Port", probe.id)
	if err != nil {
		return err
	}

	mirrorUUID, err := ovsRetrieveSkydiveProbeRowUUID(o.ovsClient, "Mirror", probe.id)
	if err != nil {
		return err
	}

	operations := []libovsdb.Operation{
		*newDeleteMirrorOP(probe),
		*newDeleteInternalOP(probe),
		*newDeletePortOP(probe),
	}

	mutatePortsUUID := []libovsdb.UUID{{GoUUID: mirrorPortUUID}}
	mutatePortsSet, _ := libovsdb.NewOvsSet(mutatePortsUUID)
	mutationPorts := libovsdb.NewMutation("ports", "delete", mutatePortsSet)

	mutateMirrorsUUID := []libovsdb.UUID{{GoUUID: mirrorUUID}}
	mutateMirrosSet, _ := libovsdb.NewOvsSet(mutateMirrorsUUID)
	mutationMirrors := libovsdb.NewMutation("mirrors", "delete", mutateMirrosSet)

	condition := libovsdb.NewCondition("_uuid", "==", libovsdb.UUID{GoUUID: bridgeUUID})
	mutateOp := libovsdb.Operation{
		Op:        "mutate",
		Table:     "Bridge",
		Mutations: []interface{}{mutationPorts, mutationMirrors},
		Where:     []interface{}{condition},
	}
	operations = append(operations, mutateOp)

	if _, err = o.ovsClient.Exec(operations...); err != nil {
		return err
	}

	return nil
}

// RegisterProbeOnPort registers a new probe on the OVS bridge
func (o *OvsMirrorProbesHandler) RegisterProbeOnPort(n *graph.Node, portUUID string, capture *types.Capture) error {
	probe := &ovsMirrorProbe{
		id:      portUUID,
		capture: capture,
		graph:   o.Graph,
		node:    n,
	}

	if err := o.registerProbeOnPort(probe, portUUID); err != nil {
		return err
	}

	o.probesLock.Lock()
	o.probes[portUUID] = probe
	o.probesLock.Unlock()

	return nil
}

func isOvsPort(n *graph.Node) bool {
	if uuid, _ := n.GetFieldString("UUID"); uuid == "" {
		return false
	}
	if tp, _ := n.GetFieldString("Type"); tp == "ovsport" {
		return true
	}

	return false
}

func (o *OvsMirrorProbesHandler) registerProbe(n *graph.Node, capture *types.Capture, e FlowProbeEventHandler) error {
	if isOvsPort(n) {
		if uuid, _ := n.GetFieldString("UUID"); uuid != "" {
			if err := o.RegisterProbeOnPort(n, uuid, capture); err != nil {
				return err
			}

			go e.OnStarted()
		}
	}
	return nil
}

// RegisterProbe registers a probe on a graph node
func (o *OvsMirrorProbesHandler) RegisterProbe(n *graph.Node, capture *types.Capture, e FlowProbeEventHandler) error {
	err := o.registerProbe(n, capture, e)
	if err != nil {
		go e.OnError(err)
	}
	return err
}

// UnregisterProbe at the graph node
func (o *OvsMirrorProbesHandler) UnregisterProbe(n *graph.Node, e FlowProbeEventHandler) error {
	if isOvsPort(n) {
		if uuid, _ := n.GetFieldString("UUID"); uuid != "" {
			if err := o.unregisterProbeFromPort(uuid); err != nil {
				return err
			}
			go e.OnStopped()
		}
	}
	return nil
}

func (o *OvsMirrorProbesHandler) cleanupOvsMirrors() {
	var operations []libovsdb.Operation

	uuids, err := ovsRetrieveSkydiveProbeRowUUIDs(o.ovsClient, "Mirror")
	if err != nil {
		logging.GetLogger().Errorf("OvsMirror cleanup error: %s", err)
		return
	}
	for _, uuid := range uuids {
		condition := libovsdb.NewCondition("_uuid", "==", libovsdb.UUID{GoUUID: uuid})
		operations = append(operations, libovsdb.Operation{Op: "delete", Table: "Mirror", Where: []interface{}{condition}})

		mutateUUID := []libovsdb.UUID{{GoUUID: uuid}}
		mutateSet, _ := libovsdb.NewOvsSet(mutateUUID)
		mutation := libovsdb.NewMutation("mirrors", "delete", mutateSet)

		where := libovsdb.NewCondition("_uuid", "!=", libovsdb.UUID{GoUUID: "abc"})
		mutateOp := libovsdb.Operation{Op: "mutate", Table: "Bridge", Mutations: []interface{}{mutation}, Where: []interface{}{where}}
		operations = append(operations, mutateOp)
	}

	uuids, err = ovsRetrieveSkydiveProbeRowUUIDs(o.ovsClient, "Mirror")
	if err != nil {
		logging.GetLogger().Errorf("OvsMirror cleanup error: %s", err)
		return
	}
	for _, uuid := range uuids {
		condition := libovsdb.NewCondition("_uuid", "==", libovsdb.UUID{GoUUID: uuid})
		operations = append(operations, libovsdb.Operation{Op: "delete", Table: "Mirror", Where: []interface{}{condition}})
	}

	uuids, err = ovsRetrieveSkydiveProbeRowUUIDs(o.ovsClient, "Port")
	if err != nil {
		logging.GetLogger().Errorf("OvsMirror cleanup error: %s", err)
		return
	}
	for _, uuid := range uuids {
		condition := libovsdb.NewCondition("_uuid", "==", libovsdb.UUID{GoUUID: uuid})
		operations = append(operations, libovsdb.Operation{Op: "delete", Table: "Port", Where: []interface{}{condition}})

		mutateUUID := []libovsdb.UUID{{GoUUID: uuid}}
		mutateSet, _ := libovsdb.NewOvsSet(mutateUUID)
		mutation := libovsdb.NewMutation("ports", "delete", mutateSet)

		where := libovsdb.NewCondition("_uuid", "!=", libovsdb.UUID{GoUUID: "abc"})
		mutateOp := libovsdb.Operation{Op: "mutate", Table: "Bridge", Mutations: []interface{}{mutation}, Where: []interface{}{where}}
		operations = append(operations, mutateOp)
	}

	if _, err = o.ovsClient.Exec(operations...); err != nil {
		logging.GetLogger().Errorf("OvsMirror cleanup error: %s", err)
	}
	logging.GetLogger().Info("OvsMirror cleanup previous mirrors")
}

// OnStarted FlowProbeEventHandler implementation
func (o *ovsMirrorProbe) OnStarted() {
	o.graph.Lock()
	tr := o.graph.StartMetadataTransaction(o.mirrorNode)
	tr.AddMetadata("Capture.State", "active")
	tr.AddMetadata("Capture.MirrorOf", o.node.ID)
	tr.Commit()
	o.graph.Unlock()
}

// OnStopped FlowProbeEventHandler implementation
func (o *ovsMirrorProbe) OnStopped() {
	o.graph.Lock()
	o.graph.DelMetadata(o.mirrorNode, "Capture")
	o.graph.Unlock()
}

// OnError FlowProbeEventHandler implementation
func (o *ovsMirrorProbe) OnError(err error) {
	o.graph.Lock()
	tr := o.graph.StartMetadataTransaction(o.mirrorNode)
	tr.AddMetadata("Capture.State", "error")
	tr.AddMetadata("Capture.Error", err.Error())
	tr.Commit()
	o.graph.Unlock()
}

func (o *ovsMirrorInterfaceHandler) onNodeEvent(n *graph.Node) {
	probeID, _ := n.GetFieldString("ExtID.skydive-probe-id")
	if probeID == "" {
		return
	}

	o.oph.probesLock.RLock()
	ovsProbe, ok := o.oph.probes[probeID]
	o.oph.probesLock.RUnlock()
	if !ok {
		return
	}

	// already started
	if ovsProbe.subProbe != nil {
		return
	}

	if !topology.IsInterfaceUp(n) {
		name, _ := n.GetFieldString("Name")
		intf, err := netlink.LinkByName(name)
		if err != nil {
			logging.GetLogger().Errorf("Error reading interface name %s: %s", name, err)
			return
		}
		netlink.LinkSetUp(intf)

		// return, wait to get the UP event
		return
	}

	subProbeTypes, ok := common.CaptureTypes["internal"]
	if !ok {
		logging.GetLogger().Errorf("Unable to find probe for this node type: internal")
		return
	}

	subProbe := o.oph.probeBundle.GetProbe(subProbeTypes.Default)
	if subProbe == nil {
		logging.GetLogger().Errorf("Unable to find probe for this capture type: %s", subProbeTypes.Default)
		return
	}

	fprobe := subProbe.(FlowProbe)
	if err := fprobe.RegisterProbe(n, ovsProbe.capture, ovsProbe); err != nil {
		logging.GetLogger().Debugf("Failed to register flow probe: %s", err)
		return
	}

	ovsProbe.mirrorNode = n
	ovsProbe.subProbe = fprobe
}

func (o *ovsMirrorInterfaceHandler) OnNodeAdded(n *graph.Node) {
	o.onNodeEvent(n)
}

func (o *ovsMirrorInterfaceHandler) OnNodeUpdated(n *graph.Node) {
	o.onNodeEvent(n)
}

func (o *ovsMirrorPortHandler) OnNodeAdded(n *graph.Node) {
	probeID, _ := n.GetFieldString("ExtID.skydive-probe-id")
	if probeID == "" {
		return
	}

	o.oph.probesLock.RLock()
	ovsProbe, ok := o.oph.probes[probeID]
	o.oph.probesLock.RUnlock()
	if !ok {
		return
	}

	o.oph.Graph.Link(n, ovsProbe.node, graph.Metadata{"RelationType": "mirroring"})
}

func (o *ovsMirrorInterfaceHandler) OnNodeDeleted(n *graph.Node) {
	probeID, _ := n.GetFieldString("ExtID.skydive-probe-id")
	if probeID == "" {
		return
	}

	o.oph.probesLock.RLock()
	ovsProbe, ok := o.oph.probes[probeID]
	o.oph.probesLock.RUnlock()
	if !ok {
		return
	}

	if ovsProbe.subProbe != nil {
		ovsProbe.subProbe.UnregisterProbe(n, ovsProbe)
	}
}

// OnConnected ovsdb event
func (o *OvsMirrorProbesHandler) OnConnected(monitor *ovsdb.OvsMonitor) {
	o.cleanupOvsMirrors()
}

// Start the probe
func (o *OvsMirrorProbesHandler) Start() {
	o.intfIndexer.AddEventListener(o.intfHandler)
	o.portIndexer.AddEventListener(o.portHandler)
	o.intfIndexer.Start()
	o.portIndexer.Start()
}

// Stop the probe
func (o *OvsMirrorProbesHandler) Stop() {
	var uuids []string

	o.probesLock.RLock()
	for uuid := range o.probes {
		uuids = append(uuids, uuid)
	}
	o.probesLock.RUnlock()

	for _, uuid := range uuids {
		o.unregisterProbeFromPort(uuid)
	}

	o.intfIndexer.RemoveEventListener(o.intfHandler)
	o.portIndexer.RemoveEventListener(o.portHandler)
	o.intfIndexer.Stop()
	o.portIndexer.Stop()

	o.cleanupOvsMirrors()
}

// NewOvsMirrorProbesHandler creates a new OVS Mirror probes
func NewOvsMirrorProbesHandler(g *graph.Graph, tb, fb *probe.Bundle) (*OvsMirrorProbesHandler, error) {
	probe := tb.GetProbe("ovsdb")
	if probe == nil {
		return nil, errors.New("Agent.ovssflow probe depends on agent.ovsdb topology probe: agent.ovssflow probe can't start properly")
	}
	p := probe.(*op.Probe)

	o := &OvsMirrorProbesHandler{
		probes:      make(map[string]*ovsMirrorProbe),
		Graph:       g,
		ovsClient:   p.OvsMon.OvsClient,
		probeBundle: fb,
		intfIndexer: graph.NewMetadataIndexer(g, g, graph.Metadata{"Type": "internal"}, "ExtID.skydive-probe-id"),
		portIndexer: graph.NewMetadataIndexer(g, g, graph.Metadata{"Type": "ovsport"}, "ExtID.skydive-probe-id"),
	}

	// monitor connection/disconnection
	p.OvsMon.AddMonitorHandler(o)

	o.intfHandler = &ovsMirrorInterfaceHandler{oph: o}
	o.portHandler = &ovsMirrorPortHandler{oph: o}

	return o, nil
}
