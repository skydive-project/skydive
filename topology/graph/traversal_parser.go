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
	GraphTraversalSequence struct {
		GraphTraversal *GraphTraversal
		steps          []traversalStep
	}

	traversalStep       interface{}
	traversalStepParams []interface{}

	traversalStepG     struct{}
	traversalStepV     struct{ params traversalStepParams }
	traversalStepE     struct{}
	traversalStepOut   struct{}
	traversalStepIn    struct{}
	traversalStepOutV  struct{}
	traversalStepInV   struct{}
	traversalStepOutE  struct{}
	traversalStepInE   struct{}
	traversalStepDedup struct{}
	traversalStepHas   struct{ params traversalStepParams }
)

var (
	ExecutionError error = errors.New("Error while executing the query")
)

type GraphTraversalParser struct {
	Graph   *Graph
	scanner *TraversalScanner
	buf     struct {
		tok Token
		lit string
		n   int
	}
}

func execStepV(last interface{}, params traversalStepParams) (interface{}, error) {
	g, ok := last.(*GraphTraversal)
	if !ok {
		return nil, ExecutionError
	}

	switch len(params) {
	case 1:
		if k, ok := params[0].(string); ok {
			return g.V(Identifier(k)), nil
		}
		return nil, ExecutionError
	case 0:
		return g.V(), nil
	default:
		return nil, ExecutionError
	}
}

func execStepHas(last interface{}, params traversalStepParams) (interface{}, error) {
	switch last.(type) {
	case *GraphTraversalV:
		return last.(*GraphTraversalV).Has(params...), nil
	case *GraphTraversalE:
		return last.(*GraphTraversalE).Has(params...), nil
	}

	return nil, ExecutionError
}

func execStepDedup(last interface{}) (interface{}, error) {
	switch last.(type) {
	case *GraphTraversalV:
		return last.(*GraphTraversalV).Dedup(), nil
	case *GraphTraversalE:
		return last.(*GraphTraversalE).Dedup(), nil
	}

	return nil, ExecutionError
}

func execStepOut(last interface{}) (interface{}, error) {
	switch last.(type) {
	case *GraphTraversalV:
		return last.(*GraphTraversalV).Out(), nil
	}

	return nil, ExecutionError
}

func execStepIn(last interface{}) (interface{}, error) {
	switch last.(type) {
	case *GraphTraversalV:
		return last.(*GraphTraversalV).In(), nil
	}

	return nil, ExecutionError
}

func execStepOutV(last interface{}) (interface{}, error) {
	switch last.(type) {
	case *GraphTraversalE:
		return last.(*GraphTraversalE).OutV(), nil
	}

	return nil, ExecutionError
}

func execStepInV(last interface{}) (interface{}, error) {
	switch last.(type) {
	case *GraphTraversalE:
		return last.(*GraphTraversalE).InV(), nil
	}

	return nil, ExecutionError
}

func execStepOutE(last interface{}) (interface{}, error) {
	switch last.(type) {
	case *GraphTraversalV:
		return last.(*GraphTraversalV).OutE(), nil
	}

	return nil, ExecutionError
}

func execStepInE(last interface{}) (interface{}, error) {
	switch last.(type) {
	case *GraphTraversalV:
		return last.(*GraphTraversalV).InE(), nil
	}

	return nil, ExecutionError
}

func (s *GraphTraversalSequence) Exec() (GraphTraversalStep, error) {
	var last interface{}
	var err error

	last = s.GraphTraversal

	for _, step := range s.steps {
		switch step.(type) {
		case *traversalStepV:
			last, err = execStepV(last, step.(*traversalStepV).params)
		case *traversalStepHas:
			last, err = execStepHas(last, step.(*traversalStepHas).params)
		case *traversalStepOut:
			last, err = execStepOut(last)
		case *traversalStepIn:
			last, err = execStepIn(last)
		case *traversalStepOutV:
			last, err = execStepOutV(last)
		case *traversalStepInV:
			last, err = execStepInV(last)
		case *traversalStepOutE:
			last, err = execStepOutE(last)
		case *traversalStepInE:
			last, err = execStepInE(last)
		case *traversalStepDedup:
			last, err = execStepDedup(last)
		}

		if err != nil {
			return nil, err
		}
	}

	res, ok := last.(GraphTraversalStep)
	if !ok {
		return nil, ExecutionError
	}

	return res, nil
}

func NewGraphTraversalParser(r io.Reader, g *Graph) *GraphTraversalParser {
	return &GraphTraversalParser{
		Graph:   g,
		scanner: NewTraversalScanner(r),
	}
}

func (p *GraphTraversalParser) parserStepParams() (traversalStepParams, error) {
	tok, lit := p.scanIgnoreWhitespace()
	if tok != LEFT_PARENTHESIS {
		return nil, fmt.Errorf("Expected left parenthesis, got: %s", lit)
	}

	params := traversalStepParams{}
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
		default:
			return nil, fmt.Errorf("Unexpected token while parsing parameters, got: %s", lit)
		}
		tok, lit = p.scanIgnoreWhitespace()
	}

	return params, nil
}

func (p *GraphTraversalParser) parserStep() (traversalStep, error) {
	tok, lit := p.scanIgnoreWhitespace()
	if tok == IDENT {
		return nil, fmt.Errorf("Expected step function, got: %s", lit)
	}

	if tok == G {
		return &traversalStepG{}, nil
	}

	params, err := p.parserStepParams()
	if err != nil {
		return nil, err
	}

	switch tok {
	case V:
		return &traversalStepV{params: params}, nil
	case OUT:
		return &traversalStepOut{}, nil
	case IN:
		return &traversalStepIn{}, nil
	case OUTV:
		return &traversalStepOutV{}, nil
	case INV:
		return &traversalStepInV{}, nil
	case OUTE:
		return &traversalStepOutE{}, nil
	case INE:
		return &traversalStepInE{}, nil
	case DEDUP:
		return &traversalStepDedup{}, nil
	case HAS:
		return &traversalStepHas{params: params}, nil
	default:
		return nil, fmt.Errorf("Expected step function, got: %s", lit)
	}
}

func (p *GraphTraversalParser) Parse() (*GraphTraversalSequence, error) {
	seq := &GraphTraversalSequence{
		GraphTraversal: NewGrahTraversal(p.Graph),
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

func (p *GraphTraversalParser) scan() (tok Token, lit string) {
	if p.buf.n != 0 {
		p.buf.n = 0
		return p.buf.tok, p.buf.lit
	}
	p.buf.tok, p.buf.lit = p.scanner.Scan()

	return p.buf.tok, p.buf.lit
}

func (p *GraphTraversalParser) scanIgnoreWhitespace() (Token, string) {
	tok, lit := p.scan()
	for tok == WS {
		tok, lit = p.scan()
	}
	return tok, lit
}

func (p *GraphTraversalParser) unscan() {
	p.buf.n = 1
}
