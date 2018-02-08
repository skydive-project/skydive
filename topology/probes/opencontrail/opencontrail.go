// +build linux

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

package opencontrail

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/graph"

	"github.com/nlewo/contrail-introspect-cli/collection"
	"github.com/nlewo/contrail-introspect-cli/descriptions"
)

// OpenContrailProbe describes a probe that reads OpenContrail database and updates the graph
type OpenContrailProbe struct {
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

// OpenContrailMdata metadata
type OpenContrailMdata struct {
	UUID    string
	Mac     string
	VRF     string
	LocalIP string
}

func (mapper *OpenContrailProbe) retrieveMetadata(metadata graph.Metadata, itf collection.Element) (*OpenContrailMdata, error) {
	name := metadata["Name"].(string)

	logging.GetLogger().Debugf("Retrieving metadata from OpenContrail for Name: %s", name)

	portUUID, _ := itf.GetField("uuid")
	if portUUID == "" {
		return nil, errors.New("No uuid field")
	}

	mac, _ := itf.GetField("mac_addr")
	if mac == "" {
		return nil, errors.New("No mac_addr field")
	}

	vrfName, _ := itf.GetField("vrf_name")
	if vrfName == "" {
		return nil, errors.New("No vrf_name field")
	}

	mdataIP, _ := itf.GetField("mdata_ip_addr")
	if mdataIP == "" {
		return nil, errors.New("No mdata_ip_addr field")
	}

	logging.GetLogger().Debugf("Interface from contrail: port: %s mac: %s", portUUID, mac)

	e := &OpenContrailMdata{
		UUID:    portUUID,
		Mac:     mac,
		VRF:     vrfName,
		LocalIP: mdataIP,
	}

	return e, nil
}

// Since the node updates is triggered by a netlink message, it happens
// the contrail vrouter agent doesn't have interface informations yet:
// for instance, the tap is first created by nova and this information
// is then propagated to contrail. We then retry to get interface from
// contrail introspect with a delay between each attempt.
func getInterfaceFromIntrospect(host string, port int, name string) (col collection.Collection, elem collection.Element, err error) {
	getFromIntrospect := func() (err error) {
		col, err = collection.LoadCollection(descriptions.Interface(), []string{fmt.Sprintf("%s:%d", host, port)})
		if err != nil {
			return
		}
		elem, err = col.SearchStrictUnique(name)
		if err != nil {
			// Close collection before retrying
			col.Close()
			return
		}
		return
	}
	err = common.Retry(getFromIntrospect, 3, 500*time.Millisecond)
	return
}

func (mapper *OpenContrailProbe) onVhostAdded(node *graph.Node, itf collection.Element) {
	phyItf, _ := itf.GetField("physical_interface")
	if phyItf == "" {
		logging.GetLogger().Errorf("Physical interface not found")
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

func (mapper *OpenContrailProbe) linkToVhost(node *graph.Node) {
	if mapper.vHost != nil {
		if !topology.HaveLayer2Link(mapper.graph, node, mapper.vHost, nil) {
			logging.GetLogger().Debugf("Link %s to %s", node.String(), mapper.vHost.String())
			topology.AddLayer2Link(mapper.graph, node, mapper.vHost, nil)
		}
	} else {
		logging.GetLogger().Debugf("Add node %s to pending link list", node.String())
		mapper.pendingLinks = append(mapper.pendingLinks, node)
	}
}

func (mapper *OpenContrailProbe) nodeUpdater() {
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

		if tp, _ := node.GetFieldString("Type"); tp == "vhost" && strings.Contains(name, "vhost") {
			mapper.onVhostAdded(node, itf)
		} else {
			logging.GetLogger().Debugf("Retrieve extIDs for %s", name)
			extIDs, err := mapper.retrieveMetadata(node.Metadata(), itf)
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

func (mapper *OpenContrailProbe) updateNode(node *graph.Node, mdata *OpenContrailMdata) {
	tr := mapper.graph.StartMetadataTransaction(node)
	defer tr.Commit()

	tr.AddMetadata("ExtID.iface-id", mdata.UUID)
	tr.AddMetadata("ExtID.attached-mac", mdata.Mac)
	tr.AddMetadata("Contrail.VRF", mdata.VRF)
	tr.AddMetadata("Contrail.LocalIP", mdata.LocalIP)
}

func (mapper *OpenContrailProbe) enhanceNode(node *graph.Node) {
	// To break update loops
	if attachedMAC, _ := node.GetFieldString("ExtID.attached-mac"); attachedMAC != "" {
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

// OnNodeUpdated event
func (mapper *OpenContrailProbe) OnNodeUpdated(n *graph.Node) {
	return
}

// OnNodeAdded event
func (mapper *OpenContrailProbe) OnNodeAdded(n *graph.Node) {
	mapper.enhanceNode(n)
}

// OnNodeDeleted event
func (mapper *OpenContrailProbe) OnNodeDeleted(n *graph.Node) {
	name, _ := n.GetFieldString("Name")
	if name == "" {
		return
	}
	if n.ID == mapper.vHost.ID {
		logging.GetLogger().Debugf("Removed %s", name)
		mapper.vHost = nil
	}
}

// Start the probe
func (mapper *OpenContrailProbe) Start() {
	go mapper.nodeUpdater()
}

// Stop the probe
func (mapper *OpenContrailProbe) Stop() {
	mapper.graph.RemoveEventListener(mapper)
	close(mapper.nodeUpdaterChan)
}

// NewOpenContrailProbeFromConfig creates a new OpenContrail probe based on configuration
func NewOpenContrailProbeFromConfig(g *graph.Graph, r *graph.Node) (*OpenContrailProbe, error) {
	host := config.GetString("opencontrail.host")
	port := config.GetInt("opencontrail.port")
	mplsUDPPort := config.GetInt("opencontrail.mpls_udp_port")
	if host == "" {
		host = "localhost"
	}
	if port == 0 {
		port = 8085
	}

	mapper := &OpenContrailProbe{graph: g, root: r, agentHost: host, agentPort: port, mplsUDPPort: mplsUDPPort}
	mapper.nodeUpdaterChan = make(chan graph.Identifier, 500)
	g.AddEventListener(mapper)
	return mapper, nil
}
