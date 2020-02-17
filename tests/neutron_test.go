// +build neutron

/*
 * Copyright (C) 2016 Red Hat, Inc.
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

package tests

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/avast/retry-go"
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/ports"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/subnets"

	gclient "github.com/skydive-project/skydive/api/client"
	g "github.com/skydive-project/skydive/gremlin"
	shttp "github.com/skydive-project/skydive/graffiti/http"
)

func TestNeutron(t *testing.T) {
	authUrl := os.Getenv("OS_AUTH_URL")
	username := os.Getenv("OS_USERNAME")
	password := os.Getenv("OS_PASSWORD")
	tenantName := os.Getenv("OS_TENANT_NAME")
	regionName := os.Getenv("OS_REGION_NAME")
	domainID := os.Getenv("OS_PROJECT_DOMAIN_ID")

	opts := gophercloud.AuthOptions{
		IdentityEndpoint: authUrl,
		Username:         username,
		Password:         password,
		TenantName:       tenantName,
		DomainID:         domainID,
	}

	if opts.DomainID == "" {
		opts.DomainName = os.Getenv("OS_PROJECT_DOMAIN_NAME")
	}

	var client *gophercloud.ServiceClient
	var netResult networks.CreateResult

	// wait a bit to have openstack ready
	fnc := func() error {
		provider, err := openstack.AuthenticatedClient(opts)
		if err != nil {
			return fmt.Errorf("Authentication error: %s", err.Error())
		}

		client, err = openstack.NewNetworkV2(provider, gophercloud.EndpointOpts{
			Name:         "neutron",
			Region:       regionName,
			Availability: gophercloud.AvailabilityPublic,
		})
		if err != nil {
			return fmt.Errorf("Failed to create neutron client: %s", err.Error())
		}

		netResult = networks.Create(client, networks.CreateOpts{Name: "skydive-test-network"})
		if netResult.Err != nil {
			return fmt.Errorf("Failed to create neutron network: %s", netResult.Err.Error())
		}

		return nil
	}
	// yes 2 minutes but that's openstack !
	if err := retry.Do(fnc, retry.Attempts(120), retry.Delay(time.Second)); err != nil {
		t.Fatalf(err.Error())
	}

	network, err := netResult.Extract()
	if err != nil {
		t.Fatalf("Failed to create neutron network: %s", err.Error())
	}

	subResult := subnets.Create(client, subnets.CreateOpts{Name: "skydive-test-subnet", NetworkID: network.ID, CIDR: "192.168.1.0/24", IPVersion: 4})
	if subResult.Err != nil {
		t.Fatalf("Failed to create neutron subnet: %s", subResult.Err.Error())
	}

	subnet, err := subResult.Extract()
	if err != nil {
		t.Fatalf("Failed to create neutron subnet: %s", err.Error())
	}

	portResult := ports.Create(client, ports.CreateOpts{NetworkID: network.ID, DeviceID: "skydive-123", DeviceOwner: "skydive-test"})
	if portResult.Err != nil {
		t.Fatalf("Failed to create neutron port: %s", subResult.Err.Error())
	}

	port, err := portResult.Extract()
	if err != nil {
		t.Fatalf("Failed to create neutron port: %s", err.Error())
	}

	defer ports.Delete(client, port.ID)
	defer subnets.Delete(client, subnet.ID)
	defer networks.Delete(client, network.ID)

	authOptions := &shttp.AuthenticationOpts{
		Username: username,
		Password: password,
	}

	subID := port.ID[0:11]
	dev := fmt.Sprintf("tap%s", subID)

	ovsctl := `ovs-vsctl add-port br-int %s -- set Interface %s external-ids:iface-id=%s`
	ovsctl += ` external-ids:iface-status=active external-ids:attached-mac=%s external-ids:vm-uuid=skydive-vm type=internal`

	setupCmds := []Cmd{
		{fmt.Sprintf(ovsctl, dev, dev, port.ID, port.MACAddress), true},
		{"sleep 1", true},
		{fmt.Sprintf("ip link set %s up", dev), true},
	}

	tearDownCmds := []Cmd{
		{fmt.Sprintf("ovs-vsctl del-port %s", dev), true},
	}
	execCmds(t, setupCmds...)
	defer execCmds(t, tearDownCmds...)

	gh := gclient.NewGremlinQueryHelper(authOptions)

	var histo bool
	retryFn := func() error {
		prefix := g.G
		if histo {
			prefix = prefix.At("-1s")
		}

		nodes, err := gh.GetNodes(prefix.V().Has("Manager", "neutron", "ExtID.vm-uuid", "skydive-vm", "Name", dev, "Neutron.PortID", port.ID))
		if err != nil {
			return fmt.Errorf(err.Error())
		}

		if len(nodes) != 1 {
			nodes, _ := gh.GetNodes(g.G.V())
			return fmt.Errorf("Should find the neutron port in the topology: %v", nodes)
		}
		return nil
	}

	// test live mode
	if err := retry.Do(retry, retry.Attempts(10), retry.Delay(time.Second)); err != nil {
		t.Error(err)
	}

	// test histo mode
	histo = true
	if err := retry.Do(retryFn, retry.Attempts(20), retry.Delay(time.Second)); err != nil {
		t.Error(err)
	}
}
