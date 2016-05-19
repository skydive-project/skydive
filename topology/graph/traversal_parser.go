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
	gremlinTraversalStepG      struct{}
	gremlinTraversalStepV      struct{ params GremlinTraversalStepParams }
	gremlinTraversalStepE      struct{}
	gremlinTraversalStepOut    struct{}
	gremlinTraversalStepIn     struct{}
	gremlinTraversalStepOutV   struct{}
	gremlinTraversalStepInV    struct{}
	gremlinTraversalStepOutE   struct{}
	gremlinTraversalStepInE    struct{}
	gremlinTraversalStepDedup  struct{}
	gremlinTraversalStepHas    struct{ params GremlinTraversalStepParams }
	gremlinTraversalStepString struct{}
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
		return last.(*GraphTraversalV).Out(), nil
	}

	return nil, ExecutionError
}

func (s *gremlinTraversalStepIn) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalV:
		return last.(*GraphTraversalV).In(), nil
	}

	return nil, ExecutionError
}

func (s *gremlinTraversalStepOutV) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalE:
		return last.(*GraphTraversalE).OutV(), nil
	}

	return nil, ExecutionError
}

func (s *gremlinTraversalStepInV) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalE:
		return last.(*GraphTraversalE).InV(), nil
	}

	return nil, ExecutionError
}

func (s *gremlinTraversalStepOutE) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalV:
		return last.(*GraphTraversalV).OutE(), nil
	}

	return nil, ExecutionError
}

func (s *gremlinTraversalStepInE) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalV:
		return last.(*GraphTraversalV).InE(), nil
	}

	return nil, ExecutionError
}

func (s *GremlinTraversalSequence) Exec() (GraphTraversalStep, error) {
	var last GraphTraversalStep
	var err error

	last = s.GraphTraversal

	for _, step := range s.steps {
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
			return params, nil
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
		return &gremlinTraversalStepOut{}, nil
	case IN:
		return &gremlinTraversalStepIn{}, nil
	case OUTV:
		return &gremlinTraversalStepOutV{}, nil
	case INV:
		return &gremlinTraversalStepInV{}, nil
	case OUTE:
		return &gremlinTraversalStepOutE{}, nil
	case INE:
		return &gremlinTraversalStepInE{}, nil
	case DEDUP:
		return &gremlinTraversalStepDedup{}, nil
	case HAS:
		return &gremlinTraversalStepHas{params: params}, nil
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
		GraphTraversal: NewGrahTraversal(p.Graph),
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
