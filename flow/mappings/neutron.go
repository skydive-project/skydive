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

package mappings

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

	//"github.com/golang/protobuf/proto"
	"github.com/pmylund/go-cache"

	"github.com/redhat-cip/skydive/config"
	//"github.com/redhat-cip/skydive/flow"
	"github.com/redhat-cip/skydive/logging"
)

type NeutronMapper struct {
	client           *gophercloud.ServiceClient
	cache            *cache.Cache
	cacheUpdaterChan chan string
}

type Attributes struct {
	TenantID string
	VNI      uint64
}

func (mapper *NeutronMapper) retrievePort(mac string) (ports.Port, error) {
	port := ports.Port{}

	opts := ports.ListOpts{MACAddress: mac}
	pager := ports.List(mapper.client, opts)
	err := pager.EachPage(func(page pagination.Page) (bool, error) {
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
		return port, errors.New("Unable to find port for mac address: " + mac)
	}

	return port, err
}

func (mapper *NeutronMapper) retrieveAttributes(mac string) Attributes {
	logging.GetLogger().Debugf("Retrieving attributes from Neutron for Mac: %s", mac)

	attrs := Attributes{}

	port, err := mapper.retrievePort(mac)
	if err != nil {
		return attrs
	}
	attrs.TenantID = port.TenantID

	result := networks.Get(mapper.client, port.NetworkID)
	network, err := provider.ExtractGet(result)
	if err != nil {
		return attrs
	}

	if err != nil {
		return attrs
	}

	segID, err := strconv.Atoi(network.SegmentationID)
	if err == nil {
		attrs.VNI = uint64(segID)
	}

	return attrs
}

func (mapper *NeutronMapper) cacheUpdater() {
	logging.GetLogger().Debug("Start Neutron cache updater")

	var mac string
	for {
		mac = <-mapper.cacheUpdaterChan

		logging.GetLogger().Debugf("Mac request received: %s", mac)

		attrs := mapper.retrieveAttributes(mac)
		mapper.cache.Set(mac, attrs, cache.DefaultExpiration)
	}
}

/*func (mapper *NeutronMapper) EnhanceInterface(mac string, attrs *flow.Flow_InterfaceAttributes) {
	a, f := mapper.cache.Get(mac)
	if f {
		ia := a.(Attributes)

		attrs.TenantID = proto.String(ia.TenantID)
		attrs.VNI = proto.Uint64(ia.VNI)

		return
	}

	mapper.cacheUpdaterChan <- mac
}*/

func NewNeutronMapper() (*NeutronMapper, error) {
	mapper := &NeutronMapper{}

	authURL := config.GetConfig().GetString("openstack.auth_url")
	username := config.GetConfig().GetString("openstack.username")
	password := config.GetConfig().GetString("openstack.password")
	tenantName := config.GetConfig().GetString("openstack.tenant_name")
	regionName := config.GetConfig().GetString("openstack.region_name")

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
	mapper.cacheUpdaterChan = make(chan string)
	go mapper.cacheUpdater()

	return mapper, nil
}
