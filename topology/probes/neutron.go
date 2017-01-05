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
	"regexp"
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
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"
)

type NeutronMapper struct {
	graph.DefaultGraphListener
	graph    *graph.Graph
	wsClient *shttp.WSAsyncClient
	client   *gophercloud.ServiceClient
	// The cache associates some metadatas to a MAC and is used to
	// detect any updates on these metadatas.
	cache           *cache.Cache
	nodeUpdaterChan chan graph.Identifier
	intfRegexp      *regexp.Regexp
	nsRegexp        *regexp.Regexp
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

var (
	PendingPortMetadata  = PortMetadata{mac: "00:00:00:00:00:00", portID: "Pending"}
	NotFoundPortMetadata = PortMetadata{mac: "00:00:00:00:00:00", portID: "NotFound"}
)

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
				mapper.cache.Set(nerr.MAC, NotFoundPortMetadata, cache.DefaultExpiration)
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

	metadata := map[string]interface{}{"Manager": "neutron"}

	if attrs.PortID != "" {
		metadata["Neutron/PortID"] = "neutron"
	}

	if attrs.TenantID != "" {
		metadata["Neutron/TenantID"] = attrs.TenantID
	}

	if attrs.NetworkID != "" {
		metadata["Neutron/NetworkID"] = attrs.NetworkID
	}

	if attrs.NetworkName != "" {
		metadata["Neutron/NetworkName"] = attrs.NetworkName
	}

	if segID, err := strconv.Atoi(attrs.VNI); err != nil && segID > 0 {
		metadata["Neutron/VNI"] = uint64(segID)
	}

	tr := mapper.graph.StartMetadataTransaction(node)
	for k, v := range metadata {
		tr.AddMetadata(k, v)
	}
	tr.Commit()

	if _, ok := node.Metadata()["ExtID/vm-uuid"]; ok {
		if mac, ok := node.Metadata()["ExtID/attached-mac"]; ok {
			if path := mapper.graph.LookupShortestPath(node, graph.Metadata{"Type": "tun"}, graph.Metadata{"RelationType": "layer2"}); len(path) > 0 {
				metadata["ExtID/vm-uuid"] = node.Metadata()["ExtID/vm-uuid"]
				metadata["ExtID/attached-mac"] = node.Metadata()["ExtID/attached-mac"]
				for i, n := range path {
					tr := mapper.graph.StartMetadataTransaction(n)
					for k, v := range metadata {
						tr.AddMetadata(k, v)
					}

					// add vm peering info, going to be used by peering probe
					if i == len(path)-1 {
						tr.AddMetadata("PeerIntfMAC", mac)
					}
					tr.Commit()
				}
			}
		}
	}
	mapper.graph.Unlock()
}

func (mapper *NeutronMapper) EnhanceNode(node *graph.Node) {
	name, ok := node.Metadata()["Name"]
	if !ok {
		return
	}

	if mapper.nsRegexp.MatchString(name.(string)) {
		mapper.graph.AddMetadata(node, "Manager", "neutron")
		return
	}

	if !mapper.intfRegexp.MatchString(name.(string)) {
		return
	}

	mac, ok := node.Metadata()["MAC"]
	if !ok {
		return
	}

	portMd, f := mapper.cache.Get(mac.(string))
	// If port metadatas have not changed, we return
	if f && (portMd == PendingPortMetadata || portMd == retrievePortMetadata(node.Metadata())) {
		return
	}
	mapper.cache.Set(mac.(string), PendingPortMetadata, cache.DefaultExpiration)

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

func NewNeutronMapper(g *graph.Graph, wsClient *shttp.WSAsyncClient, authURL, username, password, tenantName, regionName, domainName string, availability gophercloud.Availability) (*NeutronMapper, error) {
	// only looking for interfaces matching the following regex as nova, neutron interfaces match this pattern
	intfRegexp := regexp.MustCompile(`(tap|qr-|qg-|qvo)[a-fA-F0-9]{8}-[a-fA-F0-9]{2}`)
	nsRegexp := regexp.MustCompile(`(qrouter|qdhcp)-[a-fA-F0-9]{8}`)

	mapper := &NeutronMapper{graph: g, wsClient: wsClient, intfRegexp: intfRegexp, nsRegexp: nsRegexp}

	opts := gophercloud.AuthOptions{
		IdentityEndpoint: authURL,
		Username:         username,
		Password:         password,
		TenantName:       tenantName,
		DomainName:       domainName,
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

func NewNeutronMapperFromConfig(g *graph.Graph, wsCient *shttp.WSAsyncClient) (*NeutronMapper, error) {
	authURL := config.GetConfig().GetString("openstack.auth_url")
	username := config.GetConfig().GetString("openstack.username")
	password := config.GetConfig().GetString("openstack.password")
	tenantName := config.GetConfig().GetString("openstack.tenant_name")
	regionName := config.GetConfig().GetString("openstack.region_name")
	endpointType := config.GetConfig().GetString("openstack.endpoint_type")
	domainName := config.GetConfig().GetString("openstack.domain_name")

	endpointTypes := map[string]gophercloud.Availability{
		"public":   gophercloud.AvailabilityPublic,
		"admin":    gophercloud.AvailabilityAdmin,
		"internal": gophercloud.AvailabilityInternal}
	if a, ok := endpointTypes[endpointType]; !ok {
		return nil, fmt.Errorf("Endpoint type '%s' is not valid (must be 'public', 'admin' or 'internal')", endpointType)
	} else {
		return NewNeutronMapper(g, wsCient, authURL, username, password, tenantName, regionName, domainName, a)
	}
}
