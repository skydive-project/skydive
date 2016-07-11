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
		Params() []interface{}
	}
	GremlinTraversalStepParams struct {
		params []interface{}
	}

	// built in steps
	GremlinTraversalStepG struct {
		GremlinTraversalStepParams
	}
	GremlinTraversalStepV struct {
		GremlinTraversalStepParams
	}
	GremlinTraversalStepE struct {
		GremlinTraversalStepParams
	}
	GremlinTraversalStepContext struct {
		GremlinTraversalStepParams
	}
	GremlinTraversalStepOut struct {
		GremlinTraversalStepParams
	}
	GremlinTraversalStepIn struct {
		GremlinTraversalStepParams
	}
	GremlinTraversalStepOutV struct {
		GremlinTraversalStepParams
	}
	GremlinTraversalStepInV struct {
		GremlinTraversalStepParams
	}
	GremlinTraversalStepOutE struct {
		GremlinTraversalStepParams
	}
	GremlinTraversalStepInE struct {
		GremlinTraversalStepParams
	}
	GremlinTraversalStepDedup struct {
		GremlinTraversalStepParams
	}
	GremlinTraversalStepHas struct {
		GremlinTraversalStepParams
	}
	GremlinTraversalStepShortestPathTo struct {
		GremlinTraversalStepParams
	}
	GremlinTraversalStepBoth struct {
		GremlinTraversalStepParams
	}
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

func invokeStepFnc(last GraphTraversalStep, name string, gremlinStep GremlinTraversalStep) (GraphTraversalStep, error) {
	if v := reflect.ValueOf(last).MethodByName(name); v.IsValid() && !v.IsNil() {
		inputs := make([]reflect.Value, len(gremlinStep.Params()))
		for i, param := range gremlinStep.Params() {
			inputs[i] = reflect.ValueOf(param)
		}
		r := v.Call(inputs)
		step := r[0].Interface().(GraphTraversalStep)

		return step, step.Error()
	}

	return nil, ExecutionError
}

func (p *GremlinTraversalStepParams) Params() []interface{} {
	return p.params
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

func (s *GremlinTraversalStepV) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	return next
}

func (s *GremlinTraversalStepContext) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	g, ok := last.(*GraphTraversal)
	if !ok {
		return nil, ExecutionError
	}

	if len(s.params) != 1 {
		return nil, ExecutionError
	}

	return g.Context(s.params...), nil
}

func (s *GremlinTraversalStepContext) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	return next
}

func (s *GremlinTraversalStepHas) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalV:
		return last.(*GraphTraversalV).Has(s.params...), nil
	case *GraphTraversalE:
		return last.(*GraphTraversalE).Has(s.params...), nil
	}

	return invokeStepFnc(last, "Has", s)
}

func (s *GremlinTraversalStepHas) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	return next
}

func (s *GremlinTraversalStepDedup) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalV:
		return last.(*GraphTraversalV).Dedup(), nil
	case *GraphTraversalE:
		return last.(*GraphTraversalE).Dedup(), nil
	}

	return nil, ExecutionError
}

func (s *GremlinTraversalStepDedup) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	return next
}

func (s *GremlinTraversalStepOut) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalV:
		return last.(*GraphTraversalV).Out(s.params...), nil
	}

	// fallback to reflection way
	return invokeStepFnc(last, "Out", s)
}

func (s *GremlinTraversalStepOut) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	if hasStep, ok := next.(*GremlinTraversalStepHas); ok && len(s.params) == 0 {
		s.params = hasStep.params
		return s
	}
	return next
}

func (s *GremlinTraversalStepIn) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalV:
		return last.(*GraphTraversalV).In(s.params...), nil
	}

	return invokeStepFnc(last, "In", s)
}

func (s *GremlinTraversalStepIn) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	if hasStep, ok := next.(*GremlinTraversalStepHas); ok && len(s.params) == 0 {
		s.params = hasStep.params
		return s
	}
	return next
}

func (s *GremlinTraversalStepOutV) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalE:
		return last.(*GraphTraversalE).OutV(s.params...), nil
	}

	return nil, ExecutionError
}

func (s *GremlinTraversalStepOutV) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	if hasStep, ok := next.(*GremlinTraversalStepHas); ok && len(s.params) == 0 {
		s.params = hasStep.params
		return s
	}
	return next
}

func (s *GremlinTraversalStepInV) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalE:
		return last.(*GraphTraversalE).InV(s.params...), nil
	}

	return nil, ExecutionError
}

func (s *GremlinTraversalStepInV) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	if hasStep, ok := next.(*GremlinTraversalStepHas); ok && len(s.params) == 0 {
		s.params = hasStep.params
		return s
	}
	return next
}

func (s *GremlinTraversalStepOutE) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalV:
		return last.(*GraphTraversalV).OutE(s.params...), nil
	}

	return nil, ExecutionError
}

func (s *GremlinTraversalStepOutE) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	if hasStep, ok := next.(*GremlinTraversalStepHas); ok && len(s.params) == 0 {
		s.params = hasStep.params
		return s
	}
	return next
}

func (s *GremlinTraversalStepInE) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalV:
		return last.(*GraphTraversalV).InE(s.params...), nil
	}

	return nil, ExecutionError
}

func (s *GremlinTraversalStepInE) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	if hasStep, ok := next.(*GremlinTraversalStepHas); ok && len(s.params) == 0 {
		s.params = hasStep.params
		return s
	}
	return next
}

func (s *GremlinTraversalStepShortestPathTo) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
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

func (s *GremlinTraversalStepShortestPathTo) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	return next
}

func (s *GremlinTraversalStepBoth) Exec(last GraphTraversalStep) (GraphTraversalStep, error) {
	switch last.(type) {
	case *GraphTraversalV:
		return last.(*GraphTraversalV).Both(s.params...), nil
	}

	return nil, ExecutionError
}

func (s *GremlinTraversalStepBoth) Reduce(next GremlinTraversalStep) GremlinTraversalStep {
	if hasStep, ok := next.(*GremlinTraversalStepHas); ok && len(s.params) == 0 {
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
		return &GremlinTraversalStepG{}, nil
	}

	params, err := p.parseStepParams()
	if err != nil {
		return nil, err
	}

	gremlinStepParams := GremlinTraversalStepParams{params: params}

	// built in
	switch tok {
	case V:
		return &GremlinTraversalStepV{gremlinStepParams}, nil
	case OUT:
		return &GremlinTraversalStepOut{gremlinStepParams}, nil
	case IN:
		return &GremlinTraversalStepIn{gremlinStepParams}, nil
	case OUTV:
		return &GremlinTraversalStepOutV{gremlinStepParams}, nil
	case INV:
		return &GremlinTraversalStepInV{gremlinStepParams}, nil
	case OUTE:
		return &GremlinTraversalStepOutE{gremlinStepParams}, nil
	case INE:
		return &GremlinTraversalStepInE{gremlinStepParams}, nil
	case DEDUP:
		return &GremlinTraversalStepDedup{gremlinStepParams}, nil
	case HAS:
		return &GremlinTraversalStepHas{gremlinStepParams}, nil
	case SHORTESTPATHTO:
		if len(params) == 0 || len(params) > 2 {
			return nil, fmt.Errorf("ShortestPathTo predicate accept only 1 or 2 parameters")
		}
		return &GremlinTraversalStepShortestPathTo{gremlinStepParams}, nil
	case BOTH:
		return &GremlinTraversalStepBoth{gremlinStepParams}, nil
	case CONTEXT:
		return &GremlinTraversalStepContext{gremlinStepParams}, nil
	}

	// extensions
	for _, e := range p.extensions {
		step, err := e.ParseStep(tok, gremlinStepParams)
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
