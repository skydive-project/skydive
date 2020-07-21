//go:generate sh -c "go run github.com/gomatic/renderizer --name=injection --resource=injectpacket --type=PacketInjection --title='Injection' --article=an swagger_operations.tmpl > packet_injector_swagger.go"
//go:generate sh -c "go run github.com/gomatic/renderizer --name=injection --resource=injectpacket --type=PacketInjection --title='Injection' swagger_definitions.tmpl > packet_injector_swagger.json"

/*
 * Copyright (C) 2016 Red Hat, Inc.
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
	etcd "github.com/coreos/etcd/client"
	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/graffiti/api/rest"
	api "github.com/skydive-project/skydive/graffiti/api/server"
	"github.com/skydive-project/skydive/graffiti/graph"
	shttp "github.com/skydive-project/skydive/graffiti/http"
	ge "github.com/skydive-project/skydive/gremlin/traversal"
)

type packetInjectorResourceHandler struct {
	rest.ResourceHandler
}

// PacketInjectorAPI exposes the packet injector API
type PacketInjectorAPI struct {
	rest.BasicAPIHandler
	Graph *graph.Graph
}

func (pirh *packetInjectorResourceHandler) Name() string {
	return "injectpacket"
}

func (pirh *packetInjectorResourceHandler) New() rest.Resource {
	return &types.PacketInjection{}
}

// Create allocates a new packet injection
func (pi *PacketInjectorAPI) Create(r rest.Resource, opts *rest.CreateOptions) error {
	ppr := r.(*types.PacketInjection)

	if err := pi.validateRequest(ppr); err != nil {
		return err
	}
	e := pi.BasicAPIHandler.Create(ppr, opts)
	return e
}

func (pi *PacketInjectorAPI) getNode(gremlinQuery string) *graph.Node {
	res, err := ge.TopologyGremlinQuery(pi.Graph, gremlinQuery)
	if err != nil {
		return nil
	}

	for _, value := range res.Values() {
		switch value.(type) {
		case *graph.Node:
			return value.(*graph.Node)
		default:
			return nil
		}
	}
	return nil
}

// RegisterPacketInjectorAPI registers a new packet injector resource in the API
func RegisterPacketInjectorAPI(g *graph.Graph, apiServer *api.Server, kapi etcd.KeysAPI, authBackend shttp.AuthenticationBackend) (*PacketInjectorAPI, error) {
	pia := &PacketInjectorAPI{
		BasicAPIHandler: rest.BasicAPIHandler{
			ResourceHandler: &packetInjectorResourceHandler{},
			EtcdKeyAPI:      kapi,
		},
		Graph: g,
	}
	if err := apiServer.RegisterAPIHandler(pia, authBackend); err != nil {
		return nil, err
	}

	return pia, nil
}
