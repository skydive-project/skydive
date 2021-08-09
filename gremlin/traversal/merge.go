package traversal

import (
	"fmt"
	"reflect"
	"time"

	"github.com/pkg/errors"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
	"github.com/skydive-project/skydive/graffiti/logging"
)

// MergeTraversalExtension describes a new extension to enhance the topology
type MergeTraversalExtension struct {
	MergeToken traversal.Token
}

// MergeGremlinTraversalStep step aggregates elements from different revisions of the nodes into a new metadata key.
// Nodes returned by this step are copies of the nodes in the graph, not the actual nodes.
// The reason is because this step is not meant to modify nodes in the graph, just for the output.
// This step should be used with a presistant backend, so it can access previous revisions of the nodes.
// To use this step we should select a metadata key (first parameter), where the elements will be read from.
// Inside this Metadata.Key elements should have the format map[interface{}]interface{} (could be a type based on that).
// The second parameter is the metadata key where all the elements will be aggregated.
// The aggregation will with the format: map[string][]interface{}.
// All elements with the same key in the map will be joined in an slice.
// To use this step we can use a graph with a time period context, eg: G.At(1479899809,3600).V().Merge('A','B').
// Or we can define the time period in the step: G.V().Merge('A','B',1500000000,1500099999).
// Note that in this case we define the start and end time, while in "At" is start time and duration.
// In both cases, Merge step will use the nodes given by the previous step.
type MergeGremlinTraversalStep struct {
	traversal.GremlinTraversalContext
	MergeKey    string
	MergeAggKey string
	StartTime   time.Time
	EndTime     time.Time
}

// NewMergeTraversalExtension returns a new graph traversal extension
func NewMergeTraversalExtension() *MergeTraversalExtension {
	return &MergeTraversalExtension{
		MergeToken: traversalMergeToken,
	}
}

// ScanIdent recognise the word associated with this step (in uppercase) and return a token
// which represents it. Return true if it have found a match
func (e *MergeTraversalExtension) ScanIdent(s string) (traversal.Token, bool) {
	switch s {
	case "MERGE":
		return e.MergeToken, true
	}
	return traversal.IDENT, false
}

// ParseStep generate a step for a given token, having in 'p' context and params
func (e *MergeTraversalExtension) ParseStep(t traversal.Token, p traversal.GremlinTraversalContext) (traversal.GremlinTraversalStep, error) {
	switch t {
	case e.MergeToken:
	default:
		return nil, nil
	}

	var mergeKey, mergeAggKey string
	var startTime, endTime time.Time
	var ok bool

	switch len(p.Params) {
	case 2:
		mergeKey, ok = p.Params[0].(string)
		if !ok {
			return nil, errors.New("Merge first parameter have to be a string")
		}
		mergeAggKey, ok = p.Params[1].(string)
		if !ok {
			return nil, errors.New("Merge second parameter have to be a string")
		}
	case 4:
		mergeKey, ok = p.Params[0].(string)
		if !ok {
			return nil, errors.New("Merge first parameter have to be a string")
		}
		mergeAggKey, ok = p.Params[1].(string)
		if !ok {
			return nil, errors.New("Merge second parameter have to be a string")
		}
		startTimeUnixEpoch, ok := p.Params[2].(int64)
		if !ok {
			return nil, errors.New("Merge third parameter have to be a int (unix epoch time)")
		}
		startTime = time.Unix(startTimeUnixEpoch, 0)
		endTimeUnixEpoch, ok := p.Params[3].(int64)
		if !ok {
			return nil, errors.New("Merge fourth parameter have to be a int (unix epoch time)")
		}
		endTime = time.Unix(endTimeUnixEpoch, 0)
	default:
		return nil, errors.New("Merge parameter must have two or four parameters (OriginKey, DestinationKey, StartTime, EndTime)")
	}

	return &MergeGremlinTraversalStep{
		GremlinTraversalContext: p,
		MergeKey:                mergeKey,
		MergeAggKey:             mergeAggKey,
		StartTime:               startTime,
		EndTime:                 endTime,
	}, nil
}

// Exec executes the merge step
func (s *MergeGremlinTraversalStep) Exec(last traversal.GraphTraversalStep) (traversal.GraphTraversalStep, error) {
	switch tv := last.(type) {
	case *traversal.GraphTraversalV:
		return s.InterfaceMerge(tv)

	}
	return nil, traversal.ErrExecutionError
}

// Reduce merge step
func (s *MergeGremlinTraversalStep) Reduce(next traversal.GremlinTraversalStep) (traversal.GremlinTraversalStep, error) {
	return next, nil
}

// Context merge step
func (s *MergeGremlinTraversalStep) Context() *traversal.GremlinTraversalContext {
	return &s.GremlinTraversalContext
}

// InterfaceMerge for each node id, group all the elements stored in Metadata.key from the
// input nodes and put them into the newest node for each id into Metadata.aggKey.
// Merge are groupped based on its key. See mergedMetadata for an example.
// All output nodes will have Metadata.aggKey defined (empty or not).
func (s *MergeGremlinTraversalStep) InterfaceMerge(tv *traversal.GraphTraversalV) (traversal.GraphTraversalStep, error) {
	// If user has defined start/end time in the parameters, use that values instead of the ones comming with the graph
	if !s.StartTime.IsZero() && !s.EndTime.IsZero() {
		timeSlice := graph.NewTimeSlice(
			graph.Time(s.StartTime).UnixMilli(),
			graph.Time(s.EndTime).UnixMilli(),
		)
		userTimeSliceCtx := graph.Context{
			TimeSlice: timeSlice,
			TimePoint: true,
		}

		newGraph, err := tv.GraphTraversal.Graph.CloneWithContext(userTimeSliceCtx)
		if err != nil {
			return nil, err
		}
		tv.GraphTraversal.Graph = newGraph
	}

	tv.GraphTraversal.RLock()
	defer tv.GraphTraversal.RUnlock()

	// uniqNodes store the latest node for each node identifier
	uniqNodes := map[graph.Identifier]*graph.Node{}

	// elements accumulate the elements for each node id
	elements := map[graph.Identifier]map[string][]interface{}{}

	// Get the list of node ids
	nodesIDs := make([]graph.Identifier, 0, len(tv.GetNodes()))
	for _, node := range tv.GetNodes() {
		nodesIDs = append(nodesIDs, node.ID)
	}

	// Get all revision for the list of node ids
	revisionNodes := tv.GraphTraversal.Graph.GetNodesFromIDs(nodesIDs)

	// Store only the most recent nodes
	for _, rNode := range revisionNodes {
		storedNode, ok := uniqNodes[rNode.ID]
		if !ok {
			uniqNodes[rNode.ID] = rNode
		} else {
			if storedNode.Revision < rNode.Revision {
				uniqNodes[rNode.ID] = rNode
			}
		}

		// Store elements from all revisions into the "elements" variable
		elements[rNode.ID] = mergeMetadata(rNode, s.MergeKey, elements[rNode.ID])
	}

	// Move the nodes from the uniqNodes map to an slice required by TraversalV
	// Return a copy of the nodes, not the actual graph nodes, because we don't want
	// to modify nodes with this step, just append some extra info
	nodes := []*graph.Node{}
	for id, n := range uniqNodes {
		nCopy := n.Copy()

		e, ok := elements[id]
		if ok {
			// Set the stored node with the merge of previous and current node
			metadataSet := nCopy.Metadata.SetField(s.MergeAggKey, e)
			if !metadataSet {
				return nil, fmt.Errorf("unable to set elements metadata for copied node %v", id)
			}
		}

		nodes = append(nodes, nCopy)
	}

	return traversal.NewGraphTraversalV(tv.GraphTraversal, nodes), nil
}

// mergeMetadata return the merge of node.Key elements with the ones already stored in nodeMerge
// Eg.:
//   node:       Metadata.key: {"a":{x}, "b":{y}}
//   nodeMerge:                {"a":[{z}]}
//   return:     Metadata.key: {"a":[{x},{z}], "b":[{y}]}
//
// Ignore if Metadata.key has an invalid format (not a map).
// Reflect is used to be able to access map's defined in different types.
// Element aggregation data type should be map[string]interface{} to be able to be encoded with JSON
func mergeMetadata(node *graph.Node, key string, nodeMerge map[string][]interface{}) map[string][]interface{} {
	if nodeMerge == nil {
		nodeMerge = map[string][]interface{}{}
	}

	n1MergeIface, n1Err := node.GetField(key)

	if n1Err == nil {
		// Ignore Metadata.key values which are not a map
		n1MergeValue := reflect.ValueOf(n1MergeIface)

		// If the metadata value is a pointer, resolve it
		if n1MergeValue.Kind() == reflect.Ptr {
			n1MergeValue = n1MergeValue.Elem()
		}

		// Merge step only accepts a map as data origin
		if n1MergeValue.Kind() != reflect.Map {
			logging.GetLogger().Errorf("Invalid type for elements, expecting a map, but it is %v", n1MergeValue.Kind())
			return nodeMerge
		}

		iter := n1MergeValue.MapRange()
	NODE_MERGE:
		for iter.Next() {
			k := fmt.Sprintf("%v", iter.Key().Interface())
			v := iter.Value().Interface()

			// Do not append if the same element already exists
			for _, storedElement := range nodeMerge[k] {
				if reflect.DeepEqual(storedElement, v) {
					continue NODE_MERGE
				}
			}

			nodeMerge[k] = append(nodeMerge[k], v)
		}
	}

	return nodeMerge
}
