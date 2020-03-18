/*
 * Copyright (C) 2018 Red Hat, Inc.
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

package usertopology

import (
	"errors"
	"fmt"
	"strings"

	api "github.com/skydive-project/skydive/api/server"
	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/graffiti/api/rest"
	etcd "github.com/skydive-project/skydive/graffiti/etcd/client"
	"github.com/skydive-project/skydive/graffiti/graph"
	ge "github.com/skydive-project/skydive/gremlin/traversal"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology"
)

// TopologyManager describes topology manager
type TopologyManager struct {
	common.MasterElection
	graph.DefaultGraphListener
	watcher1    rest.StoppableWatcher
	watcher2    rest.StoppableWatcher
	nodeHandler *api.NodeRuleAPI
	edgeHandler *api.EdgeRuleAPI
	graph       *graph.Graph
}

// DefToMetadata converts a string in k1=v1,k2=v2,... format to a metadata object
func DefToMetadata(def string, metadata graph.Metadata) (graph.Metadata, error) {
	if def == "" {
		return metadata, nil
	}

	for _, pair := range strings.Split(def, ",") {
		pair = strings.TrimSpace(pair)

		kv := strings.Split(pair, "=")
		if len(kv)%2 != 0 {
			return nil, fmt.Errorf("attributes must be defined by pair k=v: %v", def)
		}
		key := strings.Trim(kv[0], `"`)
		value := strings.Trim(kv[1], `"`)

		metadata.SetField(key, value)
	}

	return metadata, nil
}

// OnStartAsMaster event
func (tm *TopologyManager) OnStartAsMaster() {
}

// OnStartAsSlave event
func (tm *TopologyManager) OnStartAsSlave() {
}

// OnSwitchToMaster event
func (tm *TopologyManager) OnSwitchToMaster() {
	tm.syncTopology()
}

// OnSwitchToSlave event
func (tm *TopologyManager) OnSwitchToSlave() {
}

func (tm *TopologyManager) syncTopology() {
	nodes := tm.nodeHandler.Index()
	edges := tm.edgeHandler.Index()

	for _, node := range nodes {
		n := node.(*types.NodeRule)
		tm.handleCreateNode(n)
	}

	for _, edge := range edges {
		e := edge.(*types.EdgeRule)
		tm.createEdge(e)
	}
}

func (tm *TopologyManager) createEdge(edge *types.EdgeRule) error {
	src := tm.getNodes(edge.Src)
	dst := tm.getNodes(edge.Dst)
	if len(src) < 1 || len(dst) < 1 {
		logging.GetLogger().Errorf("Source or Destination node not found")
		return errors.New("Source or Destination node not found")
	}

	switch edge.Metadata["RelationType"] {
	case "layer2":
		if !topology.HaveLayer2Link(tm.graph, src[0], dst[0]) {
			topology.AddLayer2Link(tm.graph, src[0], dst[0], edge.Metadata)
		}
	case "ownership":
		if !topology.HaveOwnershipLink(tm.graph, src[0], dst[0]) {
			topology.AddOwnershipLink(tm.graph, src[0], dst[0], nil)
		}
	default:
		// check nodes are already linked
		if tm.graph.AreLinked(src[0], dst[0], graph.Metadata{"RelationType": edge.Metadata["RelationType"]}) {
			return errors.New("Nodes are already linked")
		}
		id := graph.GenID(string(src[0].ID) + string(dst[0].ID) + edge.Metadata["RelationType"].(string))
		_, err := tm.graph.NewEdge(id, src[0], dst[0], edge.Metadata)
		if err != nil {
			return err
		}
	}
	return nil
}

func (tm *TopologyManager) nodeID(node *types.NodeRule) graph.Identifier {
	return graph.GenID(node.Metadata["Type"].(string), node.Metadata["Name"].(string))
}

func (tm *TopologyManager) createNode(node *types.NodeRule) error {
	id := tm.nodeID(node)
	node.Metadata.SetField("TID", string(id))

	//check node already exist
	if n := tm.graph.GetNode(id); n != nil {
		return nil
	}

	if node.Metadata["Type"] == "fabric" {
		node.Metadata["Probe"] = "fabric"
	}

	tm.graph.NewNode(id, node.Metadata, "")
	return nil
}

func (tm *TopologyManager) updateMetadata(query string, mdata graph.Metadata) error {
	nodes := tm.getNodes(query)
	for _, n := range nodes {
		mt := tm.graph.StartMetadataTransaction(n)
		for k, v := range mdata {
			mt.AddMetadata(k, v)
		}
		mt.Commit()
	}
	return nil
}

func (tm *TopologyManager) deleteMetadata(query string, mdata graph.Metadata) error {
	nodes := tm.getNodes(query)
	for _, n := range nodes {
		mt := tm.graph.StartMetadataTransaction(n)
		for k := range mdata {
			mt.DelMetadata(k)
		}
		mt.Commit()
	}

	tm.syncTopology()
	return nil
}

func (tm *TopologyManager) handleCreateNode(node *types.NodeRule) error {
	switch strings.ToLower(node.Action) {
	case "create":
		return tm.createNode(node)
	case "update":
		return tm.updateMetadata(node.Query, node.Metadata)
	default:
		logging.GetLogger().Errorf("Query format is wrong. supported prefixes: create and update")
		return errors.New("Query format is wrong")
	}
}

/*This needs to be replaced by gremlin + JS query*/
func (tm *TopologyManager) getNodes(gremlinQuery string) []*graph.Node {
	res, err := ge.TopologyGremlinQuery(tm.graph, gremlinQuery)
	if err != nil {
		return nil
	}

	var nodes []*graph.Node
	for _, value := range res.Values() {
		switch value.(type) {
		case *graph.Node:
			nodes = append(nodes, value.(*graph.Node))
		case []*graph.Node:
			nodes = append(nodes, value.([]*graph.Node)...)
		}
	}
	return nodes
}

func (tm *TopologyManager) handleNodeRuleRequest(action string, resource rest.Resource) error {
	node := resource.(*types.NodeRule)
	switch action {
	case "create", "set":
		return tm.handleCreateNode(node)
	case "delete":
		switch strings.ToLower(node.Action) {
		case "create":
			id := tm.nodeID(node)
			if n := tm.graph.GetNode(id); n != nil {
				tm.graph.DelNode(n)
			}
		case "update":
			tm.deleteMetadata(node.Query, node.Metadata)
		}
	}
	return nil
}

func (tm *TopologyManager) handleEdgeRuleRequest(action string, resource rest.Resource) error {
	edge := resource.(*types.EdgeRule)
	switch action {
	case "create", "set":
		return tm.createEdge(edge)
	case "delete":
		src := tm.getNodes(edge.Src)
		dst := tm.getNodes(edge.Dst)
		if len(src) < 1 || len(dst) < 1 {
			logging.GetLogger().Errorf("Source or Destination node not found")
			return nil
		}

		if link := tm.graph.GetFirstLink(src[0], dst[0], edge.Metadata); link != nil {
			if err := tm.graph.DelEdge(link); err != nil {
				logging.GetLogger().Errorf("Delete Edge failed, error: %v", err)
				return nil
			}
		} else {
			return nil
		}
	}
	return nil
}

func (tm *TopologyManager) onAPIWatcherEvent(action string, id string, resource rest.Resource) {
	switch resource.(type) {
	case *types.NodeRule:
		tm.graph.Lock()
		tm.handleNodeRuleRequest(action, resource)
		tm.graph.Unlock()
	case *types.EdgeRule:
		logging.GetLogger().Debugf("onAPIWatcherEvent edgerule")
		tm.graph.Lock()
		tm.handleEdgeRuleRequest(action, resource)
		tm.graph.Unlock()
	}
}

// OnNodeAdded event
func (tm *TopologyManager) OnNodeAdded(n *graph.Node) {
	tm.syncTopology()
}

// OnNodeUpdated event
func (tm *TopologyManager) OnNodeUpdated(n *graph.Node) {
	tm.syncTopology()
}

// Start start the topology manager
func (tm *TopologyManager) Start() {
	tm.MasterElection.StartAndWait()

	tm.watcher1 = tm.nodeHandler.AsyncWatch(tm.onAPIWatcherEvent)
	tm.watcher2 = tm.edgeHandler.AsyncWatch(tm.onAPIWatcherEvent)

	tm.graph.AddEventListener(tm)
}

// Stop stop the topology manager
func (tm *TopologyManager) Stop() {
	tm.watcher1.Stop()
	tm.watcher2.Stop()

	tm.MasterElection.Stop()

	tm.graph.RemoveEventListener(tm)
}

// NewTopologyManager returns new topology manager
func NewTopologyManager(etcdClient *etcd.Client, nodeHandler *api.NodeRuleAPI, edgeHandler *api.EdgeRuleAPI, g *graph.Graph) *TopologyManager {
	tm := &TopologyManager{
		nodeHandler: nodeHandler,
		edgeHandler: edgeHandler,
		graph:       g,
	}

	tm.MasterElection = etcdClient.NewElection("topology-manager")
	tm.MasterElection.AddEventListener(tm)

	tm.graph.Lock()
	tm.syncTopology()
	tm.graph.Unlock()
	return tm
}
