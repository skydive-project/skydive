package traversal

import (
	"github.com/pkg/errors"

	"github.com/skydive-project/skydive/graffiti/filters"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
	"github.com/skydive-project/skydive/topology"
)

// NeighborsTraversalExtension describes a new extension to enhance the topology
type NeighborsTraversalExtension struct {
	NeighborsToken traversal.Token
}

// NeighborsGremlinTraversalStep navigate the graph starting from a node, following edges
// from parent to child and from child to parent.
// It follows the same sintaxis as Ascendants and Descendants step.
// The behaviour is like Ascendants+Descendants combined.
// If only one param is defined, it is used as depth, eg: G.V('id').Neighbors(4)
// If we have an event number of parameters, they are used as edge filter, and
// depth is defaulted to one, eg.: G.V('id').Neighbors("Type","foo","RelationType","bar")
// If we have an odd, but >1, number of parameters, all but the last one are used as
// edge filters and the last one as depth, eg.: G.V('id').Neighbors("Type","foo","RelationType","bar",3)
type NeighborsGremlinTraversalStep struct {
	context    traversal.GremlinTraversalContext
	maxDepth   int64
	edgeFilter graph.ElementMatcher
	// nextStepOnlyIDs is set to true if the next step only needs node IDs and not the whole node info
	nextStepOnlyIDs bool
}

// NewNeighborsTraversalExtension returns a new graph traversal extension
func NewNeighborsTraversalExtension() *NeighborsTraversalExtension {
	return &NeighborsTraversalExtension{
		NeighborsToken: traversalNeighborsToken,
	}
}

// ScanIdent returns an associated graph token
func (e *NeighborsTraversalExtension) ScanIdent(s string) (traversal.Token, bool) {
	switch s {
	case "NEIGHBORS":
		return e.NeighborsToken, true
	}
	return traversal.IDENT, false
}

// ParseStep parses neighbors step
func (e *NeighborsTraversalExtension) ParseStep(t traversal.Token, p traversal.GremlinTraversalContext) (traversal.GremlinTraversalStep, error) {
	switch t {
	case e.NeighborsToken:
	default:
		return nil, nil
	}

	maxDepth := int64(1)
	edgeFilter, _ := topology.OwnershipMetadata().Filter()

	switch len(p.Params) {
	case 0:
	default:
		i := len(p.Params) / 2 * 2
		filter, err := traversal.ParamsToFilter(filters.BoolFilterOp_OR, p.Params[:i]...)
		if err != nil {
			return nil, errors.Wrap(err, "Neighbors accepts an optional number of key/value tuples and an optional depth")
		}
		edgeFilter = filter

		if i == len(p.Params) {
			break
		}

		fallthrough
	case 1:
		depth, ok := p.Params[len(p.Params)-1].(int64)
		if !ok {
			return nil, errors.New("Neighbors last argument must be the maximum depth specified as an integer")
		}
		maxDepth = depth
	}

	return &NeighborsGremlinTraversalStep{context: p, maxDepth: maxDepth, edgeFilter: graph.NewElementFilter(edgeFilter)}, nil
}

// getNeighbors given a list of nodes, get its neighbors nodes for "maxDepth" depth relationships.
// Edges between nodes must fulfill "edgeFilter" filter.
// Nodes passed to this function will always be in the response.
func (d *NeighborsGremlinTraversalStep) getNeighbors(g *graph.Graph, nodes []*graph.Node) []*graph.Node {
	// visitedNodes store neighors and avoid visiting twice the same node
	visitedNodes := map[graph.Identifier]interface{}{}

	// currentDepthNodesIDs slice with the nodes being processed in each depth.
	// We use "empty" while procesing the neighbors nodes to avoid extra calls to the backend.
	var currentDepthNodesIDs []graph.Identifier
	// nextDepthNodes slice were next depth nodes are being stored.
	// Initializated with the list of origin nodes where it should start from.
	nextDepthNodesIDs := make([]graph.Identifier, 0, len(nodes))

	// Mark origin nodes as already visited
	// Neighbor step will return also the origin nodes
	for _, n := range nodes {
		visitedNodes[n.ID] = struct{}{}
		nextDepthNodesIDs = append(nextDepthNodesIDs, n.ID)
	}

	// DFS
	// BFS must not be used because could lead to ignore some servers in this case:
	//   A -> B
	//   B -> C
	//   C -> D
	//   A -> C
	//   With depth=2, BFS will return A,B,C (C is visited in A->B->C, si ignored in A->C->D)
	//   DFS will return, the correct, A,B,C,D
	for i := 0; i < int(d.maxDepth); i++ {
		// Copy values from nextDepthNodes to currentDepthNodes
		currentDepthNodesIDs = make([]graph.Identifier, len(nextDepthNodesIDs))
		copy(currentDepthNodesIDs, nextDepthNodesIDs)

		nextDepthNodesIDs = nextDepthNodesIDs[:0] // Clean slice, keeping capacity
		// Get all edges for the list of nodes, filtered by edgeFilter
		// Convert the list of node ids to a list of nodes

		currentDepthNodes := make([]*graph.Node, 0, len(currentDepthNodesIDs))
		for _, nID := range currentDepthNodesIDs {
			currentDepthNodes = append(currentDepthNodes, graph.CreateNode(nID, graph.Metadata{}, graph.Unix(0, 0), "", ""))
		}
		edges := g.GetNodesEdges(currentDepthNodes, d.edgeFilter)

		for _, e := range edges {
			// Get nodeID of the other side of the edge
			// Store neighbors
			// We don't know in which side of the edge are the neighbors, so, add both sides if not already visited
			_, okParent := visitedNodes[e.Parent]
			if !okParent {
				visitedNodes[e.Parent] = struct{}{}
				// Do not walk nodes already processed
				nextDepthNodesIDs = append(nextDepthNodesIDs, e.Parent)
			}
			_, okChild := visitedNodes[e.Child]
			if !okChild {
				visitedNodes[e.Child] = struct{}{}
				nextDepthNodesIDs = append(nextDepthNodesIDs, e.Child)
			}
		}
	}

	// Return "empty" nodes (just with the ID) if the next step only need that info
	if d.nextStepOnlyIDs {
		ret := make([]*graph.Node, 0, len(visitedNodes))
		for nID := range visitedNodes {
			ret = append(ret, graph.CreateNode(nID, graph.Metadata{}, graph.Unix(0, 0), "", ""))
		}
		return ret
	}

	// Get concurrentl all nodes for the list of neighbors ids
	nodesIDs := make([]graph.Identifier, 0, len(visitedNodes))
	for n := range visitedNodes {
		nodesIDs = append(nodesIDs, n)
	}

	return g.GetNodesFromIDs(nodesIDs)
}

// Exec Neighbors step
func (d *NeighborsGremlinTraversalStep) Exec(last traversal.GraphTraversalStep) (traversal.GraphTraversalStep, error) {
	switch tv := last.(type) {
	case *traversal.GraphTraversalV:
		tv.GraphTraversal.RLock()
		neighbors := d.getNeighbors(tv.GraphTraversal.Graph, tv.GetNodes())
		tv.GraphTraversal.RUnlock()

		return traversal.NewGraphTraversalV(tv.GraphTraversal, neighbors), nil
	}
	return nil, traversal.ErrExecutionError
}

// Reduce Neighbors step
func (d *NeighborsGremlinTraversalStep) Reduce(next traversal.GremlinTraversalStep) (traversal.GremlinTraversalStep, error) {
	// Merge step only needs the ids of nodes. Saving some queries.
	if _, ok := next.(*MergeGremlinTraversalStep); ok {
		d.nextStepOnlyIDs = true
	}
	return next, nil
}

// Context Neighbors step
func (d *NeighborsGremlinTraversalStep) Context() *traversal.GremlinTraversalContext {
	return &d.context
}
