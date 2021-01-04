/*
 * Copyright (C) 2018 IBM, Inc.
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

package istio

import (
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/probe"
)

type verifierHandler func(g *graph.Graph) *resourceVerifier

type resourceVerifier struct {
	graph.DefaultGraphListener
	g                *graph.Graph
	verifier         func(*graph.Graph)
	listenerHandlers []graph.ListenerHandler
}

func (rv *resourceVerifier) OnNodeAdded(node *graph.Node) {
	rv.verifier(rv.g)
}

func (rv *resourceVerifier) OnNodeUpdated(node *graph.Node, ops []graph.PartiallyUpdatedOp) {
	rv.verifier(rv.g)
}

func (rv *resourceVerifier) OnNodeDeleted(node *graph.Node) {
	rv.verifier(rv.g)
}

func (rv *resourceVerifier) Start() error {
	for _, lh := range rv.listenerHandlers {
		lh.AddEventListener(rv)
	}
	return nil
}

func (rv *resourceVerifier) Stop() {
	for _, lh := range rv.listenerHandlers {
		lh.RemoveEventListener(rv)
	}
}

func newResourceVerifier(g *graph.Graph, lh []graph.ListenerHandler, verifier func(*graph.Graph)) *resourceVerifier {
	return &resourceVerifier{
		g:                g,
		listenerHandlers: lh,
		verifier:         verifier,
	}
}

func initResourceVerifiers(handlers []verifierHandler, g *graph.Graph) []probe.Handler {
	verifiers := []probe.Handler{}
	for _, h := range handlers {
		verifiers = append(verifiers, h(g))
	}
	return verifiers
}
