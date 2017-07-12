/*
 * Copyright (C) 2016 Red Hat, Inc.
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

package analyzer

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/skydive-project/skydive/api"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"
)

// TopologyForwarderPeer describes a topology forwarder peer
type TopologyForwarderPeer struct {
	shttp.DefaultWSClientEventHandler
	Addr        string
	Port        int
	Graph       *graph.Graph
	AuthOptions *shttp.AuthenticationOpts
	wsclient    *shttp.WSAsyncClient
	host        string
}

// TopologyForwarder describes a topology forwarder
type TopologyForwarder struct {
	shttp.DefaultWSServerEventHandler
	Graph       *graph.Graph
	AuthOptions *shttp.AuthenticationOpts
	peers       []*TopologyForwarderPeer
	wg          sync.WaitGroup
}

func (p *TopologyForwarderPeer) getHostID() string {
	client := shttp.NewRestClient(p.Addr, p.Port, p.AuthOptions)
	contentReader := bytes.NewReader([]byte{})

	var data []byte
	var info api.Info

	for {
		resp, err := client.Request("GET", "api", contentReader, nil)
		if err != nil {
			goto NotReady
		}

		if resp.StatusCode != http.StatusOK {
			goto NotReady
		}

		data, _ = ioutil.ReadAll(resp.Body)
		if len(data) == 0 {
			goto NotReady
		}

		if err := json.Unmarshal(data, &info); err != nil {
			goto NotReady
		}
		p.host = info.Host

		return p.host

	NotReady:
		time.Sleep(1 * time.Second)
	}
}

// OnConnected send the whole local graph the remote peer(analyzer) once connected
func (p *TopologyForwarderPeer) OnConnected(c *shttp.WSAsyncClient) {
	logging.GetLogger().Infof("Send the whole graph to: %s", p.host)

	p.Graph.RLock()
	defer p.Graph.RUnlock()

	// re-added all the nodes and edges
	p.wsclient.SendWSMessage(shttp.NewWSMessage(graph.Namespace, graph.SyncReplyMsgType, p.Graph))
}

func (p *TopologyForwarderPeer) connect(wg *sync.WaitGroup) {
	defer wg.Done()

	// check whether the peer is the analyzer itself or not thanks to the /api
	if p.getHostID() == config.GetConfig().GetString("host_id") {
		logging.GetLogger().Debugf("No connection to %s:%d as it's me", p.Addr, p.Port)
		return
	}

	authClient := shttp.NewAuthenticationClient(p.Addr, p.Port, p.AuthOptions)
	p.wsclient = shttp.NewWSAsyncClientFromConfig(common.AnalyzerService, p.Addr, p.Port, "/ws", authClient)
	p.wsclient.AddEventHandler(p, []string{})

	p.wsclient.Connect()
}

func (p *TopologyForwarderPeer) disconnect() {
	if p.wsclient != nil {
		p.wsclient.Disconnect()
	}
}

// OnMessage websocket event
func (a *TopologyForwarder) OnMessage(c *shttp.WSClient, msg shttp.WSMessage) {
	for _, peer := range a.peers {
		// we forward message whether the service is not an analyzer or the HosID is not the same
		// so that we forward all external messages to skydive and we avoid loop.
		if peer.wsclient != nil && (c.ClientType != common.AnalyzerService || peer.host != c.Host) {
			peer.wsclient.SendWSMessage(&msg)
		}
	}
}

func (a *TopologyForwarder) addPeer(addr string, port int, g *graph.Graph) {
	peer := &TopologyForwarderPeer{
		Addr:        addr,
		Port:        port,
		Graph:       g,
		AuthOptions: a.AuthOptions,
	}

	a.peers = append(a.peers, peer)
}

// ConnectAll peers
func (a *TopologyForwarder) ConnectAll() {
	for _, peer := range a.peers {
		a.wg.Add(1)
		go peer.connect(&a.wg)
	}
}

// DisconnectAll peers
func (a *TopologyForwarder) DisconnectAll() {
	for _, peer := range a.peers {
		peer.disconnect()
	}
	a.wg.Wait()
}

// NewTopologyForwarder creates a new topology forwarder based graph and webserver
func NewTopologyForwarder(g *graph.Graph, server *shttp.WSServer, authOptions *shttp.AuthenticationOpts) *TopologyForwarder {
	tf := &TopologyForwarder{
		Graph:       g,
		AuthOptions: authOptions,
		peers:       make([]*TopologyForwarderPeer, 0),
	}
	server.AddEventHandler(tf, []string{graph.Namespace})

	return tf
}

// NewTopologyForwarderFromConfig creates a new topology forwarder based on configration
func NewTopologyForwarderFromConfig(g *graph.Graph, server *shttp.WSServer) *TopologyForwarder {
	authOptions := &shttp.AuthenticationOpts{
		Username: config.GetConfig().GetString("auth.analyzer_username"),
		Password: config.GetConfig().GetString("auth.analyzer_password"),
	}
	tp := NewTopologyForwarder(g, server, authOptions)

	addresses, err := config.GetAnalyzerServiceAddresses()
	if err != nil {
		logging.GetLogger().Errorf("Unable to get the analyzers list: %s", err.Error())
		return nil
	}

	for _, sa := range addresses {
		tp.addPeer(sa.Addr, sa.Port, g)
	}

	return tp
}
