/*
 * Copyright (C) 2018 Red Hat, Inc.
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

package js

import (
	"bytes"
	crand "crypto/rand"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/robertkrimen/otto"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/statics"
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

// Runtime is a Skydive JavaScript runtime environment
type Runtime struct {
	*otto.Otto
	sync.Mutex
	evalQueue     chan *evalReq
	stopEventLoop chan bool
	closed        chan struct{}
	timers        map[*jsTimer]*jsTimer
	timerReady    chan *jsTimer
}

// RegisterAPIClient exports Go function required by the API to run inside the client JS VM
func (r *Runtime) RegisterAPIClient(client *shttp.CrudClient) {
	r.Set("request", func(call otto.FunctionCall) otto.Value {
		if len(call.ArgumentList) < 3 || !call.Argument(0).IsString() || !call.Argument(1).IsString() || !call.Argument(2).IsString() {
			return r.MakeCustomError("WrongArguments", "Import requires 3 string parameters")
		}

		url := call.Argument(0).String()
		method := call.Argument(1).String()
		data := []byte(call.Argument(2).String())

		resp, err := client.Request(method, url, bytes.NewReader(data), nil)
		if err != nil {
			return r.MakeCustomError("WrongRequest", err.Error())
		}
		defer resp.Body.Close()

		content, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return r.MakeCustomError("WrongResponse", err.Error())
		}

		value, err := otto.ToValue(string(content))
		if err != nil {
			return r.MakeCustomError("WrongValue", err.Error())
		}

		return value
	})
}

// RunScript executes the specified script
func (r *Runtime) RunScript(path string) otto.Value {
	file, err := os.Open(path)

	if err != nil {
		return r.MakeCustomError("FileNotFound", fmt.Sprintf("File %s could not be found", path))
	}

	b, err := ioutil.ReadAll(file)
	if err != nil {
		return r.MakeCustomError("CouldNotRead", fmt.Sprintf("File %s could not be read", path))
	}

	_, err = r.Exec(string(b))
	if err != nil {
		return r.MakeCustomError("FailedToRun", fmt.Sprintf("Failed to run %s: %s", path, err))
	}

	return otto.UndefinedValue()
}

func (r *Runtime) runEmbededScript(path string) error {
	content, err := statics.Asset(path)
	if err != nil {
		return fmt.Errorf("Failed to load %s asset: %s)", path, err)
	}

	if _, err := r.Run(string(content)); err != nil {
		return fmt.Errorf("Failed to run %s: %s", path, err)
	}
	return nil
}

func (r *Runtime) registerStandardLibray() {
	r.Run(`exports = {};`)
	r.Run(`function require() { }`)

	r.Set("run", func(call otto.FunctionCall) otto.Value {
		if len(call.ArgumentList) < 1 || !call.Argument(0).IsString() {
			return r.MakeCustomError("MissingQueryArgument", "run requires a string parameter")
		}
		path := call.Argument(0).String()
		return r.RunScript(path)
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
		r.timers[timer] = timer

		timer.timer = time.AfterFunc(timer.duration, func() {
			r.timerReady <- timer
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
			delete(r.timers, timer)
		}
		return otto.UndefinedValue()
	}

	r.Set("_setTimeout", setTimeout)
	r.Set("_setInterval", setInterval)
	r.Run(`var setTimeout = function(args) {
		if (arguments.length < 1) {
			throw TypeError("Failed to execute 'setTimeout': 1 argument required, but only 0 present.");
		}
		return _setTimeout.apply(this, arguments);
	}`)
	r.Run(`var setInterval = function(args) {
		if (arguments.length < 1) {
			throw TypeError("Failed to execute 'setInterval': 1 argument required, but only 0 present.");
		}
		return _setInterval.apply(this, arguments);
	}`)
	r.Set("clearTimeout", clearTimeout)
	r.Set("clearInterval", clearTimeout)
	r.Set("sleep", func(call otto.FunctionCall) otto.Value {
		if len(call.ArgumentList) != 1 || !call.Argument(0).IsNumber() {
			return r.MakeCustomError("MissingArgument", "Sleep requires a number parameter")
		}
		t, _ := call.Argument(0).ToInteger()
		time.Sleep(time.Duration(t) * time.Millisecond)
		return otto.NullValue()
	})
}

func (r *Runtime) runEventLoop() {
	defer close(r.closed)

	var waitForCallbacks bool

loop:
	for {
		select {
		case timer := <-r.timerReady:
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
			_, err := r.Call(`Function.call.call`, nil, arguments...)
			if err != nil {
				logging.GetLogger().Errorf("JavaScript error: %s", err)
			}

			_, inreg := r.timers[timer]
			if timer.interval && inreg {
				timer.timer.Reset(timer.duration)
			} else {
				delete(r.timers, timer)
				if waitForCallbacks && (len(r.timers) == 0) {
					break loop
				}
			}

		case req := <-r.evalQueue:
			// run the code, send the result back
			req.fn(r.Otto)
			close(req.done)
			if waitForCallbacks && (len(r.timers) == 0) {
				break loop
			}

		case waitForCallbacks = <-r.stopEventLoop:
			if !waitForCallbacks || (len(r.timers) == 0) {
				break loop
			}
		}
	}

	for _, timer := range r.timers {
		timer.timer.Stop()
		delete(r.timers, timer)
	}
}

// CompleteKeywords returns potential continuations for the given line.
// Since line is evaluated, callers need to make sure that evaluating line
// does not have side effects.
func (r *Runtime) CompleteKeywords(line string) []string {
	var results []string
	r.Do(func(vm *otto.Otto) {
		results = getCompletions(vm, line)
	})
	return results
}

// Do executes the `fn` in the event loop
func (r *Runtime) Do(fn func(*otto.Otto)) {
	done := make(chan bool)
	req := &evalReq{fn, done}
	r.evalQueue <- req
	<-done
}

// Exec queues the execution of some JavaScript code
func (r *Runtime) Exec(code string) (v otto.Value, err error) {
	r.Do(func(vm *otto.Otto) { v, err = vm.Run(code) })
	return v, err
}

// ExecFunction queues a CallFunction method
func (r *Runtime) ExecFunction(source string, params ...interface{}) (v otto.Value, err error) {
	r.Do(func(vm *otto.Otto) { v, err = r.CallFunction(source, params...) })
	return v, err
}

// ExecPromise executes a promise and return its result
func (r *Runtime) ExecPromise(source string, params ...interface{}) (v otto.Value, err error) {
	var done chan otto.Value
	r.Do(func(vm *otto.Otto) { done, err = r.CallPromise(source, params...) })
	v = <-done
	return v, err
}

// CallFunction takes the source of a function and evaluate it with the specified parameters
func (r *Runtime) CallFunction(source string, params ...interface{}) (otto.Value, error) {
	result, err := r.Run("(" + source + ")")
	if err != nil {
		return otto.UndefinedValue(), fmt.Errorf("Error while compile source %s: %s", source, result.String())
	}

	return result.Call(result, params...)
}

// CallPromise takes the source of a promise and evaluate it with the specified parameters
func (r *Runtime) CallPromise(source string, params ...interface{}) (chan otto.Value, error) {
	result, err := r.CallFunction(source, params...)
	if err != nil {
		return nil, fmt.Errorf("Error while executing function: %s", err)
	}

	if !result.IsObject() {
		return nil, fmt.Errorf("Workflow is expected to return a promise, returned %s", result.Class())
	}

	done := make(chan otto.Value)
	promise := result.Object()
	finally, _ := r.ToValue(func(call otto.FunctionCall) otto.Value {
		result = call.Argument(0)
		done <- result
		return result
	})

	result, _ = promise.Call("then", finally)
	promise = result.Object()
	promise.Call("catch", finally)

	return done, nil
}

// Start the runtime evaluation loop
func (r *Runtime) Start() {
	go r.runEventLoop()
}

// Stop the runtime evaluation loop
func (r *Runtime) Stop() {
	r.stopEventLoop <- true
}

// NewRuntime returns a new JavaScript runtime environment
func NewRuntime() (*Runtime, error) {
	r := &Runtime{
		Otto:          otto.New(),
		closed:        make(chan struct{}),
		evalQueue:     make(chan *evalReq, 20),
		stopEventLoop: make(chan bool),
		timers:        make(map[*jsTimer]*jsTimer),
		timerReady:    make(chan *jsTimer),
	}

	r.registerStandardLibray()

	bytes := make([]byte, 8)
	seed := time.Now().UnixNano()
	if _, err := crand.Read(bytes); err == nil {
		seed = int64(binary.LittleEndian.Uint64(bytes))
	}

	src := rand.NewSource(seed)
	rnd := rand.New(src)
	r.SetRandomSource(rnd.Float64)

	if err := r.runEmbededScript("js/promise-7.0.4.min.js"); err != nil {
		return nil, err
	}

	if err := r.runEmbededScript("js/promise-done-7.0.4.min.js"); err != nil {
		return nil, err
	}

	if err := r.runEmbededScript("js/api.js"); err != nil {
		return nil, err
	}

	if err := r.runEmbededScript("js/otto.js"); err != nil {
		return nil, err
	}

	if err := r.runEmbededScript("statics/js/vendor/pure-uuid.js"); err != nil {
		return nil, err
	}

	return r, nil
}
