// Copyright (c) 2019 Red Hat and/or its affiliates.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bess

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/skydive-project/skydive/filters"
	"google.golang.org/grpc"

	"github.com/nimbess/nimbess-agent/pkg/proto/bess_pb"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/service"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/probes"
)

const BessManager = "BESS"

type Probe struct {
	graph.DefaultGraphListener
	Ctx         probes.Context
	bessClient  bess_pb.BESSControlClient
	state       service.State
	bessPort    int
	bessHost    string
	bessContext context.Context
}

func (p *Probe) walkAndRender(moduleName string, parentNode *graph.Node, edgeList *[]*graph.Edge) {
	modRes, err := p.bessClient.GetModuleInfo(p.bessContext, &bess_pb.GetModuleInfoRequest{Name: moduleName})
	if err != nil {
		p.Ctx.Logger.Errorf("Error getting module information for module: %s", moduleName)
		return
	}

	nodeID := graph.GenID(moduleName)
	node := p.Ctx.Graph.GetNode(nodeID)
	if node == nil {
		node, err = p.Ctx.Graph.NewNode(nodeID, graph.Metadata{"Name": moduleName,
			"Type": modRes.GetMclass(), "Manager": BessManager})
		p.Ctx.Logger.Infof("Creating node: %s", moduleName)
		if err != nil {
			p.Ctx.Logger.Errorf("Error creating node: %s, %v", moduleName, err)
			return
		}
	}
	if parentNode != nil && !topology.HaveOwnershipLink(p.Ctx.Graph, parentNode, node) {
		topology.AddOwnershipLink(p.Ctx.Graph, parentNode, node, nil)
	}
	for _, oGate := range modRes.GetOgates() {
		p.walkAndRender(oGate.GetName(), parentNode, edgeList)
		edgeID := graph.GenID(moduleName, string(oGate.Igate), string(oGate.Ogate), oGate.GetName())
		if p.Ctx.Graph.GetEdge(edgeID) == nil {
			p.Ctx.Graph.NewEdge(edgeID, node,
				p.Ctx.Graph.GetNode(graph.GenID(oGate.GetName())),
				graph.Metadata{"RelationType": "layer2", "Directed": true, "Manager": BessManager})
		}
		*edgeList = append(*edgeList, p.Ctx.Graph.GetEdge(edgeID))
	}

}

func (p *Probe) Do(ctx context.Context, wg *sync.WaitGroup) error {
	p.Ctx.Logger.Info("Connecting to BESS")
	conn, err := grpc.Dial(fmt.Sprintf("%s:%d", p.bessHost, p.bessPort), grpc.WithInsecure())
	if err != nil {
		return err
	}
	p.Ctx.Logger.Info("Connected to BESS")
	p.bessClient = bess_pb.NewBESSControlClient(conn)
	verResponse, err := p.bessClient.GetVersion(context.Background(), &bess_pb.EmptyRequest{})
	if err != nil {
		return err
	} else if verResponse.Error != nil {
		return fmt.Errorf("could not get version: %s, %s", verResponse.GetError(), err)
	}
	p.Ctx.Logger.Infof("BESS connected with version: %s", verResponse.Version)

	metadata := graph.Metadata{
		"Name":    "bess",
		"Type":    "bess",
		"Program": "bessd",
		"Version": verResponse.GetVersion(),
		"Manager": BessManager,
	}
	wg.Add(1)

	go func() {
		defer wg.Done()
		// probe running function
		for {
			// get modules and data from bess
			modsRes, err := p.bessClient.ListModules(p.bessContext, &bess_pb.EmptyRequest{})
			if err != nil {
				p.Ctx.Logger.Errorf("Unable to query module list for BESS: %s", err)
				return
			}

			portRes, err := p.bessClient.ListPorts(p.bessContext, &bess_pb.EmptyRequest{})
			if err != nil {
				p.Ctx.Logger.Errorf("Unable to query port list for BESS: %s", err)
				return
			}

			var foundMods []string
			var foundEdges []*graph.Edge
			p.Ctx.Graph.Lock()

			// build main bess node
			bessID := graph.GenID("bessd", p.bessHost)
			bessNode := p.Ctx.Graph.GetNode(bessID)
			if bessNode == nil {
				bessNode, err = p.Ctx.Graph.NewNode(bessID, metadata)
				if err != nil {
					p.Ctx.Logger.Errorf("Failed to create bess node")
					return
				}
			}
			if !topology.HaveOwnershipLink(p.Ctx.Graph, p.Ctx.RootNode, bessNode) {
				topology.AddOwnershipLink(p.Ctx.Graph, p.Ctx.RootNode, bessNode, nil)
			}

			// find ports to figure out pipeline roots
			portMap := make(map[string]*graph.Node)
			var bessPort *graph.Node
			for _, port := range portRes.GetPorts() {
				portID := graph.GenID(port.GetName())
				bessPort = p.Ctx.Graph.GetNode(portID)
				if bessPort == nil {
					bessPort, err = p.Ctx.Graph.NewNode(portID, graph.Metadata{
						"Name":    port.GetName(),
						"MAC":     port.GetMacAddr(),
						"Type":    port.GetDriver(),
						"Manager": BessManager,
					})
					if err != nil {
						p.Ctx.Logger.Errorf("Failed to create bessPort: %v", bessPort)
						continue
					}
				}
				if !topology.HaveOwnershipLink(p.Ctx.Graph, bessNode, bessPort) {
					topology.AddOwnershipLink(p.Ctx.Graph, bessNode, bessPort, nil)
				}
				portMap[port.GetName()] = bessPort
			}

			// find modules and associate to pipelines/ports
			for _, module := range modsRes.GetModules() {
				if module.GetName() == "" {
					continue
				}
				moduleName := module.GetName()
				// find first modules in pipeline
				if module.GetMclass() == "PortInc" {
					p.walkAndRender(moduleName, portMap[strings.TrimSuffix(moduleName, "_ingress")], &foundEdges)
				}
				foundMods = append(foundMods, moduleName)
			}

			// if there are any remaining modules then they do not belong to a port pipeline, render them anyway,
			// log warning, parent node will be bess node itself
			for _, module := range foundMods {
				node := p.Ctx.Graph.GetNode(graph.GenID(module))
				if node == nil {
					p.Ctx.Logger.Warningf("Node found in bess that is not part of any port pipeline: %s", module)
					p.walkAndRender(module, bessNode, &foundEdges)
				}
			}

			// Now check for any nodes that need to be removed
			filter := graph.NewElementFilter(filters.NewTermStringFilter("Manager", BessManager))
			nodes := p.Ctx.Graph.GetNodes(filter)
			// foundNodes will contain all nodes we know should exist
			var foundNodes []string
			foundNodes = append(foundNodes, foundMods...)
			for k := range portMap {
				foundNodes = append(foundNodes, k)
			}

			for _, node := range nodes {
				modFound := false
				// special case for bess root itself
				if node.ID == bessID {
					continue
				}

				for _, module := range foundNodes {
					if node.ID == graph.GenID(module) {
						modFound = true
						break
					}
				}
				if !modFound {
					p.Ctx.Logger.Debugf("Deleting node: %+v", *node)
					p.Ctx.Graph.DelNode(node)
				}
			}

			// now check edges
			edges := p.Ctx.Graph.GetEdges(filter)
			for _, edge := range edges {
				edgeFound := false
				for _, knownEdge := range foundEdges {
					if edge == knownEdge {
						edgeFound = true
						break
					}
				}
				if !edgeFound {
					p.Ctx.Logger.Debugf("Deleting edge: %+v", *edge)
					p.Ctx.Graph.DelEdge(edge)
				}
			}

			p.Ctx.Graph.Unlock()
			time.Sleep(1 * time.Second)
		}
	}()
	return nil
}

// NewProbe returns a BESS topology probe
func NewProbe(ctx probes.Context, bundle *probe.Bundle) (probe.Handler, error) {
	p := &Probe{
		Ctx: ctx,
	}
	p.state.Store(service.StoppedState)
	p.bessHost = ctx.Config.GetString("agent.topology.bess.host")
	p.bessPort = ctx.Config.GetInt("agent.topology.bess.port")
	p.bessContext = context.Background()

	return probes.NewProbeWrapper(p), nil
}
