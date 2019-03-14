package custom1

import (
	"time"

	"github.com/pmylund/go-cache"

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
	seenFlows *cache.Cache
}

// Transform transforms a flow before being stored
func (m *FlowTransformer) Transform(f *flow.Flow) interface{} {
	// do not report new flows (i.e. the first time you see them)
	if f.FinishType != flow.FlowFinishType_TIMEOUT {
		_, seen := m.seenFlows.Get(f.UUID)
		if f.FinishType == flow.FlowFinishType_NOT_FINISHED {
			m.seenFlows.Set(f.UUID, true, cache.DefaultExpiration)
			if !seen {
				return nil
			}
		} else {
			m.seenFlows.Set(f.UUID, true, time.Minute)
		}
	} else {
		m.seenFlows.Delete(f.UUID)
	}
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

// New returns a new FlowTransformer
func New() *FlowTransformer {
	return &FlowTransformer{
		seenFlows: cache.New(10*time.Minute, 10*time.Minute),
	}
}
