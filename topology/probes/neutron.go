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
	"errors"
	"strconv"
	"time"

	"github.com/rackspace/gophercloud"
	"github.com/rackspace/gophercloud/openstack"
	"github.com/rackspace/gophercloud/openstack/networking/v2/extensions/provider"
	"github.com/rackspace/gophercloud/openstack/networking/v2/networks"
	"github.com/rackspace/gophercloud/openstack/networking/v2/ports"
	"github.com/rackspace/gophercloud/pagination"

	"github.com/pmylund/go-cache"

	"github.com/redhat-cip/skydive/config"
	"github.com/redhat-cip/skydive/logging"
	"github.com/redhat-cip/skydive/topology/graph"
)

type NeutronMapper struct {
	graph            *graph.Graph
	client           *gophercloud.ServiceClient
	cache            *cache.Cache
	nodeUpdaterChan  chan graph.Identifier
}

type Attributes struct {
	TenantID string
	VNI      string
}

func (mapper *NeutronMapper) retrievePort(mac string) (port ports.Port, err error) {
	opts := ports.ListOpts{MACAddress: mac}
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
		return port, errors.New("Unable to find port for MAC address: " + mac)
	}

	return port, err
}

func (mapper *NeutronMapper) retrieveAttributes(mac string) (*Attributes, error) {
	logging.GetLogger().Debugf("Retrieving attributes from Neutron for MAC: %s", mac)

	port, err := mapper.retrievePort(mac)
	if err != nil {
		return nil, err
	}

	result := networks.Get(mapper.client, port.NetworkID)
	network, err := provider.ExtractGet(result)
	if err != nil {
		return nil, err
	}

	return &Attributes{TenantID: port.TenantID, VNI: network.SegmentationID}, nil
}


func (mapper *NeutronMapper) nodeUpdater() {
	logging.GetLogger().Debugf("Starting Neutron updater")
	for {
		nodeID := <-mapper.nodeUpdaterChan
		node := mapper.graph.GetNode(nodeID)
		if node == nil {
			continue
		}

		mac, ok := node.Metadata()["MAC"];
		if !ok {
			continue
		}

		attrs, err := mapper.retrieveAttributes(mac.(string))
		if err != nil {
			continue
		}

		mapper.updateNode(node, attrs)
	}
	logging.GetLogger().Debugf("Stopping Neutron updater")
}

func (mapper *NeutronMapper) updateNode(node *graph.Node, attrs *Attributes) {
	mapper.graph.Lock()
	defer mapper.graph.Unlock()

	if attrs.TenantID != "" && node.Metadata()["Neutron.TenantID"] != attrs.TenantID {
		mapper.graph.AddMetadata(node, "Neutron.TenantID", attrs.TenantID)
	}

	if segID, err := strconv.Atoi(attrs.VNI); err != nil {
		if node.Metadata()["Neutron.VNI"] != uint64(segID) {
			mapper.graph.AddMetadata(node, "Neutron.VNI", uint64(segID))
		}
	}

	mapper.cache.Set(string(node.ID), attrs, cache.DefaultExpiration)
}

func (mapper *NeutronMapper) EnhanceNode(node *graph.Node) {
	mac, ok := node.Metadata()["MAC"];
	if !ok {
		return
	}

	a, f := mapper.cache.Get(mac.(string))
	if f {
		attrs := a.(Attributes)
		mapper.updateNode(node, &attrs)
		return
	}

	mapper.nodeUpdaterChan <-node.ID
}

func (mapper *NeutronMapper) OnNodeUpdated(n *graph.Node) {
	go mapper.EnhanceNode(n)
}

func (mapper *NeutronMapper) OnNodeAdded(n *graph.Node) {
	go mapper.EnhanceNode(n)
}

func (mapper *NeutronMapper) OnNodeDeleted(n *graph.Node) {
}

func (mapper *NeutronMapper) OnEdgeUpdated(n *graph.Edge) {
}

func (mapper *NeutronMapper) OnEdgeAdded(n *graph.Edge) {
}

func (mapper *NeutronMapper) OnEdgeDeleted(n *graph.Edge) {
}

func (mapper *NeutronMapper) Start() {
	go mapper.nodeUpdater()
}

func (mapper *NeutronMapper) Stop() {
	close(mapper.nodeUpdaterChan)
}

func NewNeutronMapper(g *graph.Graph, authURL string, username string, password string, tenantName string, regionName string) (*NeutronMapper, error) {
	mapper := &NeutronMapper{graph: g}

	opts := gophercloud.AuthOptions{
		IdentityEndpoint: authURL,
		Username:         username,
		Password:         password,
		TenantName:       tenantName,
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
	mapper.nodeUpdaterChan = make(chan graph.Identifier, 100)

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
