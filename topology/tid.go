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

package topology

import (
	"github.com/nu7hatch/gouuid"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/logging"
)

// TIDMapper describes the hostID nodes stored in a graph
// the mapper will broadcast node event to the registered listeners
type TIDMapper struct {
	graph.DefaultGraphListener
	Graph  *graph.Graph
	hostID graph.Identifier
}

// Start the mapper
func (t *TIDMapper) Start() {
	t.Graph.AddEventListener(t)
}

// Stop the mapper
func (t *TIDMapper) Stop() {
	t.Graph.RemoveEventListener(t)
}

func (t *TIDMapper) setTID(parent, child *graph.Node) {
	tp, _ := child.GetFieldString("Type")
	if tp == "" {
		return
	}

	var key string
	switch tp {
	case "ovsbridge", "ovsport":
		return
	case "netns":
		key, _ = child.GetFieldString("Path")
	default:
		key, _ = child.GetFieldString("Name")
	}

	if key == "" {
		return
	}

	if tid, _ := parent.GetFieldString("TID"); tid != "" {
		tid = tid + key + tp
		u, _ := uuid.NewV5(uuid.NamespaceOID, []byte(tid))
		t.Graph.AddMetadata(child, "TID", u.String())
	}
}

// onNodeEvent set TID
// TID is UUIDV5(ID/UUID) of "root" node like host, netns, ovsport, fabric
// for other nodes TID is UUIDV5(rootTID + Name + Type)
func (t *TIDMapper) onNodeEvent(n *graph.Node) {
	if _, err := n.GetFieldString("TID"); err != nil {
		if tp, err := n.GetFieldString("Type"); err == nil {
			switch tp {
			case "host":
				if name, err := n.GetFieldString("Name"); err == nil {
					u, _ := uuid.NewV5(uuid.NamespaceOID, []byte(name))
					t.hostID = graph.Identifier(u.String())
					t.Graph.AddMetadata(n, "TID", u.String())
				}
			case "netns":
				if path, _ := n.GetFieldString("Path"); path != "" {
					tid := string(t.hostID) + path + tp
					u, _ := uuid.NewV5(uuid.NamespaceOID, []byte(tid))
					t.Graph.AddMetadata(n, "TID", u.String())
				}
			case "ovsbridge", "ovsport":
				if u, _ := n.GetFieldString("UUID"); u != "" {

					tid := string(t.hostID) + u + tp

					u, _ := uuid.NewV5(uuid.NamespaceOID, []byte(tid))
					t.Graph.AddMetadata(n, "TID", u.String())
				}
			default:
				if probe, _ := n.GetFieldString("Probe"); probe == "fabric" {
					t.Graph.AddMetadata(n, "TID", string(n.ID))
				} else {
					parents := t.Graph.LookupParents(n, nil, OwnershipMetadata())
					if len(parents) > 1 {
						logging.GetLogger().Errorf("A should always only have one ownership parent: %v", n)
					} else if len(parents) == 1 {
						t.setTID(parents[0], n)
					}
				}
			}
		}
	}
}

// OnNodeUpdated event
func (t *TIDMapper) OnNodeUpdated(n *graph.Node) {
	t.onNodeEvent(n)
}

// OnNodeAdded evetn
func (t *TIDMapper) OnNodeAdded(n *graph.Node) {
	t.onNodeEvent(n)
}

// onEdgeEvent set TID for child TID nodes which is composed of the name
// the TID of the parent node and the type.
func (t *TIDMapper) onEdgeEvent(e *graph.Edge) {
	if rl, _ := e.GetFieldString("RelationType"); rl != OwnershipLink {
		return
	}

	parents, children := t.Graph.GetEdgeNodes(e, nil, nil)
	if len(parents) == 0 || len(children) == 0 {
		return
	}

	t.setTID(parents[0], children[0])
}

// OnEdgeUpdated event
func (t *TIDMapper) OnEdgeUpdated(e *graph.Edge) {
	t.onEdgeEvent(e)
}

// OnEdgeAdded event
func (t *TIDMapper) OnEdgeAdded(e *graph.Edge) {
	t.onEdgeEvent(e)
}

// OnEdgeDeleted event
func (t *TIDMapper) OnEdgeDeleted(e *graph.Edge) {
	if rl, _ := e.GetFieldString("RelationType"); rl != OwnershipLink {
		return
	}

	parents, children := t.Graph.GetEdgeNodes(e, nil, nil)
	if len(parents) == 0 || len(children) == 0 {
		return
	}

	t.Graph.DelMetadata(children[0], "TID")
}

// NewTIDMapper creates a new node mapper in the graph g
func NewTIDMapper(g *graph.Graph) *TIDMapper {
	return &TIDMapper{
		Graph: g,
	}
}
