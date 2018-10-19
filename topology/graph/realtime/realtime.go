/**
this is a realtime change handler for skydive topology graph
**/

package realtime

import (
	"fmt"
	"github.com/skydive-project/skydive/topology/graph"
	"sync"
)

type GraphHandler struct {
	sync.RWMutex
	Graph                  *graph.Graph
	NodeMap                map[string]NodeInfo
	PropertyToIdentifyNode string
}

type NodeInfo struct {
	nodes map[string]NodesConnectedToType
}

func (handler *GraphHandler) GetNodeByIdentifier(NodeIdentifier string) *graph.Node {
	ExistingNode := handler.Graph.LookupFirstNode(graph.Metadata{
		handler.PropertyToIdentifyNode: NodeIdentifier,
	})
	return ExistingNode
}

func (handler *GraphHandler) StopTrackingChangesForNode(NodeIdentifier string) {
	delete(handler.NodeMap, NodeIdentifier)
}

func (handler *GraphHandler) StartTrackingChangesForNode(NodeIdentifier string, nodes map[string]NodesConnectedToType) {
	nodeInfo, ok := handler.NodeMap[NodeIdentifier]
	if ok {
		for key, val := range nodes {
			nodeInfo.nodes[key] = val
		}
	} else {
		handler.NodeMap[NodeIdentifier] = NodeInfo{
			nodes: nodes,
		}
	}
}

func (handler *GraphHandler) getAllTrackingNodes() map[string]NodeInfo {
	return handler.NodeMap
}

// OnEdgeAdded event
func (handler *GraphHandler) OnEdgeAdded(e *graph.Edge) {
}

// OnEdgeDeleted event
func (handler *GraphHandler) OnEdgeDeleted(e *graph.Edge) {
}

// OnEdgeUpdated event
func (handler *GraphHandler) OnEdgeUpdated(e *graph.Edge) {
}

// OnNodeUpdated event
func (handler *GraphHandler) OnNodeUpdated(n *graph.Node) {
}

// OnNodeAdded event
func (handler *GraphHandler) OnNodeAdded(n *graph.Node) {
	handler.Lock()
	defer handler.Unlock()
	identifier, _ := n.GetFieldString(handler.PropertyToIdentifyNode)
	for NodeIdentifier, nodeInfo := range handler.getAllTrackingNodes() {
		if len(nodeInfo.nodes) == 0 {
			continue
		}
		nodeConnection, ok := nodeInfo.nodes[identifier]
		if !ok {
			continue
		}
		ExistingNode := handler.GetNodeByIdentifier(NodeIdentifier)
		nodeConnection.ConnectTwoNodes(handler.Graph, n, ExistingNode)
		nodeConnection.UpdateMetadata(handler.Graph, n)
		delete(nodeInfo.nodes, identifier)
	}
}

// OnNodeDeleted event
func (handler *GraphHandler) OnNodeDeleted(n *graph.Node) {
}

func (handler *GraphHandler) RemoveNode(NodeIdentifier string) {
	handler.Graph.Lock()
	defer handler.Graph.Unlock()
	handler.Lock()
	defer handler.Unlock()
	ExistingNode := handler.GetNodeByIdentifier(NodeIdentifier)
	handler.StopTrackingChangesForNode(NodeIdentifier)
	if ExistingNode == nil {
		return
	}
	handler.Graph.DelNode(ExistingNode)
}

func (handler *GraphHandler) AddNode(NodeIdentifier string, metadata *graph.Metadata, nodes map[string]NodesConnectedToType) {
	handler.Graph.Lock()
	defer handler.Graph.Unlock()
	Node := handler.GetNodeByIdentifier(NodeIdentifier)

	if Node == nil {
		Node = handler.Graph.NewNode(graph.GenID(), *metadata)
	}
	handler.Lock()
	defer handler.Unlock()
	if len(nodes) == 0 {
		handler.StartTrackingChangesForNode(NodeIdentifier, nodes)
		return
	}

	var n *graph.Node

	for nodeIdentifier, nodeConnectedToType := range nodes {
		n = handler.GetNodeByIdentifier(nodeIdentifier)
		if n == nil {
			continue
		}
		nodeConnectedToType.ConnectTwoNodes(handler.Graph, n, Node)
		nodeConnectedToType.UpdateMetadata(handler.Graph, n)
		delete(nodes, nodeIdentifier)
	}
	handler.StartTrackingChangesForNode(NodeIdentifier, nodes)
}

func (handler *GraphHandler) UpdateConnectionsForNode(NodeIdentifier string, nodes map[string]NodesConnectedToType) error {
	handler.Graph.Lock()
	defer handler.Graph.Unlock()
	Node := handler.GetNodeByIdentifier(NodeIdentifier)
	if Node == nil {
		return fmt.Errorf("No node with name %s in the graph", NodeIdentifier)
	}
	handler.Lock()
	defer handler.Unlock()
	var n *graph.Node

	for nodeIdentifier, nodeConnectedToType := range nodes {
		n = handler.GetNodeByIdentifier(nodeIdentifier)
		if n == nil {
			continue
		}
		nodeConnectedToType.ConnectTwoNodes(handler.Graph, n, Node)
		nodeConnectedToType.UpdateMetadata(handler.Graph, n)
		delete(nodes, nodeIdentifier)
	}
	handler.StartTrackingChangesForNode(NodeIdentifier, nodes)
	return nil
}

func MakeGraphHandler(g *graph.Graph, propertyToIdentifyNode string) *GraphHandler {
	handler := &GraphHandler{
		Graph:                  g,
		NodeMap:                make(map[string]NodeInfo),
		PropertyToIdentifyNode: propertyToIdentifyNode,
	}
	return handler
}
