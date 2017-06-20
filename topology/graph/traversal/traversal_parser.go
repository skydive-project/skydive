/*
 * Copyright (C) 2016 Red Hat, Inc.
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
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/topology/graph"
)

type (
	GremlinTraversalSequence struct {
		GraphTraversal *GraphTraversal
		steps          []GremlinTraversalStep
		extensions     []GremlinTraversalExtension
	}

	GremlinTraversalStep interface {
		Exec(last GraphTraversalStep) (GraphTraversalStep, error)
		Reduce(previous GremlinTraversalStep) GremlinTraversalStep
		Context() *GremlinTraversalContext
	}

	GremlinTraversalContext struct {
		StepContext GraphStepContext
		Params      []interface{}
	}

	// built in steps
	GremlinTraversalStepG struct {
		GremlinTraversalContext
	}
	GremlinTraversalStepV struct {
		GremlinTraversalContext
	}
	GremlinTraversalStepE struct {
		GremlinTraversalContext
	}
	GremlinTraversalStepContext struct {
		GremlinTraversalContext
	}
	GremlinTraversalStepOut struct {
		GremlinTraversalContext
	}
	GremlinTraversalStepIn struct {
		GremlinTraversalContext
	}
	GremlinTraversalStepOutV struct {
		GremlinTraversalContext
	}
	GremlinTraversalStepInV struct {
		GremlinTraversalContext
	}
	GremlinTraversalStepOutE struct {
		GremlinTraversalContext
	}
	GremlinTraversalStepInE struct {
		GremlinTraversalContext
	}
	GremlinTraversalStepBothE struct {
		GremlinTraversalContext
	}
	GremlinTraversalStepDedup struct {
		GremlinTraversalContext
	}
	GremlinTraversalStepHas struct {
		GremlinTraversalContext
	}
	GremlinTraversalStepHasKey struct {
		GremlinTraversalContext
	}
	GremlinTraversalStepHasNot struct {
		GremlinTraversalContext
	}
	GremlinTraversalStepShortestPathTo struct {
		GremlinTraversalContext
	}
	GremlinTraversalStepBoth struct {
		GremlinTraversalContext
	}
	GremlinTraversalStepCount struct {
		GremlinTraversalContext
	}
	GremlinTraversalStepRange struct {
		GremlinTraversalContext
	}
	GremlinTraversalStepLimit struct {
		GremlinTraversalContext
	}
	GremlinTraversalStepSort struct {
		GremlinTraversalContext
	}
	GremlinTraversalStepValues struct {
		GremlinTraversalContext
	}
	GremlinTraversalStepKeys struct {
		GremlinTraversalContext
	}
	GremlinTraversalStepSum struct {
		GremlinTraversalContext
	}
	GremlinTraversalStepMetrics struct {
		GremlinTraversalContext
	}
)

var (
	ExecutionError error = errors.New("Error while executing the query")
)

type GremlinTraversalParser struct {
	sync.RWMutex
	Graph   *graph.Graph
	scanner *GremlinTraversalScanner
	buf     struct {
		tok Token
		lit string
		n   int
	}
	extensions []GremlinTraversalExtension
}

func invokeStepFnc(last GraphTraversalStep, name string, gremlinStep GremlinTraversalStep) (GraphTraversalStep, error) {
	if v := reflect.ValueOf(last).MethodByName(name); v.IsValid() && !v.IsNil() {
		context := gremlinStep.Context()
		inputs := make([]reflect.Value, len(context.Params))
		for i, param := range context.Params {
			inputs[i] = reflect.ValueOf(param)
		}
		r := v.Call(inputs)
		step := r[0].Interface().(GraphTraversalStep)

		return step, step.Error()
	}

	return nil, fmt.Errorf("Invalid step '%s' on '%s'", name, reflect.TypeOf(last))
}

func (p *GremlinTraversalContext) ReduceRange(next GremlinTraversalStep) bool {
	if p.StepContext.PaginationRange != nil {
		return false
	}

	if rangeStep, ok := next.(*GremlinTraversalStepRange); ok {
		p.StepContext.PaginationRange = &GraphTraversalRange{rangeStep.Params[0].(int64), rangeStep.Params[1].(int64)}
	} else if limitStep, ok := next.(*GremlinTraversalStepLimit); ok {
		p.StepContext.PaginationRange = &GraphTraversalRange{0, limitStep.Params[0].(int64)}
	}

	return p.StepContext.PaginationRange != nil
}

func (p *GremlinTraversalContext) Context() *GremlinTraversalContext {
	return p
}

func (s *GremlinTraversalStepG) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	return nil, nil
}

func (s *GremlinTraversalStepG) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	return next
}

func (s *GremlinTraversalStepV) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	g, ok := last.(*GraphTraversal)
	if !ok {
		return nil, ExecutionError
	}

	g.currentStepContext = s.StepContext

	return g.V(s.Params...), nil
}

func (s *GremlinTraversalStepV) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	if s.ReduceRange(next) {
		return s
	}

	if hasStep, ok := next.(*GremlinTraversalStepHas); ok && len(s.Params) == 0 && len(hasStep.Params) >= 2 {
		s.Params = hasStep.Params
		return s
	}

	return next
}

func (s *GremlinTraversalStepE) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	g, ok := last.(*GraphTraversal)
	if !ok {
		return nil, ExecutionError
	}

	g.currentStepContext = s.StepContext

	return g.E(s.Params...), nil
}

func (s *GremlinTraversalStepE) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	if s.ReduceRange(next) {
		return s
	}

	if hasStep, ok := next.(*GremlinTraversalStepHas); ok && len(s.Params) == 0 && len(hasStep.Params) >= 2 {
		s.Params = hasStep.Params
		return s
	}

	return next
}

func (s *GremlinTraversalStepContext) Exec(last GraphTraversalStep) (_ GraphTraversalStep, err error) {
	g, ok := last.(*GraphTraversal)
	if !ok {
		return nil, ExecutionError
	}

	switch len(s.Params) {
	case 0:
		return nil, errors.New("At least one parameter must be provided to 'Context'")
	case 2:
		switch param := s.Params[1].(type) {
		case string:
			if s.Params[1], err = time.ParseDuration(param); err != nil {
				return nil, err
			}
		case int64:
			s.Params[1] = time.Duration(param) * time.Second
		default:
			return nil, errors.New("Key must be either an integer or a string")
		}
		fallthrough
	case 1:
		switch param := s.Params[0].(type) {
		case string:
			if s.Params[0], err = parseTimeContext(param); err != nil {
				return nil, err
			}
		case int64:
			if param > math.MaxInt32 {
				s.Params[0] = time.Unix(0, param*1000000)
			} else {
				s.Params[0] = time.Unix(param, 0)
			}
		default:
			return nil, errors.New("Key must be either an integer or a string")
		}
	default:
		return nil, errors.New("At most two parameters must be provided")
	}

	return g.Context(s.Params...), nil
}

func (s *GremlinTraversalStepContext) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	return next
}

func (s *GremlinTraversalStepHas) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalV:
		return last.(*GraphTraversalV).Has(s.Params...), nil
	case *GraphTraversalE:
		return last.(*GraphTraversalE).Has(s.Params...), nil
	}

	return invokeStepFnc(last, "Has", s)
}

func (s *GremlinTraversalStepHas) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	if s.ReduceRange(next) {
		return s
	}

	return next
}

func (s *GremlinTraversalStepHasKey) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalV:
		return last.(*GraphTraversalV).HasKey(s.Params[0].(string)), nil
	case *GraphTraversalE:
		return last.(*GraphTraversalE).HasKey(s.Params[0].(string)), nil
	}

	return invokeStepFnc(last, "HasKey", s)
}

func (s *GremlinTraversalStepHasKey) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	if s.ReduceRange(next) {
		return s
	}

	return next
}

func (s *GremlinTraversalStepHasNot) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalV:
		return last.(*GraphTraversalV).HasNot(s.Params[0].(string)), nil
	case *GraphTraversalE:
		return last.(*GraphTraversalE).HasNot(s.Params[0].(string)), nil
	}

	return invokeStepFnc(last, "HasNot", s)
}

func (s *GremlinTraversalStepHasNot) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	if s.ReduceRange(next) {
		return s
	}

	return next
}

func (s *GremlinTraversalStepDedup) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch g := last.(type) {
	case *GraphTraversalV:
		g.GraphTraversal.currentStepContext = s.StepContext
		return last.(*GraphTraversalV).Dedup(s.Params...), nil
	case *GraphTraversalE:
		g.GraphTraversal.currentStepContext = s.StepContext
		return last.(*GraphTraversalE).Dedup(s.Params...), nil
	}

	// fallback to reflection way
	return invokeStepFnc(last, "Dedup", s)
}

func (s *GremlinTraversalStepDedup) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	if s.ReduceRange(next) {
		return s
	}

	return next
}

func (s *GremlinTraversalStepOut) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalV:
		return last.(*GraphTraversalV).Out(s.Params...), nil
	}

	// fallback to reflection way
	return invokeStepFnc(last, "Out", s)
}

func (s *GremlinTraversalStepOut) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	if hasStep, ok := next.(*GremlinTraversalStepHas); ok && len(s.Params) == 0 {
		s.Params = hasStep.Params
		return s
	}

	if s.ReduceRange(next) {
		return s
	}

	return next
}

func (s *GremlinTraversalStepIn) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalV:
		return last.(*GraphTraversalV).In(s.Params...), nil
	}

	return invokeStepFnc(last, "In", s)
}

func (s *GremlinTraversalStepIn) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	if hasStep, ok := next.(*GremlinTraversalStepHas); ok && len(s.Params) == 0 {
		s.Params = hasStep.Params
		return s
	}

	if s.ReduceRange(next) {
		return s
	}

	return next
}

func (s *GremlinTraversalStepOutV) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalE:
		return last.(*GraphTraversalE).OutV(s.Params...), nil
	}

	return nil, ExecutionError
}

func (s *GremlinTraversalStepOutV) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	if hasStep, ok := next.(*GremlinTraversalStepHas); ok && len(s.Params) == 0 {
		s.Params = hasStep.Params
		return s
	}

	if s.ReduceRange(next) {
		return s
	}

	return next
}

func (s *GremlinTraversalStepInV) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalE:
		return last.(*GraphTraversalE).InV(s.Params...), nil
	}

	return nil, ExecutionError
}

func (s *GremlinTraversalStepInV) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	if hasStep, ok := next.(*GremlinTraversalStepHas); ok && len(s.Params) == 0 {
		s.Params = hasStep.Params
		return s
	}

	if s.ReduceRange(next) {
		return s
	}

	return next
}

func (s *GremlinTraversalStepOutE) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalV:
		return last.(*GraphTraversalV).OutE(s.Params...), nil
	}

	return nil, ExecutionError
}

func (s *GremlinTraversalStepOutE) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	if hasStep, ok := next.(*GremlinTraversalStepHas); ok && len(s.Params) == 0 {
		s.Params = hasStep.Params
		return s
	}

	if s.ReduceRange(next) {
		return s
	}

	return next
}

func (s *GremlinTraversalStepInE) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalV:
		return last.(*GraphTraversalV).InE(s.Params...), nil
	}

	return nil, ExecutionError
}

func (s *GremlinTraversalStepInE) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	if hasStep, ok := next.(*GremlinTraversalStepHas); ok && len(s.Params) == 0 {
		s.Params = hasStep.Params
		return s
	}

	if s.ReduceRange(next) {
		return s
	}

	return next
}

func (s *GremlinTraversalStepBothE) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalV:
		return last.(*GraphTraversalV).BothE(s.Params...), nil
	}

	return nil, ExecutionError
}

func (s *GremlinTraversalStepBothE) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	if hasStep, ok := next.(*GremlinTraversalStepHas); ok && len(s.Params) == 0 {
		s.Params = hasStep.Params
		return s
	}

	if s.ReduceRange(next) {
		return s
	}

	return next
}

func (s *GremlinTraversalStepShortestPathTo) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalV:
		if _, ok := s.Params[0].(graph.Metadata); !ok {
			return nil, ExecutionError
		}
		if len(s.Params) > 1 {
			if _, ok := s.Params[1].(graph.Metadata); !ok {
				return nil, ExecutionError
			}
			return last.(*GraphTraversalV).ShortestPathTo(s.Params[0].(graph.Metadata), s.Params[1].(graph.Metadata)), nil
		}
		return last.(*GraphTraversalV).ShortestPathTo(s.Params[0].(graph.Metadata), nil), nil
	}

	return nil, ExecutionError
}

func (s *GremlinTraversalStepShortestPathTo) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	return next
}

func (s *GremlinTraversalStepBoth) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalV:
		return last.(*GraphTraversalV).Both(s.Params...), nil
	}

	return invokeStepFnc(last, "Both", s)
}

func (s *GremlinTraversalStepBoth) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	if hasStep, ok := next.(*GremlinTraversalStepHas); ok && len(s.Params) == 0 {
		s.Params = hasStep.Params
		return s
	}

	if s.ReduceRange(next) {
		return s
	}

	return next
}

func (s *GremlinTraversalStepCount) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalV:
		return last.(*GraphTraversalV).Count(s.Params...), nil
	case *GraphTraversalE:
		return last.(*GraphTraversalE).Count(s.Params...), nil
	}

	return invokeStepFnc(last, "Count", s)
}

func (s *GremlinTraversalStepCount) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	return next
}

func (s *GremlinTraversalStepRange) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalV:
		return last.(*GraphTraversalV).Range(s.Params...), nil
	case *GraphTraversalE:
		return last.(*GraphTraversalE).Range(s.Params...), nil
	}

	return invokeStepFnc(last, "Range", s)
}

func (s *GremlinTraversalStepRange) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	return next
}

func (s *GremlinTraversalStepLimit) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalV:
		return last.(*GraphTraversalV).Limit(s.Params...), nil
	case *GraphTraversalE:
		return last.(*GraphTraversalE).Limit(s.Params...), nil
	}

	return invokeStepFnc(last, "Limit", s)
}

func (s *GremlinTraversalStepLimit) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	return next
}

func (s *GremlinTraversalStepSort) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalV:
		return last.(*GraphTraversalV).Sort(s.Params...), nil
	}

	return invokeStepFnc(last, "Sort", s)
}

func (s *GremlinTraversalStepSort) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	return next
}

func (s *GremlinTraversalStepValues) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	return invokeStepFnc(last, "PropertyValues", s)
}

func (s *GremlinTraversalStepValues) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	return next
}

func (s *GremlinTraversalStepKeys) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	return invokeStepFnc(last, "PropertyKeys", s)
}

func (s *GremlinTraversalStepKeys) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	return next
}

func (s *GremlinTraversalStepSum) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	return invokeStepFnc(last, "Sum", s)
}

func (s *GremlinTraversalStepSum) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	return next
}

func (s *GremlinTraversalStepMetrics) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	return invokeStepFnc(last, "Metrics", s)
}

func (s *GremlinTraversalStepMetrics) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	return next
}

func (s *GremlinTraversalSequence) Exec() (GraphTraversalStep, error) {
	var step GremlinTraversalStep
	var last GraphTraversalStep
	var err error

	last = s.GraphTraversal
	for i := 0; i < len(s.steps); {
		step = s.steps[i]

		for i = i + 1; i < len(s.steps); i = i + 1 {
			if next := step.Reduce(s.steps[i]); next != step {
				break
			}
		}

		if last, err = step.Exec(last); err != nil {
			return nil, err
		}

		if err := last.Error(); err != nil {
			return nil, err
		}
	}

	res, ok := last.(GraphTraversalStep)
	if !ok {
		return nil, ExecutionError
	}

	return res, nil
}

func (p *GremlinTraversalParser) AddTraversalExtension(e GremlinTraversalExtension) {
	p.extensions = append(p.extensions, e)
}

func NewGremlinTraversalParser(g *graph.Graph) *GremlinTraversalParser {
	return &GremlinTraversalParser{
		Graph: g,
	}
}

func (p *GremlinTraversalParser) parseStepParams() ([]interface{}, error) {
	tok, lit := p.scanIgnoreWhitespace()
	if tok != LEFT_PARENTHESIS {
		return nil, fmt.Errorf("Expected left parenthesis, got: %s", lit)
	}

	var params []interface{}
	for tok, lit := p.scanIgnoreWhitespace(); tok != RIGHT_PARENTHESIS; {
		switch tok {
		case EOF:
			return nil, errors.New("Expected right parenthesis")
		case COMMA:
		case NUMBER:
			if i, err := strconv.ParseInt(lit, 10, 64); err == nil {
				params = append(params, i)
			} else {
				if f, err := strconv.ParseFloat(lit, 64); err == nil {
					params = append(params, f)
				} else {
					return nil, fmt.Errorf("Expected number token, got: %s", lit)
				}
			}
		case STRING:
			params = append(params, lit)
		case METADATA:
			metadataParams, err := p.parseStepParams()
			if err != nil {
				return nil, err
			}
			metadata, err := SliceToMetadata(metadataParams...)
			if err != nil {
				return nil, err
			}
			params = append(params, metadata)
		case WITHIN:
			withParams, err := p.parseStepParams()
			if err != nil {
				return nil, err
			}
			params = append(params, Within(withParams...))
		case WITHOUT:
			withoutParams, err := p.parseStepParams()
			if err != nil {
				return nil, err
			}
			params = append(params, Without(withoutParams...))
		case LT:
			ltParams, err := p.parseStepParams()
			if err != nil {
				return nil, err
			}
			if len(ltParams) != 1 {
				return nil, fmt.Errorf("One parameter expected with LT: %v", ltParams)
			}
			params = append(params, Lt(ltParams[0]))
		case GT:
			gtParams, err := p.parseStepParams()
			if err != nil {
				return nil, err
			}
			if len(gtParams) != 1 {
				return nil, fmt.Errorf("One parameter expected with GT: %v", gtParams)
			}
			params = append(params, Gt(gtParams[0]))
		case LTE:
			lteParams, err := p.parseStepParams()
			if err != nil {
				return nil, err
			}
			if len(lteParams) != 1 {
				return nil, fmt.Errorf("One parameter expected with LTE: %v", lteParams)
			}
			params = append(params, Lte(lteParams[0]))
		case GTE:
			gteParams, err := p.parseStepParams()
			if err != nil {
				return nil, err
			}
			if len(gteParams) != 1 {
				return nil, fmt.Errorf("One parameter expected with GTE: %v", gteParams)
			}
			params = append(params, Gte(gteParams[0]))
		case INSIDE:
			insideParams, err := p.parseStepParams()
			if err != nil {
				return nil, err
			}
			if len(insideParams) != 2 {
				return nil, fmt.Errorf("Two parameters expected with INSIDE: %v", insideParams)
			}
			params = append(params, Inside(insideParams[0], insideParams[1]))
		case OUTSIDE:
			outsideParams, err := p.parseStepParams()
			if err != nil {
				return nil, err
			}
			if len(outsideParams) != 2 {
				return nil, fmt.Errorf("Two parameters expected with OUTSIDE: %v", outsideParams)
			}
			params = append(params, Outside(outsideParams[0], outsideParams[1]))
		case BETWEEN:
			betweenParams, err := p.parseStepParams()
			if err != nil {
				return nil, err
			}
			if len(betweenParams) != 2 {
				return nil, fmt.Errorf("Two parameters expected with BETWEEN: %v", betweenParams)
			}
			params = append(params, Between(betweenParams[0], betweenParams[1]))
		case NE:
			neParams, err := p.parseStepParams()
			if err != nil {
				return nil, err
			}
			if len(neParams) != 1 {
				return nil, fmt.Errorf("One parameter expected with NE: %v", neParams)
			}
			params = append(params, Ne(neParams[0]))
		case REGEX:
			regexParams, err := p.parseStepParams()
			if err != nil {
				return nil, err
			}
			if len(regexParams) != 1 {
				return nil, fmt.Errorf("One parameter expected with REGEX: %v", regexParams)
			}
			switch param := regexParams[0].(type) {
			case string:
				params = append(params, Regex(param))
			default:
				return nil, fmt.Errorf("REGEX predicate expects a string as parameter, got: %s", lit)
			}
		case ASC:
			params = append(params, common.SortAscending)
		case DESC:
			params = append(params, common.SortDescending)
		case CONTAINS:
			containsParams, err := p.parseStepParams()
			if err != nil {
				return nil, err
			}
			if len(containsParams) != 1 {
				return nil, fmt.Errorf("One parameter expected with CONTAINS: %v", containsParams)
			}
			params = append(params, Contains(containsParams[0]))
		default:
			return nil, fmt.Errorf("Unexpected token while parsing parameters, got: %s", lit)
		}
		tok, lit = p.scanIgnoreWhitespace()
	}

	return params, nil
}

func (p *GremlinTraversalParser) parserStep() (GremlinTraversalStep, error) {
	tok, lit := p.scanIgnoreWhitespace()
	if tok == IDENT {
		return nil, fmt.Errorf("Expected step function, got: %s", lit)
	}

	if tok == G {
		return &GremlinTraversalStepG{}, nil
	}

	params, err := p.parseStepParams()
	if err != nil {
		return nil, err
	}

	gremlinStepContext := GremlinTraversalContext{Params: params}

	// built in
	switch tok {
	case V:
		switch len(params) {
		case 0:
		case 1:
			if _, ok := params[0].(string); !ok {
				return nil, fmt.Errorf("V parameter must be a string")
			}
		default:
			return nil, fmt.Errorf("V accepts at most one parameter")
		}
		return &GremlinTraversalStepV{gremlinStepContext}, nil
	case E:
		switch len(params) {
		case 0:
		case 1:
			if _, ok := params[0].(string); !ok {
				return nil, fmt.Errorf("E parameter must be a string")
			}
		default:
			return nil, fmt.Errorf("E accepts at most one parameter")
		}
		return &GremlinTraversalStepE{gremlinStepContext}, nil
	case OUT:
		return &GremlinTraversalStepOut{gremlinStepContext}, nil
	case IN:
		return &GremlinTraversalStepIn{gremlinStepContext}, nil
	case OUTV:
		return &GremlinTraversalStepOutV{gremlinStepContext}, nil
	case INV:
		return &GremlinTraversalStepInV{gremlinStepContext}, nil
	case OUTE:
		return &GremlinTraversalStepOutE{gremlinStepContext}, nil
	case INE:
		return &GremlinTraversalStepInE{gremlinStepContext}, nil
	case BOTHE:
		return &GremlinTraversalStepBothE{gremlinStepContext}, nil
	case DEDUP:
		for _, param := range params {
			if _, ok := param.(string); !ok {
				return nil, fmt.Errorf("Dedup parameters have to be string keys")
			}
		}
		return &GremlinTraversalStepDedup{gremlinStepContext}, nil
	case HAS:
		return &GremlinTraversalStepHas{gremlinStepContext}, nil
	case HASNOT:
		switch len(params) {
		case 1:
			if _, ok := params[0].(string); ok {
				return &GremlinTraversalStepHasNot{gremlinStepContext}, nil
			}
			fallthrough
		default:
			return nil, fmt.Errorf("HasNot accepts only one parameter of type string")
		}
	case HASKEY:
		switch len(params) {
		case 1:
			if _, ok := params[0].(string); ok {
				return &GremlinTraversalStepHasKey{gremlinStepContext}, nil
			}
			fallthrough
		default:
			return nil, fmt.Errorf("HasKey accepts only one parameter of type string")
		}
	case SHORTESTPATHTO:
		if len(params) == 0 || len(params) > 2 {
			return nil, fmt.Errorf("ShortestPathTo predicate accepts only 1 or 2 parameters")
		}
		return &GremlinTraversalStepShortestPathTo{gremlinStepContext}, nil
	case BOTH:
		return &GremlinTraversalStepBoth{gremlinStepContext}, nil
	case CONTEXT:
		return &GremlinTraversalStepContext{gremlinStepContext}, nil
	case COUNT:
		if len(params) != 0 {
			return nil, fmt.Errorf("Count accepts no parameter")
		}
		return &GremlinTraversalStepCount{gremlinStepContext}, nil
	case SORT:
		switch len(params) {
		case 0:
			return &GremlinTraversalStepSort{gremlinStepContext}, nil
		case 1:
			if _, ok := params[0].(string); !ok {
				return nil, fmt.Errorf("Sort parameter has to be a string key")
			}
			gremlinStepContext.Params = []interface{}{common.SortAscending, params[0]}
			return &GremlinTraversalStepSort{gremlinStepContext}, nil
		case 2:
			if _, ok := params[0].(common.SortOrder); !ok {
				return nil, fmt.Errorf("Use ASC or DESC predicate")
			}
			if _, ok := params[1].(string); !ok {
				return nil, fmt.Errorf("Second sort parameter has to be a string key")
			}
			return &GremlinTraversalStepSort{gremlinStepContext}, nil
		default:
			return nil, fmt.Errorf("Sort accepts 1 predicate and 1 string parameter, got: %d", len(params))
		}
	case RANGE:
		if len(params) != 2 {
			return nil, fmt.Errorf("Range requires 2 parameters")
		}
		return &GremlinTraversalStepRange{gremlinStepContext}, nil
	case LIMIT:
		if len(params) != 1 {
			return nil, fmt.Errorf("Limit requires 1 parameter")
		}
		return &GremlinTraversalStepLimit{gremlinStepContext}, nil
	case VALUES:
		if len(params) != 1 {
			return nil, fmt.Errorf("Values requires 1 parameter")
		}
		if _, ok := params[0].(string); !ok {
			return nil, fmt.Errorf("Values parameter has to be a string key")
		}
		return &GremlinTraversalStepValues{gremlinStepContext}, nil
	case KEYS:
		return &GremlinTraversalStepKeys{gremlinStepContext}, nil
	case SUM:
		return &GremlinTraversalStepSum{gremlinStepContext}, nil
	case METRICS:
		return &GremlinTraversalStepMetrics{gremlinStepContext}, nil
	}

	// extensions
	for _, e := range p.extensions {
		step, err := e.ParseStep(tok, gremlinStepContext)
		if err != nil {
			return nil, err
		}
		if step != nil {
			return step, nil
		}
	}

	return nil, fmt.Errorf("Expected step function, got: %s", lit)
}

func (p *GremlinTraversalParser) Parse(r io.Reader, lockGraph bool) (*GremlinTraversalSequence, error) {
	p.Lock()
	defer p.Unlock()

	p.scanner = NewGremlinTraversalScanner(r, p.extensions)

	seq := &GremlinTraversalSequence{
		GraphTraversal: NewGraphTraversal(p.Graph, lockGraph),
		extensions:     p.extensions,
	}

	if tok, lit := p.scanIgnoreWhitespace(); tok != G {
		return nil, fmt.Errorf("found %q, expected `G`", lit)
	}

	// loop over all dot-delimited steps
	for {
		tok, lit := p.scanIgnoreWhitespace()
		if tok == EOF {
			break
		}

		if tok != DOT {
			return nil, fmt.Errorf("found %q, expected `.`", lit)
		}

		step, err := p.parserStep()
		if err != nil {
			return nil, err
		}
		seq.steps = append(seq.steps, step)
	}

	return seq, nil
}

func (p *GremlinTraversalParser) scan() (tok Token, lit string) {
	if p.buf.n != 0 {
		p.buf.n = 0
		return p.buf.tok, p.buf.lit
	}
	p.buf.tok, p.buf.lit = p.scanner.Scan()

	return p.buf.tok, p.buf.lit
}

func (p *GremlinTraversalParser) scanIgnoreWhitespace() (Token, string) {
	tok, lit := p.scan()
	for tok == WS {
		tok, lit = p.scan()
	}
	return tok, lit
}

func (p *GremlinTraversalParser) unscan() {
	p.buf.n = 1
}
