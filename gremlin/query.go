/*
 * Copyright (C) 2018 IBM, Inc.
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

package gremlin

import (
	"fmt"
	"time"

	"github.com/skydive-project/skydive/common"
)

// QueryString used to construct string representation of query
type QueryString struct {
	Query string `json:"omitempty"`
}

// NewQueryString used to create an empty query string
func NewQueryString() *QueryString {
	return &QueryString{}
}

// String convert query to string
func (g *QueryString) String() string {
	return g.Query
}

// G greate the graph token
func (g *QueryString) G() *QueryString {
	return g.Append("G")
}

// Append appends string value to query
func (g *QueryString) Append(s string) *QueryString {
	g.Query += s
	return g
}

// Appends appends dot, then string value to query
func (g *QueryString) DotAppend(s string) *QueryString {
	if s != "" {
		g.Append("." + s)
	}
	return g
}

// Append a Context() operation to query
func (g *QueryString) Context(t time.Time) *QueryString {
	if !t.IsZero() {
		g.DotAppend(fmt.Sprintf("Context(%d)", common.UnixMillis(t)))
	}
	return g
}

// V append a V() operation to query
func (g *QueryString) V() *QueryString {
	return g.DotAppend("V()")
}

// Has append a Has() operation to query
func (g *QueryString) Has(list ...*ValueString) *QueryString {
	g.DotAppend("Has(")
	first := true
	for _, v := range list {
		if !first {
			g.Append(", ")
		}
		first = false
		g.Append(v.String())
	}
	g.Append(")")
	return g
}

// HasNode append a Has() for getting graph.Node elements
func (g *QueryString) HasNode(mgr, typ, name *ValueString) *QueryString {
	return g.Has(NewValueString("Manager").Quote(), mgr, NewValueString("Type").Quote(), typ, NewValueString("Name").Quote(), name)
}

// ValueString a value used within query constructs
type ValueString struct {
	Value string `json:"omitempty"`
}

// NewValueString create a value object
func NewValueString(s string) *ValueString {
	return &ValueString{Value: s}
}

// String converts value to string
func (v *ValueString) String() string {
	return v.Value
}

// Quote used to quote string values as needed by query
func (v *ValueString) Quote() *ValueString {
	v.Value = `'` + v.Value + `'`
	return v
}

// Regex used for constructing a regexp expression string
func (v *ValueString) Regex() *ValueString {
	v.Value = "Regex(" + v.Quote().String() + ")"
	return v
}

// StartsWith construct a regexp representing all that start with string
func (v *ValueString) StartsWith() *ValueString {
	v.Value += ".*"
	return v.Regex()
}

// EndsWith construct a regexp representing all that end with string
func (v *ValueString) EndsWith() *ValueString {
	v.Value = ".*" + v.Value
	return v.Regex()
}
