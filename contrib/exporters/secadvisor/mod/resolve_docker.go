/*
 * Copyright (C) 2019 IBM, Inc.
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

package mod

import (
	"github.com/spf13/viper"

	"github.com/skydive-project/skydive/api/client"
	"github.com/skydive-project/skydive/contrib/exporters/core"
	g "github.com/skydive-project/skydive/gremlin"
)

// NewResolveDocker creates a new name resolver
func NewResolveDocker(cfg *viper.Viper) Resolver {
	gremlinClient := client.NewGremlinQueryHelper(core.CfgAuthOpts(cfg))
	return &resolveDocker{
		gremlinClient: gremlinClient,
	}
}

type resolveDocker struct {
	gremlinClient GremlinNodeGetter
}

// IPToName resolve IP address to Docker container name.
func (r *resolveDocker) IPToName(ipString, nodeTID string) (string, error) {
	// Skydive analyzer monitoring a Docker installation will hold the
	// following topology graph per each container:
	//
	// netns ---- eth0 ---- veth0 ---- docker0-bridge
	//  |
	//  |
	// container
	//
	// The IPV4 address that appears in the flow is defined on the eth0
	// node, and the container name is defined on the container node.
	// However, the flow might be captured either on eth0, veth0, or the
	// docker0 bridge interfaces (Skydive listens to all of them).
	//
	// The Gremlin expression below finds the eth0 node according to the
	// given IP address (the node holds something like: "IPV4":
	// ["172.17.0.3/16"]).  However, there might be several such nodes
	// (consider the case of Skydive analyzer aggregating traffic from
	// several machines, each running a Docker engine).
	//
	// In order to find the correct eth0 node, we look for the one which is
	// the closest (ShortestPathTo) to the TID on which the flow was
	// captured.
	//
	// We then step over incoming graph edges to nodes of type "netns", and
	// from those we step over outgoing graph edges to nodes of type
	// "container".
	//
	// This results in the correct container node according to the given IP
	// address and TID, no matter if the flow was captured on the eth0,
	// veth0, or the bridge.
	node, err := r.gremlinClient.GetNode(
		g.G.V().
			Has("IPV4", g.Regex(ipString+"/.*")).
			ShortestPathTo(g.Metadata("TID", nodeTID)).
			Dedup().
			In("Type", "netns").
			Out("Type", "container"))
	if err != nil {
		return "", err
	}

	name, err := node.GetFieldString("Name")
	if err != nil {
		return "", err
	}

	return "0_0_" + name + "_0", nil
}

// TIDToType resolve tid to type
func (r *resolveDocker) TIDToType(nodeTID string) (string, error) {
	node, err := r.gremlinClient.GetNode(g.G.V().Has("TID", nodeTID))
	if err != nil {
		return "", err
	}

	return node.GetFieldString("Type")
}
