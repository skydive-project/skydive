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
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/topology"

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
	tunProcessor *graph.Processor      // metadata indexer for regular interfaces
}

// Address describes the XML coding of the pci addres of an interface in libvirt
type Address struct {
	Type     string `xml:"type,attr,omitempty"`
	Domain   string `xml:"domain,attr"`
	Bus      string `xml:"bus,attr"`
	Slot     string `xml:"slot,attr"`
	Function string `xml:"function,attr"`
}

type Source struct {
	Address *Address `xml:"address"`
}

// DomainStateMap stringifies the state of a domain
var DomainStateMap = map[libvirtgo.DomainState]string{
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
	Type string `xml:"type,attr,omitempty"`
	Mac  *struct {
		Address string `xml:"address,attr"`
	} `xml:"mac"`
	Target *struct {
		Device string `xml:"dev,attr"`
	} `xml:"target"`
	Source  *Source `xml:"source"`
	Address Address `xml:"address"`
	Alias   *struct {
		Name string `xml:"name,attr"`
	} `xml:"alias"`
	Host *graph.Node `xml:"-"`
}

// HostDev is the XML coding of an host device attached to a domain in libvirt
type HostDev struct {
	Managed string `xml:"managed,attr,omitempty"`
	Mode    string `xml:"mode,attr,omitempty"`
	Type    string `xml:"type,attr,omitempty"`
	Driver  *struct {
		Name string `xml:"name,attr"`
	} `xml:"driver"`
	Alias *struct {
		Name string `xml:"name,attr"`
	} `xml:"alias"`
	Source  *Source     `xml:"source"`
	Address *Address    `xml:"address"`
	Host    *graph.Node `xml:"-"`
}

// Domain is the subset of XML coding of a domain in libvirt
type Domain struct {
	Interfaces  []Interface `xml:"devices>interface"`
	HostDevices []HostDev   `xml:"devices>hostdev"`
}

// getDomainInterfaces uses libvirt to get information on the interfaces of a
// domain.
func (probe *Probe) getDomainInterfaces(
	domain *libvirtgo.Domain, // domain to query
	domainNode *graph.Node, // Node representing the domain
	constraint string, // to restrict the search to a single interface (by alias)
) (interfaces []*Interface, hostdevs []*HostDev) {
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
		if constraint == "" || (itf.Alias != nil && constraint == itf.Alias.Name) {
			itf.Host = domainNode
			itfObj := itf
			interfaces = append(interfaces, &itfObj)
		}
	}
	for _, hostdev := range d.HostDevices {
		logging.GetLogger().Debugf("Found Hostdev %v", hostdev)
		if hostdev.Mode != "subsystem" || hostdev.Type != "pci" {
			continue
		}
		if hostdev.Source == nil || hostdev.Source.Address == nil {
			continue
		}
		hostdev.Host = domainNode
		hostdevs = append(hostdevs, &hostdev)
	}
	return
}

func (probe *Probe) makeHostDev(name string, source *Source) (node *graph.Node, err error) {
	if source == nil || source.Address == nil {
		err = errors.New("hostdev without PCI address")
		return
	}
	address := formatPciAddress(source.Address)
	node = probe.graph.LookupFirstChild(probe.root, graph.Metadata{
		"BusInfo": address,
	})
	if node == nil {
		m := graph.Metadata{
			"Name":    fmt.Sprintf("%s-%s", name, strings.TrimPrefix(address, "0000:")),
			"BusInfo": address,
			"Driver":  "pci-passthrough",
			"Type":    "device",
		}
		node, err = probe.graph.NewNode(graph.GenID(), m)
		if err != nil {
			return
		}
		_, err = topology.AddOwnershipLink(probe.graph, probe.root, node, nil)
	}
	return
}

// registerInterfaces puts the information collected in the graph
// interfaces is an array of collected information.
func (probe *Probe) registerInterfaces(interfaces []*Interface, hostdevs []*HostDev) {
	probe.graph.Lock()
	defer probe.graph.Unlock()
	for i, itf := range interfaces {
		if itf.Type == "hostdev" && itf.Source != nil {
			node, err := probe.makeHostDev("itf", itf.Source)
			if err != nil {
				logging.GetLogger().Errorf("Cannot get hostdev interface: %s", err)
				continue
			}
			logging.GetLogger().Debugf(
				"Libvirt hostdev interface %d on %s", i, itf.Host.Metadata["Name"])
			itf.ProcessNode(probe.graph, node)
		} else {
			var target string
			if itf.Target != nil {
				target = itf.Target.Device
			}
			if target == "" {
				continue
			}
			logging.GetLogger().Debugf(
				"Libvirt interface %s on %s", target, itf.Host.Metadata["Name"])
			probe.tunProcessor.DoAction(itf, target)
		}
	}

	for i, hdev := range hostdevs {
		node, err := probe.makeHostDev("hdev", hdev.Source)
		if err != nil {
			logging.GetLogger().Errorf("Cannot get hostdev: %s", err)
			continue
		}
		logging.GetLogger().Debugf(
			"hostdev interface %d on %s", i, hdev.Host.Metadata["Name"])
		enrichHostDev(hdev, probe.graph, node)
	}
}

func formatPciAddress(address *Address) string {
	return fmt.Sprintf(
		"%s:%s:%s.%s",
		strings.TrimPrefix(address.Domain, "0x"),
		strings.TrimPrefix(address.Bus, "0x"),
		strings.TrimPrefix(address.Slot, "0x"),
		strings.TrimPrefix(address.Function, "0x"))
}

// ProcessNode adds the libvirt interface information to a node of the graph
func (itf *Interface) ProcessNode(g *graph.Graph, node *graph.Node) bool {
	var alias string
	if itf.Alias != nil {
		alias = itf.Alias.Name
	}
	logging.GetLogger().Debugf("enrich %s", alias)
	tr := g.StartMetadataTransaction(node)
	if itf.Mac != nil {
		tr.AddMetadata("Libvirt.MAC", itf.Mac.Address)
		tr.AddMetadata("PeerIntfMAC", itf.Mac.Address)
	}
	tr.AddMetadata("Libvirt.Domain", itf.Host.Metadata["Name"])
	address := itf.Address
	formatted := formatPciAddress(&address)
	tr.AddMetadata("Libvirt.BusType", address.Type)
	tr.AddMetadata("Libvirt.BusInfo", formatted)
	tr.AddMetadata("Libvirt.Alias", alias)
	if err := tr.Commit(); err != nil {
		logging.GetLogger().Errorf("Metadata transaction failed: %s", err)
	}
	if !topology.HaveLink(g, node, itf.Host, "vlayer2") {
		if _, err := topology.AddLink(g, node, itf.Host, "vlayer2", nil); err != nil {
			logging.GetLogger().Error(err)
		}
	}
	return false
}

// enrichHostDev adds the hostdev information to the coresponding virtual function
// of an sr-iov interface
func enrichHostDev(hostdev *HostDev, g *graph.Graph, node *graph.Node) {
	tr := g.StartMetadataTransaction(node)
	tr.AddMetadata("Libvirt.Domain", hostdev.Host.Metadata["Name"])
	address := hostdev.Address
	formatted := formatPciAddress(address)
	tr.AddMetadata("Libvirt.BusType", address.Type)
	tr.AddMetadata("Libvirt.BusInfo", formatted)
	if hostdev.Alias != nil {
		tr.AddMetadata("Libvirt.Alias", hostdev.Alias.Name)
	}
	if err := tr.Commit(); err != nil {
		logging.GetLogger().Errorf("Metadata transaction failed: %s", err)
	}
	if !topology.HaveLink(g, node, hostdev.Host, "vlayer2") {
		if _, err := topology.AddLink(g, node, hostdev.Host, "vlayer2", nil); err != nil {
			logging.GetLogger().Error(err)
		}
	}
}

// getDomain access the graph node representing a libvirt domain
func (probe *Probe) getDomain(d *libvirtgo.Domain) *graph.Node {
	domainName, err := d.GetName()
	if err != nil {
		logging.GetLogger().Error(err)
		return nil
	}
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
	domainName, err := d.GetName()
	if err != nil {
		logging.GetLogger().Error(err)
		return nil
	}
	metadata := graph.Metadata{
		"Name": domainName,
		"Type": "libvirt",
	}

	domainNode := g.LookupFirstNode(metadata)
	if domainNode == nil {
		domainNode, err = g.NewNode(graph.GenID(), metadata)
		if err != nil {
			logging.GetLogger().Error(err)
			return nil
		}

		if _, err = topology.AddOwnershipLink(g, probe.root, domainNode, nil); err != nil {
			logging.GetLogger().Error(err)
			return nil
		}
	}

	state, _, err := d.GetState()
	if err != nil {
		logging.GetLogger().Errorf("Cannot update domain state for %s", domainName)
	} else {
		tr := g.StartMetadataTransaction(domainNode)
		tr.AddMetadata("State", DomainStateMap[state])
		if err = tr.Commit(); err != nil {
			logging.GetLogger().Errorf("Metadata transaction failed: %s", err)
		}
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
			interfaces, hostdevs := probe.getDomainInterfaces(d, domainNode, "")
			probe.registerInterfaces(interfaces, hostdevs)
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
		interfaces, hostdevs := probe.getDomainInterfaces(d, domainNode, event.DevAlias)
		probe.registerInterfaces(interfaces, hostdevs) // 0 or 1 device changed.
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
		interfaces, hostdevs := probe.getDomainInterfaces(&domain, domainNode, "")
		probe.registerInterfaces(interfaces, hostdevs)
	}
}

// Stop stops the probe
func (probe *Probe) Stop() {
	probe.cancel()
	probe.tunProcessor.Stop()
	if probe.cidLifecycle != -1 {
		if err := probe.conn.DomainEventDeregister(probe.cidLifecycle); err != nil {
			logging.GetLogger().Errorf("Problem during deregistration: %s", err)
		}
		if err := probe.conn.DomainEventDeregister(probe.cidDevAdded); err != nil {
			logging.GetLogger().Errorf("Problem during deregistration: %s", err)
		}
	}
	if _, err := probe.conn.Close(); err != nil {
		logging.GetLogger().Errorf("Problem during close: %s", err)
	}
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
