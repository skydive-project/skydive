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
	"errors"
	"fmt"
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

// NeutronProbe describes a topology porbe that map neutron attribues in the graph
type NeutronProbe struct {
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
	mac    string
	portID string
}

// NeutronPortNotFound error
type NeutronPortNotFound struct {
	MAC string
}

func (e NeutronPortNotFound) Error() string {
	return "Unable to find port for MAC address: " + e.MAC
}

func retrieveportMetadata(node *graph.Node) portMetadata {
	md := portMetadata{}

	// We prefer to use the 'ExtID/attached-mac' metadata to get
	// the port, and we fallback to the 'mac' metadata.
	if attachedMAC, _ := node.GetFieldString("ExtID.attached-mac"); attachedMAC != "" {
		md.mac = attachedMAC
	} else if mac, _ := node.GetFieldString("MAC"); mac != "" {
		md.mac = mac
	}

	if ifaceID, _ := node.GetFieldString("ExtID.iface-id"); ifaceID != "" {
		md.portID = ifaceID
	}
	return md
}

func (mapper *NeutronProbe) retrievePort(portMd portMetadata) (port ports.Port, err error) {
	var opts ports.ListOpts

	logging.GetLogger().Debugf("Retrieving attributes from Neutron for MAC: %s", portMd.mac)

	/* Determine the best way to search for the Neutron port.
	 * We prefer the Neutron port UUID if we have it, but will fall back
	 * to using the MAC address otherwise. */
	if portMd.portID != "" {
		opts.ID = portMd.portID
	} else {
		opts.MACAddress = portMd.mac
	}
	pager := ports.List(mapper.client, opts)

	err = pager.EachPage(func(page pagination.Page) (bool, error) {
		portList, err := ports.ExtractPorts(page)
		if err != nil {
			return false, err
		}

		for _, p := range portList {
			if p.MACAddress == portMd.mac {
				port = p
				return false, nil
			}
		}

		return true, nil
	})

	if err != nil {
		return port, err
	}

	if len(port.NetworkID) == 0 {
		return port, NeutronPortNotFound{portMd.mac}
	}

	return port, err
}

func (mapper *NeutronProbe) retrieveAttributes(portMd portMetadata) (*attributes, error) {
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

func (mapper *NeutronProbe) nodeUpdater() {
	logging.GetLogger().Debugf("Starting Neutron updater")

	for nodeID := range mapper.nodeUpdaterChan {
		mapper.graph.RLock()
		node := mapper.graph.GetNode(nodeID)
		if node == nil {
			mapper.graph.RUnlock()
			continue
		}

		if mac, _ := node.GetFieldString("MAC"); mac == "" {
			mapper.graph.RUnlock()
			continue
		}

		portMd := retrieveportMetadata(node)
		mapper.graph.RUnlock()

		attrs, err := mapper.retrieveAttributes(portMd)
		if err != nil {
			if nerr, ok := err.(NeutronPortNotFound); ok {
				logging.GetLogger().Debugf("Setting in cache not found MAC %s", nerr.MAC)
			} else {
				logging.GetLogger().Errorf("Failed to retrieve attributes for port %s/%s : %v",
					portMd.portID, portMd.mac, err)
			}
		} else {
			mapper.updateNode(node, attrs)
		}
	}
	logging.GetLogger().Debugf("Stopping Neutron updater")
}

func (mapper *NeutronProbe) updateNode(node *graph.Node, attrs *attributes) {
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
				path := mapper.graph.LookupShortestPath(node, graph.Metadata{"Name": tap}, topology.Layer2Metadata)
				mapper.graph.RUnlock()

				if len(path) == 0 {
					qbr := strings.Replace(name, "qvo", "qbr", 1)
					mapper.graph.RLock()
					path = mapper.graph.LookupShortestPath(node, graph.Metadata{"Name": qbr}, topology.Layer2Metadata)
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
func (mapper *NeutronProbe) enhanceNode(node *graph.Node) {
	name, _ := node.GetFieldString("Name")
	if name == "" {
		return
	}

	if mapper.nsRegexp.MatchString(name) {
		mapper.graph.AddMetadata(node, "Manager", "neutron")
		return
	}

	if !mapper.intfRegexp.MatchString(name) {
		return
	}

	if mac, _ := node.GetFieldString("MAC"); mac == "" {
		return
	}

	prevPortMd, f := mapper.portMetadata[node.ID]
	currPortMd := retrieveportMetadata(node)

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
func (mapper *NeutronProbe) OnNodeUpdated(n *graph.Node) {
	mapper.enhanceNode(n)
}

// OnNodeAdded event
func (mapper *NeutronProbe) OnNodeAdded(n *graph.Node) {
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
func (mapper *NeutronProbe) OnNodeDeleted(n *graph.Node) {
	delete(mapper.portMetadata, n.ID)
}

// Start the probe
func (mapper *NeutronProbe) Start() {
	go func() {
		for mapper.client == nil {
			provider, err := openstack.AuthenticatedClient(mapper.opts)
			if err != nil {
				logging.GetLogger().Errorf("keystone authentication error: %s", err)
				time.Sleep(time.Second)
				continue
			}

			client, err := openstack.NewNetworkV2(provider, gophercloud.EndpointOpts{
				Name:         "neutron",
				Region:       mapper.regionName,
				Availability: mapper.availability,
			})
			if err != nil {
				logging.GetLogger().Errorf("keystone authentication error: %s", err)
				time.Sleep(time.Second)
				continue
			}
			mapper.client = client
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
func (mapper *NeutronProbe) Stop() {
	mapper.graph.RemoveEventListener(mapper)
	close(mapper.nodeUpdaterChan)
}

// NewNeutronProbe creates a neutron probe that will enhance the graph
func NewNeutronProbe(g *graph.Graph, authURL, username, password, tenantName, regionName, domainName string, availability gophercloud.Availability) (*NeutronProbe, error) {
	// only looking for interfaces matching the following regex as nova, neutron interfaces match this pattern
	intfRegexp := regexp.MustCompile(`(tap|qr-|qg-|qvo)[a-fA-F0-9]{8}-[a-fA-F0-9]{2}`)
	nsRegexp := regexp.MustCompile(`(qrouter|qdhcp)-[a-fA-F0-9]{8}`)

	opts := gophercloud.AuthOptions{
		IdentityEndpoint: authURL,
		Username:         username,
		Password:         password,
		TenantName:       tenantName,
		DomainName:       domainName,
		AllowReauth:      true,
	}

	mapper := &NeutronProbe{
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

// NewNeutronProbeFromConfig creates a new neutron probe based on configuration
func NewNeutronProbeFromConfig(g *graph.Graph) (*NeutronProbe, error) {
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
		return NewNeutronProbe(g, authURL, username, password, tenantName, regionName, domainName, a)
	}

	return nil, fmt.Errorf("Endpoint type '%s' is not valid (must be 'public', 'admin' or 'internal')", endpointType)
}
