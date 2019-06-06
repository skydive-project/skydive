// +build linux

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

	libvirt "github.com/digitalocean/go-libvirt"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/logging"
)

// Probe is the libvirt probe
type Probe struct {
	sync.Mutex
	graph        *graph.Graph          // the graph
	conn         monitor               // libvirt connection
	interfaceMap map[string]*Interface // Found interfaces not yet connected.
	uri          string                // uri of the libvirt connection
	cancel       context.CancelFunc    // cancel function
	root         *graph.Node           // root node for ownership
	tunProcessor *graph.Processor      // metadata indexer for regular interfaces
}

// monitor abstracts a libvirt monitor
type monitor interface {
	AllDomains() ([]domain, error)
	Stop()
}

// domain abstracts a libvirt domain
type domain interface {
	GetName() (string, error)
	GetState() (DomainState, int, error)
	GetXML() ([]byte, error)
}

// Address describes the XML coding of the pci address of an interface in libvirt
type Address struct {
	Type     string `xml:"type,attr,omitempty"`
	Domain   string `xml:"domain,attr"`
	Bus      string `xml:"bus,attr"`
	Slot     string `xml:"slot,attr"`
	Function string `xml:"function,attr"`
}

// Source describe the XML coding of a libvirt source
type Source struct {
	Address *Address `xml:"address"`
}

// DomainState describes the state of a domain
type DomainState = libvirt.DomainState

// DomainStateMap stringifies the state of a domain
var DomainStateMap = map[DomainState]string{
	libvirt.DomainNostate:     "UNDEFINED",
	libvirt.DomainRunning:     "UP",
	libvirt.DomainBlocked:     "BLOCKED",
	libvirt.DomainPaused:      "PAUSED",
	libvirt.DomainShutdown:    "DOWN",
	libvirt.DomainCrashed:     "CRASHED",
	libvirt.DomainPmsuspended: "PMSUSPENDED",
	libvirt.DomainShutoff:     "DOWN",
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
	domain domain, // domain to query
	domainNode *graph.Node, // Node representing the domain
	constraint string, // to restrict the search to a single interface (by alias)
) (interfaces []*Interface, hostdevs []*HostDev) {
	rawXML, err := domain.GetXML()
	if err != nil {
		logging.GetLogger().Errorf("Cannot get XMLDesc: %s", err)
		return
	}
	d := Domain{}
	if err = xml.Unmarshal(rawXML, &d); err != nil {
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

	address := itf.Address
	formatted := formatPciAddress(&address)
	metadata := Metadata{
		MAC:     itf.Mac.Address,
		Domain:  itf.Host.Metadata["Name"].(string),
		BusType: address.Type,
		BusInfo: formatted,
		Alias:   alias,
	}

	tr := g.StartMetadataTransaction(node)
	if itf.Mac != nil {
		metadata.MAC = itf.Mac.Address
		tr.AddMetadata("PeerIntfMAC", itf.Mac.Address)
	}
	tr.AddMetadata("Libvirt", metadata)
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
func (probe *Probe) getDomain(d domain) *graph.Node {
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
func (probe *Probe) createOrUpdateDomain(d domain) *graph.Node {
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
func (probe *Probe) deleteDomain(d domain) {
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
	ctx, cancel := context.WithCancel(context.Background())
	probe.cancel = cancel

	conn, err := newMonitor(ctx, probe)
	if err != nil {
		logging.GetLogger().Error(err)
		return
	}
	probe.conn = conn

	domains, err := probe.conn.AllDomains()
	if err != nil {
		logging.GetLogger().Error(err)
		return
	}

	for _, domain := range domains {
		domainNode := probe.createOrUpdateDomain(domain)
		interfaces, hostdevs := probe.getDomainInterfaces(domain, domainNode, "")
		probe.registerInterfaces(interfaces, hostdevs)
	}
}

// Stop stops the probe
func (probe *Probe) Stop() {
	if probe.conn != nil {
		probe.conn.Stop()
	}
	probe.cancel()
	probe.tunProcessor.Stop()
}

// NewProbe creates a libvirt topology probe
func NewProbe(g *graph.Graph, uri string, root *graph.Node) *Probe {
	tunProcessor := graph.NewProcessor(g, g, graph.Metadata{"Type": "tun"}, "Name")
	probe := &Probe{
		graph:        g,
		interfaceMap: make(map[string]*Interface),
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
