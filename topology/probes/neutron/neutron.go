/*
 * Copyright (C) 2015 Red Hat, Inc.
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

package neutron

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/provider"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/ports"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/subnets"
	"github.com/gophercloud/gophercloud/pagination"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/graph"
)

// Probe describes a topology probe that maps neutron attributes in the graph
type Probe struct {
	graph.DefaultGraphListener
	graph           *graph.Graph
	client          *gophercloud.ServiceClient
	portMetadata    map[graph.Identifier]portMetadata
	nodeUpdaterChan chan graph.Identifier
	intfRegexp      *regexp.Regexp
	nsRegexp        *regexp.Regexp
	opts            gophercloud.AuthOptions
	regionName      string
	availability    gophercloud.Availability
}

// attributes neutron attributes
type attributes struct {
	PortID      string
	NetworkID   string
	NetworkName string
	TenantID    string
	IPV4        []string
	IPV6        []string
	VNI         string
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

func (mapper *Probe) retrievePortMetadata(name string, node *graph.Node) portMetadata {
	// always prefer IDs
	if ifaceID, _ := node.GetFieldString("ExtID.iface-id"); ifaceID != "" {
		return portMetadata{name: name, portID: ifaceID}
	}

	// then attached-mac
	if attachedMAC, _ := node.GetFieldString("ExtID.attached-mac"); attachedMAC != "" {
		return portMetadata{name: name, mac: attachedMAC}
	}

	// finally MAC if matching allowed interface name
	if name != "" && mapper.intfRegexp.MatchString(name) {
		if mac, _ := node.GetFieldString("MAC"); mac != "" {
			return portMetadata{name: name, mac: mac}
		}
	}

	return emptyPortMetadata
}

func (mapper *Probe) retrievePort(portMd portMetadata) (port ports.Port, err error) {
	var opts ports.ListOpts

	/* Determine the best way to search for the Neutron port.
	 * We prefer the Neutron port UUID if we have it, but will fall back
	 * to using the MAC address otherwise. */
	if portMd.portID != "" {
		opts.ID = portMd.portID
	} else {
		opts.MACAddress = portMd.mac
	}

	logging.GetLogger().Debugf("Retrieving attributes from Neutron port with options: %+v", opts)

	pager := ports.List(mapper.client, opts)

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

func (mapper *Probe) retrieveAttributes(portMd portMetadata) (*attributes, error) {
	port, err := mapper.retrievePort(portMd)
	if err != nil {
		return nil, err
	}

	type netWithProvider struct {
		networks.Network
		provider.NetworkProviderExt
	}

	var network netWithProvider
	result := networks.Get(mapper.client, port.NetworkID)
	err = result.ExtractInto(&network)

	if err != nil {
		return nil, err
	}

	var IPV4, IPV6 []string
	for _, element := range port.FixedIPs {
		subnet, err := subnets.Get(mapper.client, element.SubnetID).Extract()
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

	a := &attributes{
		PortID:      port.ID,
		NetworkID:   port.NetworkID,
		NetworkName: network.Name,
		TenantID:    port.TenantID,
		IPV4:        IPV4,
		IPV6:        IPV6,
		VNI:         network.SegmentationID,
	}

	return a, nil
}

func (mapper *Probe) nodeUpdater() {
	logging.GetLogger().Debug("Starting Neutron updater")

	for nodeID := range mapper.nodeUpdaterChan {
		mapper.graph.RLock()
		node := mapper.graph.GetNode(nodeID)
		if node == nil {
			mapper.graph.RUnlock()
			continue
		}

		name, _ := node.GetFieldString("Name")
		if name == "" {
			mapper.graph.RUnlock()
			return
		}

		portMd := mapper.retrievePortMetadata(name, node)
		mapper.graph.RUnlock()

		if portMd == emptyPortMetadata {
			continue
		}

		attrs, err := mapper.retrieveAttributes(portMd)
		if err != nil {
			logging.GetLogger().Errorf("Failed to retrieve attributes for port %s: %v", portMd.String(), err)
		} else {
			mapper.updateNode(node, attrs)
		}
	}
	logging.GetLogger().Debug("Stopping Neutron updater")
}

func (mapper *Probe) updateNode(node *graph.Node, attrs *attributes) {
	mapper.graph.Lock()
	defer mapper.graph.Unlock()

	metadata := make(map[string]interface{})

	if attrs.PortID != "" {
		metadata["Neutron.PortID"] = attrs.PortID
	}

	if attrs.TenantID != "" {
		metadata["Neutron.TenantID"] = attrs.TenantID
	}

	if attrs.NetworkID != "" {
		metadata["Neutron.NetworkID"] = attrs.NetworkID
	}

	if attrs.NetworkName != "" {
		metadata["Neutron.NetworkName"] = attrs.NetworkName
	}

	if len(attrs.IPV4) != 0 {
		metadata["Neutron.IPV4"] = attrs.IPV4
	}

	if len(attrs.IPV6) != 0 {
		metadata["Neutron.IPV6"] = attrs.IPV6
	}

	if segID, err := strconv.Atoi(attrs.VNI); err != nil && segID > 0 {
		metadata["Neutron.VNI"] = int64(segID)
	}

	tr := mapper.graph.StartMetadataTransaction(node)

	metadata["Manager"] = "neutron"
	for k, v := range metadata {
		tr.AddMetadata(k, v)
	}

	tr.Commit()

	name, _ := node.GetFieldString("Name")
	if name == "" {
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
				mapper.graph.RLock()
				path := mapper.graph.LookupShortestPath(node, graph.Metadata{"Name": tap}, topology.Layer2Metadata())
				mapper.graph.RUnlock()

				if len(path) == 0 {
					qbr := strings.Replace(name, "qvo", "qbr", 1)
					mapper.graph.RLock()
					path = mapper.graph.LookupShortestPath(node, graph.Metadata{"Name": qbr}, topology.Layer2Metadata())
					mapper.graph.RUnlock()

					if len(path) == 0 {
						return errors.New("Path not found")
					}
				}

				mapper.graph.Lock()
				defer mapper.graph.Unlock()

				for i, n := range path {
					if mapper.graph.GetNode(n.ID) == nil {
						continue
					}

					tr := mapper.graph.StartMetadataTransaction(n)
					tr.AddMetadata("ExtID.vm-uuid", uuid)
					tr.AddMetadata("ExtID.attached-mac", attachedMac)
					for k, v := range metadata {
						tr.AddMetadata(k, v)
					}

					// add vm peering info, going to be used by peering probe
					if i == len(path)-1 {
						tr.AddMetadata("PeerIntfMAC", attachedMac)
					}
					tr.Commit()
				}

				return nil
			}
			go common.Retry(retryFnc, 30, 2*time.Second)
		}
	}
}

// enhanceNode enhance the graph node with neutron metadata (Name, MAC, Manager ...)
func (mapper *Probe) enhanceNode(node *graph.Node) {
	name, _ := node.GetFieldString("Name")
	if name == "" {
		return
	}

	if mapper.nsRegexp.MatchString(name) {
		mapper.graph.AddMetadata(node, "Manager", "neutron")
		return
	}

	currPortMd := mapper.retrievePortMetadata(name, node)
	if currPortMd == emptyPortMetadata {
		return
	}

	prevPortMd, f := mapper.portMetadata[node.ID]

	// If port metadata have not changed, we return
	if f && (prevPortMd == currPortMd) {
		return
	}
	// We only try to get Neutron metadata one time per port
	// metadata values
	mapper.portMetadata[node.ID] = currPortMd

	mapper.nodeUpdaterChan <- node.ID
}

// OnNodeUpdated event
func (mapper *Probe) OnNodeUpdated(n *graph.Node) {
	mapper.enhanceNode(n)
}

// OnNodeAdded event
func (mapper *Probe) OnNodeAdded(n *graph.Node) {
	name, _ := n.GetFieldString("Name")
	attachedMAC, _ := n.GetFieldString("ExtID.attached-mac")
	if attachedMAC == "" && strings.HasPrefix(name, "tap") {
		qvo := strings.Replace(name, "tap", "qvo", 1)
		qvoNode := mapper.graph.LookupFirstNode(graph.Metadata{"Name": qvo, "Type": "veth"})
		if qvoNode != nil {
			tr := mapper.graph.StartMetadataTransaction(n)
			if attachedMAC, _ = qvoNode.GetFieldString("ExtID.attached-mac"); attachedMAC != "" {
				tr.AddMetadata("ExtID.attached-mac", attachedMAC)
			}

			if uuid, _ := qvoNode.GetFieldString("ExtID.vm-uuid"); uuid != "" {
				tr.AddMetadata("ExtID.vm-uuid", uuid)
			}
			tr.Commit()
		}
	}

	mapper.enhanceNode(n)
}

// OnNodeDeleted event
func (mapper *Probe) OnNodeDeleted(n *graph.Node) {
	delete(mapper.portMetadata, n.ID)
}

// Start the probe
func (mapper *Probe) Start() {
	go func() {
		for mapper.client == nil {
			client, err := openstack.NewClient(mapper.opts.IdentityEndpoint)
			if err != nil {
				logging.GetLogger().Errorf("failed to create neutron client: %s", err)
				time.Sleep(time.Second)
				continue
			}

			sslInsecure := config.GetBool("agent.topology.neutron.ssl_insecure")
			if sslInsecure {
				logging.GetLogger().Warningf("Skipping SSL certificates verification")
			}

			client.HTTPClient = http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: sslInsecure,
					},
				},
			}

			if err = openstack.Authenticate(client, mapper.opts); err != nil {
				logging.GetLogger().Errorf("keystone authentication error: %s", err)
				time.Sleep(time.Second)
				continue
			}

			networkClient, err := openstack.NewNetworkV2(client, gophercloud.EndpointOpts{
				Name:         "neutron",
				Region:       mapper.regionName,
				Availability: mapper.availability,
			})
			if err != nil {
				logging.GetLogger().Errorf("keystone authentication error: %s", err)
				time.Sleep(time.Second)
				continue
			}
			mapper.client = networkClient
		}
		mapper.graph.RLock()
		for _, n := range mapper.graph.GetNodes(nil) {
			mapper.enhanceNode(n)
		}
		mapper.graph.RUnlock()

		mapper.nodeUpdater()
	}()
}

// Stop the probe
func (mapper *Probe) Stop() {
	mapper.graph.RemoveEventListener(mapper)
	close(mapper.nodeUpdaterChan)
}

// NewProbe creates a neutron probe that will enhance the graph
func NewProbe(g *graph.Graph, authURL, username, password, tenantName, regionName, domainName string, availability gophercloud.Availability) (*Probe, error) {
	// only looking for interfaces matching the following regex as nova, neutron interfaces match this pattern

	intfRegexp := regexp.MustCompile(`((tap|qr-|qg-|qvo)[a-fA-F0-9\-]+)|(vnet[0-9]+)`)
	nsRegexp := regexp.MustCompile(`(qrouter|qdhcp)-[a-fA-F0-9\-]+`)

	opts := gophercloud.AuthOptions{
		IdentityEndpoint: authURL,
		Username:         username,
		Password:         password,
		TenantName:       tenantName,
		DomainName:       domainName,
		AllowReauth:      true,
	}

	mapper := &Probe{
		graph:           g,
		intfRegexp:      intfRegexp,
		nsRegexp:        nsRegexp,
		regionName:      regionName,
		availability:    availability,
		opts:            opts,
		nodeUpdaterChan: make(chan graph.Identifier, 500),
		portMetadata:    make(map[graph.Identifier]portMetadata),
	}

	g.AddEventListener(mapper)

	return mapper, nil
}

// NewProbeFromConfig creates a new neutron probe based on configuration
func NewProbeFromConfig(g *graph.Graph) (*Probe, error) {
	authURL := config.GetString("agent.topology.neutron.auth_url")
	domainName := config.GetString("agent.topology.neutron.domain_name")
	endpointType := config.GetString("agent.topology.neutron.endpoint_type")
	password := config.GetString("agent.topology.neutron.password")
	regionName := config.GetString("agent.topology.neutron.region_name")
	tenantName := config.GetString("agent.topology.neutron.tenant_name")
	username := config.GetString("agent.topology.neutron.username")

	endpointTypes := map[string]gophercloud.Availability{
		"public":   gophercloud.AvailabilityPublic,
		"admin":    gophercloud.AvailabilityAdmin,
		"internal": gophercloud.AvailabilityInternal,
	}

	if a, ok := endpointTypes[endpointType]; ok {
		return NewProbe(g, authURL, username, password, tenantName, regionName, domainName, a)
	}

	return nil, fmt.Errorf("Endpoint type '%s' is not valid (must be 'public', 'admin' or 'internal')", endpointType)
}
