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
	"github.com/redhat-cip/skydive/logging"
	"github.com/redhat-cip/skydive/topology/graph"
)

type OnDemandProbeListener struct {
	graph.DefaultGraphListener
	Graph  *graph.Graph
	Probes *FlowProbeBundle
}

type FlowProbe interface {
	RegisterProbe(n *graph.Node) error
}

func (f *OnDemandProbeListener) OnNodeAdded(n *graph.Node) {
	t := n.Metadata()["Type"]

	switch t {
	case "ovsbridge":
		probe := f.Probes.GetProbe("ovssflow")
		if probe == nil {
			break
		}

		logging.GetLogger().Infof("Register a new flow probe %s on %s", t, n.String())

		fprobe := probe.(FlowProbe)
		err := fprobe.RegisterProbe(n)
		if err != nil {
			logging.GetLogger().Errorf("Error while registering new flow probe %s: %s", t, err.Error())
		}
	}
}

func NewOnDemandProbeListener(fb *FlowProbeBundle, g *graph.Graph) *OnDemandProbeListener {
	l := &OnDemandProbeListener{
		Graph:  g,
		Probes: fb,
	}

	g.AddEventListener(l)

	return l
}
