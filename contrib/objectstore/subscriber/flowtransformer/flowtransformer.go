package flowtransformer

import (
	"fmt"
	"github.com/skydive-project/skydive/contrib/objectstore/subscriber/flowtransformer/custom1"
	"github.com/skydive-project/skydive/flow"
)

// FlowTransformer allows generic transformations of a flow
type FlowTransformer interface {
	// Transform transforms a flow before being stored
	Transform(flow *flow.Flow) interface{}
}

// New creates a new flow transformer based on a name string
func New(flowTransformerName string) (FlowTransformer, error) {
	switch flowTransformerName {
	case "custom1":
		return custom1.New(), nil
	case "":
		return nil, nil
	default:
		return nil, fmt.Errorf("Marshaller '%s' is not supported", flowTransformerName)
	}
}
