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
	"math/rand"
	"strconv"
	"testing"

	"github.com/networkservicemesh/networkservicemesh/controlplane/pkg/apis/crossconnect"
	cc "github.com/networkservicemesh/networkservicemesh/controlplane/pkg/apis/crossconnect"
	localconn "github.com/networkservicemesh/networkservicemesh/controlplane/pkg/apis/local/connection"
	remoteconn "github.com/networkservicemesh/networkservicemesh/controlplane/pkg/apis/remote/connection"
	"github.com/skydive-project/skydive/common"
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
		Context:        make(map[string]string),
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

func TestOnConnLocal_create_and_delete(t *testing.T) {
	config.Set("logging.level", "DEBUG")
	backend, err := graph.NewBackendByName("memory", nil)
	if err != nil {
		t.Errorf("Can't create the skydive backend, error: %v", err)
	}

	g := graph.NewGraph("host_test", backend, common.AnalyzerService)
	p, err := NewNsmProbe(g)
	p.Start()
	if err != nil {
		t.Errorf("Can't create the NSM probe, error: %v", err)
	}
	localSrc := createLocalSource()
	localSrc.LocalSource.GetMechanism().Parameters[localconn.NetNsInodeKey] = "1"

	localDst := createLocalDest()
	localDst.LocalDestination.GetMechanism().Parameters[localconn.NetNsInodeKey] = "2"

	cconn := &cc.CrossConnect{
		Id:          "CrossConnectID",
		Payload:     "CrossConnectPayload",
		Source:      localSrc,
		Destination: localDst,
	}

	p.onConnLocalLocal(crossconnect.CrossConnectEventType_INITIAL_STATE_TRANSFER, cconn)

	// Ensure the link is correctly created
	// TODO: test the metadata is correct
	c := p.connections[0].(*localConnectionPair)
	if c.srcInode != 1 || c.dstInode != 2 {
		t.Error("probe doesn't have the correct link")
	}

	// Add nodes to the graph
	m1 := graph.Metadata{
		"Inode": 1,
	}
	n1 := p.g.NewNode(graph.GenID(), m1)
	p.g.AddNode(n1)

	m2 := graph.Metadata{
		"Inode": 2,
	}
	n2 := p.g.NewNode(graph.GenID(), m2)
	p.g.AddNode(n2)

	// Ensure Edge is created
	if !p.g.AreLinked(n1, n2, nil) {
		t.Error("link is not created in the graph")
	}

	// TODO: test metadatas
	p.onConnLocalLocal(crossconnect.CrossConnectEventType_DELETE, cconn)
	if len(p.connections) != 0 {
		t.Error("link list is not empty after deletion")
	}

	// Ensure Edge is deleted
	if p.g.AreLinked(n1, n2, nil) {
		t.Error("link is not deleted in the graph")
	}
}

func CreateConnectionWithRemote(inodeSrc string, inodeDst string) (*cc.CrossConnect, *cc.CrossConnect) {
	localSrc := createLocalSource()
	localSrc.LocalSource.GetMechanism().Parameters[localconn.NetNsInodeKey] = inodeSrc

	localDst := createLocalDest()
	localDst.LocalDestination.GetMechanism().Parameters[localconn.NetNsInodeKey] = inodeDst

	remote := &remoteconn.Connection{
		Id:             strconv.Itoa(rand.Int()),
		NetworkService: ns,
		Context:        make(map[string]string),
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

func TestOnConnRemote_create_and_delete(t *testing.T) {
	backend, err := graph.NewBackendByName("memory", nil)
	if err != nil {
		t.Errorf("Can't create the skydive backend, error: %v", err)
	}

	g := graph.NewGraph("host_test", backend, common.AnalyzerService)
	p, err := NewNsmProbe(g)
	if err != nil {
		t.Errorf("Can't create the NSM probe, error: %v", err)
	}
	p.Start()
	cconn1, cconn2 := CreateConnectionWithRemote("1", "2")
	p.onConnLocalRemote(crossconnect.CrossConnectEventType_INITIAL_STATE_TRANSFER, cconn1)
	p.onConnRemoteLocal(crossconnect.CrossConnectEventType_INITIAL_STATE_TRANSFER, cconn2)

	srcInode, dstInode := p.connections[0].GetInodes()
	if srcInode != 1 || dstInode != 2 {
		t.Errorf("Probe doesn't have the correct connection.\nNumber of conn : %d.\nConnections are: \n %+v", len(p.connections), p.connections)
	}

	// Add nodes to the graph
	m1 := graph.Metadata{
		"Inode": 1,
	}
	n1 := p.g.NewNode(graph.GenID(), m1)
	p.g.AddNode(n1)

	m2 := graph.Metadata{
		"Inode": 2,
	}
	n2 := p.g.NewNode(graph.GenID(), m2)
	p.g.AddNode(n2)

	// Ensure Edge is created
	if !p.g.AreLinked(n1, n2, nil) {
		t.Error("link is not created in the graph")
	}

	p.onConnLocalRemote(crossconnect.CrossConnectEventType_DELETE, cconn1)

	// Ensure Edge is deleted
	if p.g.AreLinked(n1, n2, nil) {
		t.Error("link is not deleted in the graph")
	}

	p.onConnRemoteLocal(crossconnect.CrossConnectEventType_DELETE, cconn2)

	if len(p.connections) != 0 {
		t.Errorf("link list is not empty after deletion: %+v", p.connections)
	}
}

// This test creates two skydive connection
// One connection has its localSource equal to the localDest of the other
func TestOnConnTwoCrossConnectsWithTheSameSourceAndDest_create_and_delete(t *testing.T) {
	backend, err := graph.NewBackendByName("memory", nil)
	if err != nil {
		t.Errorf("Can't create the skydive backend, error: %v", err)
	}

	g := graph.NewGraph("host_test", backend, common.AnalyzerService)
	p, err := NewNsmProbe(g)
	if err != nil {
		t.Errorf("Can't create the NSM probe, error: %v", err)
	}
	p.Start()

	cconn1, cconn2 := CreateConnectionWithRemote("1", "2")
	p.onConnLocalRemote(crossconnect.CrossConnectEventType_INITIAL_STATE_TRANSFER, cconn1)
	p.onConnRemoteLocal(crossconnect.CrossConnectEventType_INITIAL_STATE_TRANSFER, cconn2)
	cconn3, cconn4 := CreateConnectionWithRemote("2", "3")
	p.onConnLocalRemote(crossconnect.CrossConnectEventType_INITIAL_STATE_TRANSFER, cconn3)
	p.onConnRemoteLocal(crossconnect.CrossConnectEventType_INITIAL_STATE_TRANSFER, cconn4)

	//ensure that two connections are avalaible in the connection list when looking for inode 2
	c, _ := p.getConnectionWithInode(2)
	if len(c) != 2 {
		t.Fatalf("Two connections should be available with inode 2, but only %d is retreived", len(c))
	}

	// Add nodes to the graph
	m1 := graph.Metadata{
		"Inode": 1,
	}
	n1 := p.g.NewNode(graph.GenID(), m1)
	p.g.AddNode(n1)

	m2 := graph.Metadata{
		"Inode": 2,
	}
	n2 := p.g.NewNode(graph.GenID(), m2)
	p.g.AddNode(n2)

	m3 := graph.Metadata{
		"Inode": 3,
	}
	n3 := p.g.NewNode(graph.GenID(), m3)
	p.g.AddNode(n3)

	// Ensure Edges are created
	if !p.g.AreLinked(n1, n2, nil) {
		t.Errorf("link between inode 1 and inode 2 is not created in the graph : %v", p.connections)
	}
	if !p.g.AreLinked(n2, n3, nil) {
		t.Errorf("link between inode 2 and inode 3 is not created in the graph : %v", p.connections)
	}

	p.onConnLocalRemote(crossconnect.CrossConnectEventType_DELETE, cconn1)
	p.onConnRemoteLocal(crossconnect.CrossConnectEventType_DELETE, cconn2)
	p.onConnLocalRemote(crossconnect.CrossConnectEventType_DELETE, cconn3)
	p.onConnRemoteLocal(crossconnect.CrossConnectEventType_DELETE, cconn4)
	// Ensure Edge is deleted
	if p.g.AreLinked(n1, n2, nil) || p.g.AreLinked(n2, n3, nil) {
		t.Error("connections are not deleted in the graph")
	}

	if len(p.connections) != 0 {
		t.Errorf("connection list is not empty after deletion, list length: %d", len(p.connections))
	}
}

// TODO : Test addconn/delconn/addNode/delNode with different orders

// TODO : Test connections to metadatas transformations
