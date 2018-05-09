/*
 * Copyright (C) 2018 Red Hat, Inc.
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

package fabric

import (
	"fmt"

	"github.com/nu7hatch/gouuid"

	apiServer "github.com/skydive-project/skydive/api/server"
	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/etcd"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/graph"
)

type FabricProbeManager struct {
	*etcd.MasterElector
	watcher apiServer.StoppableWatcher
	handler *apiServer.FabricProbeAPI
	fprobe  *FabricProbe
}

// OnStartAsMaster event
func (fm *FabricProbeManager) OnStartAsMaster() {
}

// OnStartAsSlave event
func (fm *FabricProbeManager) OnStartAsSlave() {
}

// OnSwitchToMaster event
func (fm *FabricProbeManager) OnSwitchToMaster() {
}

// OnSwitchToSlave event
func (fm *FabricProbeManager) OnSwitchToSlave() {
}

func (fm *FabricProbeManager) getOrCreateFabricNode(node *types.FabricProbe) (*graph.Node, error) {
	metadata, err := defToMetadata(node.Metadata, graph.Metadata{"Name": node.Name})
	if err != nil {
		return nil, err
	}
	if _, ok := metadata["Type"]; !ok {
		metadata["Type"] = "device"
	}

	u, _ := uuid.NewV5(uuid.NamespaceOID, []byte("fabric"+node.Name))
	id := graph.Identifier(u.String())

	if n := fm.fprobe.Graph.GetNode(id); n != nil {
		return n, nil
	}

	metadata["Probe"] = "fabric"
	fmt.Println("new node")
	return fm.fprobe.Graph.NewNode(id, metadata, ""), nil
}

func (fm *FabricProbeManager) createFabricNode(node *types.FabricProbe) {
	metadata := graph.Metadata{"Name": node.Name}
	switch node.SubType {
	case "host":
		metadata["FabricType"] = "host"
	case "port":
		metadata["FabricType"] = "port"
	case "default":
		logging.GetLogger().Errorf("Not a valid Node Type")
		return
	}

	if node.Metadata != "" {
		var err error
		metadata, err = defToMetadata(node.Metadata, metadata)
		if err != nil {
			logging.GetLogger().Errorf("Not able to parse Metadata: %s", err.Error())
		}
	}

	n, err := fm.fprobe.getOrCreateFabricNode(node.Name, metadata)
	if err != nil {
		logging.GetLogger().Error("Not able to create fabric node: %s", err.Error())
	}
	if node.SubType == "port" && node.ParentID != "" {
		if parentNode := fm.fprobe.Graph.GetNode(graph.Identifier(node.ParentID)); parentNode != nil {
			topology.AddOwnershipLink(fm.fprobe.Graph, parentNode, n, nil)
			topology.AddLayer2Link(fm.fprobe.Graph, parentNode, n, graph.Metadata{"Type": "fabric"})
			return
		}
		logging.GetLogger().Errorf("Parent Node not found with ID: %s", node.ParentID)
	}
}

func (fm *FabricProbeManager) createFabricLink(link *types.FabricProbe) {
	node1 := fm.fprobe.Graph.GetNode(graph.Identifier(link.SrcNode))
	node2 := fm.fprobe.Graph.GetNode(graph.Identifier(link.DstNode))
	if node1 == nil {
		logging.GetLogger().Errorf("Not able to find Node with ID: %s", link.SrcNode)
		return
	}
	if node2 == nil {
		logging.GetLogger().Errorf("Not able to find Node with ID: %s", link.DstNode)
		return
	}

	metadata, err := defToMetadata(link.Metadata, graph.Metadata{"Type": "fabric"})
	if err != nil {
		logging.GetLogger().Errorf("Not able to parse Metadata: %s", err.Error())
	}

	switch link.SubType {
	case "layer2":
		topology.AddLayer2Link(fm.fprobe.Graph, node1, node2, metadata)
	case "ownership":
		topology.AddOwnershipLink(fm.fprobe.Graph, node1, node2, metadata)
	case "both":
		topology.AddOwnershipLink(fm.fprobe.Graph, node1, node2, nil)
		topology.AddLayer2Link(fm.fprobe.Graph, node1, node2, metadata)
	default:
		logging.GetLogger().Errorf("Not a valid Link type")
	}
}

func (fm *FabricProbeManager) onAPIWatcherEvent(action string, id string, resource types.Resource) {
	logging.GetLogger().Debugf("New watcher event %s for %s", action, id)
	fp := resource.(*types.FabricProbe)
	switch action {
	case "create", "set":
		switch fp.Type {
		case "node":
			fm.createFabricNode(fp)
		case "link":
			fm.createFabricLink(fp)
		}
	case "delete":
	}
}

// Start the fabric probe manager
func (fm *FabricProbeManager) Start() {
	fm.MasterElector.StartAndWait()
	fm.watcher = fm.handler.AsyncWatch(fm.onAPIWatcherEvent)
}

// Stop the fabric probe manager
func (fm *FabricProbeManager) Stop() {
	fm.watcher.Stop()
	fm.MasterElector.Stop()
}

func NewFabricProbeManager(etcdClient *etcd.Client, handler *apiServer.FabricProbeAPI, fprobe *FabricProbe) *FabricProbeManager {
	elector := etcd.NewMasterElectorFromConfig(common.AnalyzerService, "fp-manager", etcdClient)

	fpm := &FabricProbeManager{
		MasterElector: elector,
		handler:       handler,
		fprobe:        fprobe,
	}

	elector.AddEventListener(fpm)

	return fpm
}
