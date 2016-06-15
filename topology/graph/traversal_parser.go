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

package graph

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
)

type (
	GremlinTraversalSequence struct {
		GraphTraversal *GraphTraversal
		steps          []GremlinTraversalStep
		extensions     []GremlinTraversalExtension
	}

	GremlinTraversalStep interface {
		Exec(last GraphTraversalStep) (GraphTraversalStep, error)
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
	Graph   *Graph
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
	if v := reflect.ValueOf(last).MethodByName(name); !v.IsNil() {
		inputs := make([]reflect.Value, len(params))
		for i, param := range params {
			inputs[i] = reflect.ValueOf(param)
		}
		r := v.Call(inputs)

		return r[0].Interface().(GraphTraversalStep), nil
	}

	return nil, ExecutionError
}

func (s *gremlinTraversalStepG) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	return nil, nil
}

func (s *gremlinTraversalStepV) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	g, ok := last.(*GraphTraversal)
	if !ok {
		return nil, ExecutionError
	}

	switch len(s.params) {
	case 1:
		if k, ok := s.params[0].(string); ok {
			return g.V(Identifier(k)), nil
		}
		return nil, ExecutionError
	case 0:
		return g.V(), nil
	default:
		return nil, ExecutionError
	}
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

func (s *gremlinTraversalStepHas) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalV:
		return last.(*GraphTraversalV).Has(s.params...), nil
	case *GraphTraversalE:
		return last.(*GraphTraversalE).Has(s.params...), nil
	}

	return nil, ExecutionError
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

func (s *gremlinTraversalStepOut) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalV:
		return last.(*GraphTraversalV).Out(s.params...), nil
	}

	// fallback to reflection way
	return invokeStepFnc(last, "Out", s.params)
}

func (s *gremlinTraversalStepIn) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalV:
		return last.(*GraphTraversalV).In(s.params...), nil
	}

	return invokeStepFnc(last, "In", s.params)
}

func (s *gremlinTraversalStepOutV) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalE:
		return last.(*GraphTraversalE).OutV(s.params...), nil
	}

	return nil, ExecutionError
}

func (s *gremlinTraversalStepInV) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalE:
		return last.(*GraphTraversalE).InV(s.params...), nil
	}

	return nil, ExecutionError
}

func (s *gremlinTraversalStepOutE) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalV:
		return last.(*GraphTraversalV).OutE(s.params...), nil
	}

	return nil, ExecutionError
}

func (s *gremlinTraversalStepInE) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalV:
		return last.(*GraphTraversalV).InE(s.params...), nil
	}

	return nil, ExecutionError
}

func (s *gremlinTraversalStepShortestPathTo) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalV:
		if _, ok := s.params[0].(Metadata); !ok {
			return nil, ExecutionError
		}
		if len(s.params) > 1 {
			if _, ok := s.params[1].(Metadata); !ok {
				return nil, ExecutionError
			}
			return last.(*GraphTraversalV).ShortestPathTo(s.params[0].(Metadata), s.params[1].(Metadata)), nil
		}
		return last.(*GraphTraversalV).ShortestPathTo(s.params[0].(Metadata)), nil
	}

	return nil, ExecutionError
}

func (s *gremlinTraversalStepBoth) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalV:
		return last.(*GraphTraversalV).Both(s.params...), nil
	}

	return nil, ExecutionError
}

func (s *GremlinTraversalSequence) nextStepToExec(i int) (GremlinTraversalStep, int) {
	step := s.steps[i]

	// check whether we can skip a step
	if i+1 < len(s.steps) {
		next := s.steps[i+1]

		switch next.(type) {
		// optimisation possible, uses the param of has for current predicate and
		// inc the position in order to skip the HAS
		case *gremlinTraversalStepHas:
			params := next.(*gremlinTraversalStepHas).params

			switch step.(type) {
			case *gremlinTraversalStepIn:
				if len(step.(*gremlinTraversalStepIn).params) == 0 {
					step.(*gremlinTraversalStepIn).params = params
					i++
				}
			case *gremlinTraversalStepOut:
				if len(step.(*gremlinTraversalStepOut).params) == 0 {
					step.(*gremlinTraversalStepOut).params = params
					i++
				}
			case *gremlinTraversalStepOutV:
				if len(step.(*gremlinTraversalStepOutV).params) == 0 {
					step.(*gremlinTraversalStepOutV).params = params
					i++
				}
			case *gremlinTraversalStepInV:
				if len(step.(*gremlinTraversalStepInV).params) == 0 {
					step.(*gremlinTraversalStepInV).params = params
					i++
				}
			case *gremlinTraversalStepOutE:
				if len(step.(*gremlinTraversalStepOutE).params) == 0 {
					step.(*gremlinTraversalStepOutE).params = params
					i++
				}
			case *gremlinTraversalStepInE:
				if len(step.(*gremlinTraversalStepInE).params) == 0 {
					step.(*gremlinTraversalStepInE).params = params
					i++
				}
			case *gremlinTraversalStepBoth:
				if len(step.(*gremlinTraversalStepBoth).params) == 0 {
					step.(*gremlinTraversalStepBoth).params = params
					i++
				}
			}
		}
	}
	i++

	// last element return -1 to specify to end
	if i == len(s.steps) {
		return step, -1
	}

	return step, i
}

func (s *GremlinTraversalSequence) Exec() (GraphTraversalStep, error) {
	var step GremlinTraversalStep
	var last GraphTraversalStep
	var err error

	last = s.GraphTraversal
	for i := 0; i != -1; {
		step, i = s.nextStepToExec(i)

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

func NewGremlinTraversalParser(r io.Reader, g *Graph) *GremlinTraversalParser {
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
			withParams, err := p.parserStepParams()
			if err != nil {
				return nil, err
			}
			params = append(params, Without(withParams...))
		case NE:
			withParams, err := p.parserStepParams()
			if err != nil {
				return nil, err
			}
			if len(withParams) != 1 {
				return nil, fmt.Errorf("One parameter expected to EQ: %v", withParams)
			}
			params = append(params, Ne(withParams[0]))
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
