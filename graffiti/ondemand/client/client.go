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

package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	cache "github.com/pmylund/go-cache"
	"github.com/safchain/insanelock"
	"github.com/skydive-project/go-debouncer"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/graffiti/api/rest"
	etcd "github.com/skydive-project/skydive/graffiti/etcd/client"
	"github.com/skydive-project/skydive/graffiti/filters"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/graffiti/ondemand"
	ws "github.com/skydive-project/skydive/graffiti/websocket"
)

// OnDemandNodeResource describes an association between a graph
// node and its corresponding on-demand resource
type OnDemandNodeResource struct {
	Node     *graph.Node
	Resource rest.Resource
}

// OnDemandClientHandler is the interface to be implemented by ondemand clients
type OnDemandClientHandler interface {
	ResourceName() string
	GetNodeResources(resource rest.Resource) []OnDemandNodeResource
	CheckState(n *graph.Node, resource rest.Resource) bool
	DecodeMessage(msg json.RawMessage) (rest.Resource, error)
	EncodeMessage(nodeID graph.Identifier, resource rest.Resource) (json.RawMessage, error)
}

// OnDemandClient describes an ondemand task client based on a websocket
type OnDemandClient struct {
	insanelock.RWMutex
	common.MasterElection
	graph.DefaultGraphListener
	graph                   *graph.Graph
	apiHandler              rest.WatchableHandler
	agentPool               ws.StructSpeakerPool
	subscriberPool          ws.StructSpeakerPool
	wsNamespace             string
	wsNotificationNamespace string
	resources               map[string]rest.Resource
	watcher                 rest.StoppableWatcher
	registeredNodes         map[graph.Identifier]map[string]bool
	deletedNodeCache        *cache.Cache
	checkForRegistration    *debouncer.Debouncer
	resourceName            string
	handler                 OnDemandClientHandler
}

type handlerNodeState struct {
	uuid    string
	started bool
}

type nodeTask struct {
	id       graph.Identifier
	host     string
	resource rest.Resource
}

func (o *OnDemandClient) removeRegisteredNode(nodeID graph.Identifier, resourceID string) {
	o.Lock()
	tasks, found := o.registeredNodes[nodeID]
	if !found {
		o.Unlock()
		return
	}
	delete(tasks, resourceID)
	if len(o.registeredNodes) == 0 {
		delete(o.registeredNodes, nodeID)
	}
	o.Unlock()
}

// OnStructMessage event, valid message type: StartReply or StopReply message
func (o *OnDemandClient) OnStructMessage(c ws.Speaker, m *ws.StructMessage) {
	var rawQuery ondemand.RawQuery
	if err := json.Unmarshal(m.Obj, &rawQuery); err != nil {
		logging.GetLogger().Errorf("unable to decode %s message: %s", o.resourceName, err)
		return
	}

	resource, err := o.handler.DecodeMessage(rawQuery.Resource)
	if err != nil {
		logging.GetLogger().Errorf("unable to decode %s: %s", o.resourceName, err)
		return
	}

	switch m.Type {
	case "StartReply":
		// not registered thus remove from registered cache
		if m.Status != http.StatusOK {
			logging.GetLogger().Debugf("%s start request failed %v", o.resourceName, m.Debug())
			o.removeRegisteredNode(rawQuery.NodeID, resource.GetID())
		} else {
			logging.GetLogger().Debugf("%s start request succeeded %v", o.resourceName, m.Debug())
		}
		o.subscriberPool.BroadcastMessage(ws.NewStructMessage(o.wsNotificationNamespace, "NodeUpdated", resource))
	case "StopReply":
		if m.Status == http.StatusOK {
			o.removeRegisteredNode(rawQuery.NodeID, resource.GetID())
		} else {
			logging.GetLogger().Debugf("%s stop request failed %v", o.resourceName, m.Debug())
		}
	}
}

func (o *OnDemandClient) registerTasks(nps map[graph.Identifier]nodeTask) {
	for _, np := range nps {
		o.registerTask(np)
	}
}

func (o *OnDemandClient) registerTask(np nodeTask) bool {
	body, err := o.handler.EncodeMessage(np.id, np.resource)
	if err != nil {
		logging.GetLogger().Errorf("Unable to encode message for agent %s: %s", np.host, err)
		return false
	}

	msg := ws.NewStructMessage(o.wsNamespace, "Start", ondemand.RawQuery{
		NodeID:   np.id,
		Resource: body,
	})

	if err := o.agentPool.SendMessageTo(msg, np.host); err != nil {
		logging.GetLogger().Errorf("Unable to send message to agent %s: %s", np.host, err)
		return false
	}

	o.Lock()
	if _, found := o.registeredNodes[np.id]; !found {
		o.registeredNodes[np.id] = make(map[string]bool)
	}
	o.registeredNodes[np.id][np.resource.GetID()] = false
	o.Unlock()

	logging.GetLogger().Debugf("Registered task on %s with resource %s", np.id, np.resource.GetID())

	return true
}

func (o *OnDemandClient) unregisterTask(node *graph.Node, resource rest.Resource) bool {
	body, err := o.handler.EncodeMessage(node.ID, resource)
	if err != nil {
		logging.GetLogger().Errorf("Unable to encode message for agent %s: %s", node.Host, err)
		return false
	}

	msg := ws.NewStructMessage(o.wsNamespace, "Stop", ondemand.RawQuery{
		NodeID:   node.ID,
		Resource: body,
	})

	if err := o.agentPool.SendMessageTo(msg, node.Host); err != nil {
		logging.GetLogger().Errorf("Unable to send message to agent %s: %s", node.Host, err)
		return false
	}

	o.removeRegisteredNode(node.ID, resource.GetID())

	return true
}

func (o *OnDemandClient) nodeTasks(nrs []OnDemandNodeResource) map[graph.Identifier]nodeTask {
	toRegister := func(nr OnDemandNodeResource) (nodeID graph.Identifier, host string, register bool) {
		// check not already registered
		tasks, ok := o.registeredNodes[nr.Node.ID]
		if ok {
			ok = tasks[nr.Resource.GetID()]
		}

		if ok {
			logging.GetLogger().Debugf("%s already registered on %s", nr.Resource.GetID(), nr.Node.ID)
			return
		}

		return nr.Node.ID, nr.Node.Host, true
	}

	nps := map[graph.Identifier]nodeTask{}
	for _, nr := range nrs {
		if nodeID, host, ok := toRegister(nr); ok {
			nps[nodeID] = nodeTask{nodeID, host, nr.Resource}
		}
	}

	return nps
}

// checkForRegistration check the resource gremlin expression in order to
// register new task.
func (o *OnDemandClient) checkForRegistrationCallback() {
	if !o.IsMaster() {
		return
	}

	o.graph.RLock()
	defer o.graph.RUnlock()

	o.RLock()
	defer o.RUnlock()

	for _, resource := range o.resources {
		if nrs := o.handler.GetNodeResources(resource); len(nrs) > 0 {
			if nps := o.nodeTasks(nrs); len(nps) > 0 {
				go o.registerTasks(nps)
			}
		}
	}
}

// OnNodeAdded graph event
func (o *OnDemandClient) OnNodeAdded(n *graph.Node) {
	if !o.IsMaster() {
		return
	}

	// a node comes up with already a resource, this could be due to a re-connect of
	// an agent. Check if the handler is still active.
	if field, err := n.GetField(o.resourceName); err == nil {
		if resources, ok := field.(map[string]interface{}); ok {
			for id := range resources {
				o.RLock()
				_, found := o.resources[id]
				o.RUnlock()

				if found {
					continue
				}

				// not present unregister it
				logging.GetLogger().Debugf("Unregister remaining %s for node %s: %s", o.resourceName, n.ID, id)
				go o.unregisterTask(n, &rest.BasicResource{UUID: id})
			}
		}
	} else {
		o.checkForRegistration.Call()
	}
}

// OnNodeUpdated graph event
func (o *OnDemandClient) OnNodeUpdated(n *graph.Node) {
	o.RLock()
	if tasks, ok := o.registeredNodes[n.ID]; ok {
		for resourceID, started := range tasks {
			if resource := o.resources[resourceID]; !started && resource != nil {
				if o.handler.CheckState(n, resource) {
					o.registeredNodes[n.ID][resourceID] = true
					o.subscriberPool.BroadcastMessage(ws.NewStructMessage(o.wsNotificationNamespace, "NodeUpdated", resource))
				}
			}
		}
	}
	o.RUnlock()

	o.checkForRegistration.Call()
}

// OnNodeDeleted graph event
func (o *OnDemandClient) OnNodeDeleted(n *graph.Node) {
	o.RLock()
	if tasks, ok := o.registeredNodes[n.ID]; ok {
		for resourceID := range tasks {
			o.subscriberPool.BroadcastMessage(ws.NewStructMessage(o.wsNotificationNamespace, "NodeUpdated", o.resources[resourceID]))
		}
	}
	delete(o.registeredNodes, n.ID)
	o.RUnlock()
}

// OnEdgeAdded graph event
func (o *OnDemandClient) OnEdgeAdded(e *graph.Edge) {
	o.checkForRegistration.Call()
}

func (o *OnDemandClient) registerResource(resource rest.Resource) {
	o.graph.RLock()
	defer o.graph.RUnlock()

	o.Lock()
	defer o.Unlock()

	o.resources[resource.GetID()] = resource

	if nrs := o.handler.GetNodeResources(resource); len(nrs) > 0 {
		if nps := o.nodeTasks(nrs); len(nps) > 0 {
			go o.registerTasks(nps)
		}
	}
}

func (o *OnDemandClient) onResourceAdded(resource rest.Resource) {
	if !o.IsMaster() {
		return
	}

	o.registerResource(resource)
}

func (o *OnDemandClient) unregisterResource(resource rest.Resource) {
	o.graph.RLock()
	defer o.graph.RUnlock()

	o.deletedNodeCache.Delete(resource.GetID())

	o.Lock()
	delete(o.resources, resource.GetID())
	o.Unlock()

	filter := filters.NewTermStringFilter(fmt.Sprintf("%ss.ID", o.resourceName), resource.GetID())
	nodes := o.graph.GetNodes(graph.NewElementFilter(filter))
	for _, node := range nodes {
		go o.unregisterTask(node, resource)
	}
}

func (o *OnDemandClient) onResourceDeleted(resource rest.Resource) {
	if !o.IsMaster() {
		// fill the cache with recent delete in order to be able to delete then
		// in case we lose the master and nobody is master yet. This cache will
		// be used when becoming master.
		o.deletedNodeCache.Set(resource.GetID(), resource, cache.DefaultExpiration)
		return
	}

	o.unregisterResource(resource)
}

// OnStartAsMaster event
func (o *OnDemandClient) OnStartAsMaster() {
}

// OnStartAsSlave event
func (o *OnDemandClient) OnStartAsSlave() {
}

// OnSwitchToMaster event
func (o *OnDemandClient) OnSwitchToMaster() {
	// try to delete recently added resource to handle case where the api got a delete but wasn't yet master
	for _, item := range o.deletedNodeCache.Items() {
		resource := item.Object.(rest.Resource)
		o.unregisterResource(resource)
	}

	for _, resource := range o.apiHandler.Index() {
		resource := resource.(rest.Resource)
		o.onResourceAdded(resource)
	}
}

// OnSwitchToSlave event
func (o *OnDemandClient) OnSwitchToSlave() {
}

func (o *OnDemandClient) onAPIWatcherEvent(action string, id string, resource rest.Resource) {
	logging.GetLogger().Debugf("New watcher event %s for %s", action, id)
	switch action {
	case "init", "create", "set", "update":
		o.subscriberPool.BroadcastMessage(ws.NewStructMessage(o.wsNotificationNamespace, "Added", resource))
		o.onResourceAdded(resource)
	case "expire", "delete":
		o.subscriberPool.BroadcastMessage(ws.NewStructMessage(o.wsNotificationNamespace, "Deleted", resource))
		o.onResourceDeleted(resource)
	}
}

// Start the task
func (o *OnDemandClient) Start() {
	o.MasterElection.AddEventListener(o)
	o.agentPool.AddStructMessageHandler(o, []string{o.wsNamespace})

	o.MasterElection.StartAndWait()

	o.checkForRegistration.Start()

	o.watcher = o.apiHandler.AsyncWatch(o.onAPIWatcherEvent)
	o.graph.AddEventListener(o)
}

// Stop the task
func (o *OnDemandClient) Stop() {
	o.watcher.Stop()
	o.MasterElection.Stop()
	o.checkForRegistration.Stop()
}

// NewOnDemandClient creates a new ondemand task client based on API, graph and websocket
func NewOnDemandClient(g *graph.Graph, ch rest.WatchableHandler, agentPool ws.StructSpeakerPool, subscriberPool ws.StructSpeakerPool, etcdClient *etcd.Client, handler OnDemandClientHandler) *OnDemandClient {
	election := etcdClient.NewElection("ondemand-client-" + handler.ResourceName())
	o := &OnDemandClient{
		MasterElection:          election,
		graph:                   g,
		handler:                 handler,
		apiHandler:              ch,
		agentPool:               agentPool,
		subscriberPool:          subscriberPool,
		wsNamespace:             ondemand.Namespace + handler.ResourceName(),
		wsNotificationNamespace: ondemand.Namespace + handler.ResourceName() + "Notification",
		resourceName:            handler.ResourceName(),
		resources:               ch.Index(),
		registeredNodes:         make(map[graph.Identifier]map[string]bool),
		deletedNodeCache:        cache.New(election.TTL()*2, election.TTL()*2),
	}
	o.checkForRegistration = debouncer.New(time.Second, o.checkForRegistrationCallback)

	return o
}
