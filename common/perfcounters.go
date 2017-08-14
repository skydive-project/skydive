/*
 * Copyright (C) 2017 Red Hat, Inc.
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

package common

import (
	"expvar"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/abbot/go-http-auth"
	"github.com/paulbellamy/ratecounter"
)

var perfMap map[string]*expvar.Map

// PerfCounterIntRate describe a performance counter
type PerfCounterIntRate struct {
	count      expvar.Int
	in         expvar.Int
	rate       expvar.Int
	rateMinute *ratecounter.RateCounter
	avg        expvar.Float
	avgMinute  *ratecounter.AvgRateCounter
	startTime  time.Time
}

// from expvar go 1.7
func expvarHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	fmt.Fprintf(w, "{\n")
	first := true
	expvar.Do(func(kv expvar.KeyValue) {
		if !first {
			fmt.Fprintf(w, ",\n")
		}
		first = false
		fmt.Fprintf(w, "%q: %s", kv.Key, kv.Value)
	})
	fmt.Fprintf(w, "\n}\n")
}

func expvarHandlerAuth(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
	expvarHandler(w, &r.Request)
}

// PerfcountersHandler return an http handler to show the values
func PerfcountersHandler() http.Handler {
	return http.HandlerFunc(expvarHandler)
}

// PerfcountersHandlerAuth retur, an http authenticated handler
func PerfcountersHandlerAuth() auth.AuthenticatedHandlerFunc {
	return expvarHandlerAuth
}

// PerfHttpHanderFuncWrap is a simple wrapped to include call to Prolog/Epilog during each handler call
func PerfHttpHanderFuncWrap(wrapped http.HandlerFunc, perf *PerfCounterIntRate) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		//@perf perf.Prolog(1)
		wrapped(w, r)
		//@perf perf.Epilog(1)
	}
}

// NewPerfCounterIntPerMin create a new performance counter
// count is the number of hit
// in is the number of call within the Prolog/Epilog
// permin is number of hit per minute
// avg-ns is the time spent within the Prolog/Epilog per call
func NewPerfCounterIntPerMin(name string) *PerfCounterIntRate {
	i := strings.LastIndex(name, ".")
	if i <= 0 {
		panic("name must contain a dot, ex: api.counter")
	}
	mapName := name[:i]
	cntName := name[i+1:]

	p := &PerfCounterIntRate{}
	p.rateMinute = ratecounter.NewRateCounter(60 * time.Second)
	p.avgMinute = ratecounter.NewAvgRateCounter(60 * time.Second)

	mroot := newOrGetPerfMap(mapName)
	m := new(expvar.Map).Init()
	mroot.Set(cntName, m)
	m.Set("count", &p.count)
	m.Set("in", &p.in)
	m.Set("permin", &p.rate)
	m.Set("avg-ns", &p.avg)
	return p
}

// Prolog of the performance counter
func (p *PerfCounterIntRate) Prolog(delta int64) {
	p.count.Add(delta)
	p.in.Add(delta)
	p.rateMinute.Incr(delta)
	p.rate.Set(p.rateMinute.Rate())
	p.startTime = time.Now()
}

// Epilog of the performance counter
func (p *PerfCounterIntRate) Epilog(delta int64) {
	p.avgMinute.Incr(time.Since(p.startTime).Nanoseconds())
	p.avg.Set(p.avgMinute.Rate())
	p.in.Add(-delta)
}

func newOrGetPerfMap(name string) *expvar.Map {
	if m, ok := perfMap[name]; ok {
		return m
	}
	perfMap[name] = expvar.NewMap(name)
	return perfMap[name]
}

func init() {
	perfMap = make(map[string]*expvar.Map)
}
