package traversal

import (
	"fmt"
	"testing"
	"time"

	"github.com/skydive-project/skydive/graffiti/filters"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
	"github.com/skydive-project/skydive/topology"
	"github.com/stretchr/testify/assert"
)

// FakeNeighborsSlowGraphBackend simulate a backend with history that could store different revisions of nodes
type FakeNeighborsSlowGraphBackend struct {
	Backend *graph.MemoryBackend
}

func (f *FakeNeighborsSlowGraphBackend) NodeAdded(n *graph.Node) error {
	return f.Backend.NodeAdded(n)
}

func (f *FakeNeighborsSlowGraphBackend) NodeDeleted(n *graph.Node) error {
	return f.Backend.NodeDeleted(n)
}

func (f *FakeNeighborsSlowGraphBackend) GetNode(i graph.Identifier, at graph.Context) []*graph.Node {
	time.Sleep(20 * time.Millisecond)
	return f.Backend.GetNode(i, at)
}

func (f *FakeNeighborsSlowGraphBackend) GetNodesFromIDs(i []graph.Identifier, at graph.Context) []*graph.Node {
	time.Sleep(40 * time.Millisecond)
	return f.Backend.GetNodesFromIDs(i, at)
}

func (f *FakeNeighborsSlowGraphBackend) GetNodeEdges(n *graph.Node, at graph.Context, m graph.ElementMatcher) []*graph.Edge {
	time.Sleep(20 * time.Millisecond)
	return f.Backend.GetNodeEdges(n, at, m)
}

func (f *FakeNeighborsSlowGraphBackend) GetNodesEdges(n []*graph.Node, at graph.Context, m graph.ElementMatcher) []*graph.Edge {
	time.Sleep(40 * time.Millisecond)
	return f.Backend.GetNodesEdges(n, at, m)
}

func (f *FakeNeighborsSlowGraphBackend) EdgeAdded(e *graph.Edge) error {
	return f.Backend.EdgeAdded(e)
}

func (f *FakeNeighborsSlowGraphBackend) EdgeDeleted(e *graph.Edge) error {
	return f.Backend.EdgeDeleted(e)
}

func (f *FakeNeighborsSlowGraphBackend) GetEdge(i graph.Identifier, at graph.Context) []*graph.Edge {
	return f.Backend.GetEdge(i, at)
}

func (f *FakeNeighborsSlowGraphBackend) GetEdgeNodes(e *graph.Edge, at graph.Context, parentMetadata graph.ElementMatcher, childMetadata graph.ElementMatcher) ([]*graph.Node, []*graph.Node) {
	return f.Backend.GetEdgeNodes(e, at, parentMetadata, childMetadata)
}

func (f *FakeNeighborsSlowGraphBackend) MetadataUpdated(e interface{}) error {
	return f.Backend.MetadataUpdated(e)
}

func (f *FakeNeighborsSlowGraphBackend) GetNodes(t graph.Context, e graph.ElementMatcher) []*graph.Node {
	return f.Backend.GetNodes(t, e)
}

func (f *FakeNeighborsSlowGraphBackend) GetEdges(t graph.Context, e graph.ElementMatcher) []*graph.Edge {
	return f.Backend.GetEdges(t, e)
}

func (f *FakeNeighborsSlowGraphBackend) IsHistorySupported() bool {
	return f.Backend.IsHistorySupported()
}

func TestGetNeighbors(t *testing.T) {
	testCases := []struct {
		desc          string
		graphNodes    []*graph.Node
		graphEdges    []*graph.Edge
		originNodes   []*graph.Node
		maxDepth      int64
		edgeFilter    graph.ElementMatcher
		onlyIDs       bool
		expectedNodes []*graph.Node
	}{
		{
			desc: "one graph node",
			graphNodes: []*graph.Node{
				graph.CreateNode(graph.Identifier("A"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
			},
			graphEdges: []*graph.Edge{},
			originNodes: []*graph.Node{
				graph.CreateNode(graph.Identifier("A"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
			},
			maxDepth:   0,
			edgeFilter: nil,
			expectedNodes: []*graph.Node{
				graph.CreateNode(graph.Identifier("A"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
			},
		},
		{
			desc: "one graph node with only ids strip all node data except id",
			graphNodes: []*graph.Node{
				graph.CreateNode(graph.Identifier("A"), graph.Metadata{"foo": "bar"}, graph.Unix(100, 0), "host", "origin"),
			},
			graphEdges: []*graph.Edge{},
			originNodes: []*graph.Node{
				graph.CreateNode(graph.Identifier("A"), graph.Metadata{"foo": "bar"}, graph.Unix(100, 0), "host", "origin"),
			},
			maxDepth:   0,
			edgeFilter: nil,
			onlyIDs:    true,
			expectedNodes: []*graph.Node{
				graph.CreateNode(graph.Identifier("A"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
			},
		},
		{
			desc: "interface connected to host and to other interface",
			graphNodes: []*graph.Node{
				graph.CreateNode(graph.Identifier("HostA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
				graph.CreateNode(graph.Identifier("IntA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
				graph.CreateNode(graph.Identifier("IntB"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
			},
			graphEdges: []*graph.Edge{
				graph.CreateEdge(
					graph.Identifier("HostA-IntA"),
					graph.CreateNode(graph.Identifier("HostA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
					graph.CreateNode(graph.Identifier("IntA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
					graph.Metadata{"RelationType": "ownership"},
					graph.Unix(0, 0),
					"",
					"",
				),
				graph.CreateEdge(
					graph.Identifier("IntA-IntB"),
					graph.CreateNode(graph.Identifier("IntA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
					graph.CreateNode(graph.Identifier("IntB"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
					graph.Metadata{"RelationType": "ConnectsTo"},
					graph.Unix(0, 0),
					"",
					"",
				),
			},
			originNodes: []*graph.Node{
				graph.CreateNode(graph.Identifier("IntA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
			},
			maxDepth:   1,
			edgeFilter: nil,
			expectedNodes: []*graph.Node{
				graph.CreateNode(graph.Identifier("HostA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
				graph.CreateNode(graph.Identifier("IntA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
				graph.CreateNode(graph.Identifier("IntB"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
			},
		},
		{
			desc: "host connected to interface and that to other interface, depth 2",
			graphNodes: []*graph.Node{
				graph.CreateNode(graph.Identifier("HostA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
				graph.CreateNode(graph.Identifier("IntA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
				graph.CreateNode(graph.Identifier("IntB"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
			},
			graphEdges: []*graph.Edge{
				graph.CreateEdge(
					graph.Identifier("HostA-IntA"),
					graph.CreateNode(graph.Identifier("HostA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
					graph.CreateNode(graph.Identifier("IntA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
					graph.Metadata{"RelationType": "ownership"},
					graph.Unix(0, 0),
					"",
					"",
				),
				graph.CreateEdge(
					graph.Identifier("IntA-IntB"),
					graph.CreateNode(graph.Identifier("IntA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
					graph.CreateNode(graph.Identifier("IntB"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
					graph.Metadata{"RelationType": "ConnectsTo"},
					graph.Unix(0, 0),
					"",
					"",
				),
			},
			originNodes: []*graph.Node{
				graph.CreateNode(graph.Identifier("HostA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
			},
			maxDepth:   2,
			edgeFilter: nil,
			expectedNodes: []*graph.Node{
				graph.CreateNode(graph.Identifier("HostA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
				graph.CreateNode(graph.Identifier("IntA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
				graph.CreateNode(graph.Identifier("IntB"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
			},
		},
		{
			desc: "two hosts connected through interfaces, depth 3",
			graphNodes: []*graph.Node{
				graph.CreateNode(graph.Identifier("HostA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
				graph.CreateNode(graph.Identifier("IntA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
				graph.CreateNode(graph.Identifier("HostB"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
				graph.CreateNode(graph.Identifier("IntB"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
			},
			graphEdges: []*graph.Edge{
				graph.CreateEdge(
					graph.Identifier("HostA-IntA"),
					graph.CreateNode(graph.Identifier("HostA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
					graph.CreateNode(graph.Identifier("IntA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
					graph.Metadata{"RelationType": "ownership"},
					graph.Unix(0, 0),
					"",
					"",
				),
				graph.CreateEdge(
					graph.Identifier("HostB-IntB"),
					graph.CreateNode(graph.Identifier("HostB"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
					graph.CreateNode(graph.Identifier("IntB"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
					graph.Metadata{"RelationType": "ownership"},
					graph.Unix(0, 0),
					"",
					"",
				),
				graph.CreateEdge(
					graph.Identifier("IntA-IntB"),
					graph.CreateNode(graph.Identifier("IntA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
					graph.CreateNode(graph.Identifier("IntB"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
					graph.Metadata{"RelationType": "ConnectsTo"},
					graph.Unix(0, 0),
					"",
					"",
				),
			},
			originNodes: []*graph.Node{
				graph.CreateNode(graph.Identifier("HostA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
			},
			maxDepth:   3,
			edgeFilter: nil,
			expectedNodes: []*graph.Node{
				graph.CreateNode(graph.Identifier("HostA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
				graph.CreateNode(graph.Identifier("IntA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
				graph.CreateNode(graph.Identifier("HostB"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
				graph.CreateNode(graph.Identifier("IntB"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
			},
		}, {
			desc: "two hosts connected through interfaces, reverse connection, depth 3",
			graphNodes: []*graph.Node{
				graph.CreateNode(graph.Identifier("HostA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
				graph.CreateNode(graph.Identifier("IntA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
				graph.CreateNode(graph.Identifier("HostB"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
				graph.CreateNode(graph.Identifier("IntB"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
			},
			graphEdges: []*graph.Edge{
				graph.CreateEdge(
					graph.Identifier("HostA-IntA"),
					graph.CreateNode(graph.Identifier("HostA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
					graph.CreateNode(graph.Identifier("IntA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
					graph.Metadata{"RelationType": "ownership"},
					graph.Unix(0, 0),
					"",
					"",
				),
				graph.CreateEdge(
					graph.Identifier("HostB-IntB"),
					graph.CreateNode(graph.Identifier("HostB"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
					graph.CreateNode(graph.Identifier("IntB"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
					graph.Metadata{"RelationType": "ownership"},
					graph.Unix(0, 0),
					"",
					"",
				),
				graph.CreateEdge(
					graph.Identifier("IntB-IntA"),
					graph.CreateNode(graph.Identifier("IntB"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
					graph.CreateNode(graph.Identifier("IntA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
					graph.Metadata{"RelationType": "ConnectsTo"},
					graph.Unix(0, 0),
					"",
					"",
				),
			},
			originNodes: []*graph.Node{
				graph.CreateNode(graph.Identifier("HostA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
			},
			maxDepth:   3,
			edgeFilter: nil,
			expectedNodes: []*graph.Node{
				graph.CreateNode(graph.Identifier("HostA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
				graph.CreateNode(graph.Identifier("IntA"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
				graph.CreateNode(graph.Identifier("HostB"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
				graph.CreateNode(graph.Identifier("IntB"), graph.Metadata{}, graph.Unix(0, 0), "", ""),
			},
		},
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			b, err := graph.NewMemoryBackend()
			if err != nil {
				t.Error(err.Error())
			}
			g := graph.NewGraph("testhost", b, "analyzer.testhost")

			for _, n := range tC.graphNodes {
				err := g.AddNode(n)
				if err != nil {
					t.Error(err.Error())
				}
			}

			for _, e := range tC.graphEdges {
				err := g.AddEdge(e)
				if err != nil {
					t.Error(err.Error())
				}
			}

			d := NeighborsGremlinTraversalStep{
				maxDepth:        tC.maxDepth,
				edgeFilter:      tC.edgeFilter,
				nextStepOnlyIDs: tC.onlyIDs,
			}
			neighbors := d.getNeighbors(g, tC.originNodes)

			assert.ElementsMatch(t, neighbors, tC.expectedNodes)

		})
	}
}

func TestNeighborsParseStep(t *testing.T) {
	ownershipFilter, err := topology.OwnershipMetadata().Filter()
	assert.Nil(t, err)

	relationTypeFooFilter, err := traversal.ParamsToFilter(filters.BoolFilterOp_OR, "RelationType", "foo")
	assert.Nil(t, err)

	relationTypeFooTypeBarFilter, err := traversal.ParamsToFilter(filters.BoolFilterOp_OR, "RelationType", "foo", "Type", "bar")
	assert.Nil(t, err)

	tests := []struct {
		name                  string
		token                 traversal.Token
		traversalCtx          traversal.GremlinTraversalContext
		expectedTraversalStep traversal.GremlinTraversalStep
		expectedError         string
	}{
		{
			name:  "non merge token",
			token: traversal.COUNT,
		},
		{
			name:  "nil traversalCtx is default values, depth one and ownership edge filter",
			token: traversalNeighborsToken,
			expectedTraversalStep: &NeighborsGremlinTraversalStep{
				maxDepth:   1,
				edgeFilter: graph.NewElementFilter(ownershipFilter),
			},
		},
		{
			name:  "one string param",
			token: traversalNeighborsToken,
			traversalCtx: traversal.GremlinTraversalContext{
				Params: []interface{}{"foo"},
			},
			expectedError: "Neighbors last argument must be the maximum depth specified as an integer",
		},
		{
			name:  "only one param, int number, is depth",
			token: traversalNeighborsToken,
			traversalCtx: traversal.GremlinTraversalContext{
				Params: []interface{}{int64(3)},
			},
			expectedTraversalStep: &NeighborsGremlinTraversalStep{
				context: traversal.GremlinTraversalContext{
					Params: []interface{}{int64(3)},
				},
				maxDepth:   3,
				edgeFilter: graph.NewElementFilter(ownershipFilter),
			},
		},
		{
			name:  "two string params are used as edge filter",
			token: traversalNeighborsToken,
			traversalCtx: traversal.GremlinTraversalContext{
				Params: []interface{}{"RelationType", "foo"},
			},
			expectedTraversalStep: &NeighborsGremlinTraversalStep{
				context: traversal.GremlinTraversalContext{
					Params: []interface{}{"RelationType", "foo"},
				},
				maxDepth:   1,
				edgeFilter: graph.NewElementFilter(relationTypeFooFilter),
			},
		},
		{
			name:  "four string params are used as edge filter and last int64 as depth",
			token: traversalNeighborsToken,
			traversalCtx: traversal.GremlinTraversalContext{
				Params: []interface{}{"RelationType", "foo", "Type", "bar", int64(5)},
			},
			expectedTraversalStep: &NeighborsGremlinTraversalStep{
				context: traversal.GremlinTraversalContext{
					Params: []interface{}{"RelationType", "foo", "Type", "bar", int64(5)},
				},
				maxDepth:   5,
				edgeFilter: graph.NewElementFilter(relationTypeFooTypeBarFilter),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			e := NeighborsTraversalExtension{NeighborsToken: traversalNeighborsToken}

			traversalStep, err := e.ParseStep(test.token, test.traversalCtx)
			if test.expectedError != "" {
				assert.EqualErrorf(t, err, test.expectedError, "error")
			} else {
				assert.Nil(t, err, "nil error")
			}

			assert.Equalf(t, test.expectedTraversalStep, traversalStep, "step")
		})
	}
}

func BenchmarkGetNeighbors(b *testing.B) {
	// Create graph with nodes and edges
	backend, err := graph.NewMemoryBackend()
	if err != nil {
		b.Error(err.Error())
	}

	slowBackend := FakeNeighborsSlowGraphBackend{backend}
	g := graph.NewGraph("testhost", &slowBackend, "analyzer.testhost")

	parentNodes := 20

	var node *graph.Node
	var nodeChild *graph.Node
	for n := 0; n < parentNodes; n++ {
		node, err = g.NewNode(graph.Identifier(fmt.Sprintf("%d", n)), graph.Metadata{})
		if err != nil {
			b.Error(err.Error())
		}

		//  Childs of this node
		for nc := 0; nc < 60; nc++ {
			nodeChild, err = g.NewNode(graph.Identifier(fmt.Sprintf("%d-%d", n, nc)), graph.Metadata{})
			if err != nil {
				b.Error(err.Error())
			}

			_, err = g.NewEdge(graph.Identifier(fmt.Sprintf("%d-%d", n, nc)), node, nodeChild, graph.Metadata{})
			if err != nil {
				b.Error(err.Error())
			}
		}
	}

	// Each node connects with its next
	nextNodeConnect := 5
	for n := 0; n < parentNodes-nextNodeConnect; n++ {
		for p := 1; p < nextNodeConnect; p++ {
			// Connect interfaces
			ifaceParentNode := g.GetNode(graph.Identifier(fmt.Sprintf("%d-%d", n, p)))
			ifaceChildNode := g.GetNode(graph.Identifier(fmt.Sprintf("%d-%d", n+p, n)))

			_, err = g.NewEdge(graph.Identifier(fmt.Sprintf("c-%d-%d", n, p)), ifaceParentNode, ifaceChildNode, graph.Metadata{})
			if err != nil {
				b.Error(err.Error())
			}
		}

	}

	// Using depth=8 we get a total of 798 neighbors
	d := NeighborsGremlinTraversalStep{
		maxDepth: 8,
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		d.getNeighbors(g, []*graph.Node{g.GetNode(graph.Identifier("1"))})
	}
}
