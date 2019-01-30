/*
 * Copyright (C) 2018 Orange
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

package nsm

import (
	"fmt"

	localconn "github.com/ligato/networkservicemesh/controlplane/pkg/apis/local/connection"
	remoteconn "github.com/ligato/networkservicemesh/controlplane/pkg/apis/remote/connection"
	"github.com/skydive-project/skydive/filters"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/logging"
)

type connection interface {
	AddEdge(*graph.Graph)
	DelEdge(*graph.Graph)
	GetSource() *localconn.Connection
	GetDest() *localconn.Connection
	GetInodes() (int64, int64)
	createMetadatas() graph.Metadata
}

type baseConnectionPair struct {
	payload  string
	srcInode int64
	dstInode int64
	src      *localconn.Connection
	dst      *localconn.Connection
}

func (b *baseConnectionPair) GetSource() *localconn.Connection {
	return b.src
}

func (b *baseConnectionPair) GetDest() *localconn.Connection {
	return b.dst
}

func (b *baseConnectionPair) GetSourceInode() int64 {
	if b.src == nil {
		return 0
	}
	i, err := getLocalInode(b.src)
	if err != nil {
		return 0
	}
	return i
}

func (b *baseConnectionPair) GetDestInode() int64 {
	if b.dst == nil {
		return 0
	}
	i, err := getLocalInode(b.dst)
	if err != nil {
		return 0
	}
	return i
}

func (b *baseConnectionPair) GetInodes() (int64, int64) {
	return b.GetSourceInode(), b.GetDestInode()
}

// A local connection is composed of only one cross-connect
type localConnectionPair struct {
	baseConnectionPair
	ID string // crossConnectID
}

// A remote connection is composed of two cross-connects
type remoteConnectionPair struct {
	baseConnectionPair
	remote *remoteconn.Connection // the remote connection shared between the two corss-connects
	srcID  string                 // The id of the cross-connect with a local connection as source
	dstID  string                 // The id of the cross-connect with a local connection as destination

}

// easyjson:json
type baseConnectionMetadata struct {
	MechanismType       string
	MechanismParameters map[string]string
	Labels              map[string]string
}

// easyjson:json
type localConnectionMetadata struct {
	IP string
	baseConnectionMetadata
}

// easyjson:json
type remoteConnectionMetadata struct {
	baseConnectionMetadata
	SourceNSM              string
	DestinationNSM         string
	NetworkServiceEndpoint string
}

// easyjson:json
type baseNSMMetadata struct {
	NetworkService string
	Payload        string
	Source         interface{}
	Destination    interface{}
}

// easyjson:json
type localNSMMetadata struct {
	CrossConnectID string
	baseNSMMetadata
}

// easyjson:json
type remoteNSMMetadata struct {
	SourceCrossConnectID      string
	DestinationCrossConnectID string
	baseNSMMetadata
	Via remoteConnectionMetadata
}

func (b *baseConnectionPair) GetNodes(g *graph.Graph) (*graph.Node, *graph.Node, error) {
	srcInode, dstInode := b.GetInodes()

	if srcInode == 0 || dstInode == 0 {
		// remote connection: src or dst is not ready
		return nil, nil, fmt.Errorf("source or destination inode is not set")
	}

	getNode := func(inode int64) *graph.Node {
		filter := graph.NewElementFilter(filters.NewTermInt64Filter("Inode", inode))
		node := g.LookupFirstNode(filter)
		return node
	}
	// Check that the nodes are in the graph
	srcNode := getNode(srcInode)
	if srcNode == nil {
		return nil, nil, fmt.Errorf("node with inode %d does not exist", srcInode)
	}
	dstNode := getNode(dstInode)
	if dstNode == nil {
		return nil, nil, fmt.Errorf("node with inode %d does not exist", dstInode)
	}

	return srcNode, dstNode, nil

}

// This function creates the Edge with correct metadata
// graph and probe should be locked
func (l *localConnectionPair) AddEdge(g *graph.Graph) {
	srcNode, dstNode, err := l.GetNodes(g)
	if err != nil {
		logging.GetLogger().Debugf("NSM: cannot create Edge in the graph, %v", err)
		return
	}

	// create Edge
	if !g.AreLinked(srcNode, dstNode, nil) {
		// generate metadatas
		g.Link(srcNode, dstNode, l.createMetadatas())
	}
}

func (l *localConnectionPair) DelEdge(g *graph.Graph) {
	srcNode, dstNode, err := l.GetNodes(g)
	if err != nil {
		logging.GetLogger().Debugf("NSM: cannot delete Edge in the graph, %v", err)
		return
	}

	// delete Edge
	if g.AreLinked(srcNode, dstNode, nil) {
		g.Unlink(srcNode, dstNode)
	}
}

func (l *localConnectionPair) createMetadatas() graph.Metadata {
	metadata := graph.Metadata{
		"NSM": localNSMMetadata{
			CrossConnectID: l.ID,
			baseNSMMetadata: baseNSMMetadata{
				Payload:        l.payload,
				NetworkService: l.GetSource().GetNetworkService(),
				Source: localConnectionMetadata{
					baseConnectionMetadata: baseConnectionMetadata{
						MechanismType:       l.GetSource().GetMechanism().GetType().String(),
						MechanismParameters: l.GetSource().GetMechanism().GetParameters(),
						Labels:              l.GetSource().GetLabels(),
					},
				},
				Destination: localConnectionMetadata{
					IP: l.GetDest().GetContext()["dst_ip"],
					baseConnectionMetadata: baseConnectionMetadata{
						MechanismType:       l.GetDest().GetMechanism().GetType().String(),
						MechanismParameters: l.GetDest().GetMechanism().GetParameters(),
						Labels:              l.GetDest().GetLabels(),
					},
				},
			},
		},
		"Directed": "true",
	}

	return metadata
}

func (r *remoteConnectionPair) AddEdge(g *graph.Graph) {
	srcNode, dstNode, err := r.GetNodes(g)
	if err != nil {
		logging.GetLogger().Debugf("NSM: cannot create Edge in the graph, %v", err)
		return
	}

	// create Edge
	if !g.AreLinked(srcNode, dstNode, nil) {

		g.Link(srcNode, dstNode, r.createMetadatas())
	}
}

func (r *remoteConnectionPair) DelEdge(g *graph.Graph) {
	srcNode, dstNode, err := r.GetNodes(g)
	if err != nil {
		logging.GetLogger().Debugf("NSM: cannot delete Edge in the graph, %v", err)
		return
	}

	// delete Edge
	if g.AreLinked(srcNode, dstNode, nil) {
		g.Unlink(srcNode, dstNode)
	}
}

func (r *remoteConnectionPair) createMetadatas() graph.Metadata {
	metadata := graph.Metadata{
		"NSM": remoteNSMMetadata{
			SourceCrossConnectID:      r.srcID,
			DestinationCrossConnectID: r.dstID,
			baseNSMMetadata: baseNSMMetadata{
				NetworkService: r.GetSource().GetNetworkService(),
				Payload:        r.payload,
				Source: localConnectionMetadata{
					IP: r.GetSource().GetContext()["src_ip"],
					baseConnectionMetadata: baseConnectionMetadata{
						MechanismType:       r.GetSource().GetMechanism().GetType().String(),
						MechanismParameters: r.GetSource().GetMechanism().GetParameters(),
						Labels:              r.GetSource().GetLabels(),
					},
				},
				Destination: localConnectionMetadata{
					IP: r.GetDest().GetContext()["dst_ip"],
					baseConnectionMetadata: baseConnectionMetadata{
						MechanismType:       r.GetDest().GetMechanism().GetType().String(),
						MechanismParameters: r.GetDest().GetMechanism().GetParameters(),
						Labels:              r.GetDest().GetLabels(),
					},
				},
			},
			Via: remoteConnectionMetadata{
				baseConnectionMetadata: baseConnectionMetadata{
					MechanismType:       r.remote.GetMechanism().GetType().String(),
					MechanismParameters: r.remote.GetMechanism().GetParameters(),
					Labels:              r.remote.GetLabels(),
				},
				SourceNSM:              r.remote.GetSourceNetworkServiceManagerName(),
				DestinationNSM:         r.remote.GetDestinationNetworkServiceManagerName(),
				NetworkServiceEndpoint: r.remote.GetNetworkServiceEndpointName(),
			},
		},
		"Directed": "true",
	}

	return metadata
}
