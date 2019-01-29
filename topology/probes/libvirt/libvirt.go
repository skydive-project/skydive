// +build libvirt,linux

/*
 * Copyright (C) 2018 Orange.
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

package libvirt

import (
	"context"
	"encoding/xml"
	"fmt"
	"sync"

	"github.com/skydive-project/skydive/config"

	libvirtgo "github.com/libvirt/libvirt-go"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/logging"
)

// Probe is the libvirt probe
type Probe struct {
	sync.Mutex
	graph        *graph.Graph          // the graph
	conn         *libvirtgo.Connect    // libvirt conection
	interfaceMap map[string]*Interface // Found interfaces not yet connected.
	cidLifecycle int                   // libvirt callback id of monitor to unregister
	cidDevAdded  int                   // second monitor on devices added to domains
	uri          string                // uri of the libvirt connection
	cancel       context.CancelFunc    // cancel function
	root         *graph.Node           // root node for ownership
	tunProcessor *graph.Processor      // metadata indexer
}

// Address describes the XML coding of the pci addres of an interface in libvirt
type Address struct {
	Type     string `xml:"type,attr"`
	Domain   string `xml:"domain,attr"`
	Bus      string `xml:"bus,attr"`
	Slot     string `xml:"slot,attr"`
	Function string `xml:"function,attr"`
}

// DomainStateMap stringifies the state of a domain
var DomainStateMap map[libvirtgo.DomainState]string = map[libvirtgo.DomainState]string{
	libvirtgo.DOMAIN_NOSTATE:     "UNDEFINED",
	libvirtgo.DOMAIN_RUNNING:     "UP",
	libvirtgo.DOMAIN_BLOCKED:     "BLOCKED",
	libvirtgo.DOMAIN_PAUSED:      "PAUSED",
	libvirtgo.DOMAIN_SHUTDOWN:    "DOWN",
	libvirtgo.DOMAIN_CRASHED:     "CRASHED",
	libvirtgo.DOMAIN_PMSUSPENDED: "PMSUSPENDED",
	libvirtgo.DOMAIN_SHUTOFF:     "DOWN",
}

// Interface is XML coding of an interface in libvirt
type Interface struct {
	Mac struct {
		Address string `xml:"address,attr"`
	} `xml:"mac"`
	Target struct {
		Device string `xml:"dev,attr"`
	} `xml:"target"`
	Address Address `xml:"address"`
	Alias   struct {
		Name string `xml:"name,attr"`
	} `xml:"alias"`
	Host *graph.Node `xml:"-"`
}

// Domain is the subset of XML coding of a domain in libvirt
type Domain struct {
	Interfaces []Interface `xml:"devices>interface"`
}

// getDomainInterfaces uses libvirt to get information on the interfaces of a
// domain.
func (probe *Probe) getDomainInterfaces(
	domain *libvirtgo.Domain, // domain to query
	domainNode *graph.Node, // Node representing the domain
	constraint string, // to restrict the search to a single interface (by alias)
) (interfaces []*Interface) {
	rawXML, err := domain.GetXMLDesc(0)
	if err != nil {
		logging.GetLogger().Errorf("Cannot get XMLDesc: %s", err)
		return
	}
	d := Domain{}
	if err = xml.Unmarshal([]byte(rawXML), &d); err != nil {
		logging.GetLogger().Errorf("XML parsing error: %s", err)
		return
	}
	for _, itf := range d.Interfaces {
		if constraint == "" || constraint == itf.Alias.Name {
			itf.Host = domainNode
			itfObj := itf
			interfaces = append(interfaces, &itfObj)
		}
	}
	return
}

// registerInterfaces puts the information collected in the graph
// interfaces is an array of collected information.
func (probe *Probe) registerInterfaces(interfaces []*Interface) {
	probe.graph.Lock()
	defer probe.graph.Unlock()
	for _, itf := range interfaces {
		name := itf.Target.Device
		if name == "" {
			continue
		}
		logging.GetLogger().Debugf(
			"Libvirt interface %s on %s", name, itf.Host)
		probe.tunProcessor.DoAction(itf, name)
	}
}

// ProcessNode adds the libvirt interface information to a node of the graph
func (itf *Interface) ProcessNode(g *graph.Graph, node *graph.Node) bool {
	logging.GetLogger().Debugf("enrich %s", itf.Alias.Name)
	tr := g.StartMetadataTransaction(node)
	tr.AddMetadata("Libvirt.MAC", itf.Mac.Address)
	tr.AddMetadata("Libvirt.Domain", itf.Host)
	address := itf.Address
	formatted := fmt.Sprintf(
		"%s:%s.%s.%s.%s", address.Type, address.Domain, address.Bus,
		address.Slot, address.Function)
	tr.AddMetadata("Libvirt.Address", formatted)
	tr.AddMetadata("Libvirt.Alias", itf.Alias.Name)
	tr.AddMetadata("PeerIntfMAC", itf.Mac.Address)
	tr.Commit()
	if !g.AreLinked(node, itf.Host, graph.Metadata{"RelationType": "vlayer2"}) {
		if _, err := g.Link(node, itf.Host, graph.Metadata{"RelationType": "vlayer2"}); err != nil {
			logging.GetLogger().Error(err)
		}
	}
	return false
}

// getDomain access the graph node representing a libvirt domain
func (probe *Probe) getDomain(d *libvirtgo.Domain) *graph.Node {
	domainName, _ := d.GetName()
	probe.graph.RLock()
	defer probe.graph.RUnlock()
	return probe.graph.LookupFirstNode(graph.Metadata{"Name": domainName, "Type": "libvirt"})
}

// createOrUpdateDomain creates a new graph node representing a libvirt domain
// if necessary and updates its state.
func (probe *Probe) createOrUpdateDomain(d *libvirtgo.Domain) *graph.Node {
	g := probe.graph
	g.Lock()
	defer g.Unlock()
	domainName, _ := d.GetName()
	metadata := graph.Metadata{
		"Name": domainName,
		"Type": "libvirt",
	}

	var err error

	domainNode := g.LookupFirstNode(metadata)
	if domainNode == nil {
		domainNode, err = g.NewNode(graph.GenID(), metadata)
		if err != nil {
			logging.GetLogger().Error(err)
			return nil
		}
		if _, err = g.Link(probe.root, domainNode, graph.Metadata{"RelationType": "ownership"}); err != nil {
			logging.GetLogger().Error(err)
			return nil
		}
	}
	state, _, err := d.GetState()
	if err != nil {
		logging.GetLogger().Errorf("Cannot update domain state for %s", domainName)
	} else {
		tr := g.StartMetadataTransaction(domainNode)
		defer tr.Commit()
		tr.AddMetadata("State", DomainStateMap[state])
	}
	return domainNode
}

// deleteDomain deletes the graph node representing a libvirt domain
func (probe *Probe) deleteDomain(d *libvirtgo.Domain) {
	domainNode := probe.getDomain(d)
	if domainNode != nil {
		probe.graph.Lock()
		defer probe.graph.Unlock()
		if err := probe.graph.DelNode(domainNode); err != nil {
			logging.GetLogger().Error(err)
		}
	}
}

// Start get all domains attached to a libvirt connection
func (probe *Probe) Start() {
	// The event loop must be registered WITH its poll loop active BEFORE the
	// connection is opened. Otherwise it just does not work.
	if err := libvirtgo.EventRegisterDefaultImpl(); err != nil {
		logging.GetLogger().Errorf("libvirt event handler:  %s", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	probe.cancel = cancel
	go func() {
		for ctx.Err() == nil {
			if err := libvirtgo.EventRunDefaultImpl(); err != nil {
				logging.GetLogger().Errorf("libvirt poll loop problem: %s", err)
			}
		}
	}()
	conn, err := libvirtgo.NewConnectReadOnly(probe.uri)
	if err != nil {
		logging.GetLogger().Errorf("Failed to create libvirt connect")
		return
	}
	probe.conn = conn
	callback := func(
		c *libvirtgo.Connect, d *libvirtgo.Domain,
		event *libvirtgo.DomainEventLifecycle,
	) {
		switch event.Event {
		case libvirtgo.DOMAIN_EVENT_UNDEFINED:
			probe.deleteDomain(d)
		case libvirtgo.DOMAIN_EVENT_STARTED:
			domainNode := probe.createOrUpdateDomain(d)
			interfaces := probe.getDomainInterfaces(d, domainNode, "")
			probe.registerInterfaces(interfaces)
		case libvirtgo.DOMAIN_EVENT_DEFINED, libvirtgo.DOMAIN_EVENT_SUSPENDED,
			libvirtgo.DOMAIN_EVENT_RESUMED, libvirtgo.DOMAIN_EVENT_STOPPED,
			libvirtgo.DOMAIN_EVENT_SHUTDOWN, libvirtgo.DOMAIN_EVENT_PMSUSPENDED,
			libvirtgo.DOMAIN_EVENT_CRASHED:
			probe.createOrUpdateDomain(d)
		}
	}
	probe.cidLifecycle, err = conn.DomainEventLifecycleRegister(nil, callback)
	if err != nil {
		logging.GetLogger().Errorf(
			"Could not register the lifecycle event handler %s", err)
	}
	callbackDeviceAdded := func(
		c *libvirtgo.Connect, d *libvirtgo.Domain,
		event *libvirtgo.DomainEventDeviceAdded,
	) {
		domainNode := probe.getDomain(d)
		interfaces := probe.getDomainInterfaces(d, domainNode, event.DevAlias)
		probe.registerInterfaces(interfaces) // 0 or 1 device changed.
	}
	probe.cidDevAdded, err = conn.DomainEventDeviceAddedRegister(nil, callbackDeviceAdded)
	if err != nil {
		logging.GetLogger().Errorf(
			"Could not register the device added event handler %s", err)
	}
	domains, err := probe.conn.ListAllDomains(0)
	if err != nil {
		logging.GetLogger().Error(err)
		return
	}
	for _, domain := range domains {
		domainNode := probe.createOrUpdateDomain(&domain)
		interfaces := probe.getDomainInterfaces(&domain, domainNode, "")
		probe.registerInterfaces(interfaces)
	}
}

// Stop stops the probe
func (probe *Probe) Stop() {
	probe.cancel()
	probe.tunProcessor.Stop()
	if probe.cidLifecycle != -1 {
		probe.conn.DomainEventDeregister(probe.cidLifecycle)
		probe.conn.DomainEventDeregister(probe.cidDevAdded)
	}
	probe.conn.Close()
}

// NewProbe creates a libvirt topology probe
func NewProbe(g *graph.Graph, uri string, root *graph.Node) *Probe {
	tunProcessor := graph.NewProcessor(g, g, graph.Metadata{"Type": "tun"}, "Name")
	probe := &Probe{
		graph:        g,
		interfaceMap: make(map[string]*Interface),
		cidLifecycle: -1,
		uri:          uri,
		root:         root,
		tunProcessor: tunProcessor,
	}
	tunProcessor.Start()
	return probe
}

// NewProbeFromConfig initializes the probe
func NewProbeFromConfig(g *graph.Graph, root *graph.Node) (*Probe, error) {
	uri := config.GetString("agent.topology.libvirt.url")
	return NewProbe(g, uri, root), nil
}
