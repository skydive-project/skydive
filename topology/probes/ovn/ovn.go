//go:generate go run ../../../scripts/gendecoder.go -package github.com/skydive-project/skydive/topology/probes/ovn

/*
 * Copyright (C) 2019 Red Hat, Inc.
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

package ovn

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/skydive-project/skydive/common"

	goovn "github.com/ebay/go-ovn"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology"
)

type ovnEvent func()

// Probe describes an OVN probe
type Probe struct {
	graph.ListenerHandler
	graph       *graph.Graph
	wg          sync.WaitGroup
	socketfile  string
	protocol    string
	server      string
	port        int
	ovndbapi    goovn.OVNDBApi
	switchPorts map[string]*goovn.LogicalSwitch
	eventChan   chan ovnEvent
	aclIndexer  *graph.Indexer
	lsIndexer   *graph.Indexer
	lspIndexer  *graph.Indexer
	lrIndexer   *graph.Indexer
	lrpIndexer  *graph.Indexer
	spLinker    *graph.ResourceLinker
	srLinker    *graph.MetadataIndexerLinker
	rpLinker    *graph.ResourceLinker
	aclLinker   *graph.ResourceLinker
	ifaceLinker *graph.MetadataIndexerLinker
}

// Metadata describes the information of an OVN object
// easyjson:json
// gendecoder
type Metadata struct {
	LSPMetadata `json:",omitempty"`
	LRPMetadata `json:",omitempty"`
	ACLMetadata `json:",omitempty"`

	ExtID   graph.Metadata `json:",omitempty" field:"Metadata"`
	Options graph.Metadata `json:",omitempty" field:"Metadata"`
}

// LSPMetadata describes the information of an OVN logical router
// easyjson:json
// gendecoder
type LSPMetadata struct {
	Addresses     []string `json:",omitempty"`
	PortSecurity  []string `json:",omitempty"`
	DHCPv4Options string   `json:",omitempty"`
	DHCPv6Options string   `json:",omitempty"`
	Type          string   `json:",omitempty"`
}

// LRPMetadata describes the information of an OVN logical router port
// easyjson:json
// gendecoder
type LRPMetadata struct {
	GatewayChassis []string       `json:",omitempty"`
	IPv6RAConfigs  graph.Metadata `json:",omitempty" field:"Metadata"`
	Networks       []string       `json:",omitempty"`
	Peer           string         `json:",omitempty"`
}

// ACLMetadata describes the information of an OVN ACL
// easyjson:json
// gendecoder
type ACLMetadata struct {
	Action    string `json:",omitempty"`
	Direction string `json:",omitempty"`
	Log       bool   `json:",omitempty"`
	Match     string `json:",omitempty"`
	Priority  int64  `json:",omitempty"`
}

// MetadataDecoder implements a json message raw decoder
func MetadataDecoder(raw json.RawMessage) (common.Getter, error) {
	var m Metadata
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("unable to unmarshal OVN metadata %s: %s", string(raw), err)
	}

	return &m, nil
}

func uuidHasher(n *graph.Node) map[string]interface{} {
	if uuid, err := n.GetFieldString("UUID"); err != nil {
		return map[string]interface{}{uuid: nil}
	}
	return nil
}

type switchPortLinker struct {
	probe *Probe
}

// GetABLinks returns all the links from a specifed logical switch to its logical ports
func (l *switchPortLinker) GetABLinks(lsNode *graph.Node) (edges []*graph.Edge) {
	probe := l.probe
	name, _ := lsNode.GetFieldString("Name")
	ports, _ := l.probe.ovndbapi.GetLogicalSwitchPortsBySwitch(name)
	for _, lp := range ports {
		if lpNode, _ := probe.lspIndexer.GetNode(lp.UUID); lpNode != nil {
			link, err := topology.NewLink(probe.graph, lsNode, lpNode, topology.OwnershipLink, nil)
			if err != nil {
				logging.GetLogger().Error(err)
				continue
			}
			edges = append(edges, link)
		}
	}
	return edges
}

// GetBALinks returns all the links from a logical switch to the specified logical port
func (l *switchPortLinker) GetBALinks(lpNode *graph.Node) (edges []*graph.Edge) {
	probe := l.probe
	uuid, _ := lpNode.GetFieldString("UUID")
	switches, _ := l.probe.ovndbapi.GetLogicalSwitches()
	for _, ls := range switches {
		ports, _ := l.probe.ovndbapi.GetLogicalSwitchPortsBySwitch(ls.Name)
		for _, lp := range ports {
			if lp.UUID == uuid {
				if lsNode, _ := probe.lsIndexer.GetNode(ls.UUID); lsNode != nil {
					link, err := topology.NewLink(probe.graph, lsNode, lpNode, topology.OwnershipLink, nil)
					if err != nil {
						logging.GetLogger().Error(link)
						continue
					}
					edges = append(edges, link)
				}
			}
		}
	}
	return edges
}

type routerPortLinker struct {
	probe *Probe
}

// GetABLinks returns all the links from a specifed logical router to its logical ports
func (l *routerPortLinker) GetABLinks(lrNode *graph.Node) (edges []*graph.Edge) {
	probe := l.probe
	name, _ := lrNode.GetFieldString("Name")
	ports, _ := l.probe.ovndbapi.GetLogicalRouterPortsByRouter(name)
	for _, lp := range ports {
		if lrpNode, _ := probe.lrpIndexer.GetNode(lp.UUID); lrpNode != nil {
			link, err := topology.NewLink(probe.graph, lrNode, lrpNode, topology.OwnershipLink, nil)
			if err != nil {
				logging.GetLogger().Error(err)
				continue
			}
			edges = append(edges, link)
		}
	}
	return edges
}

// GetBALinks returns all the links from a logical router to the specified logical port
func (l *routerPortLinker) GetBALinks(lrpNode *graph.Node) (edges []*graph.Edge) {
	probe := l.probe
	uuid, _ := lrpNode.GetFieldString("UUID")
	routers, _ := l.probe.ovndbapi.GetLogicalRouters()
	for _, lr := range routers {
		ports, _ := l.probe.ovndbapi.GetLogicalRouterPortsByRouter(lr.Name)
		for _, lp := range ports {
			if lp.UUID == uuid {
				if lrNode, _ := probe.lrIndexer.GetNode(lr.UUID); lrNode != nil {
					link, err := topology.NewLink(probe.graph, lrNode, lrpNode, topology.OwnershipLink, nil)
					if err != nil {
						logging.GetLogger().Error(link)
						continue
					}
					edges = append(edges, link)
				}
			}
		}
	}
	return edges
}

type aclLinker struct {
	probe *Probe
}

// GetABLinks returns all the links from a specified port group to its ACLs
func (l *aclLinker) GetABLinks(lsNode *graph.Node) (edges []*graph.Edge) {
	name, _ := lsNode.GetFieldString("Name")
	acls, _ := l.probe.ovndbapi.GetACLsBySwitch(name)
	for _, acl := range acls {
		if aclNode, _ := l.probe.aclIndexer.GetNode(acl.UUID); aclNode != nil {
			if link, _ := topology.NewLink(l.probe.graph, lsNode, aclNode, topology.OwnershipLink, nil); link != nil {
				edges = append(edges, link)
			}
		}
	}
	return edges
}

// GetBALinks returns all the links from a port group to the specified ACL
func (l *aclLinker) GetBALinks(aclNode *graph.Node) (edges []*graph.Edge) {
	uuid, _ := aclNode.GetFieldString("Name")
	switches, _ := l.probe.ovndbapi.GetLogicalSwitches()
	for _, ls := range switches {
		acls, _ := l.probe.ovndbapi.GetACLsBySwitch(ls.Name)
		for _, acl := range acls {
			if acl.UUID == uuid {
				if lsNode, _ := l.probe.lsIndexer.GetNode(ls.UUID); lsNode != nil {
					link, err := topology.NewLink(l.probe.graph, lsNode, aclNode, topology.OwnershipLink, nil)
					if err != nil {
						logging.GetLogger().Error(link)
						continue
					}
					edges = append(edges, link)
				}
			}
		}
	}
	return edges
}

func (p *Probe) registerNode(indexer *graph.Indexer, uuid string, metadata graph.Metadata) {
	logging.GetLogger().Debugf("Registering OVN object with UUID %s and metadata %+v", uuid, metadata)

	p.graph.Lock()
	defer p.graph.Unlock()

	id := graph.GenID(uuid)
	node, _ := indexer.GetNode(uuid)
	if node == nil {
		n, err := p.graph.NewNode(id, metadata)
		if err != nil {
			logging.GetLogger().Error(err)
			return
		}
		node = n
	} else {
		p.graph.SetMetadata(node, metadata)
	}

	indexer.Index(id, node, map[string]interface{}{uuid: node})
}

func (p *Probe) unregisterNode(indexer *graph.Indexer, uuid string) {
	logging.GetLogger().Debugf("Unregistering OVN object with UUID %s", uuid)

	p.graph.Lock()
	defer p.graph.Unlock()

	node, _ := indexer.GetNode(uuid)
	if node != nil {
		p.graph.DelNode(node)
		indexer.Unindex(node.ID, node)
	}
}

func (p *Probe) logicalSwitchMetadata(ls *goovn.LogicalSwitch) graph.Metadata {
	return graph.Metadata{
		"Type":    "logical_switch",
		"Name":    ls.Name,
		"Manager": "ovn",
		"UUID":    ls.UUID,
		"OVN": &Metadata{
			ExtID: common.NormalizeValue(ls.ExternalID).(map[string]interface{}),
		},
	}
}

func (p *Probe) logicalRouterMetadata(lr *goovn.LogicalRouter) graph.Metadata {
	return graph.Metadata{
		"Type":    "logical_router",
		"Name":    lr.Name,
		"Manager": "ovn",
		"UUID":    lr.UUID,
		"OVN": &Metadata{
			ExtID: common.NormalizeValue(lr.ExternalID).(map[string]interface{}),
		},
	}
}

// OnLogicalSwitchCreate is called when a logical switch is created
func (p *Probe) OnLogicalSwitchCreate(ls *goovn.LogicalSwitch) {
	p.eventChan <- func() { p.registerNode(p.lsIndexer, ls.UUID, p.logicalSwitchMetadata(ls)) }
}

// OnLogicalSwitchDelete is called when a logical switch is deleted
func (p *Probe) OnLogicalSwitchDelete(ls *goovn.LogicalSwitch) {
	p.eventChan <- func() { p.unregisterNode(p.lsIndexer, ls.UUID) }
}

func (p *Probe) logicalPortMetadata(lp *goovn.LogicalSwitchPort) graph.Metadata {
	return graph.Metadata{
		"Type":    "logical_port",
		"Name":    lp.Name,
		"UUID":    lp.UUID,
		"Manager": "ovn",
		"OVN": &Metadata{
			LSPMetadata: LSPMetadata{
				Addresses:     lp.Addresses,
				PortSecurity:  lp.PortSecurity,
				DHCPv4Options: lp.DHCPv4Options,
				DHCPv6Options: lp.DHCPv6Options,
				Type:          lp.Type,
			},
			ExtID:   common.NormalizeValue(lp.ExternalID).(map[string]interface{}),
			Options: common.NormalizeValue(lp.Options).(map[string]interface{}),
		},
	}
}

func (p *Probe) logicalRouterPortMetadata(lp *goovn.LogicalRouterPort) graph.Metadata {
	return graph.Metadata{
		"Type":    "logical_port",
		"Name":    lp.Name,
		"UUID":    lp.UUID,
		"Manager": "ovn",
		"Enabled": lp.Enabled,
		"MAC":     lp.MAC,
		"OVN": &Metadata{
			LRPMetadata: LRPMetadata{
				GatewayChassis: lp.GatewayChassis,
				IPv6RAConfigs:  common.NormalizeValue(lp.IPv6RAConfigs).(map[string]interface{}),
				Networks:       lp.Networks,
				Peer:           lp.Peer,
			},
			ExtID:   common.NormalizeValue(lp.ExternalID).(map[string]interface{}),
			Options: common.NormalizeValue(lp.Options).(map[string]interface{}),
		},
	}

}

// OnLogicalPortCreate is called when a logical port is created on a switch
func (p *Probe) OnLogicalPortCreate(lp *goovn.LogicalSwitchPort) {
	p.eventChan <- func() { p.registerNode(p.lspIndexer, lp.UUID, p.logicalPortMetadata(lp)) }
}

// OnLogicalPortDelete is called when a logical is deleted from a switch
func (p *Probe) OnLogicalPortDelete(lp *goovn.LogicalSwitchPort) {
	p.eventChan <- func() { p.unregisterNode(p.lspIndexer, lp.UUID) }
}

// OnDHCPOptionsCreate is called when DHCP options are created
func (p *Probe) OnDHCPOptionsCreate(*goovn.DHCPOptions) {
}

// OnDHCPOptionsDelete is called when DHCP options are deleted
func (p *Probe) OnDHCPOptionsDelete(*goovn.DHCPOptions) {
}

// OnLoadBalancerCreate is called when DHCP options are created
func (p *Probe) OnLoadBalancerCreate(*goovn.LoadBalancer) {
}

// OnLoadBalancerDelete is called when DHCP options are deleted
func (p *Probe) OnLoadBalancerDelete(*goovn.LoadBalancer) {
}

// OnLogicalRouterCreate is called when a logical router is created
func (p *Probe) OnLogicalRouterCreate(ls *goovn.LogicalRouter) {
	p.eventChan <- func() { p.registerNode(p.lrIndexer, ls.UUID, p.logicalRouterMetadata(ls)) }
}

// OnLogicalRouterDelete is called when a logical router is deleted
func (p *Probe) OnLogicalRouterDelete(ls *goovn.LogicalRouter) {
	p.eventChan <- func() { p.unregisterNode(p.lrIndexer, ls.UUID) }
}

// OnLogicalRouterPortCreate is called when a logical port is created on a router
func (p *Probe) OnLogicalRouterPortCreate(lp *goovn.LogicalRouterPort) {
	p.eventChan <- func() { p.registerNode(p.lrpIndexer, lp.UUID, p.logicalRouterPortMetadata(lp)) }
}

// OnLogicalRouterPortDelete is called when a logical port is removed from a router
func (p *Probe) OnLogicalRouterPortDelete(lp *goovn.LogicalRouterPort) {
	p.eventChan <- func() { p.unregisterNode(p.lrpIndexer, lp.UUID) }
}

// OnQoSCreate is called when QoS is created
func (p *Probe) OnQoSCreate(*goovn.QoS) {
}

// OnQoSDelete is called when QoS is deleted
func (p *Probe) OnQoSDelete(*goovn.QoS) {
}

func (p *Probe) aclMetadata(acl *goovn.ACL) graph.Metadata {
	return graph.Metadata{
		"Type":    "acl",
		"Name":    acl.UUID,
		"Manager": "ovn",
		"OVN": Metadata{
			ACLMetadata: ACLMetadata{
				Action:    acl.Action,
				Direction: acl.Direction,
				Log:       acl.Log,
				Match:     acl.Match,
				Priority:  int64(acl.Priority),
			},
			ExtID: common.NormalizeValue(acl.ExternalID).(map[string]interface{}),
		},
	}
}

// OnACLCreate is called when an ACL is created
func (p *Probe) OnACLCreate(acl *goovn.ACL) {
	p.eventChan <- func() { p.registerNode(p.aclIndexer, acl.UUID, p.aclMetadata(acl)) }
}

// OnACLDelete is called when an ACL is deleted
func (p *Probe) OnACLDelete(acl *goovn.ACL) {
	p.eventChan <- func() { p.unregisterNode(p.aclIndexer, acl.UUID) }
}

// OnError is called when an error occurred in an indexer
func (p *Probe) OnError(err error) {
	logging.GetLogger().Error(err)
}

// Start the probe
func (p *Probe) Start() {
	p.lsIndexer.Start()
	p.lspIndexer.Start()
	p.lrIndexer.Start()
	p.lrpIndexer.Start()
	p.aclIndexer.Start()
	p.spLinker.Start()
	p.rpLinker.Start()
	p.aclLinker.Start()
	p.srLinker.Start()
	p.ifaceLinker.Start()

	var err error
	logging.GetLogger().Debugf("Trying to get an OVN DB api")
	p.ovndbapi, err = goovn.GetInstance(p.socketfile, p.protocol, p.server, p.port, p)
	if err != nil {
		logging.GetLogger().Error(err)
		return
	}
	logging.GetLogger().Debugf("Successfully got an OVN DB api")

	// Initial synchronization
	switches, _ := p.ovndbapi.GetLogicalSwitches()
	for _, ls := range switches {
		p.OnLogicalSwitchCreate(ls)

		ports, _ := p.ovndbapi.GetLogicalSwitchPortsBySwitch(ls.Name)
		for _, lp := range ports {
			p.OnLogicalPortCreate(lp)
		}

		acls, _ := p.ovndbapi.GetACLsBySwitch(ls.Name)
		for _, acl := range acls {
			p.OnACLCreate(acl)
		}
	}

	routers, _ := p.ovndbapi.GetLogicalRouters()
	for _, lr := range routers {
		p.OnLogicalRouterCreate(lr)

		ports, _ := p.ovndbapi.GetLogicalRouterPortsByRouter(lr.Name)
		for _, lp := range ports {
			p.OnLogicalRouterPortCreate(lp)
		}
	}

	p.wg.Add(1)

	go func() {
		defer p.wg.Done()

		for eventCallback := range p.eventChan {
			eventCallback()
		}
	}()
}

// Stop the probe
func (p *Probe) Stop() {
	close(p.eventChan)
	p.wg.Wait()
	p.lsIndexer.Stop()
	p.lspIndexer.Stop()
	p.lrIndexer.Stop()
	p.lrpIndexer.Stop()
	p.aclIndexer.Stop()
	p.spLinker.Stop()
	p.aclLinker.Stop()
	p.rpLinker.Stop()
	p.ifaceLinker.Stop()
}

// NewProbe creates a new graph OVS database probe
func NewProbe(g *graph.Graph, address string) (*Probe, error) {
	port, socketfile, server := 0, "", ""

	protocol, target, err := common.ParseAddr(address)
	if err != nil {
		return nil, err
	}

	switch protocol {
	case "unix":
		protocol, socketfile = goovn.UNIX, target
	case "tcp":

		sa, err := common.ServiceAddressFromString(target)
		if err != nil {
			return nil, err
		}
		protocol, server, port = goovn.TCP, sa.Addr, sa.Port
	default:
		return nil, fmt.Errorf("unsupported protocol %s", protocol)
	}

	probe := &Probe{
		graph:      g,
		protocol:   protocol,
		socketfile: socketfile,
		server:     server,
		port:       port,
		eventChan:  make(chan ovnEvent, 50),
		aclIndexer: graph.NewIndexer(g, nil, uuidHasher, false),
		lsIndexer:  graph.NewIndexer(g, nil, uuidHasher, false),
		lspIndexer: graph.NewIndexer(g, nil, uuidHasher, false),
		lrIndexer:  graph.NewIndexer(g, nil, uuidHasher, false),
		lrpIndexer: graph.NewIndexer(g, nil, uuidHasher, false),
	}

	// Link logical switches to their ports
	probe.spLinker = graph.NewResourceLinker(g,
		[]graph.ListenerHandler{probe.lsIndexer},
		[]graph.ListenerHandler{probe.lspIndexer},
		&switchPortLinker{probe: probe}, nil)

	// Link logical routers to their ports
	probe.rpLinker = graph.NewResourceLinker(g,
		[]graph.ListenerHandler{probe.lrIndexer},
		[]graph.ListenerHandler{probe.lrpIndexer},
		&routerPortLinker{probe: probe}, nil)

	// Link ports switches to their ACLs
	probe.aclLinker = graph.NewResourceLinker(g,
		[]graph.ListenerHandler{probe.lspIndexer},
		[]graph.ListenerHandler{probe.aclIndexer},
		&aclLinker{probe: probe}, nil)

	// We create a metadata indexer linker to link the logical switch ports that have
	// an Options.router-port attribute to the logical router port with the specified name
	lspIndexer := graph.NewMetadataIndexer(g, probe.lspIndexer, nil, "OVN.Options.router-port")
	lspIndexer.Start()

	lrpIndexer := graph.NewMetadataIndexer(g, probe.lrpIndexer, graph.Metadata{"Type": "logical_port"}, "Name")
	lrpIndexer.Start()

	probe.srLinker = graph.NewMetadataIndexerLinker(g, lspIndexer, lrpIndexer, graph.Metadata{"RelationType": "layer2"})

	// We create an other metadata indexer linker to link the OVS interfaces to their logical OVN port
	// To do so, we use the ExtID.iface-id attribute to link an interface to the logical
	// switch port with the specified name
	ifaceIndexer := graph.NewMetadataIndexer(g, g, nil, "ExtID.iface-id")
	ifaceIndexer.Start()

	lspIndexer = graph.NewMetadataIndexer(g, probe.lspIndexer, graph.Metadata{"Type": "logical_port"}, "Name")
	lspIndexer.Start()

	probe.ifaceLinker = graph.NewMetadataIndexerLinker(g, ifaceIndexer, lspIndexer, graph.Metadata{"RelationType": "mapping"})

	// Handle linkers errors
	probe.aclLinker.AddEventListener(probe)
	probe.rpLinker.AddEventListener(probe)
	probe.spLinker.AddEventListener(probe)
	probe.srLinker.AddEventListener(probe)
	probe.ifaceLinker.AddEventListener(probe)

	return probe, nil
}
