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

	layers "github.com/google/gopacket/layers"

	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
)

// RawPacketsTraversalExtension describes a new extension to enhance the topology
type RawPacketsTraversalExtension struct {
	RawPacketsToken traversal.Token
}

// RawPacketsGremlinTraversalStep rawpackets step
type RawPacketsGremlinTraversalStep struct {
	traversal.GremlinTraversalContext
}

// RawPacketsTraversalStep rawpackets step
type RawPacketsTraversalStep struct {
	GraphTraversal *traversal.GraphTraversal
	rawPackets     map[string]*flow.RawPackets
	error          error
}

// NewRawPacketsTraversalExtension returns a new graph traversal extension
func NewRawPacketsTraversalExtension() *RawPacketsTraversalExtension {
	return &RawPacketsTraversalExtension{
		RawPacketsToken: traversalRawPacketsToken,
	}
}

// ScanIdent returns an associated graph token
func (e *RawPacketsTraversalExtension) ScanIdent(s string) (traversal.Token, bool) {
	switch s {
	case "RAWPACKETS":
		return e.RawPacketsToken, true
	}
	return traversal.IDENT, false
}

// ParseStep parse metrics step
func (e *RawPacketsTraversalExtension) ParseStep(t traversal.Token, p traversal.GremlinTraversalContext) (traversal.GremlinTraversalStep, error) {
	switch t {
	case e.RawPacketsToken:
		return &RawPacketsGremlinTraversalStep{GremlinTraversalContext: p}, nil
	}
	return nil, nil
}

// Exec RawPackets step
func (r *RawPacketsGremlinTraversalStep) Exec(last traversal.GraphTraversalStep) (traversal.GraphTraversalStep, error) {
	switch last.(type) {
	case *FlowTraversalStep:
		fs := last.(*FlowTraversalStep)
		return fs.RawPackets(r.StepContext), nil
	}

	return nil, traversal.ErrExecutionError
}

// Reduce RawPackets step
func (r *RawPacketsGremlinTraversalStep) Reduce(next traversal.GremlinTraversalStep) (traversal.GremlinTraversalStep, error) {
	return next, nil
}

// Context RawPackets step
func (r *RawPacketsGremlinTraversalStep) Context() *traversal.GremlinTraversalContext {
	return &r.GremlinTraversalContext
}

// Values returns list of raw packets
func (r *RawPacketsTraversalStep) Values() []interface{} {
	if len(r.rawPackets) == 0 {
		return []interface{}{}
	}
	return []interface{}{r.rawPackets}
}

// MarshalJSON serialize in JSON
func (r *RawPacketsTraversalStep) MarshalJSON() ([]byte, error) {
	values := r.Values()
	r.GraphTraversal.RLock()
	defer r.GraphTraversal.RUnlock()
	return json.Marshal(values)
}

// Error returns traversal error
func (r *RawPacketsTraversalStep) Error() error {
	return r.error
}

// BPF returns only the raw packets that matches the specified BPF filter
func (r *RawPacketsTraversalStep) BPF(ctx traversal.StepContext, s ...interface{}) *RawPacketsTraversalStep {
	if r.error != nil {
		return &RawPacketsTraversalStep{error: r.error}
	}

	if len(s) != 1 {
		return &RawPacketsTraversalStep{error: errors.New("BPF requires 1 parameter")}
	}

	filter, ok := s[0].(string)
	if !ok {
		return &RawPacketsTraversalStep{error: errors.New("BPF parameter has to be a string")}
	}

	// While very improbable, we may have different link types so we keep
	// a map of BPF filters for the link types
	bpfFilters := make(map[layers.LinkType]*flow.BPF)
	rawPackets := make(map[string]*flow.RawPackets)
	for key, value := range r.rawPackets {
		var err error
		bpf, ok := bpfFilters[value.LinkType]
		if !ok {
			bpf, err = flow.NewBPF(value.LinkType, flow.DefaultCaptureLength, filter)
			if err != nil {
				return &RawPacketsTraversalStep{error: err}
			}
			bpfFilters[value.LinkType] = bpf
		}

		var filteredPackets []*flow.RawPacket
		for _, packet := range value.RawPackets {
			if bpf.Matches(packet.Data) {
				filteredPackets = append(filteredPackets, packet)
			}
		}
		rawPackets[key] = &flow.RawPackets{
			LinkType:   value.LinkType,
			RawPackets: filteredPackets,
		}
	}

	return &RawPacketsTraversalStep{GraphTraversal: r.GraphTraversal, rawPackets: rawPackets}
}
