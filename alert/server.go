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
	"sync"
	"time"

	"github.com/robertkrimen/otto"
	"github.com/skydive-project/skydive/api"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/etcd"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"
	"github.com/skydive-project/skydive/topology/graph/traversal"
)

const (
	Namespace = "Alert"
)

const (
	WEBHOOK = 1 + iota
	SCRIPT
)

type GremlinAlert struct {
	*api.Alert
	lastEval          interface{}
	kind              int
	data              string
	traversalSequence *traversal.GremlinTraversalSequence
	gremlinParser     *traversal.GremlinTraversalParser
}

func (ga *GremlinAlert) Evaluate() (interface{}, error) {
	// If the alert is a simple Gremlin query, avoid
	// converting to JavaScript
	if ga.traversalSequence != nil {
		result, err := ga.traversalSequence.Exec()
		if err != nil {
			return nil, err
		}

		values := result.Values()
		if len(values) > 0 {
			return values, nil
		}

		return nil, nil
	}

	// Fallback to JavaScript
	vm := otto.New()
	vm.Set("Gremlin", func(call otto.FunctionCall) otto.Value {
		if len(call.ArgumentList) < 1 || !call.Argument(0).IsString() {
			return vm.MakeCustomError("MissingQueryArgument", "Gremlin requires a string parameter")
		}

		query := call.Argument(0).String()

		// TODO(sbaubeau) Cache the queries
		ts, err := ga.gremlinParser.Parse(strings.NewReader(query), false)
		if err != nil {
			return vm.MakeCustomError("ParseError", err.Error())
		}

		result, err := ts.Exec()
		if err != nil {
			return vm.MakeCustomError("ExecuteError", err.Error())
		}

		source, err := result.MarshalJSON()
		if err != nil {
			return vm.MakeCustomError("MarshalError", err.Error())
		}

		jsonObj, err := vm.Object("obj = " + string(source))
		if err != nil {
			return vm.MakeCustomError("JSONError", err.Error())
		}

		logging.GetLogger().Infof("Gremlin returned %+v", jsonObj)
		r, _ := vm.ToValue(jsonObj)
		return r
	})

	// Create an alias '$' for 'Gremlin'
	gremlin, _ := vm.Get("Gremlin")
	vm.Set("$", gremlin)

	result, err := vm.Run(ga.Expression)
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

func (ga *GremlinAlert) Trigger(payload []byte) error {
	switch ga.kind {
	case WEBHOOK:
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
	case SCRIPT:
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

func NewGremlinAlert(alert *api.Alert, p *traversal.GremlinTraversalParser) (*GremlinAlert, error) {
	ts, _ := p.Parse(strings.NewReader(alert.Expression), false)

	ga := &GremlinAlert{
		Alert:             alert,
		traversalSequence: ts,
		gremlinParser:     p,
	}

	if strings.HasPrefix(alert.Action, "http://") || strings.HasPrefix(alert.Action, "https://") {
		ga.kind = WEBHOOK
		ga.data = alert.Action
	} else if strings.HasPrefix(alert.Action, "file://") {
		ga.kind = SCRIPT
		ga.data = alert.Action[7:]
	}

	return ga, nil
}

type AlertServer struct {
	sync.RWMutex
	Graph         *graph.Graph
	WSServer      *shttp.WSServer
	AlertHandler  api.APIHandler
	watcher       api.StoppableWatcher
	graphAlerts   map[string]*GremlinAlert
	alertTimers   map[string]chan bool
	gremlinParser *traversal.GremlinTraversalParser
	elector       *etcd.EtcdMasterElector
}

type AlertMessage struct {
	UUID       string
	Timestamp  time.Time
	ReasonData interface{}
}

func (a *AlertServer) TriggerAlert(al *GremlinAlert, data interface{}) error {
	msg := AlertMessage{
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
		if err := al.Trigger(payload); err != nil {
			logging.GetLogger().Infof("Failed to trigger alert: %s", err.Error())
		}
	}()

	wsMsg := shttp.NewWSMessage(Namespace, "Alert", msg)
	a.WSServer.Lock()
	a.WSServer.BroadcastWSMessage(wsMsg)
	a.WSServer.Unlock()

	logging.GetLogger().Debugf("Alert %s of type %s was triggerred", al.UUID, al.Action)
	return nil
}

func (a *AlertServer) evaluateAlert(al *GremlinAlert) error {
	if !a.elector.IsMaster() {
		return nil
	}

	data, err := al.Evaluate()
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
			return a.TriggerAlert(al, data)
		}
	} else {
		// Gremlin query returned no datas, or Javascript expression was unsuccessful
		// Reset the lastEval to be able to trigger the alert next time
		al.lastEval = nil
	}

	return nil
}

func (a *AlertServer) EvaluateAlerts(alerts map[string]*GremlinAlert) {
	a.RLock()
	defer a.RUnlock()

	for _, al := range alerts {
		if err := a.evaluateAlert(al); err != nil {
			logging.GetLogger().Warning(err.Error())
		}
	}
}

func (a *AlertServer) OnNodeUpdated(n *graph.Node) {
	a.EvaluateAlerts(a.graphAlerts)
}

func (a *AlertServer) OnNodeAdded(n *graph.Node) {
	a.EvaluateAlerts(a.graphAlerts)
}

func (a *AlertServer) OnNodeDeleted(n *graph.Node) {
	a.EvaluateAlerts(a.graphAlerts)
}

func (a *AlertServer) OnEdgeAdded(e *graph.Edge) {
	a.EvaluateAlerts(a.graphAlerts)
}

func (a *AlertServer) OnEdgeUpdated(e *graph.Edge) {
	a.EvaluateAlerts(a.graphAlerts)
}

func (a *AlertServer) OnEdgeDeleted(e *graph.Edge) {
	a.EvaluateAlerts(a.graphAlerts)
}

func parseTrigger(trigger string) (string, string) {
	splits := strings.SplitN(trigger, ":", 2)
	if len(splits) == 2 {
		return splits[0], splits[1]
	}
	return splits[0], ""
}

func (a *AlertServer) RegisterAlert(apiAlert *api.Alert) error {
	alert, err := NewGremlinAlert(apiAlert, a.gremlinParser)
	if err != nil {
		return err
	}

	logging.GetLogger().Debugf("Registering new alert: %+v", alert)

	a.Graph.RLock()
	a.evaluateAlert(alert)
	a.Graph.RUnlock()

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
					a.Graph.RLock()
					if err := a.evaluateAlert(alert); err != nil {
						logging.GetLogger().Warning(err.Error())
					}
					a.Graph.RUnlock()
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

func (a *AlertServer) UnregisterAlert(id string) {
	logging.GetLogger().Debugf("Alert deleted: %s", id)

	a.Lock()
	defer a.Unlock()

	if ch, found := a.alertTimers[id]; found {
		close(ch)
		delete(a.alertTimers, id)
	} else {
		delete(a.graphAlerts, id)
	}
}

func (a *AlertServer) onAPIWatcherEvent(action string, id string, resource api.APIResource) {
	switch action {
	case "init", "create", "set", "update":
		if err := a.RegisterAlert(resource.(*api.Alert)); err != nil {
			logging.GetLogger().Errorf("Failed to register alert: %s", err.Error())
		}
	case "expire", "delete":
		a.UnregisterAlert(id)
	}
}

func (a *AlertServer) Start() {
	a.elector.StartAndWait()

	a.watcher = a.AlertHandler.AsyncWatch(a.onAPIWatcherEvent)
	a.Graph.AddEventListener(a)
}

func (a *AlertServer) Stop() {
	a.elector.Stop()
}

func NewAlertServer(ah api.APIHandler, wsServer *shttp.WSServer, parser *traversal.GremlinTraversalParser, etcdClient *etcd.EtcdClient) *AlertServer {
	elector := etcd.NewEtcdMasterElectorFromConfig(common.AnalyzerService, "alert-server", etcdClient)

	as := &AlertServer{
		WSServer:      wsServer,
		AlertHandler:  ah,
		Graph:         parser.Graph,
		graphAlerts:   make(map[string]*GremlinAlert),
		alertTimers:   make(map[string]chan bool),
		gremlinParser: parser,
		elector:       elector,
	}

	return as
}
