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

// NewResolveVM creates a new name resolver
func NewResolveVM(cfg *viper.Viper) Resolver {
	gremlinClient := client.NewGremlinQueryHelper(core.CfgAuthOpts(cfg))

	return &resolveVM{
		gremlinClient: gremlinClient,
	}
}

type resolveVM struct {
	gremlinClient *client.GremlinQueryHelper
}

// IPToName resolve ip to name
func (r *resolveVM) IPToName(ipString, nodeTID string) (string, error) {
	node, err := r.gremlinClient.GetNode(g.G.V().Has("RoutingTables.Src", ipString).In().Has("Type", "host"))
	if err != nil {
		return "", err
	}
	return node.GetFieldString("Name")
}

// TIDToType resolve tid to type
func (r *resolveVM) TIDToType(nodeTID string) (string, error) {
	node, err := r.gremlinClient.GetNode(g.G.V().Has("TID", nodeTID))
	if err != nil {
		return "", err
	}
	return node.GetFieldString("Type")
}
