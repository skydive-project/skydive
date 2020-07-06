//go:generate go run github.com/skydive-project/skydive/graffiti/gendecoder -package github.com/skydive-project/skydive/topology/probes/neutron
//go:generate go run github.com/mailru/easyjson/easyjson $GOFILE

/*
 * Copyright (C) 2015 Red Hat, Inc.
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

package neutron

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/avast/retry-go"
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/provider"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/ports"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/subnets"
	"github.com/gophercloud/gophercloud/pagination"

	"github.com/skydive-project/skydive/graffiti/getter"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology"
	tp "github.com/skydive-project/skydive/topology/probes"
)

// Probe describes a topology probe that maps neutron attributes in the graph
type Probe struct {
	graph.DefaultGraphListener
	Ctx             tp.Context
	client          *gophercloud.ServiceClient
	portMetadata    map[graph.Identifier]portMetadata
	nodeUpdaterChan chan graph.Identifier
	intfRegexp      *regexp.Regexp
	nsRegexp        *regexp.Regexp
	opts            gophercloud.AuthOptions
	regionName      string
	availability    gophercloud.Availability
}

// Metadata describes a Neutron port
// easyjson:json
// gendecoder
type Metadata struct {
	PortID      string   `json:",omitempty"`
	TenantID    string   `json:",omitempty"`
	NetworkID   string   `json:",omitempty"`
	NetworkName string   `json:",omitempty"`
	IPV4        []string `json:",omitempty"`
	IPV6        []string `json:",omitempty"`
	VNI         int64    `json:",omitempty"`
}

// MetadataDecoder implements a json message raw decoder
func MetadataDecoder(raw json.RawMessage) (getter.Getter, error) {
	var m Metadata
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("unable to unmarshal Neutron metadata %s: %s", string(raw), err)
	}

	return &m, nil
}

// portMetadata neutron metadata
type portMetadata struct {
	name   string
	mac    string
	portID string
}

var emptyPortMetadata = portMetadata{}

func (p *portMetadata) String() string {
	return fmt.Sprintf("Name: `%s`, ID: `%s`, MAC: `%s`", p.name, p.portID, p.mac)
}

func (p *Probe) retrievePortMetadata(name string, node *graph.Node) portMetadata {
	// always prefer IDs
	if ifaceID, _ := node.GetFieldString("ExtID.iface-id"); ifaceID != "" {
		return portMetadata{name: name, portID: ifaceID}
	}

	// then attached-mac
	if attachedMAC, _ := node.GetFieldString("ExtID.attached-mac"); attachedMAC != "" {
		return portMetadata{name: name, mac: attachedMAC}
	}

	// finally MAC if matching allowed interface name
	if name != "" && p.intfRegexp.MatchString(name) {
		if mac, _ := node.GetFieldString("MAC"); mac != "" {
			return portMetadata{name: name, mac: mac}
		}
	}

	return emptyPortMetadata
}

func (p *Probe) retrievePort(portMd portMetadata) (port ports.Port, err error) {
	var opts ports.ListOpts

	/* Determine the best way to search for the Neutron port.
	 * We prefer the Neutron port UUID if we have it, but will fall back
	 * to using the MAC address otherwise. */
	if portMd.portID != "" {
		opts.ID = portMd.portID
	} else {
		opts.MACAddress = portMd.mac
	}

	p.Ctx.Logger.Debugf("Retrieving attributes from Neutron port with options: %+v", opts)

	pager := ports.List(p.client, opts)

	err = pager.EachPage(func(page pagination.Page) (bool, error) {
		portList, err := ports.ExtractPorts(page)
		if err != nil {
			return false, err
		}

		if len(portList) > 0 {
			port = portList[0]
			return false, nil
		}

		return true, nil
	})

	if err != nil {
		return port, err
	}

	if len(port.NetworkID) == 0 {
		return port, fmt.Errorf("Unable to find port with options: %+v", opts)
	}

	return port, nil
}

func (p *Probe) retrieveAttributes(portMd portMetadata) (*Metadata, error) {
	port, err := p.retrievePort(portMd)
	if err != nil {
		return nil, err
	}

	type netWithProvider struct {
		networks.Network
		provider.NetworkProviderExt
	}

	var network netWithProvider
	result := networks.Get(p.client, port.NetworkID)
	err = result.ExtractInto(&network)

	if err != nil {
		return nil, err
	}

	var IPV4, IPV6 []string
	for _, element := range port.FixedIPs {
		subnet, err := subnets.Get(p.client, element.SubnetID).Extract()
		if err != nil {
			return nil, err
		}
		ip := element.IPAddress + "/" + strings.Split(subnet.CIDR, "/")[1]
		if subnet.IPVersion == 4 {
			IPV4 = append(IPV4, ip)
		} else {
			IPV6 = append(IPV6, ip)
		}
	}

	VNI, _ := strconv.Atoi(network.SegmentationID)

	a := &Metadata{
		PortID:      port.ID,
		NetworkID:   port.NetworkID,
		NetworkName: network.Name,
		TenantID:    port.TenantID,
		IPV4:        IPV4,
		IPV6:        IPV6,
		VNI:         int64(VNI),
	}

	return a, nil
}

func (p *Probe) nodeUpdater() {
	p.Ctx.Logger.Debug("Starting Neutron updater")

	for nodeID := range p.nodeUpdaterChan {
		p.Ctx.Graph.RLock()
		node := p.Ctx.Graph.GetNode(nodeID)
		if node == nil {
			p.Ctx.Graph.RUnlock()
			continue
		}

		name, _ := node.GetFieldString("Name")
		if name == "" {
			p.Ctx.Graph.RUnlock()
			return
		}

		portMd := p.retrievePortMetadata(name, node)
		p.Ctx.Graph.RUnlock()

		if portMd == emptyPortMetadata {
			continue
		}

		attrs, err := p.retrieveAttributes(portMd)
		if err != nil {
			p.Ctx.Logger.Errorf("Failed to retrieve attributes for port %s: %v", portMd.String(), err)
		} else {
			p.updateNode(node, attrs)
		}
	}
	p.Ctx.Logger.Debug("Stopping Neutron updater")
}

func (p *Probe) updateNode(node *graph.Node, attrs *Metadata) {
	p.Ctx.Graph.Lock()
	defer p.Ctx.Graph.Unlock()

	name, _ := node.GetFieldString("Name")
	if name == "" {
		return
	}

	tr := p.Ctx.Graph.StartMetadataTransaction(node)
	tr.AddMetadata("Manager", "neutron")
	tr.AddMetadata("Neutron", attrs)
	if strings.HasPrefix(name, "tap") {
		if attachedMac, _ := node.GetFieldString("ExtID.attached-mac"); attachedMac != "" {
			tr.AddMetadata("PeerIntfMAC", attachedMac)
		}
	}
	err := tr.Commit()
	if err != nil {
		p.Ctx.Logger.Error("Commit failed %+v : %v", tr, err)
		return
	}
	if !strings.HasPrefix(name, "qvo") {
		return
	}

	// tap to qvo path
	tap := strings.Replace(name, "qvo", "tap", 1)

	if uuid, _ := node.GetFieldString("ExtID.vm-uuid"); uuid != "" {
		if attachedMac, _ := node.GetFieldString("ExtID.attached-mac"); attachedMac != "" {
			retryFnc := func() error {
				p.Ctx.Graph.RLock()
				path := p.Ctx.Graph.LookupShortestPath(node, graph.Metadata{"Name": tap}, topology.Layer2Metadata())
				p.Ctx.Graph.RUnlock()

				if len(path) == 0 {
					qbr := strings.Replace(name, "qvo", "qbr", 1)
					p.Ctx.Graph.RLock()
					path = p.Ctx.Graph.LookupShortestPath(node, graph.Metadata{"Name": qbr}, topology.Layer2Metadata())
					p.Ctx.Graph.RUnlock()

					if len(path) == 0 {
						return errors.New("Path not found")
					}
				}

				p.Ctx.Graph.Lock()
				defer p.Ctx.Graph.Unlock()

				for i, n := range path {
					if p.Ctx.Graph.GetNode(n.ID) == nil {
						continue
					}

					tr := p.Ctx.Graph.StartMetadataTransaction(n)
					tr.AddMetadata("ExtID.vm-uuid", uuid)
					tr.AddMetadata("ExtID.attached-mac", attachedMac)

					// add vm peering info, going to be used by peering probe
					if i == len(path)-1 {
						tr.AddMetadata("PeerIntfMAC", attachedMac)
					}
					err := tr.Commit()
					if err != nil {
						p.Ctx.Logger.Error("Commit failed %+v : %v", tr, err)
						continue
					}
				}

				return nil
			}
			go retry.Do(retryFnc, retry.Attempts(30), retry.Delay(2*time.Second), retry.DelayType(retry.FixedDelay))
		}
	}
}

// enhanceNode enhance the graph node with neutron metadata (Name, MAC, Manager ...)
func (p *Probe) enhanceNode(node *graph.Node) {
	name, _ := node.GetFieldString("Name")
	if name == "" {
		return
	}

	if p.nsRegexp.MatchString(name) {
		if err := p.Ctx.Graph.AddMetadata(node, "Manager", "neutron"); err != nil {
			p.Ctx.Logger.Error(err)
		}
		return
	}

	currPortMd := p.retrievePortMetadata(name, node)
	if currPortMd == emptyPortMetadata {
		return
	}

	prevPortMd, f := p.portMetadata[node.ID]

	// If port metadata have not changed, we return
	if f && (prevPortMd == currPortMd) {
		return
	}
	// We only try to get Neutron metadata one time per port
	// metadata values
	p.portMetadata[node.ID] = currPortMd

	p.nodeUpdaterChan <- node.ID
}

// OnNodeUpdated event
func (p *Probe) OnNodeUpdated(n *graph.Node, ops []graph.PartiallyUpdatedOp) {
	p.enhanceNode(n)
}

// OnNodeAdded event
func (p *Probe) OnNodeAdded(n *graph.Node) {
	name, _ := n.GetFieldString("Name")
	attachedMAC, _ := n.GetFieldString("ExtID.attached-mac")
	if attachedMAC == "" && strings.HasPrefix(name, "tap") {
		qvo := strings.Replace(name, "tap", "qvo", 1)
		qvoNode := p.Ctx.Graph.LookupFirstNode(graph.Metadata{"Name": qvo, "Type": "veth"})
		if qvoNode != nil {
			tr := p.Ctx.Graph.StartMetadataTransaction(n)
			if attachedMAC, _ = qvoNode.GetFieldString("ExtID.attached-mac"); attachedMAC != "" {
				tr.AddMetadata("ExtID.attached-mac", attachedMAC)
			}

			if uuid, _ := qvoNode.GetFieldString("ExtID.vm-uuid"); uuid != "" {
				tr.AddMetadata("ExtID.vm-uuid", uuid)
			}
			err := tr.Commit()
			if err != nil {
				p.Ctx.Logger.Error("Commit failed %+v : %v", tr, err)
				return
			}
		}
	}

	p.enhanceNode(n)
}

// OnNodeDeleted event
func (p *Probe) OnNodeDeleted(n *graph.Node) {
	delete(p.portMetadata, n.ID)
}

// Start the probe
func (p *Probe) Start() error {
	p.Ctx.Graph.AddEventListener(p)

	go func() {
		for p.client == nil {
			client, err := openstack.NewClient(p.opts.IdentityEndpoint)
			if err != nil {
				p.Ctx.Logger.Errorf("failed to create neutron client: %s", err)
				time.Sleep(time.Second)
				continue
			}

			sslInsecure := p.Ctx.Config.GetBool("agent.topology.neutron.ssl_insecure")
			if sslInsecure {
				p.Ctx.Logger.Warningf("Skipping SSL certificates verification")
			}

			client.HTTPClient = http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: sslInsecure,
					},
				},
			}

			if err = openstack.Authenticate(client, p.opts); err != nil {
				p.Ctx.Logger.Errorf("keystone authentication error: %s", err)
				time.Sleep(time.Second)
				continue
			}

			networkClient, err := openstack.NewNetworkV2(client, gophercloud.EndpointOpts{
				Name:         "neutron",
				Region:       p.regionName,
				Availability: p.availability,
			})
			if err != nil {
				p.Ctx.Logger.Errorf("keystone authentication error: %s", err)
				time.Sleep(time.Second)
				continue
			}
			p.client = networkClient
		}
		p.Ctx.Graph.RLock()
		for _, n := range p.Ctx.Graph.GetNodes(nil) {
			p.enhanceNode(n)
		}
		p.Ctx.Graph.RUnlock()

		p.nodeUpdater()
	}()

	return nil
}

// Stop the probe
func (p *Probe) Stop() {
	p.Ctx.Graph.RemoveEventListener(p)
	close(p.nodeUpdaterChan)
}

// NewProbe returns a new Neutron topology probe
func NewProbe(ctx tp.Context, bundle *probe.Bundle) (probe.Handler, error) {
	authURL := ctx.Config.GetString("agent.topology.neutron.auth_url")
	domainName := ctx.Config.GetString("agent.topology.neutron.domain_name")
	endpointType := ctx.Config.GetString("agent.topology.neutron.endpoint_type")
	password := ctx.Config.GetString("agent.topology.neutron.password")
	regionName := ctx.Config.GetString("agent.topology.neutron.region_name")
	tenantName := ctx.Config.GetString("agent.topology.neutron.tenant_name")
	username := ctx.Config.GetString("agent.topology.neutron.username")

	endpointTypes := map[string]gophercloud.Availability{
		"public":   gophercloud.AvailabilityPublic,
		"admin":    gophercloud.AvailabilityAdmin,
		"internal": gophercloud.AvailabilityInternal,
	}

	availability, ok := endpointTypes[endpointType]
	if !ok {
		return nil, fmt.Errorf("Endpoint type '%s' is not valid (must be 'public', 'admin' or 'internal')", endpointType)
	}

	return &Probe{
		Ctx: ctx,
		// only looking for interfaces matching the following regex as nova, neutron interfaces match this pattern
		intfRegexp: regexp.MustCompile(`((tap|qr-|qg-|qvo)[a-fA-F0-9\-]+)|(vnet[0-9]+)`),
		nsRegexp:   regexp.MustCompile(`(qrouter|qdhcp)-[a-fA-F0-9\-]+`),
		opts: gophercloud.AuthOptions{
			IdentityEndpoint: authURL,
			Username:         username,
			Password:         password,
			TenantName:       tenantName,
			DomainName:       domainName,
			AllowReauth:      true,
		},
		regionName:      regionName,
		availability:    availability,
		nodeUpdaterChan: make(chan graph.Identifier, 500),
		portMetadata:    make(map[graph.Identifier]portMetadata),
	}, nil
}

// Register registers graph metadata decoders
func Register() {
	graph.NodeMetadataDecoders["Neutron"] = MetadataDecoder
}
