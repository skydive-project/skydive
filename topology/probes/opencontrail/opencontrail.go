// +build linux,opencontrail

/*
 * Copyright (C) 2016 Orange, Inc.
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

package opencontrail

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/avast/retry-go"
	"github.com/nlewo/contrail-introspect-cli/collection"
	"github.com/nlewo/contrail-introspect-cli/descriptions"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology"
	tp "github.com/skydive-project/skydive/topology/probes"
)

// Probe describes a probe that reads OpenContrail database and updates the graph
type Probe struct {
	graph.DefaultGraphListener
	Ctx                     tp.Context
	nodeUpdaterChan         chan graph.Identifier
	vHost                   *graph.Node
	pendingLinks            []*graph.Node
	agentHost               string
	agentPort               int
	mplsUDPPort             int
	routingTables           map[int]*RoutingTable
	routingTableUpdaterChan chan RoutingTableUpdate
	cancelCtx               context.Context
	cancelFunc              context.CancelFunc
}

func (p *Probe) retrieveMetadata(metadata graph.Metadata, itf collection.Element) (*Metadata, error) {
	name := metadata["Name"].(string)

	p.Ctx.Logger.Debugf("Retrieving metadata from OpenContrail for Name: %s", name)

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
	vrfID, err := getVrfIDFromIntrospect(p.agentHost, p.agentPort, vrfName)
	if err != nil {
		return nil, errors.New("No vrf_id found")
	}

	mdataIP, _ := itf.GetField("mdata_ip_addr")
	if mdataIP == "" {
		return nil, errors.New("No mdata_ip_addr field")
	}

	p.Ctx.Logger.Debugf("Interface from contrail: port: %s mac: %s", portUUID, mac)

	return &Metadata{
		UUID:    portUUID,
		MAC:     mac,
		VRF:     vrfName,
		VRFID:   int64(vrfID),
		LocalIP: mdataIP,
	}, nil
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
	// Retry during about 8 seconds
	err = retry.Do(getFromIntrospect, retry.Attempts(5), retry.Delay(250*time.Millisecond))
	return
}

func getVrfIDFromIntrospect(host string, port int, vrfName string) (vrfID int, err error) {
	col, err := collection.LoadCollection(descriptions.Vrf(), []string{fmt.Sprintf("%s:%d", host, port)})
	if err != nil {
		return
	}
	elem, err := col.SearchStrictUnique(vrfName)
	if err != nil {
		col.Close()
		return 0, err
	}
	field, _ := elem.GetField("ucindex")
	if field == "" {
		return 0, errors.New("No ucindex field")
	}
	vrfID, err = strconv.Atoi(field)
	if err != nil {
		return 0, err
	}
	return
}

func (p *Probe) onVhostAdded(node *graph.Node, itf collection.Element) {
	phyItf, _ := itf.GetField("physical_interface")
	if phyItf == "" {
		p.Ctx.Logger.Errorf("Physical interface not found")
		return
	}

	p.vHost = node

	m := graph.Metadata{"Name": phyItf}
	nodes := p.Ctx.Graph.LookupChildren(p.Ctx.RootNode, m, graph.Metadata{"RelationType": "ownership"})
	switch {
	case len(nodes) == 0:
		p.Ctx.Logger.Errorf("Physical interface %s not found", phyItf)
		return
	case len(nodes) > 1:
		p.Ctx.Logger.Errorf("Multiple physical interfaces found : %v", nodes)
		return
	}

	p.linkToVhost(nodes[0])

	for _, n := range p.pendingLinks {
		p.linkToVhost(n)
	}
	p.pendingLinks = p.pendingLinks[:0]

	p.Ctx.Graph.AddMetadata(nodes[0], "MPLSUDPPort", p.mplsUDPPort)
}

func (p *Probe) linkToVhost(node *graph.Node) {
	if p.vHost != nil {
		if !topology.HaveLayer2Link(p.Ctx.Graph, node, p.vHost) {
			p.Ctx.Logger.Debugf("Link %s to %s", node.String(), p.vHost.String())
			topology.AddLayer2Link(p.Ctx.Graph, node, p.vHost, nil)
		}
	} else {
		p.Ctx.Logger.Debugf("Add node %s to pending link list", node.String())
		p.pendingLinks = append(p.pendingLinks, node)
	}
}

func (p *Probe) nodeUpdater() {
	body := func(nodeID graph.Identifier) {
		p.Ctx.Graph.RLock()
		node := p.Ctx.Graph.GetNode(nodeID)
		if node == nil {
			p.Ctx.Graph.RUnlock()
			return
		}
		name, _ := node.GetFieldString("Name")
		p.Ctx.Graph.RUnlock()

		if name == "" {
			return
		}

		col, itf, err := getInterfaceFromIntrospect(p.agentHost, p.agentPort, name)
		if err != nil {
			p.Ctx.Logger.Debugf("%s\n", err)
			return
		}
		defer col.Close()

		p.Ctx.Graph.Lock()
		defer p.Ctx.Graph.Unlock()

		// We get the node again to be sure to have the latest
		// version.
		// NOTE(safchain) does this really useful, I mean why getter one more time the same node ?
		node = p.Ctx.Graph.GetNode(nodeID)
		if node == nil {
			return
		}

		if n, _ := node.GetFieldString("Name"); n != name {
			p.Ctx.Logger.Warningf("Node with name %s has changed", name)
			return
		}

		if tp, _ := node.GetFieldString("Type"); tp == "vhost" && strings.Contains(name, "vhost") {
			p.onVhostAdded(node, itf)
		} else {
			p.Ctx.Logger.Debugf("Retrieve extIDs for %s", name)
			extIDs, err := p.retrieveMetadata(node.Metadata, itf)
			if err != nil {
				return
			}
			p.updateNode(node, extIDs)
			p.linkToVhost(node)
			p.OnInterfaceAdded(int(extIDs.VRFID), extIDs.UUID)
		}

	}

	p.Ctx.Logger.Debugf("Starting OpenContrail updater (using the vrouter agent on %s:%d)", p.agentHost, p.agentPort)
	for nodeID := range p.nodeUpdaterChan {
		// We launch the node update in a routine because
		// several retries can be realized to get the
		// interface from the contrail introspect
		go body(nodeID)
	}
	p.Ctx.Logger.Debugf("Stopping OpenContrail updater")
}

func (p *Probe) updateNode(node *graph.Node, mdata *Metadata) {
	tr := p.Ctx.Graph.StartMetadataTransaction(node)
	defer tr.Commit()

	tr.AddMetadata("ExtID.iface-id", mdata.UUID)
	tr.AddMetadata("ExtID.attached-mac", mdata.MAC)
	tr.AddMetadata("Contrail", mdata)
}

func (p *Probe) enhanceNode(node *graph.Node) {
	// To break update loops
	if attachedMAC, _ := node.GetFieldString("ExtID.attached-mac"); attachedMAC != "" {
		return
	}

	ifType, _ := node.GetFieldString("Type")
	if ifType == "" {
		return
	}

	if ifType != "host" && ifType != "netns" {
		p.nodeUpdaterChan <- node.ID
	}
}

// OnNodeAdded event
func (p *Probe) OnNodeAdded(n *graph.Node) {
	p.enhanceNode(n)
}

// OnNodeDeleted event
func (p *Probe) OnNodeDeleted(n *graph.Node) {
	name, _ := n.GetFieldString("Name")
	if name == "" {
		return
	}
	if p.vHost != nil && n.ID == p.vHost.ID {
		p.Ctx.Logger.Debugf("Removed %s", name)
		p.vHost = nil
	}
	interfaceUUID, _ := n.GetFieldString("ExtID.iface-id")
	if interfaceUUID != "" {
		p.OnInterfaceDeleted(interfaceUUID)
	}
}

// Start the probe
func (p *Probe) Start() error {
	p.cancelCtx, p.cancelFunc = context.WithCancel(context.Background())

	p.Ctx.Graph.AddEventListener(p)
	go p.nodeUpdater()
	go p.rtMonitor()

	return nil
}

// Stop the probe
func (p *Probe) Stop() {
	p.cancelFunc()
	p.Ctx.Graph.RemoveEventListener(p)
	close(p.nodeUpdaterChan)
}

// NewProbe returns a new OpenContrail topology probe
func NewProbe(ctx tp.Context, bundle *probe.Bundle) (probe.Handler, error) {
	return &Probe{
		Ctx:                     ctx,
		agentHost:               ctx.Config.GetString("opencontrail.host"),
		agentPort:               ctx.Config.GetInt("opencontrail.port"),
		mplsUDPPort:             ctx.Config.GetInt("opencontrail.mpls_udp_port"),
		nodeUpdaterChan:         make(chan graph.Identifier, 500),
		routingTables:           make(map[int]*RoutingTable),
		routingTableUpdaterChan: make(chan RoutingTableUpdate, 500),
	}, nil
}

// Register registers graph metadata decoders
func Register() {
	graph.NodeMetadataDecoders["Contrail"] = MetadataDecoder
}
