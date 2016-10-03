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
	"strconv"
	"time"

	"github.com/pmylund/go-cache"
	"github.com/rackspace/gophercloud"
	"github.com/rackspace/gophercloud/openstack"
	"github.com/rackspace/gophercloud/openstack/networking/v2/extensions/provider"
	"github.com/rackspace/gophercloud/openstack/networking/v2/networks"
	"github.com/rackspace/gophercloud/openstack/networking/v2/ports"
	"github.com/rackspace/gophercloud/pagination"

	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"
)

type NeutronMapper struct {
	graph.DefaultGraphListener
	graph           *graph.Graph
	client          *gophercloud.ServiceClient
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

func (e NeutronPortNotFound) Error() string {
	return "Unable to find port for MAC address: " + e.MAC
}

func (mapper *NeutronMapper) retrievePort(metadata graph.Metadata) (port ports.Port, err error) {
	var opts ports.ListOpts
	var mac string

	/* If we have a MAC address for a device attached to the interface, that is the one that
	 * will be associated with the Neutron port. */
	if attached_mac, ok := metadata["ExtID/attached-mac"]; ok {
		mac = attached_mac.(string)
	} else {
		mac = metadata["MAC"].(string)
	}

	logging.GetLogger().Debugf("Retrieving attributes from Neutron for MAC: %s", mac)

	/* Determine the best way to search for the Neutron port.
	 * We prefer the Neutron port UUID if we have it, but will fall back
	 * to using the MAC address otherwise. */
	if portid, ok := metadata["ExtID/iface-id"]; ok {
		opts.ID = portid.(string)
	} else {
		opts.MACAddress = mac
	}
	pager := ports.List(mapper.client, opts)
	err = pager.EachPage(func(page pagination.Page) (bool, error) {
		portList, err := ports.ExtractPorts(page)
		if err != nil {
			return false, err
		}

		for _, p := range portList {
			if p.MACAddress == mac {
				port = p
				return true, nil
			}
		}

		return true, nil
	})

	if len(port.NetworkID) == 0 {
		return port, NeutronPortNotFound{mac}
	}

	return port, err
}

func (mapper *NeutronMapper) retrieveAttributes(metadata graph.Metadata) (*Attributes, error) {
	port, err := mapper.retrievePort(metadata)
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

		attrs, err := mapper.retrieveAttributes(node.Metadata())
		if err != nil {
			if nerr, ok := err.(NeutronPortNotFound); ok {
				logging.GetLogger().Debugf("Setting in cache not found MAC " + nerr.MAC)
				mapper.cache.Set(nerr.MAC, nil, cache.DefaultExpiration)
			}
			continue
		}

		mapper.updateNode(node, attrs)
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

	mapper.cache.Set(node.Metadata()["MAC"].(string), attrs, cache.DefaultExpiration)
}

func (mapper *NeutronMapper) EnhanceNode(node *graph.Node) {
	mac, ok := node.Metadata()["MAC"]
	if !ok {
		return
	}

	_, f := mapper.cache.Get(mac.(string))
	if f {
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

func NewNeutronMapper(g *graph.Graph, authURL string, username string, password string, tenantName string, regionName string) (*NeutronMapper, error) {
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

	/* TODO(safchain) add config param for the Availability */
	client, err := openstack.NewNetworkV2(provider, gophercloud.EndpointOpts{
		Name:         "neutron",
		Region:       regionName,
		Availability: gophercloud.AvailabilityPublic,
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

	return NewNeutronMapper(g, authURL, username, password, tenantName, regionName)
}
