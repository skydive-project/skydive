package traversal

import (
	"github.com/pkg/errors"
	"strings"
)

type TraversalGraphFilter struct {
	traversalParser *GremlinTraversalParser
}

// Initialize TraversalFilter with Parser
func NewTraversalGraphFilter(trParser *GremlinTraversalParser) *TraversalGraphFilter {
	return &TraversalGraphFilter{traversalParser: trParser}
}

// Return filtered sub-graph for a given query (in Gremlin format)
func (tf *TraversalGraphFilter) GetFilteredGraphElements(query string) ([]interface{}, error) {
	if query == "" || tf.traversalParser == nil {
		return nil, errors.New("Empty query or uninitialized traversal parser")
	}
	gremlinTraversalSeq, err := tf.traversalParser.Parse(strings.NewReader(query), true)
	if err != nil {
		return nil, err
	}
	res, err := gremlinTraversalSeq.Exec()
	if err != nil {
		return nil, err
	}
	return res.Values(), nil
}
