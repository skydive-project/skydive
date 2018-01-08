// +build !linux

package probes

import (
	"sync"

	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"
)

// NetNSProbe describes a netlink probe in a network namespace
type NetNSProbe struct {
	sync.RWMutex
	Graph        *graph.Graph
	Root         *graph.Node
	NetLinkProbe *NetLinkProbe
}

// NetNs describes a network namespace path associated with a device / inode
type NetNs struct {
}

// Register a new network namespace path
func (u *NetNSProbe) Register(path string, name string) *graph.Node {
	logging.GetLogger().Errorf("Register Network Namespace unimplemented: %s", path)
	return nil
}

// Unregister a network namespace path
func (u *NetNSProbe) Unregister(path string) {
}

func NewNetNSProbe(g *graph.Graph, n *graph.Node, nlProbe *NetLinkProbe) (*NetNSProbe, error) {
	return nil, ErrNotImplemented
}

func (u *NetNSProbe) Watch(path string) {
}

func (u *NetNSProbe) Start() {
}

func (u *NetNSProbe) Stop() {
}
