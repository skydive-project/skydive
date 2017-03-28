/*
 * Copyright (C) 2016 Orange, Inc.
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

package probes

import (
	"errors"
	"fmt"
	"time"

	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"

	"github.com/nlewo/contrail-introspect-cli/collection"
	"github.com/nlewo/contrail-introspect-cli/descriptions"
)

type OpenContrailMapper struct {
	graph.DefaultGraphListener
	graph           *graph.Graph
	root            *graph.Node
	nodeUpdaterChan chan graph.Identifier
	vHost           *graph.Node
	pendingLinks    []*graph.Node
	agentHost       string
	agentPort       int
	mplsUDPPort     int
}

type ExtIDs struct {
	IfaceID     string
	AttachedMac string
}

func (mapper *OpenContrailMapper) retrieveExtIDs(metadata graph.Metadata, itf collection.Element) (*ExtIDs, error) {
	name := metadata["Name"].(string)

	logging.GetLogger().Debugf("Retrieving extIDs from OpenContrail for Name: %s", name)

	portUUID, _ := itf.GetField("uuid")
	if portUUID == "" {
		return nil, errors.New("No uuid field")
	}

	mac, _ := itf.GetField("mac_addr")
	if mac == "" {
		return nil, errors.New("No mac_addr field")
	}

	logging.GetLogger().Debugf("Interface from contrail: port: %s mac: %s", portUUID, mac)

	e := &ExtIDs{
		IfaceID:     portUUID,
		AttachedMac: mac,
	}

	return e, nil
}

// Since the node update is triggered by a netlink message, it happens
// the contrail vrouter agent doesn't have interface informations yet:
// for instance, the tap is first created by nova and this information
// is then propagated to contrail. We then retry to get interface from
// contrail introspect with a delay between each attempt.
func getInterfaceFromIntrospect(host string, port int, name string) (collection.Collection, collection.Element, error) {
	var err error
	try := 3
	delay := 500 * time.Millisecond

	for i := 0; i < try; i++ {
		col, e := collection.LoadCollection(descriptions.Interface(), []string{fmt.Sprintf("%s:%d", host, port)})
		err = e
		if e == nil {
			itf, e := col.SearchStrictUnique(name)
			err = e
			if e == nil {
				return col, itf, e
			}
		}
		col.Close()
		logging.GetLogger().Debugf("Retry %d: Load interface collection (previous error message: %s)\n", i+1, err)
		time.Sleep(delay)
	}
	return collection.Collection{}, collection.Element{}, err

}

func (mapper *OpenContrailMapper) onVhostAdded(node *graph.Node, itf collection.Element) {
	phyItf, _ := itf.GetField("physical_interface")
	if phyItf == "" {
		return
	}

	mapper.vHost = node

	m := graph.Metadata{"Name": phyItf}
	nodes := mapper.graph.LookupChildren(mapper.root, m, graph.Metadata{"RelationType": "ownership"})
	switch {
	case len(nodes) == 0:
		logging.GetLogger().Errorf("Physical interface %s not found", phyItf)
		return
	case len(nodes) > 1:
		logging.GetLogger().Errorf("Multiple physical interfaces found : %v", nodes)
		return
	}

	mapper.linkToVhost(nodes[0])

	for _, n := range mapper.pendingLinks {
		mapper.linkToVhost(n)
	}
	mapper.pendingLinks = mapper.pendingLinks[:0]

	mapper.graph.AddMetadata(nodes[0], "MPLSUDPPort", mapper.mplsUDPPort)
}

func (mapper *OpenContrailMapper) linkToVhost(node *graph.Node) {
	if mapper.vHost != nil {
		md := graph.Metadata{"RelationType": "layer2"}
		if !mapper.graph.AreLinked(node, mapper.vHost, md) {
			logging.GetLogger().Debugf("Link %s to %s", node.String(), mapper.vHost.String())
			mapper.graph.Link(node, mapper.vHost, md)
		}
	} else {
		logging.GetLogger().Debugf("Add node %s to pending link list", node.String())
		mapper.pendingLinks = append(mapper.pendingLinks, node)
	}
}

func (mapper *OpenContrailMapper) nodeUpdater() {
	body := func(nodeID graph.Identifier) {
		mapper.graph.RLock()
		node := mapper.graph.GetNode(nodeID)
		if node == nil {
			mapper.graph.RUnlock()
			return
		}
		name, _ := node.GetFieldString("Name")
		mapper.graph.RUnlock()

		if name == "" {
			return
		}

		col, itf, err := getInterfaceFromIntrospect(mapper.agentHost, mapper.agentPort, name)
		if err != nil {
			logging.GetLogger().Debugf("%s\n", err)
			return
		}
		defer col.Close()

		mapper.graph.Lock()
		defer mapper.graph.Unlock()

		// We get the node again to be sure to have the latest
		// version.
		// NOTE(safchain) does this really useful, I mean why getter one more time the same node ?
		node = mapper.graph.GetNode(nodeID)
		if node == nil {
			return
		}

		if n, _ := node.GetFieldString("Name"); n != name {
			logging.GetLogger().Warningf("Node with name %s has changed", name)
			return
		}

		if tp, _ := node.GetFieldString("Type"); tp == "vhost" {
			mapper.onVhostAdded(node, itf)
		} else {
			logging.GetLogger().Debugf("Retrieve extIDs for %s", name)
			extIDs, err := mapper.retrieveExtIDs(node.Metadata(), itf)
			if err != nil {
				return
			}
			mapper.updateNode(node, extIDs)
			mapper.linkToVhost(node)
		}
	}

	logging.GetLogger().Debugf("Starting OpenContrail updater (using the vrouter agent on %s:%d)", mapper.agentHost, mapper.agentPort)
	for nodeID := range mapper.nodeUpdaterChan {
		// We launch the node update in a routine because
		// several retries can be realized to get the
		// interface from the contrail introspect
		go body(nodeID)
	}
	logging.GetLogger().Debugf("Stopping OpenContrail updater")
}

func (mapper *OpenContrailMapper) updateNode(node *graph.Node, extIDs *ExtIDs) {
	tr := mapper.graph.StartMetadataTransaction(node)
	defer tr.Commit()

	tr.AddMetadata("ExtID/iface-id", extIDs.IfaceID)
	tr.AddMetadata("ExtID/attached-mac", extIDs.AttachedMac)
}

func (mapper *OpenContrailMapper) enhanceNode(node *graph.Node) {
	// To break update loops
	if attachedMAC, _ := node.GetFieldString("ExtID/attached-mac"); attachedMAC != "" {
		return
	}

	ifType, _ := node.GetFieldString("Type")
	if ifType == "" {
		return
	}

	if ifType != "host" && ifType != "netns" {
		mapper.nodeUpdaterChan <- node.ID
	}
}

func (mapper *OpenContrailMapper) OnNodeUpdated(n *graph.Node) {
	return
}

func (mapper *OpenContrailMapper) OnNodeAdded(n *graph.Node) {
	mapper.enhanceNode(n)
}

func (mapper *OpenContrailMapper) OnNodeDeleted(n *graph.Node) {
	name, _ := n.GetFieldString("Name")
	if name == "" {
		return
	}
	if n.ID == mapper.vHost.ID {
		logging.GetLogger().Debugf("Removed %s", name)
		mapper.vHost = nil
	}
}

func (mapper *OpenContrailMapper) Start() {
	go mapper.nodeUpdater()
}

func (mapper *OpenContrailMapper) Stop() {
	mapper.graph.RemoveEventListener(mapper)
	close(mapper.nodeUpdaterChan)
}

func NewOpenContrailMapper(g *graph.Graph, r *graph.Node) *OpenContrailMapper {
	host := config.GetConfig().GetString("opencontrail.host")
	port := config.GetConfig().GetInt("opencontrail.port")
	mplsUDPPort := config.GetConfig().GetInt("opencontrail.mpls_udp_port")
	if host == "" {
		host = "localhost"
	}
	if port == 0 {
		port = 8085
	}

	mapper := &OpenContrailMapper{graph: g, root: r, agentHost: host, agentPort: port, mplsUDPPort: mplsUDPPort}
	mapper.nodeUpdaterChan = make(chan graph.Identifier, 500)
	g.AddEventListener(mapper)
	return mapper
}
