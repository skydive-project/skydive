//go:generate stringer -type=Token $GOFILE

/*
 * Copyright (C) 2016 Red Hat, Inc.
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
	LEFTPARENTHESIS
	RIGHTPARENTHESIS
	STRING
	NUMBER

	// Keywords
	G
	V
	E
	HAS
	HASKEY
	HASNOT
	HASEITHER
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
	NEE
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
	VALUEMAP
	KEYS
	SUM
	ASC
	DESC
	IPV4RANGE
	SUBGRAPH
	FOREVER
	NOW
	AS
	SELECT

	TRUE
	FALSE

	// extensions token have to start after 1000
)
