package traversal

import (
	"testing"
	"time"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
	"github.com/stretchr/testify/assert"
)

// FakeMergeGraphBackend simulate a backend with history that could store different revisions of nodes
type FakeMergeGraphBackend struct {
	graph.MemoryBackend
	Nodes []*graph.Node
}

func (b *FakeMergeGraphBackend) IsHistorySupported() bool {
	return true
}

func (b *FakeMergeGraphBackend) GetNode(i graph.Identifier, at graph.Context) []*graph.Node {
	nodes := []*graph.Node{}
	for _, n := range b.Nodes {
		if n.ID == i {
			nodes = append(nodes, n)
		}
	}
	return nodes
}

func (b *FakeMergeGraphBackend) GetNodesFromIDs(identifierList []graph.Identifier, at graph.Context) []*graph.Node {
	nodes := []*graph.Node{}
	for _, n := range b.Nodes {
		for _, i := range identifierList {
			if n.ID == i {
				nodes = append(nodes, n)
			}
		}
	}
	return nodes
}

func TestMergeMetadataNilNodeMerge(t *testing.T) {
	key := "Merge"

	metadataNode1 := graph.Metadata{key: map[string]interface{}{
		"abc": map[interface{}]string{"descr": "foo"},
	}}
	node := CreateNode("nodeA", metadataNode1, graph.TimeUTC(), 1)

	nodeMergeAgg := mergeMetadata(node, key, nil)

	expected := map[string][]interface{}{
		"abc": {
			map[interface{}]string{"descr": "foo"},
		},
	}

	assert.Equal(t, expected, nodeMergeAgg)
}

func TestMergeMetadataPointerValue(t *testing.T) {
	key := "Merge"

	value := map[string]interface{}{
		"abc": map[interface{}]string{"descr": "foo"},
	}

	metadataNode1 := graph.Metadata{key: &value}
	node := CreateNode("nodeA", metadataNode1, graph.TimeUTC(), 1)

	nodeMergeAgg := mergeMetadata(node, key, nil)

	expected := map[string][]interface{}{
		"abc": {
			map[interface{}]string{"descr": "foo"},
		},
	}

	assert.Equal(t, expected, nodeMergeAgg)
}

func TestMergeMetadata(t *testing.T) {
	tests := []struct {
		name       string
		nodesMerge []interface{}
		expected   map[string][]interface{}
	}{
		{
			name:     "no nodes",
			expected: map[string][]interface{}{},
		},
		{
			name: "one node",
			nodesMerge: []interface{}{
				map[string]interface{}{
					"abc": map[string]string{"descr": "foo"},
				}},
			expected: map[string][]interface{}{
				"abc": {
					map[string]string{"descr": "foo"},
				},
			},
		},
		{
			name: "two nodes, different keys",
			nodesMerge: []interface{}{
				map[string]interface{}{
					"abc": map[string]string{"descr": "foo"},
				},
				map[string]interface{}{
					"xyz": map[string]string{"descr": "bar"},
				}},
			expected: map[string][]interface{}{
				"abc": {
					map[string]string{"descr": "foo"},
				},
				"xyz": {
					map[string]string{"descr": "bar"},
				},
			},
		},
		{
			name: "two nodes, same keys",
			nodesMerge: []interface{}{
				map[string]interface{}{
					"abc": map[string]string{"descr": "foo"},
				},
				map[string]interface{}{
					"abc": map[string]string{"descr": "bar"},
				}},
			expected: map[string][]interface{}{
				"abc": {
					map[string]string{"descr": "foo"},
					map[string]string{"descr": "bar"},
				},
			},
		},
		{
			name: "two nodes, repeating one event, should be removed",
			nodesMerge: []interface{}{
				map[string]interface{}{
					"abc": map[string]string{"descr": "foo"},
				},
				map[string]interface{}{
					"abc": map[string]string{"descr": "foo"},
					"xxx": map[string]string{"descr": "bar"},
				}},
			expected: map[string][]interface{}{
				"abc": {
					map[string]string{"descr": "foo"},
				},
				"xxx": {
					map[string]string{"descr": "bar"},
				},
			},
		},
	}

	key := "Merge"

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			nodeMergeAgg := map[string][]interface{}{}

			for _, nodeMerge := range test.nodesMerge {
				metadataNode1 := graph.Metadata{key: nodeMerge}
				node := CreateNode("nodeA", metadataNode1, graph.TimeUTC(), 1)

				nodeMergeAgg = mergeMetadata(node, key, nodeMergeAgg)
			}

			assert.Equal(t, test.expected, nodeMergeAgg)
		})
	}
}

func TestInterfaceMerge(t *testing.T) {
	tests := []struct {
		name      string
		InNodes   []*graph.Node
		key       string
		aggKey    string
		startTime time.Time
		endTime   time.Time
		// Expected nodes
		OutNodes []*graph.Node
	}{
		{
			name: "no input nodes",
		},
		{
			// Node passes the step without being modified
			name:   "one input node without key defined",
			key:    "Merge",
			aggKey: "MergeAgg",
			InNodes: []*graph.Node{
				CreateNode("A", graph.Metadata{}, graph.Time(time.Unix(0, 0)), 1),
			},
			OutNodes: []*graph.Node{
				CreateNode("A", graph.Metadata{
					"MergeAgg": map[string][]interface{}{},
				}, graph.Time(time.Unix(0, 0)), 1),
			},
		},
		{
			name:   "one input node with key defined but empty",
			key:    "Merge",
			aggKey: "MergeAgg",
			InNodes: []*graph.Node{
				CreateNode("A", graph.Metadata{
					"Merge": map[string]interface{}{},
				}, graph.Time(time.Unix(0, 0)), 1),
			},
			OutNodes: []*graph.Node{
				CreateNode("A", graph.Metadata{
					"Merge":    map[string]interface{}{},
					"MergeAgg": map[string][]interface{}{},
				}, graph.Time(time.Unix(0, 0)), 1),
			},
		},
		{
			name:   "one input node with key defined and one element",
			key:    "Merge",
			aggKey: "MergeAgg",
			InNodes: []*graph.Node{
				CreateNode("A", graph.Metadata{
					"Merge": map[string]interface{}{"e1": map[string]interface{}{"desc": "a"}},
				}, graph.Time(time.Unix(0, 0)), 1),
			},
			OutNodes: []*graph.Node{
				CreateNode("A", graph.Metadata{
					"Merge": map[string]interface{}{"e1": map[string]interface{}{"desc": "a"}},
					"MergeAgg": map[string][]interface{}{"e1": {
						map[string]interface{}{"desc": "a"},
					}},
				}, graph.Time(time.Unix(0, 0)), 1),
			},
		},
		{
			name:   "one input node with a complex element",
			key:    "Merge",
			aggKey: "MergeAgg",
			InNodes: []*graph.Node{
				CreateNode("A", graph.Metadata{
					"Merge": map[string]interface{}{
						"e1": map[string]interface{}{
							"desc": "a",
							"TTL":  45,
							"Payload": []interface{}{
								map[string]interface{}{"Key": "foo"},
								map[string]interface{}{"Value": "bar"},
							},
						},
					},
				}, graph.Time(time.Unix(0, 0)), 1),
			},
			OutNodes: []*graph.Node{
				CreateNode("A", graph.Metadata{
					"Merge": map[string]interface{}{
						"e1": map[string]interface{}{
							"desc": "a",
							"TTL":  45,
							"Payload": []interface{}{
								map[string]interface{}{"Key": "foo"},
								map[string]interface{}{"Value": "bar"},
							},
						},
					},
					"MergeAgg": map[string][]interface{}{
						"e1": {
							map[string]interface{}{
								"desc": "a",
								"TTL":  45,
								"Payload": []interface{}{
									map[string]interface{}{"Key": "foo"},
									map[string]interface{}{"Value": "bar"},
								},
							},
						},
					},
				}, graph.Time(time.Unix(0, 0)), 1),
			},
		},
		{
			name:   "two different input nodes with key defined and one element each one",
			key:    "Merge",
			aggKey: "MergeAxx",
			InNodes: []*graph.Node{
				CreateNode("A", graph.Metadata{
					"Merge": map[string]interface{}{"e1": map[string]interface{}{"desc": "a"}},
				}, graph.Time(time.Unix(0, 0)), 1),
				CreateNode("B", graph.Metadata{
					"Merge": map[string]interface{}{"e1": map[string]interface{}{"desc": "a"}},
				}, graph.Time(time.Unix(0, 0)), 1),
			},
			OutNodes: []*graph.Node{
				CreateNode("A", graph.Metadata{
					"Merge": map[string]interface{}{"e1": map[string]interface{}{"desc": "a"}},
					"MergeAxx": map[string][]interface{}{"e1": {
						map[string]interface{}{"desc": "a"},
					}},
				}, graph.Time(time.Unix(0, 0)), 1),
				CreateNode("B", graph.Metadata{
					"Merge": map[string]interface{}{"e1": map[string]interface{}{"desc": "a"}},
					"MergeAxx": map[string][]interface{}{"e1": {
						map[string]interface{}{"desc": "a"},
					}},
				}, graph.Time(time.Unix(0, 0)), 1),
			},
		},
		{
			name:   "one node, with a previous version, both without key defined",
			key:    "Merge",
			aggKey: "MergeAgg",
			InNodes: []*graph.Node{
				CreateNode("A", graph.Metadata{}, graph.Time(time.Unix(0, 0)), 1),
				CreateNode("A", graph.Metadata{}, graph.Time(time.Unix(0, 0)), 2),
			},
			OutNodes: []*graph.Node{
				CreateNode("A", graph.Metadata{
					"MergeAgg": map[string][]interface{}{},
				}, graph.Time(time.Unix(0, 0)), 2),
			},
		},
		{
			name:   "one node, with a previous version, both with key defined but empty",
			key:    "Merge",
			aggKey: "MergeAgg",
			InNodes: []*graph.Node{
				CreateNode("A", graph.Metadata{
					"Merge": map[string]interface{}{},
				}, graph.Time(time.Unix(0, 0)), 1),
				CreateNode("A", graph.Metadata{
					"Merge": map[string]interface{}{},
				}, graph.Time(time.Unix(0, 0)), 2),
			},
			OutNodes: []*graph.Node{
				CreateNode("A", graph.Metadata{
					"Merge":    map[string]interface{}{},
					"MergeAgg": map[string][]interface{}{},
				}, graph.Time(time.Unix(0, 0)), 2),
			},
		},
		{
			name:   "one node, with a previous version, both with key defined, same event different content",
			key:    "Merge",
			aggKey: "MergeAgg",
			InNodes: []*graph.Node{
				CreateNode("A", graph.Metadata{
					"Merge": map[string]interface{}{"e1": map[string]interface{}{"desc": "a"}},
				}, graph.Time(time.Unix(0, 0)), 1),
				CreateNode("A", graph.Metadata{
					"Merge": map[string]interface{}{"e1": map[string]interface{}{"desc": "b"}},
				}, graph.Time(time.Unix(0, 0)), 2),
			},
			OutNodes: []*graph.Node{
				CreateNode("A", graph.Metadata{
					"Merge": map[string]interface{}{"e1": map[string]interface{}{"desc": "b"}},
					"MergeAgg": map[string][]interface{}{"e1": {
						map[string]interface{}{"desc": "a"},
						map[string]interface{}{"desc": "b"},
					}},
				}, graph.Time(time.Unix(0, 0)), 2),
			},
		},
		{
			name:   "one node, with a previous version, first one without event, second one with event",
			key:    "Merge",
			aggKey: "MergeAgg",
			InNodes: []*graph.Node{
				CreateNode("A", graph.Metadata{
					"Merge": map[string]interface{}{},
				}, graph.Time(time.Unix(0, 0)), 1),
				CreateNode("A", graph.Metadata{
					"Merge": map[string]interface{}{"e1": map[string]interface{}{"desc": "b"}},
				}, graph.Time(time.Unix(0, 0)), 2),
			},
			OutNodes: []*graph.Node{
				CreateNode("A", graph.Metadata{
					"Merge": map[string]interface{}{"e1": map[string]interface{}{"desc": "b"}},
					"MergeAgg": map[string][]interface{}{"e1": {
						map[string]interface{}{"desc": "b"},
					}},
				}, graph.Time(time.Unix(0, 0)), 2),
			},
		},
		{
			name:   "one node, with a previous version, first one with event, second one without event",
			key:    "Merge",
			aggKey: "MergeAgg",
			InNodes: []*graph.Node{
				CreateNode("A", graph.Metadata{
					"Merge": map[string]interface{}{"e1": map[string]interface{}{"desc": "a"}},
				}, graph.Time(time.Unix(0, 0)), 1),
				CreateNode("A", graph.Metadata{
					"Merge": map[string]interface{}{},
				}, graph.Time(time.Unix(0, 0)), 2),
			},
			OutNodes: []*graph.Node{
				CreateNode("A", graph.Metadata{
					"Merge": map[string]interface{}{},
					"MergeAgg": map[string][]interface{}{"e1": {
						map[string]interface{}{"desc": "a"},
					}},
				}, graph.Time(time.Unix(0, 0)), 2),
			},
		},
		{
			name:   "one node, with two previous versions, first with, second without, third with",
			key:    "Merge",
			aggKey: "MergeAgg",
			InNodes: []*graph.Node{
				CreateNode("A", graph.Metadata{
					"Merge": map[string]interface{}{"e1": map[string]interface{}{"desc": "a"}},
				}, graph.Time(time.Unix(0, 0)), 1),
				CreateNode("A", graph.Metadata{
					"Merge": map[string]interface{}{},
				}, graph.Time(time.Unix(0, 0)), 2),
				CreateNode("A", graph.Metadata{
					"Merge": map[string]interface{}{"e1": map[string]interface{}{"desc": "c"}},
				}, graph.Time(time.Unix(0, 0)), 3),
			},
			OutNodes: []*graph.Node{
				CreateNode("A", graph.Metadata{
					"Merge": map[string]interface{}{"e1": map[string]interface{}{"desc": "c"}},
					"MergeAgg": map[string][]interface{}{"e1": {
						map[string]interface{}{"desc": "a"},
						map[string]interface{}{"desc": "c"},
					}},
				}, graph.Time(time.Unix(0, 0)), 3),
			},
		},
		{
			name:      "memory backend does not filter nodes by date, startTime and endTime are ignored",
			key:       "Merge",
			aggKey:    "MergeAgg",
			startTime: time.Unix(100, 0),
			endTime:   time.Unix(200, 0),
			InNodes: []*graph.Node{
				CreateNode("A", graph.Metadata{
					"Merge": map[string]interface{}{"e1": map[string]interface{}{"desc": "a"}},
				}, graph.Time(time.Unix(300, 0)), 1),
			},
			OutNodes: []*graph.Node{
				CreateNode("A", graph.Metadata{
					"Merge": map[string]interface{}{"e1": map[string]interface{}{"desc": "a"}},
					"MergeAgg": map[string][]interface{}{"e1": {
						map[string]interface{}{"desc": "a"},
					}},
				}, graph.Time(time.Unix(300, 0)), 1),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			b := FakeMergeGraphBackend{
				Nodes: test.InNodes,
			}
			g := graph.NewGraph("testhost", &b, "analyzer.testhost")

			gt := traversal.NewGraphTraversal(g, false)
			tvIn := traversal.NewGraphTraversalV(gt, test.InNodes)

			s := MergeGremlinTraversalStep{
				MergeKey:    test.key,
				MergeAggKey: test.aggKey,
				StartTime:   test.startTime,
				EndTime:     test.endTime,
			}
			ts, err := s.InterfaceMerge(tvIn)
			if err != nil {
				t.Error(err.Error())
			}

			tvOut, ok := ts.(*traversal.GraphTraversalV)
			if !ok {
				t.Errorf("Invalid GraphTraversal type")
			}

			assert.ElementsMatch(t, test.OutNodes, tvOut.GetNodes())
		})
	}
}

func TestInterfaceMergeDoNotModifyOriginNodes(t *testing.T) {
	n := CreateNode("A", graph.Metadata{
		"Merge": map[string]interface{}{"e1": map[string]interface{}{"desc": "a"}},
	}, graph.Time(time.Unix(0, 0)), 1)

	nCopy := CreateNode("A", graph.Metadata{
		"Merge": map[string]interface{}{"e1": map[string]interface{}{"desc": "a"}},
	}, graph.Time(time.Unix(0, 0)), 1)

	b := FakeMergeGraphBackend{
		Nodes: []*graph.Node{n},
	}
	g := graph.NewGraph("testhost", &b, "analyzer.testhost")

	gt := traversal.NewGraphTraversal(g, false)
	tvIn := traversal.NewGraphTraversalV(gt, []*graph.Node{n})

	s := MergeGremlinTraversalStep{
		MergeKey:    "Merge",
		MergeAggKey: "AggMerge",
	}
	_, err := s.InterfaceMerge(tvIn)
	assert.Nil(t, err)

	// Node stored in the graph should not be modified
	assert.Equal(t, b.GetNode("A", graph.Context{})[0], nCopy)
}

func TestEventsParseStep(t *testing.T) {
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
			name:          "nil traversalCtx",
			token:         traversalMergeToken,
			expectedError: "Merge parameter must have two or four parameters (OriginKey, DestinationKey, StartTime, EndTime)",
		},
		{
			name:  "only one param",
			token: traversalMergeToken,
			traversalCtx: traversal.GremlinTraversalContext{
				Params: []interface{}{"foo"},
			},
			expectedError: "Merge parameter must have two or four parameters (OriginKey, DestinationKey, StartTime, EndTime)",
		},
		{
			name:  "two param not string",
			token: traversalMergeToken,
			traversalCtx: traversal.GremlinTraversalContext{
				Params: []interface{}{1, 2},
			},
			expectedError: "Merge first parameter have to be a string",
		},
		{
			name:  "two string params",
			token: traversalMergeToken,
			traversalCtx: traversal.GremlinTraversalContext{
				Params: []interface{}{"key", "aggKey"},
			},
			expectedTraversalStep: &MergeGremlinTraversalStep{
				GremlinTraversalContext: traversal.GremlinTraversalContext{
					Params: []interface{}{"key", "aggKey"},
				},
				MergeKey:    "key",
				MergeAggKey: "aggKey",
			},
		},
		{
			name:  "four valid params",
			token: traversalMergeToken,
			traversalCtx: traversal.GremlinTraversalContext{
				Params: []interface{}{"key", "aggKey", int64(1627987976), int64(1627987977)},
			},
			expectedTraversalStep: &MergeGremlinTraversalStep{
				GremlinTraversalContext: traversal.GremlinTraversalContext{
					Params: []interface{}{"key", "aggKey", int64(1627987976), int64(1627987977)},
				},
				MergeKey:    "key",
				MergeAggKey: "aggKey",
				StartTime:   time.Unix(1627987976, 0),
				EndTime:     time.Unix(1627987977, 0),
			},
		},
		{
			name:  "invalid start date",
			token: traversalMergeToken,
			traversalCtx: traversal.GremlinTraversalContext{
				Params: []interface{}{"foo", "bar", "123456789", "123456789"},
			},
			expectedError: "Merge third parameter have to be a int (unix epoch time)",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			e := MergeTraversalExtension{MergeToken: traversalMergeToken}

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

// CreateNode func to create nodes with a specific node revision
func CreateNode(id string, m graph.Metadata, t graph.Time, revision int64) *graph.Node {
	n := graph.CreateNode(graph.Identifier(id), m, t, "host", "orig")
	n.Revision = revision
	return n
}
