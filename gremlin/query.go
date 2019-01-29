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
	"strings"
	"time"

	"github.com/skydive-project/skydive/common"
)

// QueryString used to construct string representation of query
type QueryString string

// NewQueryStringFromArgument via inferance creates a correct ValueString
func NewQueryStringFromArgument(v interface{}) QueryString {
	switch t := v.(type) {
	case QueryString:
		return t
	case string:
		return QueryString(t)
	case fmt.Stringer:
		return QueryString(t.String())
	default:
		panic(fmt.Sprintf("argument %v: type %T not supported", t, v))
	}
}

// String converts value to string
func (q QueryString) String() string {
	return string(q)
}

// G represents the base graph token
const G = QueryString("G")

// Append appends string value to query
func (q QueryString) appends(s string) QueryString {
	return QueryString(string(q) + s)
}

func (q QueryString) newQueryString(name string, list ...interface{}) QueryString {
	q = q.appends(fmt.Sprintf(".%s(", name))
	first := true
	for _, v := range list {
		if !first {
			q = q.appends(", ")
		}
		first = false
		q = q.appends(string(NewValueStringFromArgument(v)))
	}
	return q.appends(")")
}

// Aggregates append a Aggregates() operation to query
func (q QueryString) Aggregates(list ...interface{}) QueryString {
	return q.newQueryString("Aggregates", list...)
}

// At append a At() operation to query
func (q QueryString) At(list ...interface{}) QueryString {
	return q.newQueryString("At", list...)
}

// Both append a Both() operation to query
func (q QueryString) Both(list ...interface{}) QueryString {
	return q.newQueryString("Both", list...)
}

// BPF append a PBF() operation to query
func (q QueryString) BPF(list ...interface{}) QueryString {
	return q.newQueryString("BPF", list...)
}

// CaptureNode append a CaptureNode() operation to query
func (q QueryString) CaptureNode() QueryString {
	return q.newQueryString("CaptureNode")
}

// Context append a Context() operation to query
func (q QueryString) Context(list ...interface{}) QueryString {
	newQ := q.appends(".Context(")
	first := true
	for _, v := range list {
		if !first {
			newQ = newQ.appends(", ")
		} else {
			switch t := v.(type) {
			case string:
				up := strings.ToUpper(t)
				if up == "NOW" || up == "FOREVER" {
					newQ = newQ.appends(up)
					first = false
					continue
				}
			}
		}
		first = false
		switch t := v.(type) {
		case time.Time:
			if t.IsZero() {
				return q
			}
			newQ = newQ.appends(fmt.Sprintf("%d", common.UnixMillis(t)))
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			newQ = newQ.appends(fmt.Sprintf("%d", t))
		default:
			panic(fmt.Sprintf("argument %v: type %T not supported", t, v))
		}
	}
	return newQ.appends(")")
}

// Count append a Count() operation to query
func (q QueryString) Count() QueryString {
	return q.newQueryString("Count")
}

// Dedup append a Dedup() operation to query
func (q QueryString) Dedup() QueryString {
	return q.newQueryString("Dedup")
}

// Flows append a Flows() operation to query
func (q QueryString) Flows(list ...interface{}) QueryString {
	return q.newQueryString("Flows", list...)
}

// Has append a Has() operation to query
func (q QueryString) Has(list ...interface{}) QueryString {
	return q.newQueryString("Has", list...)
}

// HasKey append a HasKey() operation to query
func (q QueryString) HasKey(v interface{}) QueryString {
	return q.newQueryString("HasKey", v)
}

// Hops append a Hops() operation to query
func (q QueryString) Hops() QueryString {
	return q.newQueryString("Hops")
}

// In append a In() operation to query
func (q QueryString) In() QueryString {
	return q.newQueryString("In")
}

// InE append a InE() operation to query
func (q QueryString) InE() QueryString {
	return q.newQueryString("InE")
}

// InV append a InV() operation to query
func (q QueryString) InV() QueryString {
	return q.newQueryString("InV")
}

// BothE append a BothE() operation to query
func (q QueryString) BothE() QueryString {
	return q.newQueryString("BothE")
}

// BothV append a BothV() operation to query
func (q QueryString) BothV() QueryString {
	return q.newQueryString("BothV")
}

// Metrics append a Metrics() operation to query
func (q QueryString) Metrics(key ...interface{}) QueryString {
	return q.newQueryString("Metrics", key...)
}

// Sum append a Sum() operation to query
func (q QueryString) Sum(list ...interface{}) QueryString {
	return q.newQueryString("Sum", list...)
}

// Nodes append a Nodes() operation to query
func (q QueryString) Nodes() QueryString {
	return q.newQueryString("Nodes")
}

// Out append a Out() operation to query
func (q QueryString) Out(list ...interface{}) QueryString {
	return q.newQueryString("Out", list...)
}

// OutE append a OutE() operation to query
func (q QueryString) OutE(list ...interface{}) QueryString {
	return q.newQueryString("OutE", list...)
}

// OutV append a OutV() operation to query
func (q QueryString) OutV(list ...interface{}) QueryString {
	return q.newQueryString("OutV", list...)
}

// RawPackets append a RawPackets() operation to query
func (q QueryString) RawPackets() QueryString {
	return q.newQueryString("RawPackets")
}

// ShortestPathTo append a ShortestPathTo() operation to query
func (q QueryString) ShortestPathTo(list ...interface{}) QueryString {
	return q.newQueryString("ShortestPathTo", list...)
}

// Sort append a Sort() operation to query
func (q QueryString) Sort(list ...interface{}) QueryString {
	return q.newQueryString("Sort", list...)
}

// Sockets append a Sockets() operation to query
func (q QueryString) Sockets() QueryString {
	return q.newQueryString("Sockets")
}

// V append a V() operation to query
func (q QueryString) V(list ...interface{}) QueryString {
	return q.newQueryString("V", list...)
}
