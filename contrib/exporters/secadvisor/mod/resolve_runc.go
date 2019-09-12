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
	"github.com/skydive-project/skydive/graffiti/graph"
	g "github.com/skydive-project/skydive/gremlin"
)

// GremlinNodeGetter interface allows access to get topology nodes according to
// a gremlin query.
type GremlinNodeGetter interface {
	GetNodes(query interface{}) ([]*graph.Node, error)
	GetNode(query interface{}) (*graph.Node, error)
}

// NewResolveRunc creates a new name resolver
func NewResolveRunc(cfg *viper.Viper) Resolver {
	gremlinClient := client.NewGremlinQueryHelper(core.CfgAuthOpts(cfg))
	return &resolveRunc{
		gremlinClient: gremlinClient,
	}
}

type resolveRunc struct {
	gremlinClient GremlinNodeGetter
}

// IPToName resolve ip to name
func (r *resolveRunc) IPToName(ipString, nodeTID string) (string, error) {
	node, err := r.gremlinClient.GetNode(g.G.V().Has("TID", nodeTID).Out("Runc.Hosts.IP", ipString))
	if err != nil {
		return "", err
	}

	name, err := node.GetFieldString("Runc.Hosts.Hostname")
	if err != nil {
		return "", err
	}

	return "0_0_" + name + "_0", nil
}

// TIDToType resolve tid to type
func (r *resolveRunc) TIDToType(nodeTID string) (string, error) {
	node, err := r.gremlinClient.GetNode(g.G.V().Has("TID", nodeTID))
	if err != nil {
		return "", err
	}

	return node.GetFieldString("Type")
}
