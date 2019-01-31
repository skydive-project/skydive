package custom1

import (
	"github.com/skydive-project/skydive/flow"
)

// Flow represents a transformed flow
type Flow struct {
	UUID             string
	LayersPath       string
	Network          *flow.FlowLayer
	Transport        *flow.TransportLayer
	LastUpdateMetric *flow.FlowMetric
	Metric           *flow.FlowMetric
	Start            int64
	Last             int64
	FinishType       flow.FlowFinishType
}

// FlowTransformer is a custom transformer for flows
type FlowTransformer struct {
}

// Transform transforms a flow before being stored
func (m *FlowTransformer) Transform(f *flow.Flow) interface{} {
	return &Flow{
		UUID:             f.UUID,
		LayersPath:       f.LayersPath,
		Network:          f.Network,
		Transport:        f.Transport,
		LastUpdateMetric: f.LastUpdateMetric,
		Metric:           f.Metric,
		Start:            f.Start,
		Last:             f.Last,
		FinishType:       f.FinishType,
	}
}

// New returns a new Custom1Marshaller
func New() *FlowTransformer {
	return &FlowTransformer{}
}
