/*
 * Copyright (C) 2018 Red Hat, Inc.
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

package js

import (
	"bytes"
	crand "crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/robertkrimen/otto"
	"github.com/skydive-project/skydive/api/server"
	"github.com/skydive-project/skydive/api/types"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/statics"
	"github.com/skydive-project/skydive/topology/graph"
	"github.com/skydive-project/skydive/topology/graph/traversal"
)

type evalReq struct {
	fn   func(vm *otto.Otto)
	done chan bool
}

// jsTimer is a single timer instance with a callback function
type jsTimer struct {
	timer    *time.Timer
	duration time.Duration
	interval bool
	call     otto.FunctionCall
}

// JSRE is a Skydive JavaScript runtime environment
type JSRE struct {
	*otto.Otto
	evalQueue     chan *evalReq
	stopEventLoop chan bool
	closed        chan struct{}
	timers        map[*jsTimer]*jsTimer
	timerReady    chan *jsTimer
}

// RegisterAPIServer
func (jsre *JSRE) RegisterAPIServer(g *graph.Graph, gremlinParser *traversal.GremlinTraversalParser, server *server.Server) {
	queryGremlin := func(query string) otto.Value {
		ts, err := gremlinParser.Parse(strings.NewReader(query))
		if err != nil {
			return jsre.MakeCustomError("ParseError", err.Error())
		}

		result, err := ts.Exec(g, false)
		if err != nil {
			return jsre.MakeCustomError("ExecuteError", err.Error())
		}

		source, err := result.MarshalJSON()
		if err != nil {
			return jsre.MakeCustomError("MarshalError", err.Error())
		}

		jsonObj, err := jsre.Object("obj = " + string(source))
		if err != nil {
			return jsre.MakeCustomError("JSONError", err.Error())
		}

		logging.GetLogger().Infof("Gremlin returned %+v (from %s, query %s)", jsonObj, source, query)
		r, _ := jsre.ToValue(jsonObj)
		return r
	}

	jsre.Set("Gremlin", func(call otto.FunctionCall) otto.Value {
		if len(call.ArgumentList) < 1 || !call.Argument(0).IsString() {
			return jsre.MakeCustomError("MissingQueryArgument", "Gremlin requires a string parameter")
		}

		query := call.Argument(0).String()

		return queryGremlin(query)
	})

	jsre.Set("request", func(call otto.FunctionCall) otto.Value {
		if len(call.ArgumentList) < 3 || !call.Argument(0).IsString() || !call.Argument(1).IsString() || !call.Argument(2).IsString() {
			return jsre.MakeCustomError("WrongArguments", "Import requires 3 string parameters")
		}

		url := call.Argument(0).String()
		method := call.Argument(1).String()
		data := []byte(call.Argument(2).String())

		subs := strings.Split(url, "/") // filepath.Base(url)
		if len(subs) < 3 {
			return jsre.MakeCustomError("WrongArgument", fmt.Sprintf("Malformed URL %s", url))
		}
		resource := subs[2]

		// For topology query, we directly call the Gremlin engine
		if resource == "topology" {
			query := types.TopologyParam{}
			if err := json.Unmarshal(data, &query); err != nil {
				return jsre.MakeCustomError("WrongArgument", fmt.Sprintf("Invalid query %s", string(data)))
			}

			return queryGremlin(query.GremlinQuery)
		}

		// This a CRUD call
		handler := server.GetHandler(resource)

		var err error
		var content interface{}

		switch method {
		case "POST":
			res := handler.New()
			if err := json.Unmarshal([]byte(data), res); err != nil {
				return jsre.MakeCustomError("UnmarshalError", err.Error())
			}
			if err := handler.Create(res); err != nil {
				return jsre.MakeCustomError("CreateError", err.Error())
			}
			b, _ := json.Marshal(res)
			content = string(b)

		case "DELETE":
			if len(subs) < 4 {
				return jsre.MakeCustomError("WrongArgument", "No ID specified")
			}
			handler.Delete(subs[3])

		case "GET":
			if len(subs) < 4 {
				resources := handler.Index()
				b, _ := json.Marshal(resources)
				content = string(b)
			} else {
				id := subs[3]
				obj, found := handler.Get(id)
				if !found {
					return jsre.MakeCustomError("NotFound", fmt.Sprintf("%s %s could not be found", resource, id))
				}
				b, _ := json.Marshal(obj)
				content = string(b)
			}
		}

		value, err := otto.ToValue(content)
		if err != nil {
			return jsre.MakeCustomError("WrongValue", err.Error())
		}

		return value
	})
}

func (jsre *JSRE) RegisterAPIClient(client *shttp.CrudClient) {
	jsre.Set("request", func(call otto.FunctionCall) otto.Value {
		if len(call.ArgumentList) < 3 || !call.Argument(0).IsString() || !call.Argument(1).IsString() || !call.Argument(2).IsString() {
			return jsre.MakeCustomError("WrongArguments", "Import requires 3 string parameters")
		}

		url := call.Argument(0).String()
		method := call.Argument(1).String()
		data := []byte(call.Argument(2).String())

		resp, err := client.Request(method, url, bytes.NewReader(data), nil)
		if err != nil {
			return jsre.MakeCustomError("WrongRequest", err.Error())
		}
		defer resp.Body.Close()

		content, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return jsre.MakeCustomError("WrongResponse", err.Error())
		}

		value, err := otto.ToValue(string(content))
		if err != nil {
			return jsre.MakeCustomError("WrongValue", err.Error())
		}

		return value
	})
}

// RunScript executes the specified script
func (jsre *JSRE) RunScript(path string) otto.Value {
	file, err := os.Open(path)

	if err != nil {
		return jsre.MakeCustomError("FileNotFound", fmt.Sprintf("File %s could not be found", path))
	}

	b, err := ioutil.ReadAll(file)
	if err != nil {
		return jsre.MakeCustomError("CouldNotRead", fmt.Sprintf("File %s could not be read", path))
	}

	_, err = jsre.Exec(string(b))
	if err != nil {
		return jsre.MakeCustomError("FailedToRun", fmt.Sprintf("Failed to run %s: %s", path, err))
	}

	return otto.UndefinedValue()
}

func (jsre *JSRE) runEmbededScript(path string) error {
	if content, err := statics.Asset(path); err != nil {
		return fmt.Errorf("Failed to load %s asset: %s)", path, err)
	} else {
		if _, err = jsre.Run(string(content)); err != nil {
			return fmt.Errorf("Failed to run %s: %s", path, err)
		}
	}
	return nil
}

func (jsre *JSRE) registerStandardLibray() {
	jsre.Run(`exports = {};`)
	jsre.Run(`function require() { }`)

	jsre.Set("run", func(call otto.FunctionCall) otto.Value {
		if len(call.ArgumentList) < 1 || !call.Argument(0).IsString() {
			return jsre.MakeCustomError("MissingQueryArgument", "run requires a string parameter")
		}
		path := call.Argument(0).String()
		return jsre.RunScript(path)
	})

	newTimer := func(call otto.FunctionCall, interval bool) (*jsTimer, otto.Value) {
		delay, _ := call.Argument(1).ToInteger()
		if 0 >= delay {
			delay = 1
		}
		timer := &jsTimer{
			duration: time.Duration(delay) * time.Millisecond,
			call:     call,
			interval: interval,
		}
		jsre.timers[timer] = timer

		timer.timer = time.AfterFunc(timer.duration, func() {
			jsre.timerReady <- timer
		})

		value, err := call.Otto.ToValue(timer)
		if err != nil {
			panic(err)
		}
		return timer, value
	}

	setTimeout := func(call otto.FunctionCall) otto.Value {
		_, value := newTimer(call, false)
		return value
	}

	setInterval := func(call otto.FunctionCall) otto.Value {
		_, value := newTimer(call, true)
		return value
	}

	clearTimeout := func(call otto.FunctionCall) otto.Value {
		timer, _ := call.Argument(0).Export()
		if timer, ok := timer.(*jsTimer); ok {
			timer.timer.Stop()
			delete(jsre.timers, timer)
		}
		return otto.UndefinedValue()
	}

	jsre.Set("_setTimeout", setTimeout)
	jsre.Set("_setInterval", setInterval)
	jsre.Run(`var setTimeout = function(args) {
		if (arguments.length < 1) {
			throw TypeError("Failed to execute 'setTimeout': 1 argument required, but only 0 present.");
		}
		return _setTimeout.apply(this, arguments);
	}`)
	jsre.Run(`var setInterval = function(args) {
		if (arguments.length < 1) {
			throw TypeError("Failed to execute 'setInterval': 1 argument required, but only 0 present.");
		}
		return _setInterval.apply(this, arguments);
	}`)
	jsre.Set("clearTimeout", clearTimeout)
	jsre.Set("clearInterval", clearTimeout)
}

func (jsre *JSRE) runEventLoop() {
	defer close(jsre.closed)

	var waitForCallbacks bool

loop:
	for {
		select {
		case timer := <-jsre.timerReady:
			var arguments []interface{}
			if len(timer.call.ArgumentList) > 2 {
				tmp := timer.call.ArgumentList[2:]
				arguments = make([]interface{}, 2+len(tmp))
				for i, value := range tmp {
					arguments[i+2] = value
				}
			} else {
				arguments = make([]interface{}, 1)
			}
			arguments[0] = timer.call.ArgumentList[0]
			_, err := jsre.Call(`Function.call.call`, nil, arguments...)
			if err != nil {
				logging.GetLogger().Errorf("JavaScript error: %s", err)
			}

			_, inreg := jsre.timers[timer]
			if timer.interval && inreg {
				timer.timer.Reset(timer.duration)
			} else {
				delete(jsre.timers, timer)
				if waitForCallbacks && (len(jsre.timers) == 0) {
					break loop
				}
			}

		case req := <-jsre.evalQueue:
			// run the code, send the result back
			req.fn(jsre.Otto)
			close(req.done)
			if waitForCallbacks && (len(jsre.timers) == 0) {
				break loop
			}

		case waitForCallbacks = <-jsre.stopEventLoop:
			if !waitForCallbacks || (len(jsre.timers) == 0) {
				break loop
			}
		}
	}

	for _, timer := range jsre.timers {
		timer.timer.Stop()
		delete(jsre.timers, timer)
	}
}

// CompleteKeywords returns potential continuations for the given line.
// Since line is evaluated, callers need to make sure that evaluating line
// does not have side effects.
func (jsre *JSRE) CompleteKeywords(line string) []string {
	var results []string
	jsre.Do(func(vm *otto.Otto) {
		results = getCompletions(vm, line)
	})
	return results
}

// Do executes the `fn` in the event loop
func (jsre *JSRE) Do(fn func(*otto.Otto)) {
	done := make(chan bool)
	req := &evalReq{fn, done}
	jsre.evalQueue <- req
	<-done
}

// Exec executes some JavaScript code
func (jsre *JSRE) Exec(code string) (v otto.Value, err error) {
	jsre.Do(func(vm *otto.Otto) { v, err = vm.Run(code) })
	return v, err
}

// Start the runtime evaluation loop
func (jsre *JSRE) Start() {
	go jsre.runEventLoop()
}

// Stop the runtime evaluation loop
func (jsre *JSRE) Stop() {
}

// NewJSRE returns a new JavaScript runtime environment
func NewJSRE() (*JSRE, error) {
	jsre := &JSRE{
		Otto:          otto.New(),
		closed:        make(chan struct{}),
		evalQueue:     make(chan *evalReq, 20),
		stopEventLoop: make(chan bool),
		timers:        make(map[*jsTimer]*jsTimer),
		timerReady:    make(chan *jsTimer),
	}

	jsre.registerStandardLibray()

	bytes := make([]byte, 8)
	seed := time.Now().UnixNano()
	if _, err := crand.Read(bytes); err == nil {
		seed = int64(binary.LittleEndian.Uint64(bytes))
	}

	src := rand.NewSource(seed)
	r := rand.New(src)
	jsre.SetRandomSource(r.Float64)

	if err := jsre.runEmbededScript("js/promise-7.0.4.min.js"); err != nil {
		return nil, err
	}

	if err := jsre.runEmbededScript("js/promise-done-7.0.4.min.js"); err != nil {
		return nil, err
	}

	if err := jsre.runEmbededScript("js/api.js"); err != nil {
		return nil, err
	}

	if err := jsre.runEmbededScript("js/otto.js"); err != nil {
		return nil, err
	}

	if err := jsre.runEmbededScript("statics/js/vendor/pure-uuid.js"); err != nil {
		return nil, err
	}

	return jsre, nil
}
