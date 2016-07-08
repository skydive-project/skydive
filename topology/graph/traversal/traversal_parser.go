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
	"reflect"
	"strconv"

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
	}
	GremlinTraversalStepParams []interface{}

	// built in steps
	gremlinTraversalStepG              struct{}
	gremlinTraversalStepV              struct{ params GremlinTraversalStepParams }
	gremlinTraversalStepE              struct{}
	gremlinTraversalStepContext        struct{ params GremlinTraversalStepParams }
	gremlinTraversalStepOut            struct{ params GremlinTraversalStepParams }
	gremlinTraversalStepIn             struct{ params GremlinTraversalStepParams }
	gremlinTraversalStepOutV           struct{ params GremlinTraversalStepParams }
	gremlinTraversalStepInV            struct{ params GremlinTraversalStepParams }
	gremlinTraversalStepOutE           struct{ params GremlinTraversalStepParams }
	gremlinTraversalStepInE            struct{ params GremlinTraversalStepParams }
	gremlinTraversalStepDedup          struct{}
	gremlinTraversalStepHas            struct{ params GremlinTraversalStepParams }
	gremlinTraversalStepShortestPathTo struct{ params GremlinTraversalStepParams }
	gremlinTraversalStepBoth           struct{ params GremlinTraversalStepParams }
)

var (
	ExecutionError error = errors.New("Error while executing the query")
)

type GremlinTraversalParser struct {
	Graph   *graph.Graph
	Reader  io.Reader
	scanner *GremlinTraversalScanner
	buf     struct {
		tok Token
		lit string
		n   int
	}
	extensions []GremlinTraversalExtension
}

func invokeStepFnc(last GraphTraversalStep, name string, params GremlinTraversalStepParams) (GraphTraversalStep, error) {
	if v := reflect.ValueOf(last).MethodByName(name); v.IsValid() && !v.IsNil() {
		inputs := make([]reflect.Value, len(params))
		for i, param := range params {
			inputs[i] = reflect.ValueOf(param)
		}
		r := v.Call(inputs)
		step := r[0].Interface().(GraphTraversalStep)

		return step, step.Error()
	}

	return nil, ExecutionError
}

func (s *gremlinTraversalStepG) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	return nil, nil
}

func (s *gremlinTraversalStepG) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	return next
}

func (s *gremlinTraversalStepV) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	g, ok := last.(*GraphTraversal)
	if !ok {
		return nil, ExecutionError
	}

	switch len(s.params) {
	case 1:
		if k, ok := s.params[0].(string); ok {
			return g.V(graph.Identifier(k)), nil
		}
		return nil, ExecutionError
	case 0:
		return g.V(), nil
	default:
		return nil, ExecutionError
	}
}

func (s *gremlinTraversalStepV) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	return next
}

func (s *gremlinTraversalStepContext) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	g, ok := last.(*GraphTraversal)
	if !ok {
		return nil, ExecutionError
	}

	if len(s.params) != 1 {
		return nil, ExecutionError
	}

	return g.Context(s.params...), nil
}

func (s *gremlinTraversalStepContext) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	return next
}

func (s *gremlinTraversalStepHas) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalV:
		return last.(*GraphTraversalV).Has(s.params...), nil
	case *GraphTraversalE:
		return last.(*GraphTraversalE).Has(s.params...), nil
	}

	return invokeStepFnc(last, "Has", s.params)
}

func (s *gremlinTraversalStepHas) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	return next
}

func (s *gremlinTraversalStepDedup) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalV:
		return last.(*GraphTraversalV).Dedup(), nil
	case *GraphTraversalE:
		return last.(*GraphTraversalE).Dedup(), nil
	}

	return nil, ExecutionError
}

func (s *gremlinTraversalStepDedup) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	return next
}

func (s *gremlinTraversalStepOut) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalV:
		return last.(*GraphTraversalV).Out(s.params...), nil
	}

	// fallback to reflection way
	return invokeStepFnc(last, "Out", s.params)
}

func (s *gremlinTraversalStepOut) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	if hasStep, ok := next.(*gremlinTraversalStepHas); ok && len(s.params) == 0 {
		s.params = hasStep.params
		return s
	}
	return next
}

func (s *gremlinTraversalStepIn) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalV:
		return last.(*GraphTraversalV).In(s.params...), nil
	}

	return invokeStepFnc(last, "In", s.params)
}

func (s *gremlinTraversalStepIn) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	if hasStep, ok := next.(*gremlinTraversalStepHas); ok && len(s.params) == 0 {
		s.params = hasStep.params
		return s
	}
	return next
}

func (s *gremlinTraversalStepOutV) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalE:
		return last.(*GraphTraversalE).OutV(s.params...), nil
	}

	return nil, ExecutionError
}

func (s *gremlinTraversalStepOutV) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	if hasStep, ok := next.(*gremlinTraversalStepHas); ok && len(s.params) == 0 {
		s.params = hasStep.params
		return s
	}
	return next
}

func (s *gremlinTraversalStepInV) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalE:
		return last.(*GraphTraversalE).InV(s.params...), nil
	}

	return nil, ExecutionError
}

func (s *gremlinTraversalStepInV) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	if hasStep, ok := next.(*gremlinTraversalStepHas); ok && len(s.params) == 0 {
		s.params = hasStep.params
		return s
	}
	return next
}

func (s *gremlinTraversalStepOutE) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalV:
		return last.(*GraphTraversalV).OutE(s.params...), nil
	}

	return nil, ExecutionError
}

func (s *gremlinTraversalStepOutE) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	if hasStep, ok := next.(*gremlinTraversalStepHas); ok && len(s.params) == 0 {
		s.params = hasStep.params
		return s
	}
	return next
}

func (s *gremlinTraversalStepInE) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalV:
		return last.(*GraphTraversalV).InE(s.params...), nil
	}

	return nil, ExecutionError
}

func (s *gremlinTraversalStepInE) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	if hasStep, ok := next.(*gremlinTraversalStepHas); ok && len(s.params) == 0 {
		s.params = hasStep.params
		return s
	}
	return next
}

func (s *gremlinTraversalStepShortestPathTo) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalV:
		if _, ok := s.params[0].(graph.Metadata); !ok {
			return nil, ExecutionError
		}
		if len(s.params) > 1 {
			if _, ok := s.params[1].(graph.Metadata); !ok {
				return nil, ExecutionError
			}
			return last.(*GraphTraversalV).ShortestPathTo(s.params[0].(graph.Metadata), s.params[1].(graph.Metadata)), nil
		}
		return last.(*GraphTraversalV).ShortestPathTo(s.params[0].(graph.Metadata)), nil
	}

	return nil, ExecutionError
}

func (s *gremlinTraversalStepShortestPathTo) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	return next
}

func (s *gremlinTraversalStepBoth) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalV:
		return last.(*GraphTraversalV).Both(s.params...), nil
	}

	return nil, ExecutionError
}

func (s *gremlinTraversalStepBoth) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	if hasStep, ok := next.(*gremlinTraversalStepHas); ok && len(s.params) == 0 {
		s.params = hasStep.params
		return s
	}
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

func NewGremlinTraversalParser(r io.Reader, g *graph.Graph) *GremlinTraversalParser {
	return &GremlinTraversalParser{
		Graph:  g,
		Reader: r,
	}
}

func (p *GremlinTraversalParser) parserStepParams() (GremlinTraversalStepParams, error) {
	tok, lit := p.scanIgnoreWhitespace()
	if tok != LEFT_PARENTHESIS {
		return nil, fmt.Errorf("Expected left parenthesis, got: %s", lit)
	}

	params := GremlinTraversalStepParams{}
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
			metadataParams, err := p.parserStepParams()
			if err != nil {
				return nil, err
			}
			metadata, err := SliceToMetadata(metadataParams...)
			if err != nil {
				return nil, err
			}
			params = append(params, metadata)
		case WITHIN:
			withParams, err := p.parserStepParams()
			if err != nil {
				return nil, err
			}
			params = append(params, Within(withParams...))
		case WITHOUT:
			withoutParams, err := p.parserStepParams()
			if err != nil {
				return nil, err
			}
			params = append(params, Without(withoutParams...))
		case LT:
			ltParams, err := p.parserStepParams()
			if err != nil {
				return nil, err
			}
			if len(ltParams) != 1 {
				return nil, fmt.Errorf("One parameter expected with LT: %v", ltParams)
			}
			params = append(params, Lt(ltParams[0]))
		case GT:
			gtParams, err := p.parserStepParams()
			if err != nil {
				return nil, err
			}
			if len(gtParams) != 1 {
				return nil, fmt.Errorf("One parameter expected with GT: %v", gtParams)
			}
			params = append(params, Gt(gtParams[0]))
		case LTE:
			lteParams, err := p.parserStepParams()
			if err != nil {
				return nil, err
			}
			if len(lteParams) != 1 {
				return nil, fmt.Errorf("One parameter expected with LTE: %v", lteParams)
			}
			params = append(params, Lte(lteParams[0]))
		case GTE:
			gteParams, err := p.parserStepParams()
			if err != nil {
				return nil, err
			}
			if len(gteParams) != 1 {
				return nil, fmt.Errorf("One parameter expected with GTE: %v", gteParams)
			}
			params = append(params, Gte(gteParams[0]))
		case INSIDE:
			insideParams, err := p.parserStepParams()
			if err != nil {
				return nil, err
			}
			if len(insideParams) != 2 {
				return nil, fmt.Errorf("Two parameters expected with INSIDE: %v", insideParams)
			}
			params = append(params, Inside(insideParams[0], insideParams[1]))
		case BETWEEN:
			betweenParams, err := p.parserStepParams()
			if err != nil {
				return nil, err
			}
			if len(betweenParams) != 2 {
				return nil, fmt.Errorf("Two parameters expected with BETWEEN: %v", betweenParams)
			}
			params = append(params, Between(betweenParams[0], betweenParams[1]))
		case NE:
			neParams, err := p.parserStepParams()
			if err != nil {
				return nil, err
			}
			if len(neParams) != 1 {
				return nil, fmt.Errorf("One parameter expected with NE: %v", neParams)
			}
			params = append(params, Ne(neParams[0]))
		case REGEX:
			regexParams, err := p.parserStepParams()
			if err != nil {
				return nil, err
			}
			if len(regexParams) != 1 {
				return nil, fmt.Errorf("One parameter expected with Regex: %v", regexParams)
			}
			switch param := regexParams[0].(type) {
			case string:
				params = append(params, Regex(param))
			default:
				return nil, fmt.Errorf("Regex predicate expect a string a parameter, got: %s", lit)
			}
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
		return &gremlinTraversalStepG{}, nil
	}

	params, err := p.parserStepParams()
	if err != nil {
		return nil, err
	}

	// built in
	switch tok {
	case V:
		return &gremlinTraversalStepV{params: params}, nil
	case OUT:
		return &gremlinTraversalStepOut{params: params}, nil
	case IN:
		return &gremlinTraversalStepIn{params: params}, nil
	case OUTV:
		return &gremlinTraversalStepOutV{params: params}, nil
	case INV:
		return &gremlinTraversalStepInV{params: params}, nil
	case OUTE:
		return &gremlinTraversalStepOutE{params: params}, nil
	case INE:
		return &gremlinTraversalStepInE{params: params}, nil
	case DEDUP:
		return &gremlinTraversalStepDedup{}, nil
	case HAS:
		return &gremlinTraversalStepHas{params: params}, nil
	case SHORTESTPATHTO:
		if len(params) == 0 || len(params) > 2 {
			return nil, fmt.Errorf("ShortestPathTo predicate accept only 1 or 2 parameters")
		}
		return &gremlinTraversalStepShortestPathTo{params: params}, nil
	case BOTH:
		return &gremlinTraversalStepBoth{params: params}, nil
	case CONTEXT:
		return &gremlinTraversalStepContext{params: params}, nil
	}

	// extensions
	for _, e := range p.extensions {
		step, err := e.ParseStep(tok, params)
		if err != nil {
			return nil, err
		}
		if step != nil {
			return step, nil
		}
	}

	return nil, fmt.Errorf("Expected step function, got: %s", lit)
}

func (p *GremlinTraversalParser) Parse() (*GremlinTraversalSequence, error) {
	p.scanner = NewGremlinTraversalScanner(p.Reader, p.extensions)

	seq := &GremlinTraversalSequence{
		GraphTraversal: NewGraphTraversal(p.Graph),
		extensions:     p.extensions,
	}

	if tok, lit := p.scanIgnoreWhitespace(); tok != G {
		return nil, fmt.Errorf("found %q, expected G", lit)
	}

	// loop over all dot-delimited steps
	for {
		tok, lit := p.scanIgnoreWhitespace()
		if tok == EOF {
			break
		}

		if tok != DOT {
			return nil, fmt.Errorf("found %q, expected .", lit)
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
