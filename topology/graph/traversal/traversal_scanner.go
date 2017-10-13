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
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
)

// Token represents a lexical token.
type Token int

// Default language token, extension token start at 1000
const (
	// Special tokens
	ILLEGAL Token = iota
	EOF
	WS

	// Literals
	IDENT

	// Misc characters
	COMMA
	DOT
	LEFT_PARENTHESIS
	RIGHT_PARENTHESIS
	STRING
	NUMBER

	// Keywords
	G
	V
	E
	HAS
	HASKEY
	HASNOT
	OUT
	IN
	OUTV
	INV
	BOTHV
	OUTE
	INE
	BOTHE
	DEDUP
	WITHIN
	WITHOUT
	METADATA
	SHORTESTPATHTO
	NE
	BOTH
	CONTEXT
	REGEX
	LT
	GT
	LTE
	GTE
	INSIDE
	OUTSIDE
	BETWEEN
	COUNT
	RANGE
	LIMIT
	SORT
	VALUES
	KEYS
	SUM
	METRICS
	ASC
	DESC
	IPV4RANGE
	OR
	SUBGRAPH

	// extensions token have to start after 1000
)

// GremlinTraversalScanner describes a buffer scanner for Gremlin expression extension
type GremlinTraversalScanner struct {
	reader     *bufio.Reader
	extensions []GremlinTraversalExtension
}

// NewGremlinTraversalScanner creates a new Gremlin expression scanner
func NewGremlinTraversalScanner(r io.Reader, e []GremlinTraversalExtension) *GremlinTraversalScanner {
	return &GremlinTraversalScanner{
		reader:     bufio.NewReader(r),
		extensions: e,
	}
}

// extract elements from braces optionally separated by comma, e.g. '(abc, cde(xyz))' => [abc, cde(xyz)]
func (s *GremlinTraversalScanner) scanBraces() ([]string, error) {

	buf := &bytes.Buffer{}
	buffers := []*bytes.Buffer{}
	level := 0

	isInsideString := false
	isEscaping := false

loop:
	for {
		ch := s.read()

		if isInsideString {
			if ch == '\'' {
				isInsideString = isEscaping
				isEscaping = false
			} else if ch == '\\' {
				isEscaping = true
			}
			buf.WriteRune(ch)

		} else if ch == '\'' {
			isInsideString = true
			isEscaping = false
			buf.WriteRune(ch)

		} else {
			switch ch {
			case '(':
				level += 1
				if level > 1 {
					buf.WriteRune(ch)
				}
			case ')':
				level -= 1
				switch {
				case level > 0:
					buf.WriteRune(ch)
				case level < 0:
					return nil, fmt.Errorf("Unbalanced brackets, current string=%s, previous=%s", buf.String(), buffers)
				case level == 0:
					buffers = append(buffers, buf)
					break loop
				}
			case ',':
				switch {
				case level == 1:
					buffers = append(buffers, buf)
					buf = &bytes.Buffer{}
				case level > 1:
					buf.WriteRune(ch)
				default:
					return nil, fmt.Errorf("The string doesn't start with brackets, current string=%s, previous=%s", buf.String(), buffers)
				}
			case eof:
				return nil, fmt.Errorf("Unable to read nested traversal: eof reached (%d)", level)
			default:
				if level >= 1 {
					buf.WriteRune(ch)
				} else {
					return nil, fmt.Errorf("The string doesn't start with brackets, current string=%s, previous=%s", buf.String(), buffers)
				}
			}
		}
	}

	result := make([]string, 0)
	for _, buf := range buffers {
		tmp := strings.TrimSpace(buf.String())
		if len(tmp) > 0 {
			result = append(result, tmp)
		}
	}

	return result, nil
}

// Scan and returns tokens
func (s *GremlinTraversalScanner) Scan() (tok Token, lit string) {
	ch := s.read()

	if isWhitespace(ch) {
		return s.scanWhitespace()
	} else if isDigit(ch) {
		s.unread()
		return s.scanNumber()
	} else if isString(ch) {
		return s.scanString()
	} else if isLetter(ch) {
		s.unread()
		return s.scanIdent()
	}

	switch ch {
	case eof:
		return EOF, ""
	case '(':
		return LEFT_PARENTHESIS, string(ch)
	case ')':
		return RIGHT_PARENTHESIS, string(ch)
	case ',':
		return COMMA, string(ch)
	case '.':
		return DOT, string(ch)
	}

	return ILLEGAL, string(ch)
}

func (s *GremlinTraversalScanner) scanWhitespace() (tok Token, lit string) {
	var buf bytes.Buffer

	for {
		if ch := s.read(); ch == eof {
			break
		} else if !isWhitespace(ch) {
			s.unread()
			break
		} else {
			buf.WriteRune(ch)
		}
	}

	return WS, buf.String()
}

func (s *GremlinTraversalScanner) scanNumber() (tok Token, lit string) {
	var buf bytes.Buffer
	buf.WriteRune(s.read())

	for {
		if ch := s.read(); isLetter(ch) {
			return ILLEGAL, string(ch)
		} else if ch == eof || (!isDigit(ch) && ch != '.') {
			s.unread()
			break
		} else {
			_, _ = buf.WriteRune(ch)
		}
	}

	return NUMBER, buf.String()
}

func (s *GremlinTraversalScanner) scanString() (tok Token, lit string) {
	var buf bytes.Buffer

	for {
		if ch := s.read(); ch == '"' || ch == '\'' || ch == eof {
			break
		} else {
			_, _ = buf.WriteRune(ch)
		}
	}

	return STRING, buf.String()
}

func (s *GremlinTraversalScanner) scanIdent() (tok Token, lit string) {
	var buf bytes.Buffer
	buf.WriteRune(s.read())

	for {
		if ch := s.read(); ch == eof {
			break
		} else if !isLetter(ch) && !isDigit(ch) && ch != '_' {
			s.unread()
			break
		} else {
			_, _ = buf.WriteRune(ch)
		}
	}

	us := strings.ToUpper(buf.String())

	switch us {
	case "G":
		return G, buf.String()
	case "V":
		return V, buf.String()
	case "E":
		return E, buf.String()
	case "HAS":
		return HAS, buf.String()
	case "HASKEY":
		return HASKEY, buf.String()
	case "HASNOT":
		return HASNOT, buf.String()
	case "OUT":
		return OUT, buf.String()
	case "IN":
		return IN, buf.String()
	case "OUTV":
		return OUTV, buf.String()
	case "INV":
		return INV, buf.String()
	case "BOTHV":
		return BOTHV, buf.String()
	case "OUTE":
		return OUTE, buf.String()
	case "INE":
		return INE, buf.String()
	case "BOTHE":
		return BOTHE, buf.String()
	case "WITHIN":
		return WITHIN, buf.String()
	case "WITHOUT":
		return WITHOUT, buf.String()
	case "DEDUP":
		return DEDUP, buf.String()
	case "METADATA":
		return METADATA, buf.String()
	case "SHORTESTPATHTO":
		return SHORTESTPATHTO, buf.String()
	case "NE":
		return NE, buf.String()
	case "BOTH":
		return BOTH, buf.String()
	case "CONTEXT", "AT":
		return CONTEXT, buf.String()
	case "REGEX":
		return REGEX, buf.String()
	case "LT":
		return LT, buf.String()
	case "GT":
		return GT, buf.String()
	case "LTE":
		return LTE, buf.String()
	case "GTE":
		return GTE, buf.String()
	case "INSIDE":
		return INSIDE, buf.String()
	case "BETWEEN":
		return BETWEEN, buf.String()
	case "COUNT":
		return COUNT, buf.String()
	case "RANGE":
		return RANGE, buf.String()
	case "LIMIT":
		return LIMIT, buf.String()
	case "SORT":
		return SORT, buf.String()
	case "VALUES":
		return VALUES, buf.String()
	case "KEYS":
		return KEYS, buf.String()
	case "SUM":
		return SUM, buf.String()
	case "METRICS":
		return METRICS, buf.String()
	case "ASC":
		return ASC, buf.String()
	case "DESC":
		return DESC, buf.String()
	case "IPV4RANGE":
		return IPV4RANGE, buf.String()
	case "OR", "UNION":
		return OR, buf.String()
	case "SUBGRAPH":
		return SUBGRAPH, buf.String()
	}

	for _, e := range s.extensions {
		if t, ok := e.ScanIdent(us); ok {
			return t, buf.String()
		}
	}

	return IDENT, buf.String()
}

func (s *GremlinTraversalScanner) read() rune {
	ch, _, err := s.reader.ReadRune()
	if err != nil {
		return eof
	}
	return ch
}

func (s *GremlinTraversalScanner) unread() {
	s.reader.UnreadRune()
}

func isString(ch rune) bool {
	return ch == '"' || ch == '\''
}

func isWhitespace(ch rune) bool {
	return ch == ' ' || ch == '\t' || ch == '\n'
}

func isLetter(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func isDigit(ch rune) bool {
	return (ch >= '0' && ch <= '9')
}

var eof = rune(0)
