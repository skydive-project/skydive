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

package probes

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/provider"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/ports"
	"github.com/gophercloud/gophercloud/pagination"
	"github.com/pmylund/go-cache"

	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"
)

type NeutronMapper struct {
	graph.DefaultGraphListener
	graph  *graph.Graph
	client *gophercloud.ServiceClient
	// The cache associates some metadatas to a MAC and is used to
	// detect any updates on these metadatas.
	cache           *cache.Cache
	nodeUpdaterChan chan graph.Identifier
}

type Attributes struct {
	PortID      string
	NetworkID   string
	NetworkName string
	TenantID    string
	VNI         string
}

type NeutronPortNotFound struct {
	MAC string
}

type PortMetadata struct {
	mac    string
	portID string
}

func (e NeutronPortNotFound) Error() string {
	return "Unable to find port for MAC address: " + e.MAC
}

func retrievePortMetadata(metadata graph.Metadata) PortMetadata {
	md := PortMetadata{}

	// We prefer to use the 'ExtID/attached-mac' metadata to get
	// the port, and we fallback to the 'mac' metadata.
	if attached_mac, ok := metadata["ExtID/attached-mac"]; ok {
		md.mac = attached_mac.(string)
	} else if mac, ok := metadata["MAC"]; ok {
		md.mac = mac.(string)
	}

	if iface_id, ok := metadata["ExtID/iface-id"]; ok {
		md.portID = iface_id.(string)
	}
	return md
}

func (mapper *NeutronMapper) retrievePort(portMd PortMetadata) (port ports.Port, err error) {
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

func (mapper *NeutronMapper) retrieveAttributes(portMd PortMetadata) (*Attributes, error) {
	port, err := mapper.retrievePort(portMd)
	if err != nil {
		return nil, err
	}

	result := networks.Get(mapper.client, port.NetworkID)
	network, err := provider.ExtractGet(result)
	if err != nil {
		return nil, err
	}

	a := &Attributes{
		PortID:      port.ID,
		NetworkID:   port.NetworkID,
		NetworkName: network.Name,
		TenantID:    port.TenantID,
		VNI:         network.SegmentationID,
	}

	return a, nil
}

func (mapper *NeutronMapper) nodeUpdater() {
	logging.GetLogger().Debugf("Starting Neutron updater")
	for nodeID := range mapper.nodeUpdaterChan {
		node := mapper.graph.GetNode(nodeID)
		if node == nil {
			continue
		}

		if _, ok := node.Metadata()["MAC"]; !ok {
			continue
		}

		portMd := retrievePortMetadata(node.Metadata())
		attrs, err := mapper.retrieveAttributes(portMd)
		if err != nil {
			if nerr, ok := err.(NeutronPortNotFound); ok {
				logging.GetLogger().Debugf("Setting in cache not found MAC %s", nerr.MAC)
				mapper.cache.Set(nerr.MAC, PortMetadata{}, cache.DefaultExpiration)
			} else {
				logging.GetLogger().Errorf("Failed to retrieve attributes for port %s/%s : %v",
					portMd.portID, portMd.mac, err)
			}
			continue
		}
		mapper.updateNode(node, attrs)
		mapper.cache.Set(node.Metadata()["MAC"].(string), portMd, cache.DefaultExpiration)

	}
	logging.GetLogger().Debugf("Stopping Neutron updater")
}

func (mapper *NeutronMapper) updateNode(node *graph.Node, attrs *Attributes) {
	mapper.graph.Lock()
	defer mapper.graph.Unlock()

	tr := mapper.graph.StartMetadataTransaction(node)
	defer tr.Commit()

	tr.AddMetadata("Manager", "neutron")

	if attrs.PortID != "" {
		tr.AddMetadata("Neutron/PortID", attrs.PortID)
	}

	if attrs.TenantID != "" {
		tr.AddMetadata("Neutron/TenantID", attrs.TenantID)
	}

	if attrs.NetworkID != "" {
		tr.AddMetadata("Neutron/NetworkID", attrs.NetworkID)
	}

	if attrs.NetworkName != "" {
		tr.AddMetadata("Neutron/NetworkName", attrs.NetworkName)
	}

	if segID, err := strconv.Atoi(attrs.VNI); err != nil && segID > 0 {
		tr.AddMetadata("Neutron/VNI", uint64(segID))
	}
}

func (mapper *NeutronMapper) EnhanceNode(node *graph.Node) {
	mac, ok := node.Metadata()["MAC"]
	if !ok {
		return
	}

	portMd, f := mapper.cache.Get(mac.(string))
	// If port metadatas have not changed, we return
	if f && portMd == retrievePortMetadata(node.Metadata()) {
		return
	}

	mapper.nodeUpdaterChan <- node.ID
}

func (mapper *NeutronMapper) OnNodeUpdated(n *graph.Node) {
	mapper.EnhanceNode(n)
}

func (mapper *NeutronMapper) OnNodeAdded(n *graph.Node) {
	mapper.EnhanceNode(n)
}

func (mapper *NeutronMapper) Start() {
	go mapper.nodeUpdater()
}

func (mapper *NeutronMapper) Stop() {
	mapper.graph.RemoveEventListener(mapper)
	close(mapper.nodeUpdaterChan)
}

func NewNeutronMapper(g *graph.Graph, authURL string, username string, password string, tenantName string, regionName string, availability gophercloud.Availability) (*NeutronMapper, error) {
	mapper := &NeutronMapper{graph: g}

	opts := gophercloud.AuthOptions{
		IdentityEndpoint: authURL,
		Username:         username,
		Password:         password,
		TenantName:       tenantName,
		AllowReauth:      true,
	}

	provider, err := openstack.AuthenticatedClient(opts)
	if err != nil {
		return nil, err
	}

	client, err := openstack.NewNetworkV2(provider, gophercloud.EndpointOpts{
		Name:         "neutron",
		Region:       regionName,
		Availability: availability,
	})
	if err != nil {
		return nil, err
	}
	mapper.client = client

	// Create a cache with a default expiration time of 5 minutes, and which
	// purges expired items every 30 seconds
	expire := config.GetConfig().GetInt("cache.expire")
	cleanup := config.GetConfig().GetInt("cache.cleanup")
	mapper.cache = cache.New(time.Duration(expire)*time.Second, time.Duration(cleanup)*time.Second)
	mapper.nodeUpdaterChan = make(chan graph.Identifier, 500)

	g.AddEventListener(mapper)

	return mapper, nil
}

func NewNeutronMapperFromConfig(g *graph.Graph) (*NeutronMapper, error) {
	authURL := config.GetConfig().GetString("openstack.auth_url")
	username := config.GetConfig().GetString("openstack.username")
	password := config.GetConfig().GetString("openstack.password")
	tenantName := config.GetConfig().GetString("openstack.tenant_name")
	regionName := config.GetConfig().GetString("openstack.region_name")
	endpointType := config.GetConfig().GetString("openstack.endpoint_type")

	endpointTypes := map[string]gophercloud.Availability{
		"public":   gophercloud.AvailabilityPublic,
		"admin":    gophercloud.AvailabilityAdmin,
		"internal": gophercloud.AvailabilityInternal}
	if a, ok := endpointTypes[endpointType]; !ok {
		return nil, fmt.Errorf("Endpoint type '%s' is not valid (must be 'public', 'admin' or 'internal')", endpointType)
	} else {
		return NewNeutronMapper(g, authURL, username, password, tenantName, regionName, a)
	}
}
