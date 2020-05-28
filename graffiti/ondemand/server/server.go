/*
 * Copyright (C) 2019 Red Hat, Inc.
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

package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/safchain/insanelock"

	"github.com/skydive-project/skydive/graffiti/api/rest"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/graffiti/ondemand"
	ws "github.com/skydive-project/skydive/graffiti/websocket"
)

type activeTask struct {
	graph    *graph.Graph
	node     *graph.Node
	resource rest.Resource
	task     ondemand.Task
	handler  OnDemandServerHandler
}

// OnDemandServer describes an ondemand task server based on websocket
type OnDemandServer struct {
	insanelock.RWMutex
	graph.DefaultGraphListener
	ws.DefaultSpeakerEventHandler
	Graph        *graph.Graph
	clientPool   *ws.StructClientPool
	activeTasks  map[graph.Identifier]map[string]*activeTask
	wsNamespace  string
	resourceName string
	handler      OnDemandServerHandler
}

// OnDemandServerHandler is the interface to be implemented by ondemand servers
type OnDemandServerHandler interface {
	ResourceName() string
	DecodeMessage(msg json.RawMessage) (rest.Resource, error)
	CreateTask(*graph.Node, rest.Resource) (interface{}, error)
	RemoveTask(*graph.Node, rest.Resource, interface{}) error
}

// ErrTaskNotFound used when a task is not found for a specific node
var ErrTaskNotFound = errors.New("task not found")

func (o *OnDemandServer) registerTask(n *graph.Node, resource rest.Resource) bool {
	logging.GetLogger().Debugf("Attempting to register %s %s on node %s", o.resourceName, resource.GetID(), n.ID)

	if _, err := n.GetFieldString("Type"); err != nil {
		logging.GetLogger().Infof("Unable to register task type of node unknown %v", n)
		return false
	}

	o.Lock()
	defer o.Unlock()

	if tasks, active := o.activeTasks[n.ID]; active {
		if _, found := tasks[resource.GetID()]; found {
			logging.GetLogger().Debugf("A task already exists for %s on node %s", resource.GetID(), n.ID)
			return false
		}
	}

	task, err := o.handler.CreateTask(n, resource)
	if err != nil {
		logging.GetLogger().Errorf("Failed to register %s task: %s", o.resourceName, err)
		return false
	}

	active := &activeTask{
		graph:    o.Graph,
		node:     n,
		resource: resource,
		task:     task,
		handler:  o.handler,
	}

	if _, found := o.activeTasks[n.ID]; !found {
		o.activeTasks[n.ID] = make(map[string]*activeTask)
	}
	o.activeTasks[n.ID][resource.GetID()] = active

	logging.GetLogger().Debugf("New active task on: %v (%v)", n, resource)
	return true
}

// unregisterTask should be executed under graph lock
func (o *OnDemandServer) unregisterTask(n *graph.Node, resource rest.Resource) error {
	o.RLock()
	var active *activeTask
	tasks, isActive := o.activeTasks[n.ID]
	if isActive {
		active, isActive = tasks[resource.GetID()]
	}
	o.RUnlock()

	if !isActive {
		return ErrTaskNotFound
	}

	name, _ := n.GetFieldString("Name")
	logging.GetLogger().Debugf("Attempting to unregister task on node %s (%s)", name, n.ID)

	if err := o.handler.RemoveTask(n, active.resource, active.task); err != nil {
		return err
	}

	o.Lock()
	if tasks, found := o.activeTasks[n.ID]; found {
		delete(tasks, resource.GetID())
	}
	if len(o.activeTasks[n.ID]) == 0 {
		delete(o.activeTasks, n.ID)
	}
	o.Unlock()

	return nil
}

// OnStructMessage websocket message, valid message type are Start, Stop
func (o *OnDemandServer) OnStructMessage(c ws.Speaker, msg *ws.StructMessage) {
	var enveloppe ondemand.RawQuery
	if err := json.Unmarshal(msg.Obj, &enveloppe); err != nil {
		logging.GetLogger().Errorf("Unable to decode message %v", msg)
		return
	}

	resource, err := o.handler.DecodeMessage(enveloppe.Resource)
	if err != nil {
		logging.GetLogger().Errorf("Unable to decode message %v", o.resourceName)
		return
	}

	query := ondemand.Query{NodeID: enveloppe.NodeID, Resource: resource}

	status := http.StatusBadRequest

	o.Graph.Lock()

	switch msg.Type {
	case "Start":
		n := o.Graph.GetNode(graph.Identifier(query.NodeID))
		if n == nil {
			logging.GetLogger().Errorf("Unknown node %s for new %s", query.NodeID, o.resourceName)
			status = http.StatusNotFound
			break
		}

		status = http.StatusOK
		if _, err := n.GetFieldString(fmt.Sprintf("%s.ID", o.resourceName)); err == nil {
			logging.GetLogger().Debugf("%s already started on node %s", n.ID, o.resourceName)
		} else {
			if ok := o.registerTask(n, resource); !ok {
				status = http.StatusInternalServerError
			}
		}

	case "Stop":
		n := o.Graph.GetNode(graph.Identifier(query.NodeID))
		if n == nil {
			logging.GetLogger().Errorf("Unknown node %s for new %s", query.NodeID, o.resourceName)
			status = http.StatusNotFound
			break
		}

		status = http.StatusOK
		if err := o.unregisterTask(n, resource); err != nil {
			if err == ErrTaskNotFound {
				logging.GetLogger().Warningf("Failed to unregister %s on node %s, %s", o.resourceName, n.ID, err)
				status = http.StatusNotFound
			} else {
				logging.GetLogger().Errorf("Failed to unregister %s on node %s, %s", o.resourceName, n.ID, err)
				status = http.StatusInternalServerError
			}
		}
	}

	// be sure to unlock before sending message
	o.Graph.Unlock()

	reply := msg.Reply(&query, msg.Type+"Reply", status)
	c.SendMessage(reply)
}

// OnNodeDeleted graph event
func (o *OnDemandServer) OnNodeDeleted(n *graph.Node) {
	o.RLock()
	tasks, found := o.activeTasks[n.ID]
	if found {
		for _, task := range tasks {
			capture := task.resource
			defer func() {
				if err := o.unregisterTask(n, capture); err != nil {
					logging.GetLogger().Errorf("Failed to unregister %s %s on node %s", o.resourceName, capture.GetID(), n.ID)
				}
			}()
		}
	}
	o.RUnlock()
}

// Start the task
func (o *OnDemandServer) Start() error {
	o.Graph.AddEventListener(o)
	o.clientPool.AddStructMessageHandler(o, []string{o.wsNamespace})
	return nil
}

// Stop the task
func (o *OnDemandServer) Stop() {
	o.Graph.RemoveEventListener(o)

	o.Graph.Lock()
	for _, tasks := range o.activeTasks {
		for _, active := range tasks {
			o.unregisterTask(active.node, active.resource)
		}
	}
	o.Graph.Unlock()
}

// NewOnDemandServer creates a new Ondemand tasks server based on graph and websocket
func NewOnDemandServer(g *graph.Graph, pool *ws.StructClientPool, handler OnDemandServerHandler) (*OnDemandServer, error) {
	return &OnDemandServer{
		Graph:        g,
		clientPool:   pool,
		activeTasks:  make(map[graph.Identifier]map[string]*activeTask),
		wsNamespace:  ondemand.Namespace + handler.ResourceName(),
		resourceName: handler.ResourceName(),
		handler:      handler,
	}, nil
}
