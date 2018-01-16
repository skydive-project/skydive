// +build !linux

package probes

import (
	"errors"
	"net"
	"sync"

	"github.com/skydive-project/skydive/topology/graph"
)

var (
	ErrNotImplemented = errors.New("not implemented")
)

// NetNsNetLinkProbe describes a topology probe based on netlink in a network namespace
type NetNsNetLinkProbe struct {
	sync.RWMutex
	Graph  *graph.Graph
	Root   *graph.Node
	NsPath string
}

// NetLinkProbe describes a list NetLink NameSpace probe to enhance the graph
type NetLinkProbe struct {
	sync.RWMutex
	Graph *graph.Graph
}

// RouteTable describes a list of Routes
type RoutingTable struct {
	Id     int
	Src    net.IP `json:"Src,omitempty"`
	Routes []Route
}

// Route describes a route
type Route struct {
	Prefix   string    `json:"Prefix,omitempty"`
	Nexthops []NextHop `json:"Nexthops,omitempty"`
}

// NextHop describes a next hop
type NextHop struct {
	Priority int    `json:"Priority,omitempty"`
	Ip       net.IP `json:"Ip,omitempty"`
}

func NewNetLinkProbe(g *graph.Graph) (*NetLinkProbe, error) {
	return nil, ErrNotImplemented
}

func (u *NetLinkProbe) Start() {
}

func (u *NetLinkProbe) Stop() {
}

func (u *NetLinkProbe) Register(nsPath string, root *graph.Node) (*NetNsNetLinkProbe, error) {
	return nil, ErrNotImplemented
}
