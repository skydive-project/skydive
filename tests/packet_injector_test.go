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
	"testing"
	"time"

	"github.com/skydive-project/skydive/api"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/tests/helper"
)

func TestPacketInjector(t *testing.T) {
	test := &Test{
		setupCmds: []helper.Cmd{
			{"ovs-vsctl add-br br-pi", true},
			{"ip netns add pi-vm1", true},
			{"ip link add pi-vm1-eth0 type veth peer name pi-eth-src netns pi-vm1", true},
			{"ip link set pi-vm1-eth0 up", true},
			{"ip netns exec pi-vm1 ip link set pi-eth-src up", true},
			{"ip netns exec pi-vm1 ip address add 169.254.33.33/24 dev pi-eth-src", true},
			{"ovs-vsctl add-port br-pi pi-vm1-eth0", true},
			{"ip netns add pi-vm2", true},
			{"ip link add pi-vm2-eth0 type veth peer name pi-eth-dst netns pi-vm2", true},
			{"ip link set pi-vm2-eth0 up", true},
			{"ip netns exec pi-vm2 ip link set pi-eth-dst up", true},
			{"ip netns exec pi-vm2 ip address add 169.254.33.34/24 dev pi-eth-dst", true},
			{"ovs-vsctl add-port br-pi pi-vm2-eth0", true},
		},

		setupFunction: func(c *TestContext) (err error) {
			packet := &api.PacketParamsReq{
				Src:   "G.V().Has('Name', 'pi-eth-src')",
				Dst:   "G.V().Has('Name', 'pi-eth-dst')",
				Type:  "icmp",
				Count: 10,
			}

			return common.Retry(func() error {
				return c.client.Create("injectpacket", &packet)
			}, 10, time.Second)
		},

		tearDownCmds: []helper.Cmd{
			{"ovs-vsctl del-br br-pi", true},
			{"ip link del pi-vm1-eth0", true},
			{"ip netns del pi-vm1", true},
			{"ip link del pi-vm2-eth0", true},
			{"ip netns del pi-vm2", true},
		},

		captures: []TestCapture{
			{gremlin: `G.V().Has('Name', 'pi-eth-src').ShortestPathTo(Metadata('Name', 'pi-eth-dst'), Metadata('RelationType', 'layer2'))`},
		},

		check: func(c *TestContext) error {
			gremlin := "g"
			if !c.time.IsZero() {
				gremlin += fmt.Sprintf(".Context(%d)", common.UnixMillis(c.time))
			}
			gremlin += ".V().Flows().Has('Network.A', '169.254.33.33').Has('Network.B', '169.254.33.34').Dedup()"

			flows, err := c.gh.GetFlows(gremlin)
			if err != nil {
				return err
			}

			if len(flows) != 1 {
				return fmt.Errorf("Expected one flow, got %+v", flows)
			}

			return nil
		},
	}

	RunTest(t, test)
}
