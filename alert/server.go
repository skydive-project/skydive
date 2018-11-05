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

package alert

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"reflect"
	"strings"
	"time"

	api "github.com/skydive-project/skydive/api/server"
	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/etcd"
	"github.com/skydive-project/skydive/js"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"
	"github.com/skydive-project/skydive/topology/graph/traversal"
	ws "github.com/skydive-project/skydive/websocket"
)

const (
	// Namespace is the alerting WebSocket namespace
	Namespace = "Alert"
)

const (
	actionWebHook = 1 + iota
	actionScript
)

// GremlinAlert represents an alert that will be triggered if its associated
// Gremlin expression returns a non empty result.
type GremlinAlert struct {
	*types.Alert
	graph             *graph.Graph
	lastEval          interface{}
	kind              int
	data              string
	traversalSequence *traversal.GremlinTraversalSequence
	gremlinParser     *traversal.GremlinTraversalParser
}

func (ga *GremlinAlert) evaluate(server *api.Server, vm *js.Runtime, lockGraph bool) (interface{}, error) {
	// If the alert is a simple Gremlin query, avoid
	// converting to JavaScript
	if ga.traversalSequence != nil {
		result, err := ga.traversalSequence.Exec(ga.graph, lockGraph)
		if err != nil {
			return nil, err
		}

		values := result.Values()
		if len(values) > 0 {
			return result, nil
		}

		return nil, nil
	}

	// Fallback to JavaScript
	result, err := vm.Exec(ga.Expression)
	if err != nil {
		return nil, fmt.Errorf("Error while executing Javascript '%s': %s", ga.Expression, err.Error())
	}

	if result.Class() == "Error" {
		s, _ := result.ToString()
		return nil, errors.New(s)
	}

	success, _ := result.ToBoolean()
	logging.GetLogger().Debugf("Evaluation of '%s' returned %+v => %+v (%+v)", ga.Expression, result, success, result.Class())

	if success {
		v, err := result.Export()
		if err != nil {
			return nil, err
		}

		switch v := v.(type) {
		case []map[string]interface{}:
			if len(v) > 0 {
				return v, nil
			}
		case []interface{}:
			if len(v) > 0 {
				return v, nil
			}
		case map[string]interface{}:
			if len(v) > 0 {
				return v, nil
			}
		default:
			return v, nil
		}
	}

	return nil, nil
}

func (ga *GremlinAlert) trigger(payload []byte) error {
	switch ga.kind {
	case actionWebHook:
		client := &http.Client{}

		req, err := http.NewRequest("POST", ga.data, bytes.NewReader(payload))
		if err != nil {
			return fmt.Errorf("Failed to post alert to %s: %s", ga.data, err.Error())
		}

		req.Close = true
		_, err = client.Do(req)
		if err != nil {
			return fmt.Errorf("Error while posting alert to %s: %s", ga.data, err.Error())
		}
	case actionScript:
		logging.GetLogger().Debugf("Executing command '%s'", ga.data)

		cmd := exec.Command(ga.data)
		stdin, err := cmd.StdinPipe()
		if err != nil {
			return fmt.Errorf("Failed to get stdin for command '%s': %s", ga.data, err.Error())
		}

		if _, err = stdin.Write(payload); err != nil {
			return fmt.Errorf("Failed to write to stdin for '%s': %s", ga.data, err.Error())
		}
		stdin.Write([]byte("\n"))

		output, err := cmd.CombinedOutput()
		if err != nil {
			return err
		}

		logging.GetLogger().Infof("Command successfully executed '%s': %s", cmd.Path, output)
		stdin.Close()
	}

	return nil
}

// NewGremlinAlert returns a new gremlin based alert
func NewGremlinAlert(alert *types.Alert, g *graph.Graph, p *traversal.GremlinTraversalParser) (*GremlinAlert, error) {
	ts, _ := p.Parse(strings.NewReader(alert.Expression))

	ga := &GremlinAlert{
		Alert:             alert,
		traversalSequence: ts,
		gremlinParser:     p,
		graph:             g,
	}

	if strings.HasPrefix(alert.Action, "http://") || strings.HasPrefix(alert.Action, "https://") {
		ga.kind = actionWebHook
		ga.data = alert.Action
	} else if strings.HasPrefix(alert.Action, "file://") {
		ga.kind = actionScript
		ga.data = alert.Action[7:]
	}

	return ga, nil
}

// Server describes an alerting alerts that evaluates registered
// alerts on graph events or periodically and trigger them if their condition
// evaluates to true
type Server struct {
	common.RWMutex
	*etcd.MasterElector
	Graph         *graph.Graph
	Pool          ws.StructSpeakerPool
	AlertHandler  api.Handler
	apiServer     *api.Server
	watcher       api.StoppableWatcher
	graphAlerts   map[string]*GremlinAlert
	alertTimers   map[string]chan bool
	gremlinParser *traversal.GremlinTraversalParser
	runtime       *js.Runtime
}

// Message describes a websocket message that is sent by the alerting
// server when an alert was triggered
type Message struct {
	UUID       string
	Timestamp  time.Time
	ReasonData interface{}
}

func (a *Server) triggerAlert(al *GremlinAlert, data interface{}) error {
	msg := Message{
		UUID:       al.UUID,
		Timestamp:  time.Now().UTC(),
		ReasonData: data,
	}

	logging.GetLogger().Infof("Triggering alert %s of type %s", al.UUID, al.Action)

	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("Failed to marshal alert to JSON: %s", err.Error())
	}

	go func() {
		if err := al.trigger(payload); err != nil {
			logging.GetLogger().Infof("Failed to trigger alert: %s", err.Error())
		}
	}()

	wsMsg := ws.NewStructMessage(Namespace, "Alert", msg)
	a.Pool.BroadcastMessage(wsMsg)

	logging.GetLogger().Debugf("Alert %s of type %s was triggerred", al.UUID, al.Action)
	return nil
}

func (a *Server) evaluateAlert(al *GremlinAlert, lockGraph bool) error {
	if !a.IsMaster() {
		return nil
	}

	data, err := al.evaluate(a.apiServer, a.runtime, lockGraph)
	if err != nil {
		return err
	}

	if data != nil {
		// Gremlin query/Javascript expression returned datas.
		// Alert must but sent if those datas differ from the one that trigger
		// the previous alert.
		equal := reflect.DeepEqual(reflect.ValueOf(data).Interface(), al.lastEval)
		if !equal {
			al.lastEval = data
			return a.triggerAlert(al, data)
		}
	} else {
		// Gremlin query returned no datas, or Javascript expression was unsuccessful
		// Reset the lastEval to be able to trigger the alert next time
		al.lastEval = nil
	}

	return nil
}

// Evaluate all the registered alerts
func (a *Server) evaluateAlerts(alerts map[string]*GremlinAlert, lockGraph bool) {
	a.RLock()
	defer a.RUnlock()

	for _, al := range alerts {
		if err := a.evaluateAlert(al, lockGraph); err != nil {
			logging.GetLogger().Warning(err.Error())
		}
	}
}

// OnNodeUpdated event
func (a *Server) OnNodeUpdated(n *graph.Node) {
	a.evaluateAlerts(a.graphAlerts, false)
}

// OnNodeAdded event
func (a *Server) OnNodeAdded(n *graph.Node) {
	a.evaluateAlerts(a.graphAlerts, false)
}

// OnNodeDeleted event
func (a *Server) OnNodeDeleted(n *graph.Node) {
	a.evaluateAlerts(a.graphAlerts, false)
}

// OnEdgeAdded event
func (a *Server) OnEdgeAdded(e *graph.Edge) {
	a.evaluateAlerts(a.graphAlerts, false)
}

// OnEdgeUpdated event
func (a *Server) OnEdgeUpdated(e *graph.Edge) {
	a.evaluateAlerts(a.graphAlerts, false)
}

// OnEdgeDeleted event
func (a *Server) OnEdgeDeleted(e *graph.Edge) {
	a.evaluateAlerts(a.graphAlerts, false)
}

func parseTrigger(trigger string) (string, string) {
	splits := strings.SplitN(trigger, ":", 2)
	if len(splits) == 2 {
		return splits[0], splits[1]
	}
	return splits[0], ""
}

func (a *Server) registerAlert(apiAlert *types.Alert) error {
	alert, err := NewGremlinAlert(apiAlert, a.Graph, a.gremlinParser)
	if err != nil {
		return err
	}

	logging.GetLogger().Debugf("Registering new alert: %+v", alert)

	a.evaluateAlert(alert, true)

	trigger, data := parseTrigger(apiAlert.Trigger)
	switch trigger {
	case "duration":
		duration, err := time.ParseDuration(data)
		if err != nil {
			return err
		}

		done := make(chan bool)
		go func() {
			ticker := time.NewTicker(duration)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					if err := a.evaluateAlert(alert, true); err != nil {
						logging.GetLogger().Warning(err.Error())
					}
				case <-done:
					return
				}
			}
		}()
		a.Lock()
		a.alertTimers[apiAlert.UUID] = done
		a.Unlock()
	case "graph":
		fallthrough
	default:
		a.Lock()
		a.graphAlerts[apiAlert.UUID] = alert
		a.Unlock()
	}

	return nil
}

func (a *Server) unregisterAlert(id string) {
	logging.GetLogger().Debugf("Unregistering alert: %s", id)

	a.Lock()
	defer a.Unlock()

	if ch, found := a.alertTimers[id]; found {
		close(ch)
		delete(a.alertTimers, id)
	} else {
		delete(a.graphAlerts, id)
	}
}

func (a *Server) onAPIWatcherEvent(action string, id string, resource types.Resource) {
	switch action {
	case "init", "create", "set", "update":
		if err := a.registerAlert(resource.(*types.Alert)); err != nil {
			logging.GetLogger().Errorf("Failed to register alert: %s", err.Error())
		}
	case "expire", "delete":
		a.unregisterAlert(id)
	}
}

// Start the alerting server
func (a *Server) Start() {
	a.StartAndWait()

	a.watcher = a.AlertHandler.AsyncWatch(a.onAPIWatcherEvent)
	a.Graph.AddEventListener(a)
}

// Stop the alerting server
func (a *Server) Stop() {
	a.MasterElector.Stop()
}

// NewServer creates a new alerting server
func NewServer(apiServer *api.Server, pool ws.StructSpeakerPool, graph *graph.Graph, parser *traversal.GremlinTraversalParser, etcdClient *etcd.Client) (*Server, error) {
	elector := etcd.NewMasterElectorFromConfig(common.AnalyzerService, "alert-server", etcdClient)

	runtime, err := js.NewRuntime()
	if err != nil {
		return nil, err
	}

	runtime.Start()
	runtime.RegisterAPIServer(graph, parser, apiServer)

	as := &Server{
		MasterElector: elector,
		Pool:          pool,
		AlertHandler:  apiServer.GetHandler("alert"),
		Graph:         graph,
		graphAlerts:   make(map[string]*GremlinAlert),
		alertTimers:   make(map[string]chan bool),
		gremlinParser: parser,
		apiServer:     apiServer,
		runtime:       runtime,
	}

	return as, nil
}
