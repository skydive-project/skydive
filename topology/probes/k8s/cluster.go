/*
 * Copyright 2018 IBM Corp.
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

package k8s

import (
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology"
)

const Cluster = "cluster"

var clusterNode *graph.Node

type clusterCache struct {
	*graph.EventHandler
	graph *graph.Graph
}

func (c *clusterCache) addClusterNode(clusterName string) error {
	c.graph.Lock()
	defer c.graph.Unlock()

	m := graph.Metadata{"Name": clusterName}

	var err error
	metadata := NewMetadata(Manager, Cluster, m, nil, clusterName)
	if len(clusterName) > 0 {
		metadata.SetFieldAndNormalize(ClusterNameField, clusterName)
	}
	clusterNode, err = c.graph.NewNode(graph.GenID(), metadata)
	if err != nil {
		return err
	}

	c.NotifyEvent(graph.NodeAdded, clusterNode)
	logging.GetLogger().Infof("Added cluster{Name: %s}", clusterName)
	return nil
}

func (c *clusterCache) Start() error {
	return nil
}

func (c *clusterCache) Stop() {
}

func newClusterProbe(g *graph.Graph, clusterName string) Subprobe {
	c := &clusterCache{
		EventHandler: graph.NewEventHandler(100),
		graph:        g,
	}
	c.addClusterNode(clusterName)
	return c
}

func initClusterSubprobe(g *graph.Graph, manager, clusterName string) {
	if subprobes[manager] == nil {
		subprobes[manager] = make(map[string]Subprobe)
	}
	subprobe := newClusterProbe(g, clusterName)
	PutSubprobe(manager, Cluster, subprobe)
}

type clusterLinker struct {
	graph.DefaultLinker
	*graph.ResourceLinker
	g             *graph.Graph
	objectIndexer *graph.MetadataIndexer
}

func (linker *clusterLinker) createEdge(cluster, object *graph.Node) *graph.Edge {
	id := graph.GenID(string(cluster.ID), string(object.ID))
	return linker.g.CreateEdge(id, cluster, object, topology.OwnershipMetadata(), graph.TimeUTC(), "")
}

// GetBALinks returns all the incoming links for a node
func (linker *clusterLinker) GetBALinks(objectNode *graph.Node) (edges []*graph.Edge) {
	edges = append(edges, linker.createEdge(clusterNode, objectNode))
	return
}

func newClusterLinker(g *graph.Graph, manager string, types ...string) probe.Handler {
	rl := graph.NewResourceLinker(
		g,
		nil,
		ListSubprobes(manager, types...),
		&clusterLinker{g: g},
		topology.OwnershipMetadata(),
	)

	linker := &Linker{
		ResourceLinker: rl,
	}
	rl.AddEventListener(linker)

	return linker
}
