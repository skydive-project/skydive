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

	"github.com/skydive-project/skydive/graffiti/graph/traversal"
	"github.com/skydive-project/skydive/topology"
)

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

// Exec NextHop step
func (nh *NextHopGremlinTraversalStep) Exec(last traversal.GraphTraversalStep) (traversal.GraphTraversalStep, error) {
	switch tv := last.(type) {
	case *traversal.GraphTraversalV:
		nextHops := make(map[string]*topology.NextHop)
		tv.GraphTraversal.RLock()
		for _, node := range tv.GetNodes() {
			if nextHop, err := topology.GetNextHop(node, nh.ip); err == nil {
				nextHops[string(node.ID)] = nextHop
			}
		}
		tv.GraphTraversal.RUnlock()

		return NewNextHopTraversalStep(tv.GraphTraversal, nextHops), nil
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
	nexthops       map[string]*topology.NextHop
	error          error
}

// NewNextHopTraversalStep creates a new traversal nexthop step
func NewNextHopTraversalStep(gt *traversal.GraphTraversal, value map[string]*topology.NextHop) *NextHopTraversalStep {
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
