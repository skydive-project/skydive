// +build !libvirt

/*
 * This topology probe is a faked one probe to replace real libvirt
 *probe if user doesnt want to compile with libvirt support
 */

package libvirt

import (
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/topology/graph"
)

// LibVirtProbe describes a LibVirt topology graph that enhance the graph
type LibVirtProbe struct {
	Graph *graph.Graph
	Root  *graph.Node
}

// Start the probe
func (probe *LibVirtProbe) Start() {
}

// Stop the probe
func (probe *LibVirtProbe) Stop() {
}

// NewDProbe creates a new topology libvirt probe
func NewProbe(g *graph.Graph, root *graph.Node) (*LibVirtProbe, error) {
	return nil, common.ErrNotImplemented
}
