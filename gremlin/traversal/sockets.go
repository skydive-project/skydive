/*
 * Copyright (C) 2018 Red Hat, Inc.
 *
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 *
 */

package traversal

import (
	"encoding/json"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/filters"
	"github.com/skydive-project/skydive/topology/graph"
	"github.com/skydive-project/skydive/topology/graph/traversal"
	"github.com/skydive-project/skydive/topology/probes/socketinfo"
)

// SocketsTraversalExtension describes a new extension to enhance the topology
type SocketsTraversalExtension struct {
	SocketsToken traversal.Token
}

// SocketsGremlinTraversalStep describes the Sockets gremlin traversal step
type SocketsGremlinTraversalStep struct {
	traversal.GremlinTraversalContext
}

// NewSocketsTraversalExtension returns a new graph traversal extension
func NewSocketsTraversalExtension() *SocketsTraversalExtension {
	return &SocketsTraversalExtension{
		SocketsToken: traversalSocketsToken,
	}
}

// ScanIdent returns an associated graph token
func (e *SocketsTraversalExtension) ScanIdent(s string) (traversal.Token, bool) {
	switch s {
	case "SOCKETS":
		return e.SocketsToken, true
	}
	return traversal.IDENT, false
}

// ParseStep parse connections step
func (e *SocketsTraversalExtension) ParseStep(t traversal.Token, p traversal.GremlinTraversalContext) (traversal.GremlinTraversalStep, error) {
	switch t {
	case e.SocketsToken:
		return &SocketsGremlinTraversalStep{GremlinTraversalContext: p}, nil
	}
	return nil, nil
}

// Exec executes the metrics step
func (s *SocketsGremlinTraversalStep) Exec(last traversal.GraphTraversalStep) (traversal.GraphTraversalStep, error) {
	switch tv := last.(type) {
	case *traversal.GraphTraversalV:
		return Sockets(s.StepContext, tv), nil
	case *FlowTraversalStep:
		return tv.Sockets(s.StepContext), nil
	}
	return nil, traversal.ErrExecutionError
}

// Reduce flow step
func (s *SocketsGremlinTraversalStep) Reduce(next traversal.GremlinTraversalStep) traversal.GremlinTraversalStep {
	return next
}

// Context sockets step
func (s *SocketsGremlinTraversalStep) Context() *traversal.GremlinTraversalContext {
	return &s.GremlinTraversalContext
}

// SocketsTraversalStep connections step
type SocketsTraversalStep struct {
	GraphTraversal *traversal.GraphTraversal
	sockets        map[string][]*socketinfo.ConnectionInfo
	error          error
}

// PropertyValues returns a flow field value
func (s *SocketsTraversalStep) PropertyValues(ctx traversal.StepContext, keys ...interface{}) *traversal.GraphTraversalValue {
	if s.error != nil {
		return traversal.NewGraphTraversalValueFromError(s.error)
	}

	key := keys[0].(string)
	var values []interface{}
	for _, sockets := range s.sockets {
		for _, socket := range sockets {
			v, err := socket.GetField(key)
			if err != nil {
				return traversal.NewGraphTraversalValueFromError(common.ErrFieldNotFound)
			}
			values = append(values, v)
		}
	}

	return traversal.NewGraphTraversalValue(s.GraphTraversal, values)
}

// Has step
func (s *SocketsTraversalStep) Has(ctx traversal.StepContext, params ...interface{}) *SocketsTraversalStep {
	if s.error != nil {
		return s
	}

	filter, err := paramsToFilter(params...)
	if err != nil {
		return &SocketsTraversalStep{error: err}
	}

	s.GraphTraversal.RLock()
	defer s.GraphTraversal.RUnlock()

	flowSockets := make(map[string][]*socketinfo.ConnectionInfo)
	for id, sockets := range s.sockets {
		var socks []*socketinfo.ConnectionInfo
		for _, socket := range sockets {
			if filter == nil || filter.Eval(socket) {
				socks = append(socks, socket)
			}
		}
		if len(socks) > 0 {
			flowSockets[id] = socks
		}
	}

	return &SocketsTraversalStep{GraphTraversal: s.GraphTraversal, sockets: flowSockets}
}

// Values returns list of socket informations
func (s *SocketsTraversalStep) Values() []interface{} {
	if len(s.sockets) == 0 {
		return []interface{}{}
	}
	return []interface{}{s.sockets}
}

// MarshalJSON serialize in JSON
func (s *SocketsTraversalStep) MarshalJSON() ([]byte, error) {
	values := s.Values()
	s.GraphTraversal.RLock()
	defer s.GraphTraversal.RUnlock()
	return json.Marshal(values)
}

// Error returns traversal error
func (s *SocketsTraversalStep) Error() error {
	return s.error
}

func getSockets(n *graph.Node) (sockets []*socketinfo.ConnectionInfo) {
	if socks, err := n.GetField("Sockets"); err == nil {
		for _, socket := range socks.([]interface{}) {
			var conn socketinfo.ConnectionInfo
			if err = conn.Decode(socket); err == nil {
				sockets = append(sockets, &conn)
			}
		}
	}
	return
}

// NewSocketIndexer returns a new socket graph indexer
func NewSocketIndexer(g *graph.Graph) *graph.Indexer {
	hashNode := func(n *graph.Node) map[string]interface{} {
		sockets := getSockets(n)
		kv := make(map[string]interface{}, len(sockets))
		for _, socket := range sockets {
			kv[socket.Hash()] = socket
		}
		return kv
	}

	graphIndexer := graph.NewIndexer(g, g, hashNode, true)
	socketFilter := graph.NewElementFilter(filters.NewNotNullFilter("Sockets"))
	for _, node := range g.GetNodes(socketFilter) {
		graphIndexer.OnNodeAdded(node)
	}
	return graphIndexer
}
