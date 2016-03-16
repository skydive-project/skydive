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

package probes

import (
	"os"
	"strings"

	retcd "github.com/coreos/etcd/client"
	"golang.org/x/net/context"

	"github.com/redhat-cip/skydive/logging"
	"github.com/redhat-cip/skydive/storage/etcd"
	"github.com/redhat-cip/skydive/topology"
	"github.com/redhat-cip/skydive/topology/graph"
)

type OnDemandProbeListener struct {
	graph.DefaultGraphListener
	Graph      *graph.Graph
	Probes     *FlowProbeBundle
	EtcdClient *etcd.EtcdClient
	host       string
}

type FlowProbe interface {
	RegisterProbe(n *graph.Node) error
	UnregisterProbe(n *graph.Node) error
}

func (o *OnDemandProbeListener) applyProbeAction(action string, n *graph.Node) {
	t := n.Metadata()["Type"]

	switch t {
	case "ovsbridge":
		probe := o.Probes.GetProbe("ovssflow")
		if probe == nil {
			break
		}

		logging.GetLogger().Infof("%s flow probe %s, %s", action, t, n.String())

		fprobe := probe.(FlowProbe)

		var err error
		switch action {
		case "register":
			err = fprobe.RegisterProbe(n)
		case "unregister":
			err = fprobe.UnregisterProbe(n)
		}

		if err != nil {
			logging.GetLogger().Errorf("%s error for flow probe %s: %s", action, t, err.Error())
		}
	}
}

func (o *OnDemandProbeListener) OnNodeAdded(n *graph.Node) {
	nodes := o.Graph.LookupShortestPath(n, graph.Metadata{"Type": "host"}, topology.IsOwnershipEdge)
	if len(nodes) == 0 {
		return
	}

	path := topology.NodePath{nodes}.Marshal()
	if _, err := o.EtcdClient.KeysApi.Get(context.Background(), "/capture/"+path, nil); err != nil {
		// try using the wildcard instead of the host
		wildcard := "*/" + topology.NodePath{nodes[:len(nodes)-1]}.Marshal()
		if _, err = o.EtcdClient.KeysApi.Get(context.Background(), "/capture/"+wildcard, nil); err != nil {
			return
		}
	}

	o.applyProbeAction("register", n)
}

func (o *OnDemandProbeListener) OnNodeUpdated(n *graph.Node) {
	o.OnNodeAdded(n)
}

func (o *OnDemandProbeListener) OnEdgeAdded(e *graph.Edge) {
	parent, child := o.Graph.GetEdgeNodes(e)
	if parent == nil || child == nil {
		return
	}

	if parent.Metadata()["Type"] == "ovsbridge" {
		o.OnNodeAdded(parent)
		return
	}

	if child.Metadata()["Type"] == "ovsbridge" {
		o.OnNodeAdded(child)
		return
	}
}

func (o *OnDemandProbeListener) OnNodeDeleted(n *graph.Node) {
	o.applyProbeAction("unregister", n)
}

func (o *OnDemandProbeListener) onCaptureAdded(probePath string) {
	o.Graph.Lock()
	defer o.Graph.Unlock()

	if node := topology.LookupNodeFromNodePathString(o.Graph, probePath); node != nil {
		o.applyProbeAction("unregister", node)
	}
}

func (o *OnDemandProbeListener) onCaptureDeleted(probePath string) {
	o.Graph.Lock()
	defer o.Graph.Unlock()

	if node := topology.LookupNodeFromNodePathString(o.Graph, probePath); node != nil {
		o.applyProbeAction("unregister", node)
	}
}

func (o *OnDemandProbeListener) probePathFromNodeKey(k string) string {
	probePath := strings.TrimPrefix(k, "/capture/")
	return strings.Replace(probePath, "*", o.host+"[Type=host]", 1)
}

func (o *OnDemandProbeListener) watchEtcd() {
	watcher := o.EtcdClient.KeysApi.Watcher("/capture/", &retcd.WatcherOptions{Recursive: true})
	go func() {
		for {
			resp, err := watcher.Next(context.Background())
			if err != nil {
				return
			}

			if resp.Node.Dir {
				continue
			}

			switch resp.Action {
			case "create":
				fallthrough
			case "set":
				fallthrough
			case "update":
				o.onCaptureAdded(o.probePathFromNodeKey(resp.Node.Key))
			case "expire":
				fallthrough
			case "delete":
				o.onCaptureDeleted(o.probePathFromNodeKey(resp.Node.Key))
			}
		}
	}()
}

func (o *OnDemandProbeListener) Start() error {
	go o.watchEtcd()

	o.Graph.AddEventListener(o)

	return nil
}

func (o *OnDemandProbeListener) Stop() {

}

func NewOnDemandProbeListener(fb *FlowProbeBundle, g *graph.Graph, e *etcd.EtcdClient) (*OnDemandProbeListener, error) {
	h, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	return &OnDemandProbeListener{
		Graph:      g,
		Probes:     fb,
		EtcdClient: e,
		host:       h,
	}, nil
}
