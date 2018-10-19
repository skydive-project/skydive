package realtime

import (
	// "reflect"
	"testing"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/topology/graph"
)

func makeGraphRealtimeHandler(PropertyToIdentifyNode string) *GraphHandler {
	graph_backend, _ := graph.NewBackendByName("memory", nil)
	graph := graph.NewGraph("test", graph_backend, common.AgentService)
	graphHandler := MakeGraphHandler(graph, PropertyToIdentifyNode)
	graph.AddEventListener(graphHandler)
	return graphHandler
}

func TestMakeSureLayersAttachedInRealtime(t *testing.T) {
	layerNodes := make(map[string]NodesConnectedToType)
	layerNodes["target0"] = NodesConnectedToType{
		Name: "target0",
		Type: VLAYER_CONNECTION_TYPE,
		Metadata: map[string]string{
			"test":  "test",
			"test1": "test1",
		},
	}
	handler := makeGraphRealtimeHandler("Name")
	handler.AddNode("source1", &graph.Metadata{
		"Name": "source1",
	}, layerNodes)
	handler.AddNode("target0", &graph.Metadata{
		"Name": "target0",
	}, make(map[string]NodesConnectedToType))
	source := handler.GetNodeByIdentifier("source1")
	target := handler.GetNodeByIdentifier("target0")
	if !handler.Graph.AreLinked(source, target, nil) {
		t.Errorf("node source doesnt have a link to target node")
	}
}

func TestRemoveNode(t *testing.T) {
	handler := makeGraphRealtimeHandler("Name")
	handler.AddNode("target0", &graph.Metadata{
		"Name": "target0",
	}, make(map[string]NodesConnectedToType))
	target := handler.GetNodeByIdentifier("target0")
	if target == nil {
		t.Errorf("node wasnt added to the graph")
	}
	handler.RemoveNode("target0")
	target = handler.GetNodeByIdentifier("target0")
	if target != nil {
		t.Errorf("node still exists in the graph")
	}
}
