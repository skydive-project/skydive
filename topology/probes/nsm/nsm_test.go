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
	"math/rand"
	"strconv"
	"testing"

	cc "github.com/networkservicemesh/networkservicemesh/controlplane/api/crossconnect"
	localconn "github.com/networkservicemesh/networkservicemesh/controlplane/api/local/connection"
	remoteconn "github.com/networkservicemesh/networkservicemesh/controlplane/api/remote/connection"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/graffiti/graph"
)

const ns = "ns_test"

func createLocalConn() *localconn.Connection {
	mech := &localconn.Mechanism{
		Type:       localconn.MechanismType_DEFAULT_INTERFACE,
		Parameters: make(map[string]string),
	}

	c := &localconn.Connection{
		NetworkService: ns,
		Mechanism:      mech,
		Labels:         make(map[string]string),
	}
	return c
}

func createLocalSource() *cc.CrossConnect_LocalSource {
	c := createLocalConn()
	c.Id = "id_src_conn"

	localSrc := &cc.CrossConnect_LocalSource{LocalSource: c}
	return localSrc
}

func createLocalDest() *cc.CrossConnect_LocalDestination {
	c := createLocalConn()
	c.Id = "id_src_conn"

	localDst := &cc.CrossConnect_LocalDestination{LocalDestination: c}
	return localDst
}

func createConnectionLocalOnly(inodeSrc string, inodeDst string) *cc.CrossConnect {

	localSrc := createLocalSource()
	localSrc.LocalSource.GetMechanism().Parameters[localconn.NetNsInodeKey] = inodeSrc

	localDst := createLocalDest()
	localDst.LocalDestination.GetMechanism().Parameters[localconn.NetNsInodeKey] = inodeDst

	cconn := &cc.CrossConnect{
		Id:          "CrossConnectID",
		Payload:     "CrossConnectPayload",
		Source:      localSrc,
		Destination: localDst,
	}
	return cconn
}

func createConnectionWithRemote(inodeSrc string, inodeDst string) (*cc.CrossConnect, *cc.CrossConnect) {
	localSrc := createLocalSource()
	localSrc.LocalSource.GetMechanism().Parameters[localconn.NetNsInodeKey] = inodeSrc

	localDst := createLocalDest()
	localDst.LocalDestination.GetMechanism().Parameters[localconn.NetNsInodeKey] = inodeDst

	remote := &remoteconn.Connection{
		Id:             strconv.Itoa(rand.Int()),
		NetworkService: ns,
		Labels:         make(map[string]string),
	}

	remoteDst := &cc.CrossConnect_RemoteDestination{RemoteDestination: remote}

	cconn1 := &cc.CrossConnect{
		Id:          strconv.Itoa(rand.Int()),
		Payload:     "CrossConnectPayload",
		Source:      localSrc,
		Destination: remoteDst,
	}

	remoteSrc := &cc.CrossConnect_RemoteSource{RemoteSource: remote}

	cconn2 := &cc.CrossConnect{
		Id:          strconv.Itoa(rand.Int()),
		Payload:     "CrossConnectPayload",
		Source:      remoteSrc,
		Destination: localDst,
	}

	return cconn1, cconn2
}

func setupProbe(t *testing.T) *Probe {
	config.Set("logging.level", "DEBUG")
	backend, err := graph.NewMemoryBackend()

	if err != nil {
		t.Fatalf("Can't create the probe, error: %v", err)
		return nil
	}
	g := graph.NewGraph("host_test", backend, "test")
	p, err := NewNsmProbe(g)
	if err != nil {
		t.Fatalf("Can't create the probe, error: %v", err)
		return nil
	}
	p.Start()

	return p
}

func TestOnConnLocal_create_and_delete(t *testing.T) {
	p := setupProbe(t)

	cconn := createConnectionLocalOnly("1", "2")

	p.onConnLocalLocal(cc.CrossConnectEventType_INITIAL_STATE_TRANSFER, cconn, "url")

	// Ensure the link is correctly created
	// TODO: test the metadata is correct
	srcInode, dstInode := p.connections[0].getInodes()
	if srcInode != 1 || dstInode != 2 {
		t.Error("probe doesn't have the correct link")
	}

	// Add nodes to the graph
	m1 := graph.Metadata{
		"Inode": 1,
	}
	n1, _ := p.g.NewNode(graph.GenID(), m1)
	p.g.AddNode(n1)

	m2 := graph.Metadata{
		"Inode": 2,
	}
	n2, _ := p.g.NewNode(graph.GenID(), m2)
	p.g.AddNode(n2)

	// Ensure Edge is created
	if !p.g.AreLinked(n1, n2, nil) {
		t.Error("link is not created in the graph")
	}

	// TODO: test metadatas
	p.onConnLocalLocal(cc.CrossConnectEventType_DELETE, cconn, "url")
	if len(p.connections) != 0 {
		t.Error("link list is not empty after deletion")
	}

	// Ensure Edge is deleted
	if p.g.AreLinked(n1, n2, nil) {
		t.Error("link is not deleted in the graph")
	}
}

func TestOnConnRemote_create_and_delete(t *testing.T) {
	p := setupProbe(t)
	cconn1, cconn2 := createConnectionWithRemote("1", "2")
	p.onConnLocalRemote(cc.CrossConnectEventType_INITIAL_STATE_TRANSFER, cconn1, "url1")
	p.onConnRemoteLocal(cc.CrossConnectEventType_INITIAL_STATE_TRANSFER, cconn2, "url2")

	srcInode, dstInode := p.connections[0].getInodes()
	if srcInode != 1 || dstInode != 2 {
		t.Errorf("Probe doesn't have the correct connection.\nNumber of conn : %d.\nConnections are: \n %+v", len(p.connections), p.connections)
	}

	// Add nodes to the graph
	m1 := graph.Metadata{
		"Inode": 1,
	}
	n1, _ := p.g.NewNode(graph.GenID(), m1)
	p.g.AddNode(n1)

	m2 := graph.Metadata{
		"Inode": 2,
	}
	n2, _ := p.g.NewNode(graph.GenID(), m2)
	p.g.AddNode(n2)

	// Ensure Edge is created
	if !p.g.AreLinked(n1, n2, nil) {
		t.Error("link is not created in the graph")
	}

	p.onConnLocalRemote(cc.CrossConnectEventType_DELETE, cconn1, "url1")

	// Ensure Edge is deleted
	if p.g.AreLinked(n1, n2, nil) {
		t.Error("link is not deleted in the graph")
	}

	p.onConnRemoteLocal(cc.CrossConnectEventType_DELETE, cconn2, "url2")

	if len(p.connections) != 0 {
		t.Errorf("link list is not empty after deletion: %+v", p.connections)
	}
}

// This test creates two skydive connections
// One connection has its localSource equal to the localDest of the other
// conn1->conn2->conn3
// ensure that two skydive edges are created in the graph
func TestOnConnTwoCrossConnectsWithTheSameSourceAndDest_create_and_delete(t *testing.T) {
	p := setupProbe(t)

	cconn1, cconn2 := createConnectionWithRemote("1", "2")
	p.onConnLocalRemote(cc.CrossConnectEventType_INITIAL_STATE_TRANSFER, cconn1, "url1")
	p.onConnRemoteLocal(cc.CrossConnectEventType_INITIAL_STATE_TRANSFER, cconn2, "url2")
	cconn3, cconn4 := createConnectionWithRemote("2", "3")
	p.onConnLocalRemote(cc.CrossConnectEventType_INITIAL_STATE_TRANSFER, cconn3, "url2")
	p.onConnRemoteLocal(cc.CrossConnectEventType_INITIAL_STATE_TRANSFER, cconn4, "url3")

	// ensure that two connections are available in the connection list when looking for inode 2
	c, _ := p.getConnectionsReadyWithInode(2)
	if len(c) != 2 {
		t.Fatalf("Two connections should be available with inode 2, but only %d is retreived", len(c))
	}

	// Add nodes to the graph
	m1 := graph.Metadata{
		"Inode": 1,
	}
	n1, _ := p.g.NewNode(graph.GenID(), m1)
	p.g.AddNode(n1)

	m2 := graph.Metadata{
		"Inode": 2,
	}
	n2, _ := p.g.NewNode(graph.GenID(), m2)
	p.g.AddNode(n2)

	m3 := graph.Metadata{
		"Inode": 3,
	}
	n3, _ := p.g.NewNode(graph.GenID(), m3)
	p.g.AddNode(n3)

	// Ensure Edges are created
	if !p.g.AreLinked(n1, n2, nil) {
		t.Errorf("link between inode 1 and inode 2 is not created in the graph : %v", p.connections)
	}
	if !p.g.AreLinked(n2, n3, nil) {
		t.Errorf("link between inode 2 and inode 3 is not created in the graph : %v", p.connections)
	}

	p.onConnLocalRemote(cc.CrossConnectEventType_DELETE, cconn1, "url1")
	p.onConnRemoteLocal(cc.CrossConnectEventType_DELETE, cconn2, "url2")
	p.onConnLocalRemote(cc.CrossConnectEventType_DELETE, cconn3, "url2")
	p.onConnRemoteLocal(cc.CrossConnectEventType_DELETE, cconn4, "url3")
	// Ensure Edge is deleted
	if p.g.AreLinked(n1, n2, nil) || p.g.AreLinked(n2, n3, nil) {
		t.Error("connections are not deleted in the graph")
	}

	if len(p.connections) != 0 {
		t.Errorf("connection list is not empty after deletion, list length: %d", len(p.connections))
	}
}

// Test that an UPDATE message for an existing connection doesn't add a new connection
func TestUpdateExistingConnection(t *testing.T) {
	p := setupProbe(t)

	cconn := createConnectionLocalOnly("1", "2")

	p.onConnLocalLocal(cc.CrossConnectEventType_INITIAL_STATE_TRANSFER, cconn, "url1")

	p.onConnLocalLocal(cc.CrossConnectEventType_UPDATE, cconn, "url1")

	if len(p.connections) != 1 {
		t.Errorf("connection count should be equal to 1 after receiving an UPDATE message for an existing connection, connection count: %d", len(p.connections))
	}

}

func TestConnectionsWithIdenticalRemoteId(t *testing.T) {
	// TODO

}

func TestDownCrossConnectFilteredOut(t *testing.T) {
	// TODO
}

// TODO : Test addconn/delconn/addNode/delNode with different orders

// TODO : Test connections to metadatas transformations
