// +build !windows

/*
 * Copyright (C) 2019 Orange
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

package nsm

import (
	"fmt"

	localconn "github.com/networkservicemesh/networkservicemesh/controlplane/api/local/connection"
	remoteconn "github.com/networkservicemesh/networkservicemesh/controlplane/api/remote/connection"
	"github.com/skydive-project/skydive/filters"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/logging"
)

type connection interface {
	addEdge(*graph.Graph)
	delEdge(*graph.Graph)
	getSource() *localconn.Connection
	getDest() *localconn.Connection
	getInodes() (int64, int64)
	isCrossConnectOwner(string, string) bool
	printCrossConnect() string
	createMetadata() graph.Metadata
}

type baseConnectionPair struct {
	payload string
	src     *localconn.Connection
	dst     *localconn.Connection
}

func (b *baseConnectionPair) getSource() *localconn.Connection {
	return b.src
}

func (b *baseConnectionPair) getDest() *localconn.Connection {
	return b.dst
}

func (b *baseConnectionPair) getSourceInode() int64 {
	if b.src == nil {
		return 0
	}
	i, err := getLocalInode(b.src)
	if err != nil {
		return 0
	}
	return i
}

func (b *baseConnectionPair) getDestInode() int64 {
	if b.dst == nil {
		return 0
	}
	i, err := getLocalInode(b.dst)
	if err != nil {
		return 0
	}
	return i
}

func (b *baseConnectionPair) getInodes() (int64, int64) {
	return b.getSourceInode(), b.getDestInode()
}

// what makes a crossConnect unique is the nsmgr that reports it, and its ID
type crossConnect struct {
	url string
	ID  string
}

// A local connection is composed of only one cross-connect
type localConnectionPair struct {
	baseConnectionPair
	cc *crossConnect // crossConnectID
}

// A remote connection is composed of two cross-connects
type remoteConnectionPair struct {
	baseConnectionPair
	remote *remoteconn.Connection // the remote connection shared between the two corss-connects
	srcCc  *crossConnect          // The id of the cross-connect with a local connection as source
	dstCc  *crossConnect          // The id of the cross-connect with a local connection as destination

}

func (b *baseConnectionPair) getNodes(g *graph.Graph) (*graph.Node, *graph.Node, error) {
	srcInode, dstInode := b.getInodes()

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

func (l *localConnectionPair) isCrossConnectOwner(url string, id string) bool {
	return l.cc.ID == id && l.cc.url == url
}

func (l *localConnectionPair) printCrossConnect() string {
	srcInode, dstInode := l.getInodes()
	return fmt.Sprintf("local crossconnect url: %s, id: %s, source inode: %d, destination inode: %d", l.cc.url, l.cc.ID, srcInode, dstInode)
}

func (l *localConnectionPair) addEdge(g *graph.Graph) {
	srcNode, dstNode, err := l.getNodes(g)
	if err != nil {
		logging.GetLogger().Debugf("NSM: cannot create Edge in the graph, %v", err)
		return
	}

	// create Edge
	if !g.AreLinked(srcNode, dstNode, nil) {
		// generate metadatas
		logging.GetLogger().Debugf("NSM: adding edge from %v to %v", srcNode, dstNode)
		g.Link(srcNode, dstNode, l.createMetadata())
	}
}

func (l *localConnectionPair) delEdge(g *graph.Graph) {
	srcNode, dstNode, err := l.getNodes(g)
	if err != nil {
		logging.GetLogger().Debugf("NSM: cannot delete Edge in the graph, %v", err)
		return
	}

	logging.GetLogger().Debugf("NSM: deleting edge from %v to %v", srcNode, dstNode)
	g.Unlink(srcNode, dstNode)
}

func (l *localConnectionPair) createMetadata() graph.Metadata {
	metadata := graph.Metadata{
		"NSM": &EdgeMetadata{
			BaseNSMMetadata: BaseNSMMetadata{
				Payload:        l.payload,
				NetworkService: l.getSource().GetNetworkService(),
				Source: LocalConnectionMetadata{
					BaseConnectionMetadata: BaseConnectionMetadata{
						MechanismType:       l.getSource().GetMechanism().GetType().String(),
						MechanismParameters: l.getSource().GetMechanism().GetParameters(),
						Labels:              l.getSource().GetLabels(),
					},
				},
				Destination: LocalConnectionMetadata{
					IP: l.getDest().GetContext().GetIpContext().GetDstIpAddr(),
					BaseConnectionMetadata: BaseConnectionMetadata{
						MechanismType:       l.getDest().GetMechanism().GetType().String(),
						MechanismParameters: l.getDest().GetMechanism().GetParameters(),
						Labels:              l.getDest().GetLabels(),
					},
				},
			},
			LocalNSMMetadata: LocalNSMMetadata{
				CrossConnectID: l.cc.ID,
			},
		},
		"Directed": "true",
	}

	return metadata
}

func (r *remoteConnectionPair) isCrossConnectOwner(url string, id string) bool {
	if r.srcCc != nil && r.srcCc.ID == id && r.srcCc.url == url {
		return true
	}
	if r.dstCc != nil && r.dstCc.ID == id && r.dstCc.url == url {
		return true
	}
	return false
}

func (r *remoteConnectionPair) printCrossConnect() string {
	srcInode, dstInode := r.getInodes()
	s := fmt.Sprintf("remote crossconnects with remote id: %s", r.remote.Id)
	if r.srcCc != nil {
		s += fmt.Sprintf(", src url: %s, source crossconnect id: %s, source inode: %d", r.srcCc.url, r.srcCc.ID, srcInode)
	} else {
		s += ", source crossconnect is not set"
	}
	if r.dstCc != nil {
		s += fmt.Sprintf(", destination url: %s, destination crossconnect id: %s, destination inode %d", r.dstCc.url, r.dstCc.ID, dstInode)
	} else {
		s += ", destination crossconnect is not set"
	}

	return s
}

func (r *remoteConnectionPair) addEdge(g *graph.Graph) {
	srcNode, dstNode, err := r.getNodes(g)
	if err != nil {
		logging.GetLogger().Debugf("NSM: cannot create Edge in the graph, %v", err)
		return
	}

	// create Edge
	if !g.AreLinked(srcNode, dstNode, nil) {
		logging.GetLogger().Debugf("NSM: creating Edge from %v to %v", srcNode, dstNode)
		g.Link(srcNode, dstNode, r.createMetadata())
	}
}

func (r *remoteConnectionPair) delEdge(g *graph.Graph) {
	srcNode, dstNode, err := r.getNodes(g)
	if err != nil {
		logging.GetLogger().Debugf("NSM: cannot delete Edge in the graph, %v", err)
		return
	}

	logging.GetLogger().Debugf("NSM: deleting Edge from %v to %v", srcNode, dstNode)
	g.Unlink(srcNode, dstNode)
}

func (r *remoteConnectionPair) createMetadata() graph.Metadata {
	metadata := graph.Metadata{
		"NSM": &EdgeMetadata{
			BaseNSMMetadata: BaseNSMMetadata{
				NetworkService: r.getSource().GetNetworkService(),
				Payload:        r.payload,
				Source: LocalConnectionMetadata{
					IP: r.getSource().GetContext().GetIpContext().GetSrcIpAddr(),
					BaseConnectionMetadata: BaseConnectionMetadata{
						MechanismType:       r.getSource().GetMechanism().GetType().String(),
						MechanismParameters: r.getSource().GetMechanism().GetParameters(),
						Labels:              r.getSource().GetLabels(),
					},
				},
				Destination: LocalConnectionMetadata{
					IP: r.getDest().GetContext().GetIpContext().GetDstIpAddr(),
					BaseConnectionMetadata: BaseConnectionMetadata{
						MechanismType:       r.getDest().GetMechanism().GetType().String(),
						MechanismParameters: r.getDest().GetMechanism().GetParameters(),
						Labels:              r.getDest().GetLabels(),
					},
				},
			},
			RemoteNSMMetadata: RemoteNSMMetadata{
				SourceCrossConnectID:      r.srcCc.ID,
				DestinationCrossConnectID: r.dstCc.ID,
				Via: RemoteConnectionMetadata{
					BaseConnectionMetadata: BaseConnectionMetadata{
						MechanismType:       r.remote.GetMechanism().GetType().String(),
						MechanismParameters: r.remote.GetMechanism().GetParameters(),
						Labels:              r.remote.GetLabels(),
					},
					SourceNSM:              r.remote.GetSourceNetworkServiceManagerName(),
					DestinationNSM:         r.remote.GetDestinationNetworkServiceManagerName(),
					NetworkServiceEndpoint: r.remote.GetNetworkServiceEndpointName(),
				},
			},
		},
		"Directed": "true",
	}

	return metadata
}
