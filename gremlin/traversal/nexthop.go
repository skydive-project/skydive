/*
 * Copyright (C) 2018 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy ofthe License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specificlanguage governing permissions and
 * limitations under the License.
 *
 */

package traversal

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
	"github.com/skydive-project/skydive/topology/probes/netlink"
)

//NextHop a nexthop of IP
type NextHop struct {
	IP      string
	MAC     string
	IfIndex int64
}

// NextHopTraversalExtension describes a new extension to enhance the topology
type NextHopTraversalExtension struct {
	NextHopToken traversal.Token
}

// NextHopGremlinTraversalStep nexthops step
type NextHopGremlinTraversalStep struct {
	context traversal.GremlinTraversalContext
	ip      net.IP
}

// NewNextHopTraversalExtension returns a new graph traversal extension
func NewNextHopTraversalExtension() *NextHopTraversalExtension {
	return &NextHopTraversalExtension{
		NextHopToken: traversalNextHopToken,
	}
}

// ScanIdent returns an associated graph token
func (e *NextHopTraversalExtension) ScanIdent(s string) (traversal.Token, bool) {
	switch s {
	case "NEXTHOP":
		return e.NextHopToken, true
	}
	return traversal.IDENT, false
}

// ParseStep parses nexthops step
func (e *NextHopTraversalExtension) ParseStep(t traversal.Token, p traversal.GremlinTraversalContext) (traversal.GremlinTraversalStep, error) {
	switch t {
	case e.NextHopToken:
	default:
		return nil, nil
	}

	if len(p.Params) != 1 {
		return nil, fmt.Errorf("NextHop accepts one parameter : %v", p.Params)
	}

	ipStr, ok := p.Params[0].(string)
	if !ok {
		return nil, errors.New("NextHop parameter have to be a string")
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return nil, errors.New("NextHop parameter have to be a valid IP address")
	}

	return &NextHopGremlinTraversalStep{context: p, ip: ip}, nil
}

func getNextHops(nodes []*graph.Node, ip net.IP) map[string]*NextHop {
	nexthops := map[string]*NextHop{}

nodeloop:
	for _, node := range nodes {
		neighbors, err := node.GetField("Neighbors")

		tables, err := node.GetField("RoutingTables")
		if err != nil {
			continue
		}
		rts := tables.(*netlink.RoutingTables)
		for _, t := range *rts {
			var defaultRouteIP net.IP
			var defaultIfIndex int64
			for _, r := range t.Routes {
				ipnet := net.IPNet(r.Prefix)
				if r.Prefix.IsDefaultRoute() {
					defaultRouteIP = r.NextHops[0].IP
					defaultIfIndex = r.NextHops[0].IfIndex
				} else if ipnet.Contains(ip) {
					nextIP := r.NextHops[0].IP
					if nextIP == nil {
						nexthops[string(node.ID)] = &NextHop{IfIndex: r.NextHops[0].IfIndex}
						break nodeloop
					}
					nh := &NextHop{IP: nextIP.String(), IfIndex: r.NextHops[0].IfIndex}
					if neighbors != nil {
						nh.MAC = getMAC(neighbors.(*netlink.Neighbors), nextIP)
					}
					nexthops[string(node.ID)] = nh

					break nodeloop
				}
			}
			if defaultRouteIP != nil {
				nh := &NextHop{IP: defaultRouteIP.String(), IfIndex: defaultIfIndex}
				if neighbors != nil {
					nh.MAC = getMAC(neighbors.(*netlink.Neighbors), defaultRouteIP)
				}
				nexthops[string(node.ID)] = nh
			}
		}
	}
	return nexthops
}

func getMAC(neighbors *netlink.Neighbors, ip net.IP) string {
	for _, n := range *neighbors {
		if n.IP.Equal(ip) {
			return n.MAC
		}
	}
	return ""
}

// Exec NextHop step
func (nh *NextHopGremlinTraversalStep) Exec(last traversal.GraphTraversalStep) (traversal.GraphTraversalStep, error) {

	switch tv := last.(type) {
	case *traversal.GraphTraversalV:
		tv.GraphTraversal.RLock()
		nhs := getNextHops(tv.GetNodes(), nh.ip)
		tv.GraphTraversal.RUnlock()

		return NewNextHopTraversalStep(tv.GraphTraversal, nhs), nil
	}
	return nil, traversal.ErrExecutionError
}

// Reduce nexthop step
func (nh *NextHopGremlinTraversalStep) Reduce(next traversal.GremlinTraversalStep) (traversal.GremlinTraversalStep, error) {
	return next, nil
}

// Context NextHop step
func (nh *NextHopGremlinTraversalStep) Context() *traversal.GremlinTraversalContext {
	return &nh.context
}

// NextHopTraversalStep traversal step of nexthop
type NextHopTraversalStep struct {
	GraphTraversal *traversal.GraphTraversal
	nexthops       map[string]*NextHop
	error          error
}

// NewNextHopTraversalStep creates a new traversal nexthop step
func NewNextHopTraversalStep(gt *traversal.GraphTraversal, value map[string]*NextHop) *NextHopTraversalStep {
	tv := &NextHopTraversalStep{
		GraphTraversal: gt,
		nexthops:       value,
	}

	return tv
}

// NewNextHopTraversalStepFromError creates a new traversal nexthop step
func NewNextHopTraversalStepFromError(err ...error) *NextHopTraversalStep {
	tv := &NextHopTraversalStep{}

	if len(err) > 0 {
		tv.error = err[0]
	}

	return tv
}

// Values return the nexthops
func (t *NextHopTraversalStep) Values() []interface{} {
	if len(t.nexthops) == 0 {
		return []interface{}{}
	}
	return []interface{}{t.nexthops}
}

// MarshalJSON serialize in JSON
func (t *NextHopTraversalStep) MarshalJSON() ([]byte, error) {
	values := t.Values()
	t.GraphTraversal.RLock()
	defer t.GraphTraversal.RUnlock()
	return json.Marshal(values)
}

func (t *NextHopTraversalStep) Error() error {
	return t.error
}
