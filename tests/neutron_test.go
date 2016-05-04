// +build neutron

/*
 * Copyright (C) 2016 Red Hat, Inc.
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

package tests

import (
	"fmt"
	"os"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/rackspace/gophercloud"
	"github.com/rackspace/gophercloud/openstack"
	"github.com/rackspace/gophercloud/openstack/networking/v2/networks"
	"github.com/rackspace/gophercloud/openstack/networking/v2/ports"
	"github.com/rackspace/gophercloud/pagination"

	"github.com/redhat-cip/skydive/tests/helper"
	"github.com/redhat-cip/skydive/topology/graph"
)

const confNeutron = `---
ws_pong_timeout: 5

agent:
  listen: 58081
  flowtable_expire: 300
  flowtable_update: 20
  topology:
    probes:
      - netlink
      - netns
      - ovsdb
      - neutron

cache:
  expire: 300
  cleanup: 30

sflow:
  listen: 55000

ovs:
  ovsdb: {{.OvsdbPort}}

etcd:
  embedded: true
  port: 2374
  data_dir: /tmp
  servers: http://localhost:2374

logging:
  default: {{.LogLevel}}

openstack:
  auth_url: {{.OsAuthUrl}}
  username: {{.OsUsername}}
  password: {{.OsPassword}}
  tenant_name: {{.OsTenantName}}
  region_name: {{.OsRegionName}}
`

func TestNeutron(t *testing.T) {
	g := newGraph(t)

	authUrl := os.Getenv("OS_AUTH_URL")
	username := os.Getenv("OS_USERNAME")
	password := os.Getenv("OS_PASSWORD")
	tenantName := os.Getenv("OS_TENANT_NAME")
	regionName := os.Getenv("OS_REGION_NAME")
	ovsdbPort := os.Getenv("SKYDIVE_OVSDB_REMOTE_PORT")

	params := map[string]interface{}{
		"OsAuthUrl":    authUrl,
		"OsUsername":   username,
		"OsPassword":   password,
		"OsTenantName": tenantName,
		"OsRegionName": regionName,
		"OvsdbPort":    ovsdbPort,
	}
	agent := helper.StartAgentWithConfig(t, confNeutron, params)
	defer agent.Stop()

	opts := gophercloud.AuthOptions{
		IdentityEndpoint: authUrl,
		Username:         username,
		Password:         password,
		TenantName:       tenantName,
	}

	provider, err := openstack.AuthenticatedClient(opts)
	if err != nil {
		t.Fatal(err.Error())
	}

	client, err := openstack.NewNetworkV2(provider, gophercloud.EndpointOpts{
		Name:         "neutron",
		Region:       regionName,
		Availability: gophercloud.AvailabilityPublic,
	})
	if err != nil {
		t.Fatalf("Failed to create neutron client: %s", err.Error())
	}

	result := networks.Create(client, networks.CreateOpts{Name: "skydive-test-network"})
	if result.Err != nil {
		t.Fatalf("Failed to create neutron network: %s", result.Err.Error())
	}

	network, err := result.Extract()
	if err != nil {
		t.Fatalf("Failed to create neutron network: %s", err.Error())
	}

	defer networks.Delete(client, network.ID)

	setupCmds := []helper.Cmd{
		{fmt.Sprintf("neutron subnet-create --name skydive-test-subnet-%s %s 10.0.0.0/24", network.ID, network.ID), false},
		{fmt.Sprintf("neutron-debug probe-create %s", network.ID), false},
	}

	tearDownCmds := []helper.Cmd{
		{fmt.Sprintf("neutron subnet-delete skydive-test-subnet-%s", network.ID), false},
	}

	var port *ports.Port
	testPassed := false
	onChange := func(ws *websocket.Conn) {
		g.Lock()
		defer g.Unlock()

		if port == nil {
			portListOpts := ports.ListOpts{}
			pager := ports.List(client, portListOpts)
			err = pager.EachPage(func(page pagination.Page) (bool, error) {
				portList, err := ports.ExtractPorts(page)
				if err != nil {
					return false, err
				}

				for _, p := range portList {
					if p.DeviceOwner == "network:probe" && p.NetworkID == network.ID {
						port = &p
						return false, nil
					}
				}

				return true, nil
			})

			if port != nil {
				tearDownCmds = append(tearDownCmds, helper.Cmd{})
				copy(tearDownCmds[1:], tearDownCmds[0:])
				tearDownCmds[0] = helper.Cmd{fmt.Sprintf("neutron-debug probe-delete %s", port.ID), false}
			}
		}

		if !testPassed && len(g.GetNodes()) >= 1 && len(g.GetEdges()) >= 1 && port != nil {
			if g.LookupFirstNode(graph.Metadata{"Name": fmt.Sprintf("qdhcp-%s", network.ID), "Type": "netns"}) != nil {
				if g.LookupFirstNode(graph.Metadata{"Name": fmt.Sprintf("qprobe-%s", port.ID), "Type": "netns"}) != nil {
					if g.LookupFirstNode(graph.Metadata{"Type": "internal", "Driver": "openvswitch", "Manager": "neutron", "Neutron.NetworkID": network.ID}) != nil {
						testPassed = true
						ws.Close()
					}
				}
			}
		}
	}

	testTopology(t, g, setupCmds, onChange)
	if !testPassed {
		t.Error("test not executed or failed")
	}

	testCleanup(t, g, tearDownCmds, []string{fmt.Sprintf("qdhcp-%s", network.ID), fmt.Sprintf("qprobe-%s", port.ID)})
}
