package realtime

import (
	"fmt"

	skydivegraph "github.com/skydive-project/skydive/topology/graph"
)

const (
	VLAYER_CONNECTION_TYPE uint = 1
)

type NodesConnectedToType struct {
	Name     string
	Type     uint
	Metadata map[string]string
}

var nodeConnectors map[uint]func(graph *skydivegraph.Graph, source *skydivegraph.Node, target *skydivegraph.Node) = map[uint]func(graph *skydivegraph.Graph, source *skydivegraph.Node, target *skydivegraph.Node){
	VLAYER_CONNECTION_TYPE: func(graph *skydivegraph.Graph, source *skydivegraph.Node, target *skydivegraph.Node) {
		graph.Link(source, target, skydivegraph.Metadata{"RelationType": "vlayer2"})
	},
}

func (t *NodesConnectedToType) ConnectTwoNodes(graph *skydivegraph.Graph, source *skydivegraph.Node, target *skydivegraph.Node) error {
	nodeConnector, ok := nodeConnectors[t.Type]
	if !ok {
		return fmt.Errorf("No handler defined for Node type %d", t.Type)
	}
	nodeConnector(graph, source, target)
	return nil
}

func (t *NodesConnectedToType) UpdateMetadata(graph *skydivegraph.Graph, node *skydivegraph.Node) error {
	tr := graph.StartMetadataTransaction(node)
	for key, val := range t.Metadata {
		tr.AddMetadata(key, val)
	}
	tr.Commit()
	return nil
}
