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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	gclient "github.com/skydive-project/skydive/cmd/client"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/tests/helper"
)

func TestSFlowProbeNode(t *testing.T) {
	test := &Test{
		setupCmds: []helper.Cmd{
			{"ovs-vsctl add-br br-spn", true},
			{"ovs-vsctl add-port br-spn spn-intf1 -- set interface spn-intf1 type=internal", true},
			{"ip address add 169.254.33.33/24 dev spn-intf1", true},
			{"ip link set spn-intf1 up", true},
		},

		setupFunction: func(c *TestContext) error {
			helper.ExecCmds(t, helper.Cmd{Cmd: "ping -c 5 -I spn-intf1 169.254.33.34", Check: false})
			return nil
		},

		tearDownCmds: []helper.Cmd{
			{"ovs-vsctl del-br br-spn", true},
		},

		captures: []TestCapture{
			{gremlin: `g.V().Has("Name", "br-spn", "Type", "ovsbridge")`},
		},

		check: func(c *TestContext) error {
			prefix := "g"
			if !c.time.IsZero() {
				prefix += fmt.Sprintf(".Context('-%dns')", time.Now().Sub(c.time).Nanoseconds())
			}

			gh := c.gh
			node, err := gh.GetNode(prefix + `.V().Has("Name", "br-spn", "Type", "ovsbridge")`)
			if err != nil {
				return err
			}

			flows, err := gh.GetFlows(prefix + fmt.Sprintf(`.Flows("NodeTID", "%s", "LayersPath", "Ethernet/ARP/Payload")`, node.Metadata()["TID"].(string)))
			if err != nil {
				return err
			}

			if len(flows) == 0 {
				return errors.New("Unable to find a flow with the expected NodeTID")
			}

			return nil
		},
	}

	RunTest(t, test)
}

func TestSFlowNodeTIDOvsInternalNetNS(t *testing.T) {
	test := &Test{
		setupCmds: []helper.Cmd{
			{"ovs-vsctl add-br br-sntoin", true},
			{"ovs-vsctl add-port br-sntoin sntoin-intf1 -- set interface sntoin-intf1 type=internal", true},
			{"ip netns add sntoin-vm1", true},
			{"ip link set sntoin-intf1 netns sntoin-vm1", true},
			{"ip netns exec sntoin-vm1 ip address add 169.254.33.33/24 dev sntoin-intf1", true},
			{"ip netns exec sntoin-vm1 ip link set sntoin-intf1 up", true},
		},

		setupFunction: func(c *TestContext) error {
			helper.ExecCmds(t, helper.Cmd{Cmd: "ip netns exec sntoin-vm1 ping -c 5 -I sntoin-intf1 169.254.33.34", Check: false})
			return nil
		},

		tearDownCmds: []helper.Cmd{
			{"ip netns del sntoin-vm1", true},
			{"ovs-vsctl del-br br-sntoin", true},
		},

		captures: []TestCapture{
			{gremlin: `g.V().Has("Name", "br-sntoin", "Type", "ovsbridge")`},
		},

		check: func(c *TestContext) error {
			prefix := "g"
			if !c.time.IsZero() {
				prefix += fmt.Sprintf(".Context('-%dns')", time.Now().Sub(c.time).Nanoseconds())
			}

			gh := c.gh
			node, err := gh.GetNode(prefix + `.V().Has("Name", "br-sntoin", "Type", "ovsbridge")`)
			if err != nil {
				return err
			}

			flows, err := gh.GetFlows(prefix + fmt.Sprintf(`.Flows("NodeTID", "%s", "LayersPath", "Ethernet/ARP/Payload")`, node.Metadata()["TID"].(string)))
			if err != nil {
				return err
			}

			if len(flows) == 0 {
				return errors.New("Unable to find a flow with the expected NodeTID")
			}

			return nil
		},
	}

	RunTest(t, test)
}

func TestSFlowTwoNodeTID(t *testing.T) {
	test := &Test{
		setupCmds: []helper.Cmd{
			{"ovs-vsctl add-br br-stnt1", true},
			{"ovs-vsctl add-port br-stnt1 stnt-intf1 -- set interface stnt-intf1 type=internal", true},
			{"ip netns add stnt-vm1", true},
			{"ip link set stnt-intf1 netns stnt-vm1", true},
			{"ip netns exec stnt-vm1 ip address add 169.254.33.33/24 dev stnt-intf1", true},
			{"ip netns exec stnt-vm1 ip link set stnt-intf1 up", true},

			{"ovs-vsctl add-br br-stnt2", true},
			{"ovs-vsctl add-port br-stnt2 stnt-intf2 -- set interface stnt-intf2 type=internal", true},
			{"ip netns add stnt-vm2", true},
			{"ip link set stnt-intf2 netns stnt-vm2", true},
			{"ip netns exec stnt-vm2 ip address add 169.254.33.34/24 dev stnt-intf2", true},
			{"ip netns exec stnt-vm2 ip link set stnt-intf2 up", true},

			// interfaces used to link br-stnt1 and br-stnt2 without a patch
			{"ovs-vsctl add-port br-stnt1 stnt-link1 -- set interface stnt-link1 type=internal", true},
			{"ip link set stnt-link1 up", true},
			{"ovs-vsctl add-port br-stnt2 stnt-link2 -- set interface stnt-link2 type=internal", true},
			{"ip link set stnt-link2 up", true},

			{"brctl addbr br-stnt-link", true},
			{"ip link set br-stnt-link up", true},
			{"brctl addif br-stnt-link stnt-link1", true},
			{"brctl addif br-stnt-link stnt-link2", true},
		},

		setupFunction: func(c *TestContext) error {
			helper.ExecCmds(t, helper.Cmd{Cmd: "ip netns exec stnt-vm2 ping -c 5 -I stnt-intf2 169.254.33.33", Check: false})
			return nil
		},

		tearDownCmds: []helper.Cmd{
			{"ip netns del stnt-vm1", true},
			{"ip netns del stnt-vm2", true},
			{"ovs-vsctl del-br br-stnt1", true},
			{"ovs-vsctl del-br br-stnt2", true},
			{"sleep 1", true},
			{"ip link set br-stnt-link down", true},
			{"brctl delbr br-stnt-link", true},
		},

		captures: []TestCapture{
			{gremlin: `G.V().Has('Name', 'br-stnt1', 'Type', 'ovsbridge')`},
			{gremlin: `G.V().Has('Name', 'br-stnt2', 'Type', 'ovsbridge')`},
		},

		check: func(c *TestContext) error {
			prefix := "g"
			if !c.time.IsZero() {
				prefix += fmt.Sprintf(".Context(%d)", common.UnixMillis(c.time))
			}

			gh := c.gh
			flows, err := gh.GetFlows(prefix + ".V().Has('Type', 'ovsbridge').Flows().Has('LayersPath', 'Ethernet/IPv4/ICMPv4/Payload')")
			if err != nil {
				return err
			}

			if len(flows) != 2 {
				return fmt.Errorf("Should have 2 flow entries one per NodeTID got: %d", len(flows))
			}

			node1, err := gh.GetNode(prefix + `.V().Has("Name", "br-stnt1", "Type", "ovsbridge").HasKey("TID")`)
			if err != nil {
				return err
			}

			node2, err := gh.GetNode(prefix + `.V().Has("Name", "br-stnt2", "Type", "ovsbridge").HasKey("TID")`)
			if err != nil {
				return err
			}

			tid1, _ := node1.GetFieldString("TID")
			tid2, _ := node2.GetFieldString("TID")

			if flows[0].NodeTID != tid1 && flows[0].NodeTID != tid2 {
				t.Errorf("Bad NodeTID for the first flow: %s", flows[0].NodeTID)
			}

			if flows[1].NodeTID != tid1 && flows[1].NodeTID != tid2 {
				t.Errorf("Bad NodeTID for the second flow: %s", flows[1].NodeTID)
			}

			if flows[0].NodeTID != node1.Metadata()["TID"].(string) &&
				flows[0].NodeTID != node2.Metadata()["TID"].(string) {
				return fmt.Errorf("Bad NodeTID for the first flow: %s", flows[0].NodeTID)
			}

			if flows[1].NodeTID != node1.Metadata()["TID"].(string) &&
				flows[1].NodeTID != node2.Metadata()["TID"].(string) {
				return fmt.Errorf("Bad NodeTID for the second flow: %s", flows[1].NodeTID)
			}

			if flows[0].TrackingID != flows[1].TrackingID {
				return fmt.Errorf("Both flows should have the same TrackingID: %v", flows)
			}

			if flows[0].UUID == flows[1].UUID {
				return fmt.Errorf("Both flows should have different UUID: %v", flows)
			}

			return nil
		},
	}

	RunTest(t, test)
}

func TestBPF(t *testing.T) {
	test := &Test{
		setupCmds: []helper.Cmd{
			{"brctl addbr br-bpf", true},
			{"ip link set br-bpf up", true},
			{"ip netns add bpf-vm1", true},
			{"ip link add name bpf-vm1-eth0 type veth peer name eth0 netns bpf-vm1", true},
			{"ip link set bpf-vm1-eth0 up", true},
			{"ip netns exec bpf-vm1 ip link set eth0 up", true},
			{"ip netns exec bpf-vm1 ip address add 169.254.66.66/24 dev eth0", true},
			{"brctl addif br-bpf bpf-vm1-eth0", true},

			{"ip netns add bpf-vm2", true},
			{"ip link add name bpf-vm2-eth0 type veth peer name eth0 netns bpf-vm2", true},
			{"ip link set bpf-vm2-eth0 up", true},
			{"ip netns exec bpf-vm2 ip link set eth0 up", true},
			{"ip netns exec bpf-vm2 ip address add 169.254.66.67/24 dev eth0", true},
			{"brctl addif br-bpf bpf-vm2-eth0", true},
		},

		setupFunction: func(c *TestContext) error {
			helper.ExecCmds(t, helper.Cmd{Cmd: "ip netns exec bpf-vm1 ping -c 5 169.254.66.67", Check: false})
			return nil
		},

		tearDownCmds: []helper.Cmd{
			{"ip link set br-bpf down", true},
			{"brctl delbr br-bpf", true},
			{"ip link del bpf-vm1-eth0", true},
			{"ip link del bpf-vm2-eth0", true},
			{"ip netns del bpf-vm1", true},
			{"ip netns del bpf-vm2", true},
		},

		captures: []TestCapture{
			{gremlin: `G.V().Has('Name', 'bpf-vm1-eth0')`, bpf: "icmp and host 169.254.66.67"},
		},

		check: func(c *TestContext) error {
			prefix := "g"
			if !c.time.IsZero() {
				prefix += fmt.Sprintf(".Context(%d)", common.UnixMillis(c.time))
			}

			flows, err := c.gh.GetFlows(prefix + `.V().Has('Name', 'bpf-vm1-eth0').Flows()`)
			if err != nil {
				return err
			}

			if len(flows) != 1 {
				return fmt.Errorf("Should only get icmp packets, got : %v", flows)
			}

			return nil
		},
	}

	RunTest(t, test)
}

func TestPCAPProbe(t *testing.T) {
	test := &Test{
		setupCmds: []helper.Cmd{
			{"brctl addbr br-pp", true},
			{"ip link set br-pp up", true},
			{"ip netns add pp-vm1", true},
			{"ip link add name pp-vm1-eth0 type veth peer name eth0 netns pp-vm1", true},
			{"ip link set pp-vm1-eth0 up", true},
			{"ip netns exec pp-vm1 ip link set eth0 up", true},
			{"ip netns exec pp-vm1 ip address add 169.254.66.66/24 dev eth0", true},
			{"brctl addif br-pp pp-vm1-eth0", true},

			{"ip netns add pp-vm2", true},
			{"ip link add name pp-vm2-eth0 type veth peer name eth0 netns pp-vm2", true},
			{"ip link set pp-vm2-eth0 up", true},
			{"ip netns exec pp-vm2 ip link set eth0 up", true},
			{"ip netns exec pp-vm2 ip address add 169.254.66.67/24 dev eth0", true},
			{"brctl addif br-pp pp-vm2-eth0", true},
		},

		setupFunction: func(c *TestContext) error {
			helper.ExecCmds(t, helper.Cmd{Cmd: "ip netns exec pp-vm1 ping -c 5 169.254.66.67", Check: false})
			return nil
		},

		tearDownCmds: []helper.Cmd{
			{"ip link set br-pp down", true},
			{"brctl delbr br-pp", true},
			{"ip link del pp-vm1-eth0", true},
			{"ip link del pp-vm2-eth0", true},
			{"ip netns del pp-vm1", true},
			{"ip netns del pp-vm2", true},
		},

		captures: []TestCapture{
			{gremlin: `G.V().Has('Name', 'br-pp', 'Type', 'bridge')`, kind: "pcap"},
		},

		check: func(c *TestContext) error {
			prefix := "g"
			if !c.time.IsZero() {
				prefix += fmt.Sprintf(".Context(%d)", common.UnixMillis(c.time))
			}

			gh := c.gh
			node, err := gh.GetNode(prefix + `.V().Has("Name", "br-pp", "Type", "bridge")`)
			if err != nil {
				return err
			}

			flows, err := gh.GetFlows(fmt.Sprintf(prefix+`.Flows().Has("NodeTID", "%s")`, node.Metadata()["TID"]))
			if err != nil {
				return err
			}

			if len(flows) == 0 {
				return fmt.Errorf("Unable to find a flow with the expected NodeTID: %v", flows)
			}

			return nil
		},
	}

	RunTest(t, test)
}

func TestSFlowSrcDstPath(t *testing.T) {
	test := &Test{
		setupCmds: []helper.Cmd{
			{"ovs-vsctl add-br br-ssdp", true},

			{"ovs-vsctl add-port br-ssdp ssdp-intf1 -- set interface ssdp-intf1 type=internal", true},
			{"ip netns add ssdp-vm1", true},
			{"ip link set ssdp-intf1 netns ssdp-vm1", true},
			{"ip netns exec ssdp-vm1 ip address add 169.254.33.33/24 dev ssdp-intf1", true},
			{"ip netns exec ssdp-vm1 ip link set ssdp-intf1 up", true},

			{"ovs-vsctl add-port br-ssdp ssdp-intf2 -- set interface ssdp-intf2 type=internal", true},
			{"ip netns add ssdp-vm2", true},
			{"ip link set ssdp-intf2 netns ssdp-vm2", true},
			{"ip netns exec ssdp-vm2 ip address add 169.254.33.34/24 dev ssdp-intf2", true},
			{"ip netns exec ssdp-vm2 ip link set ssdp-intf2 up", true},
		},

		setupFunction: func(c *TestContext) error {
			helper.ExecCmds(t, helper.Cmd{Cmd: "ip netns exec ssdp-vm1 ping -c 5 169.254.33.34", Check: false})
			return nil
		},

		tearDownCmds: []helper.Cmd{
			{"ip netns del ssdp-vm1", true},
			{"ip netns del ssdp-vm2", true},
			{"ovs-vsctl del-br br-ssdp", true},
		},

		captures: []TestCapture{
			{gremlin: `G.V().Has('Name', 'br-ssdp', 'Type', 'ovsbridge')`},
		},

		check: func(c *TestContext) error {
			prefix := "g"
			if !c.time.IsZero() {
				prefix += fmt.Sprintf(".Context(%d)", common.UnixMillis(c.time))
			}

			gh := c.gh
			node1, err := gh.GetNode(prefix + `.V().Has("Name", "ssdp-intf1", "Type", "internal")`)
			if err != nil {
				return err
			}

			node2, err := gh.GetNode(prefix + `.V().Has("Name", "ssdp-intf2", "Type", "internal")`)
			if err != nil {
				return err
			}

			within := fmt.Sprintf(`Within("%s", "%s")`, node1.Metadata()["TID"], node2.Metadata()["TID"])
			flows, err := gh.GetFlows(fmt.Sprintf(prefix+`.Flows().Has("ANodeTID", %s, "BNodeTID", %s)`, within, within))
			if err != nil {
				return err
			}

			if len(flows) == 0 {
				flows, _ = gh.GetFlows(fmt.Sprintf(prefix + `.Flows()`))
				gr := fmt.Sprintf(prefix+`.Flows().Has("ANodeTID", %s, "BNodeTID", %s)`, within, within)
				return fmt.Errorf("Unable to find flows with the expected path %s: %v", gr, flows)
			}

			return nil
		},
	}

	RunTest(t, test)
}

func TestFlowGremlin(t *testing.T) {
	test := &Test{
		setupCmds: []helper.Cmd{
			{"ovs-vsctl add-br br-fg", true},
			{"ovs-vsctl add-port br-fg fg-intf1 -- set interface fg-intf1 type=internal", true},
			{"ip address add 169.254.33.33/24 dev fg-intf1", true},
			{"ip link set fg-intf1 up", true},
		},

		setupFunction: func(c *TestContext) error {
			helper.ExecCmds(t, helper.Cmd{Cmd: "ping -c 5 -I fg-intf1 169.254.33.34", Check: false})
			return nil
		},

		tearDownCmds: []helper.Cmd{
			{"ovs-vsctl del-br br-fg", true},
		},

		captures: []TestCapture{
			{gremlin: `G.V().Has('Name', 'br-fg', 'Type', 'ovsbridge')`},
		},

		check: func(c *TestContext) error {
			prefix := "g"
			if !c.time.IsZero() {
				prefix += fmt.Sprintf(".Context(%d)", common.UnixMillis(c.time))
			}

			gh := c.gh
			node, err := gh.GetNode(prefix + `.V().Has("Name", "br-fg", "Type", "ovsbridge")`)
			if err != nil {
				return err
			}

			var count int64
			gh.Query(prefix+`.V().Has("Name", "br-fg", "Type", "ovsbridge").Count()`, &count)
			if count != 1 {
				return fmt.Errorf("Should return 1, got: %d", count)
			}

			tid, _ := node.GetFieldString("TID")
			if tid == "" {
				return errors.New("Node TID not Found")
			}

			flows, _ := gh.GetFlows(prefix + `.V().Has("Name", "br-fg", "Type", "ovsbridge").Flows().Has("NodeTID", "` + tid + `")`)
			if len(flows) == 0 {
				return fmt.Errorf("Should return at least 1 flow, got: %v", flows)
			}

			flowsOpt, _ := gh.GetFlows(prefix + `.V().Has("Name", "br-fg", "Type", "ovsbridge").Flows().Has("NodeTID", "` + tid + `")`)
			if len(flowsOpt) != len(flows) {
				return fmt.Errorf("Should return the same number of flows that without optimisation, got: %v", flowsOpt)
			}

			nodes, _ := gh.GetNodes(prefix + `.V().Has("Name", "br-fg", "Type", "ovsbridge").Flows().Has("NodeTID", "` + tid + `").Out()`)
			if len(nodes) != 0 {
				return fmt.Errorf("Should return no destination node, got %d", len(nodes))
			}

			nodes, _ = gh.GetNodes(prefix + `.V().Has("Name", "br-fg", "Type", "ovsbridge").Flows().Has("NodeTID", "` + tid + `").Both().Dedup()`)
			if len(nodes) != 1 {
				return fmt.Errorf("Should return one node, got %d", len(nodes))
			}

			nodes, _ = gh.GetNodes(prefix + `.V().Has("Name", "br-fg", "Type", "ovsbridge").Flows().Has("NodeTID", "` + tid + `").In().Dedup()`)
			if len(nodes) != 1 {
				return fmt.Errorf("Should return one source node, got %d", len(nodes))
			}

			gh.Query(prefix+`.V().Has("Name", "br-fg", "Type", "ovsbridge").Flows().Has("NodeTID", "`+tid+`").Count()`, &count)
			if int(count) != len(flows) {
				return fmt.Errorf("Gremlin count doesn't correspond to the number of flows, got: %v, expected: %v", len(flows), count)
			}

			return nil
		},
	}

	RunTest(t, test)
}

func queryFlowMetrics(gh *gclient.GremlinQueryHelper, bridge string, timeContext int64, pings int64) error {
	graphGremlin := "g"
	if timeContext != -1 {
		graphGremlin += fmt.Sprintf(".Context(%d)", timeContext)
	}

	ovsGremlin := graphGremlin + fmt.Sprintf(`.V().Has("Name", "%s", "Type", "ovsbridge")`, bridge)
	if _, err := gh.GetNode(ovsGremlin); err != nil {
		return err
	}

	gremlin := ovsGremlin + `.Flows().Has("LayersPath", Regex(".*ICMPv4.*"))`

	icmp, err := gh.GetFlows(gremlin)
	if err != nil {
		return err
	}

	switch len(icmp) {
	case 0:
		return errors.New("Should return one icmp flow, got none")
	case 1:
	default:
		return fmt.Errorf("Should return only one icmp flow, got: %v", icmp)
	}
	if icmp[0].LayersPath != "Ethernet/IPv4/ICMPv4/Payload" {
		return fmt.Errorf("Wrong layer path, should be 'Ethernet/IPv4/ICMPv4/Payload', got %s", icmp[0].LayersPath)
	}

	ethernet := icmp[0].Metric
	if ethernet.BAPackets != pings {
		return fmt.Errorf("Number of packets is wrong, got: %v", ethernet.BAPackets)
	}

	if ethernet.ABBytes < pings*1066 || ethernet.BABytes < pings*1066 {
		return fmt.Errorf("Number of bytes is wrong, got: %v", ethernet.BABytes)
	}

	flows, err := gh.GetFlows(fmt.Sprintf(`%s.Flows().Has("LayersPath", "Ethernet/IPv4/ICMPv4/Payload", "Metric.ABPackets", %d)`, ovsGremlin, pings))
	if len(flows) != 1 || flows[0].Metric.BAPackets != pings {
		return fmt.Errorf("Number of packets is wrong, got %d, flows: %v (error: %+v)", len(flows), flows, err)
	}

	ipv4 := icmp[0].Metric // FIXME double check protocol Network = IPv4/v6
	if flows[0].Metric.ABBytes < ipv4.ABBytes {
		return fmt.Errorf("Layers bytes error, got: %v", icmp)
	}

	pingLen := icmp[0].Metric.ABBytes
	metric, err := gh.GetFlowMetric(gremlin + fmt.Sprintf(`.Has("Metric.ABBytes", Gt(%d))`, pingLen-1))
	if err != nil || metric.ABBytes < pingLen {
		return fmt.Errorf("Number of bytes is wrong, got: %v (error: %+v)", metric, err)
	}

	metric, err = gh.GetFlowMetric(gremlin + fmt.Sprintf(`.Has("Metric.ABBytes", Gt(%d))`, pingLen))
	if err != gclient.NotFound {
		return fmt.Errorf("Wrong number of flow, should have none, got : %v", metric)
	}

	metric, err = gh.GetFlowMetric(gremlin + fmt.Sprintf(`.Has("Metric.ABBytes", Gte(%d))`, pingLen))
	if err != nil || metric == nil || metric.ABBytes < pingLen {
		return fmt.Errorf("Number of bytes is wrong, got: %v", metric)
	}

	metric, err = gh.GetFlowMetric(gremlin + fmt.Sprintf(`.Has("Metric.ABBytes", Gte(%d))`, pingLen+1))
	if err != gclient.NotFound {
		return fmt.Errorf("Wrong number of flow, should have none, got : %v", metric)
	}

	metric, err = gh.GetFlowMetric(gremlin + fmt.Sprintf(`.Has("Metric.ABBytes", Lt(%d))`, pingLen+1))
	if err != nil || metric.ABBytes > pingLen {
		return fmt.Errorf("Number of bytes is wrong, got: %v", metric)
	}

	metric, err = gh.GetFlowMetric(gremlin + fmt.Sprintf(`.Has("Metric.ABBytes", Lt(%d))`, pingLen))
	if err != gclient.NotFound {
		return fmt.Errorf("Wrong number of flow, should have none, got : %v", metric)
	}

	metric, err = gh.GetFlowMetric(gremlin + fmt.Sprintf(`.Has("Metric.ABBytes", Lte(%d))`, pingLen))
	if err != nil || metric == nil || metric.ABBytes > pingLen {
		return fmt.Errorf("Number of bytes is wrong, got: %v", metric)
	}

	metric, err = gh.GetFlowMetric(gremlin + fmt.Sprintf(`.Has("Metric.ABBytes", Lte(%d))`, pingLen-1))
	if err != gclient.NotFound {
		return fmt.Errorf("Wrong number of flow, should have none, got : %v", metric)
	}

	metric, err = gh.GetFlowMetric(gremlin + fmt.Sprintf(`.Has("Metric.ABBytes", Inside(%d, %d))`, pingLen-1, pingLen+1))
	if err != nil || metric == nil || metric.ABBytes <= pingLen-1 || metric.ABBytes >= pingLen+1 {
		return fmt.Errorf("Number of bytes is wrong, got: %v", metric)
	}

	metric, err = gh.GetFlowMetric(gremlin + fmt.Sprintf(`.Has("Metric.ABBytes", Inside(%d, %d))`, pingLen, pingLen+1))
	if err != gclient.NotFound {
		return fmt.Errorf("Wrong number of flow, should have none, got : %v", metric)
	}

	metric, err = gh.GetFlowMetric(gremlin + fmt.Sprintf(`.Has("Metric.ABBytes", Between(%d, %d))`, pingLen, pingLen+1))
	if metric == nil || metric.ABBytes <= pingLen-1 || metric.ABBytes >= pingLen+1 {
		return fmt.Errorf("Number of bytes is wrong, got: %v", metric)
	}

	metric, err = gh.GetFlowMetric(gremlin + fmt.Sprintf(`.Has("Metric.ABBytes", Between(%d, %d))`, pingLen, pingLen))
	if err != gclient.NotFound {
		return fmt.Errorf("Wrong number of flow, should have none, got : %v", metric)
	}

	return nil
}

func TestFlowMetrics(t *testing.T) {
	test := &Test{
		setupCmds: []helper.Cmd{
			{"ovs-vsctl add-br br-fm", true},

			{"ovs-vsctl add-port br-fm fm-intf1 -- set interface fm-intf1 type=internal", true},
			{"ip netns add fm-vm1", true},
			{"ip link set fm-intf1 netns fm-vm1", true},
			{"ip netns exec fm-vm1 ip address add 169.254.33.33/24 dev fm-intf1", true},
			{"ip netns exec fm-vm1 ip link set fm-intf1 up", true},

			{"ovs-vsctl add-port br-fm fm-intf2 -- set interface fm-intf2 type=internal", true},
			{"ip netns add fm-vm2", true},
			{"ip link set fm-intf2 netns fm-vm2", true},
			{"ip netns exec fm-vm2 ip address add 169.254.33.34/24 dev fm-intf2", true},
			{"ip netns exec fm-vm2 ip link set fm-intf2 up", true},

			// wait to have everything ready, sflow, interfaces
			{"sleep 2", false},
		},

		setupFunction: func(c *TestContext) error {
			helper.ExecCmds(t, helper.Cmd{Cmd: "ip netns exec fm-vm1 ping -c 1 -s 1024 -I fm-intf1 169.254.33.34", Check: false})
			return nil
		},

		tearDownCmds: []helper.Cmd{
			{"ip netns del fm-vm1", true},
			{"ip netns del fm-vm2", true},
			{"ovs-vsctl del-br br-fm", true},
		},

		captures: []TestCapture{
			{gremlin: `G.V().Has('Name', 'br-fm', 'Type', 'ovsbridge')`},
		},

		check: func(c *TestContext) error {
			t := int64(-1)
			if !c.time.IsZero() {
				t = common.UnixMillis(c.time)
			}

			return queryFlowMetrics(c.gh, "br-fm", t, 1)
		},
	}

	RunTest(t, test)
}

func TestFlowMetricsStep(t *testing.T) {
	test := &Test{
		mode: OneShot,
		setupCmds: []helper.Cmd{
			{"ovs-vsctl add-br br-fms", true},

			{"ovs-vsctl add-port br-fms fms-intf1 -- set interface fms-intf1 type=internal", true},
			{"ip netns add fms-vm1", true},
			{"ip link set fms-intf1 netns fms-vm1", true},
			{"ip netns exec fms-vm1 ip address add 169.254.33.33/24 dev fms-intf1", true},
			{"ip netns exec fms-vm1 ip link set fms-intf1 up", true},

			{"ovs-vsctl add-port br-fms fms-intf2 -- set interface fms-intf2 type=internal", true},
			{"ip netns add fms-vm2", true},
			{"ip link set fms-intf2 netns fms-vm2", true},
			{"ip netns exec fms-vm2 ip address add 169.254.33.34/24 dev fms-intf2", true},
			{"ip netns exec fms-vm2 ip link set fms-intf2 up", true},
		},

		setupFunction: func(c *TestContext) error {
			helper.ExecCmds(t,
				helper.Cmd{Cmd: "ip netns exec fms-vm1 ping -c 15 -s 1024 -I fms-intf1 169.254.33.34", Check: false},
				helper.Cmd{Cmd: "sleep 15", Check: false},
			)
			return nil
		},

		tearDownCmds: []helper.Cmd{
			{"ip netns del fms-vm1", true},
			{"ip netns del fms-vm2", true},
			{"ovs-vsctl del-br br-fms", true},
		},

		captures: []TestCapture{
			{gremlin: `G.V().Has('Name', 'br-fms', 'Type', 'ovsbridge')`},
		},

		check: func(c *TestContext) error {
			gh := c.gh
			gremlin := fmt.Sprintf("g.Context(%d, %d)", common.UnixMillis(c.startTime), c.startTime.Unix()-c.setupTime.Unix()+5)
			gremlin += `.V().Has("Name", "br-fms", "Type", "ovsbridge").Flows()`

			tm, err := gh.GetMetric(gremlin + `.Has("LayersPath", "Ethernet/IPv4/ICMPv4/Payload").Dedup().Metrics().Sum()`)
			if err != nil {
				flows, _ := gh.GetFlows(gremlin)
				return fmt.Errorf("Could not find metrics (%+v) for flows %s", tm, helper.FlowsToString(flows))
			}
			metric := tm.Metric.(*flow.FlowMetric)

			if metric.ABPackets != 15 || metric.BAPackets != 15 || metric.ABBytes < 15360 || metric.BABytes < 15360 {
				flows, _ := gh.GetFlows(gremlin)
				return fmt.Errorf("Wrong metric returned, got : %+v for flows %+v, request: %s", metric, helper.FlowsToString(flows), gremlin+`.Has("LayersPath", "Ethernet/IPv4/ICMPv4/Payload").Dedup().Metrics().Sum()`)
			}

			checkMetrics := func(metrics map[string][]*common.TimedMetric) error {
				if len(metrics) != 1 {
					return fmt.Errorf("Should return only one metric array (%+v)", metrics)
				}

				// check it's sorted
				var start int64
				for _, metricsOfID := range metrics {
					if len(metricsOfID) <= 1 {
						return fmt.Errorf("metric array should have more that 1 element (%+v)", metricsOfID)
					}

					for _, tm := range metricsOfID {
						if tm.Start < start {
							return fmt.Errorf("Metrics not correctly sorted (%+v)", metricsOfID)
						}
						start = tm.Start
					}
				}

				return nil
			}

			metrics, err := gh.GetMetrics(gremlin + `.Has("LayersPath", "Ethernet/IPv4/ICMPv4/Payload").Dedup().Metrics()`)
			if err != nil || len(metrics) == 0 {
				return fmt.Errorf("Could not find metrics (%+v)", metrics)
			}

			if err = checkMetrics(metrics); err != nil {
				return err
			}

			metrics, err = gh.GetMetrics(gremlin + `.Has("LayersPath", "Ethernet/IPv4/ICMPv4/Payload").Dedup().Metrics().Aggregates()`)
			if err != nil || len(metrics) == 0 {
				return fmt.Errorf("Could not find metrics (%+v)", metrics)
			}

			if err = checkMetrics(metrics); err != nil {
				return err
			}

			return nil
		},
	}

	RunTest(t, test)
}

func TestFlowHops(t *testing.T) {
	test := &Test{
		setupCmds: []helper.Cmd{
			{"ovs-vsctl add-br br-fh", true},

			{"ovs-vsctl add-port br-fh fh-intf1 -- set interface fh-intf1 type=internal", true},
			{"ip netns add fh-vm1", true},
			{"ip link set fh-intf1 netns fh-vm1", true},
			{"ip netns exec fh-vm1 ip address add 169.254.33.33/24 dev fh-intf1", true},
			{"ip netns exec fh-vm1 ip link set fh-intf1 up", true},

			{"ovs-vsctl add-port br-fh fh-intf2 -- set interface fh-intf2 type=internal", true},
			{"ip netns add fh-vm2", true},
			{"ip link set fh-intf2 netns fh-vm2", true},
			{"ip netns exec fh-vm2 ip address add 169.254.33.34/24 dev fh-intf2", true},
			{"ip netns exec fh-vm2 ip link set fh-intf2 up", true},

			// wait to have everything ready, sflow, interfaces
			{"sleep 2", false},
		},

		setupFunction: func(c *TestContext) error {
			helper.ExecCmds(t, helper.Cmd{Cmd: "ip netns exec fh-vm1 ping -c 1 -s 1024 -I fh-intf1 169.254.33.34", Check: false})
			return nil
		},

		tearDownCmds: []helper.Cmd{
			{"ip netns del fh-vm1", true},
			{"ip netns del fh-vm2", true},
			{"ovs-vsctl del-br br-fh", true},
		},

		captures: []TestCapture{
			{gremlin: `G.V().Has('Name', 'br-fh', 'Type', 'ovsbridge')`},
		},

		// since the agent update ticker is about 10 sec according to the configuration
		// we should wait 11 sec to have the first update and the MetricRange filled
		check: func(c *TestContext) error {
			prefix := "g"
			if !c.time.IsZero() {
				prefix += fmt.Sprintf(".Context(%d)", common.UnixMillis(c.time))
			}
			prefix += `.V().Has("Name", "br-fh", "Type", "ovsbridge")`

			gh := c.gh
			gremlin := prefix + `.Flows().Has("LayersPath", "Ethernet/IPv4/ICMPv4/Payload")`
			flows, err := gh.GetFlows(gremlin)
			if err != nil {
				return err
			}

			if len(flows) != 1 {
				return errors.New("We should receive only one ICMPv4 flow")
			}

			gremlin = fmt.Sprintf(prefix+`.Flows().Has("TrackingID", "%s").Nodes()`, flows[0].TrackingID)
			tnodes, err := gh.GetNodes(gremlin)
			if err != nil {
				return err
			}

			if len(tnodes) != 3 {
				return errors.New("We should have 3 nodes NodeTID,A,B")
			}

			gremlin = prefix + `.Flows().Has("LayersPath", "Ethernet/IPv4/ICMPv4/Payload").Hops()`
			nodes, err := gh.GetNodes(gremlin)
			if err != nil {
				return err
			}

			if len(nodes) != 1 {
				return errors.New("We should have 1 node NodeTID")
			}

			found := false
			m := nodes[0].Metadata()
			for _, n := range tnodes {
				if n.MatchMetadata(m) == true {
					found = true
					break
				}
			}
			if !found {
				return errors.New("We should found the Hops nodes in the TrackingID nodes")
			}

			return nil
		},
	}

	RunTest(t, test)
}

func TestIPv6FlowHopsIPv6(t *testing.T) {
	if !common.IPv6Supported() {
		t.Skipf("Platform doesn't support IPv6")
	}

	test := &Test{
		setupCmds: []helper.Cmd{
			{"ovs-vsctl add-br br-ipv6fh", true},

			{"ovs-vsctl add-port br-ipv6fh ipv6fh-intf1 -- set interface ipv6fh-intf1 type=internal", true},
			{"ip netns add ipv6fh-vm1", true},
			{"ip link set ipv6fh-intf1 netns ipv6fh-vm1", true},
			{"ip netns exec ipv6fh-vm1 ip address add fd49:37c8:5229::1/48 dev ipv6fh-intf1", true},
			{"ip netns exec ipv6fh-vm1 ip link set ipv6fh-intf1 up", true},

			{"ovs-vsctl add-port br-ipv6fh ipv6fh-intf2 -- set interface ipv6fh-intf2 type=internal", true},
			{"ip netns add ipv6fh-vm2", true},
			{"ip link set ipv6fh-intf2 netns ipv6fh-vm2", true},
			{"ip netns exec ipv6fh-vm2 ip address add fd49:37c8:5229::2/48 dev ipv6fh-intf2", true},
			{"ip netns exec ipv6fh-vm2 ip link set ipv6fh-intf2 up", true},

			// wait to have everything ready, sflow, interfaces
			{"sleep 2", false},
		},

		setupFunction: func(c *TestContext) error {
			helper.ExecCmds(t, helper.Cmd{Cmd: "ip netns exec ipv6fh-vm1 ping6 -c 5 -s 1024 -I ipv6fh-intf1 fd49:37c8:5229::2", Check: false})
			return nil
		},

		tearDownCmds: []helper.Cmd{
			{"ip netns del ipv6fh-vm1", true},
			{"ip netns del ipv6fh-vm2", true},
			{"ovs-vsctl del-br br-ipv6fh", true},
		},

		captures: []TestCapture{
			{gremlin: `G.V().Has('Name', 'br-ipv6fh', 'Type', 'ovsbridge')`},
		},

		// since the agent update ticker is about 10 sec according to the configuration
		// we should wait 11 sec to have the first update and the MetricRange filled
		check: func(c *TestContext) error {
			prefix := "g"
			if !c.time.IsZero() {
				prefix += fmt.Sprintf(".Context(%d)", common.UnixMillis(c.time))
			}
			prefix += `.V().Has("Name", "br-ipv6fh", "Type", "ovsbridge")`

			gh := c.gh
			gremlin := prefix + `.Flows().Has("LayersPath", "Ethernet/IPv6/ICMPv6/Payload")`
			// filterIPv6AddrAnd() as we received multicast/broadcast from fresh registered interfaces announcement
			allFlows, err := gh.GetFlows(gremlin)
			if err != nil {
				return err
			}

			flows := helper.FilterIPv6AddrAnd(allFlows, "fd49:37c8:5229::1", "fd49:37c8:5229::2")
			if len(flows) != 1 {
				return errors.New("We should receive only one ICMPv6 flow")
			}

			gremlin = fmt.Sprintf(prefix+`.Flows().Has("TrackingID", "%s").Nodes()`, flows[0].TrackingID)
			tnodes, err := gh.GetNodes(gremlin)
			if err != nil {
				return err
			}

			if len(tnodes) != 3 {
				return fmt.Errorf("We should have 3 nodes NodeTID,A,B, got %+v", tnodes)
			}

			/* Dedup() here for same reason than above ^^^ */
			gremlin = prefix + `.Flows().Has("LayersPath", "Ethernet/IPv6/ICMPv6/Payload").Hops().Dedup()`
			nodes, err := gh.GetNodes(gremlin)
			if err != nil {
				return err
			}

			if len(nodes) != 1 {
				return errors.New("We should have 1 node NodeTID")
			}

			found := false
			m := nodes[0].Metadata()
			for _, n := range tnodes {
				if n.MatchMetadata(m) == true {
					found = true
					break
				}
			}
			if !found {
				return errors.New("We should found the Hops nodes in the TrackingID nodes")
			}

			return nil
		},
	}

	RunTest(t, test)
}

func TestFlowGRETunnel(t *testing.T) {
	testFlowTunnel(t, "br-fgt", "gre", false, "192.168.0.1", "192.168.0.2",
		"172.16.0.1", "172.16.0.2", "192.168.0.0/24")
}

func TestFlowVxlanTunnel(t *testing.T) {
	testFlowTunnel(t, "br-fvt", "vxlan", false, "192.168.0.1", "192.168.0.2",
		"172.16.0.1", "172.16.0.2", "192.168.0.0/24")
}

func TestIPv6FlowGRETunnelIPv6(t *testing.T) {
	t.Skip("Fedora seems didn't support ip6gre tunnel for the moment")

	if !common.IPv6Supported() {
		t.Skipf("Platform doesn't support IPv6")
	}

	testFlowTunnel(t, "br-fgtv6", "gre", true, "fdfe:38b:489c::1", "fdfe:38b:489c::2",
		"fd49:37c8:5229::1", "fd49:37c8:5229::2", "fdfe:38b:489c::/48")
}

func testFlowTunnel(t *testing.T, bridge string, tunnelType string, ipv6 bool, IP1, IP2, tunnelIP1, tunnelIP2, addrRange string) {
	var (
		tunnel1Add   string
		tunnel2Add   string
		greMode      string
		icmpVersion  string
		prefix       string
		tunnelPrefix string
	)

	if ipv6 {
		greMode = "ip6gre"
		icmpVersion = "ICMPv6"
		prefix = "/128"
		tunnelPrefix = "/48"
	} else {
		greMode = "gre"
		icmpVersion = "ICMPv4"
		prefix = "/32"
		tunnelPrefix = "/24"
	}

	if tunnelType == "gre" {
		tunnel1Add = fmt.Sprintf("sudo ip netns exec tunnel-vm1 ip tunnel add tunnel mode %s remote %s local %s ttl 255", greMode, tunnelIP2, tunnelIP1)
		tunnel2Add = fmt.Sprintf("sudo ip netns exec tunnel-vm2 ip tunnel add tunnel mode %s remote %s local %s ttl 255", greMode, tunnelIP1, tunnelIP2)
	} else {
		tunnel1Add = fmt.Sprintf("sudo ip netns exec tunnel-vm1 ip link add tunnel type vxlan id 10 remote %s local %s ttl 255 dev eth0 dstport 4789", tunnelIP2, tunnelIP1)
		tunnel2Add = fmt.Sprintf("sudo ip netns exec tunnel-vm2 ip link add tunnel type vxlan id 10 remote %s local %s ttl 255 dev eth0 dstport 4789", tunnelIP1, tunnelIP2)
	}

	test := &Test{
		setupCmds: []helper.Cmd{
			{"sudo ovs-vsctl add-br " + bridge, true},

			{"sudo ip netns add tunnel-vm1", true},
			{"sudo ip link add tunnel-vm1-eth0 type veth peer name eth0 netns tunnel-vm1", true},
			{"sudo ip link set tunnel-vm1-eth0 up", true},

			{"sudo ip netns exec tunnel-vm1 ip link set eth0 up", true},
			{fmt.Sprintf("sudo ip netns exec tunnel-vm1 ip address add %s%s dev eth0", tunnelIP1, tunnelPrefix), true},

			{"sudo ip netns add tunnel-vm2", true},
			{"sudo ip link add tunnel-vm2-eth0 type veth peer name eth0 netns tunnel-vm2", true},
			{"sudo ip link set tunnel-vm2-eth0 up", true},
			{"sudo ip netns exec tunnel-vm2 ip link set eth0 up", true},
			{fmt.Sprintf("sudo ip netns exec tunnel-vm2 ip address add %s%s dev eth0", tunnelIP2, tunnelPrefix), true},

			{fmt.Sprintf("sudo ovs-vsctl add-port %s tunnel-vm1-eth0", bridge), true},
			{fmt.Sprintf("sudo ovs-vsctl add-port %s tunnel-vm2-eth0", bridge), true},

			{tunnel1Add, true},
			{"sudo ip netns exec tunnel-vm1 ip link set tunnel up", true},
			{"sudo ip netns exec tunnel-vm1 ip link add name dummy0 type dummy", true},
			{"sudo ip netns exec tunnel-vm1 ip link set dummy0 up", true},
			{fmt.Sprintf("sudo ip netns exec tunnel-vm1 ip a add %s%s dev dummy0", IP1, prefix), true},
			{fmt.Sprintf("sudo ip netns exec tunnel-vm1 ip r add %s dev tunnel", addrRange), true},

			{tunnel2Add, true},
			{"sudo ip netns exec tunnel-vm2 ip link set tunnel up", true},
			{"sudo ip netns exec tunnel-vm2 ip link add name dummy0 type dummy", true},
			{"sudo ip netns exec tunnel-vm2 ip link set dummy0 up", true},
			{fmt.Sprintf("sudo ip netns exec tunnel-vm2 ip a add %s%s dev dummy0", IP2, prefix), true},
			{fmt.Sprintf("sudo ip netns exec tunnel-vm2 ip r add %s dev tunnel", addrRange), true},
		},

		tearDownCmds: []helper.Cmd{
			{"ip netns del tunnel-vm1", true},
			{"ip netns del tunnel-vm2", true},
			{"ovs-vsctl del-br " + bridge, true},
		},

		captures: []TestCapture{
			{gremlin: `G.V().Has('Name', 'tunnel-vm1').Out().Has('Name', 'tunnel')`},
			{gremlin: `G.V().Has('Name', 'tunnel-vm2-eth0')`},
		},

		setupFunction: func(c *TestContext) error {
			helper.ExecCmds(t, helper.Cmd{Cmd: fmt.Sprintf("ip netns exec tunnel-vm1 ping -c 5 -I %s %s", IP1, IP2), Check: false})
			return nil
		},

		check: func(c *TestContext) error {
			prefix := "g"
			if !c.time.IsZero() {
				prefix += fmt.Sprintf(".Context(%d)", common.UnixMillis(c.time))
			}

			gh := c.gh
			nodes, err := gh.GetNodes(prefix + `.V().Has('Name', 'tunnel-vm1').Out().Has('Name', 'tunnel')`)
			if err != nil {
				return err
			}

			if len(nodes) == 0 {
				return errors.New("Found no node")
			}

			node := nodes[0]
			tid, ok := node.Metadata()["TID"]
			if !ok {
				return fmt.Errorf("Node %s has no TID", node.ID)
			}

			flowsInnerTunnel, err := gh.GetFlows(prefix + fmt.Sprintf(`.Flows().Has("NodeTID", "%s", "Application", "%s")`, tid.(string), icmpVersion))
			if err != nil {
				return err
			}

			if len(flowsInnerTunnel) != 1 {
				return fmt.Errorf("We should have only one %s flow in the tunnel %v", icmpVersion, helper.FlowsToString(flowsInnerTunnel))
			}

			trackingID := flowsInnerTunnel[0].TrackingID
			flowsBridge, err := gh.GetFlows(prefix + fmt.Sprintf(`.V().Has('Name', 'tunnel-vm2-eth0').Flows().Has('TrackingID', '%s', 'Application', '%s').Dedup()`, trackingID, icmpVersion))
			if err != nil {
				return err
			}

			if len(flowsBridge) == 0 {
				return fmt.Errorf("TrackingID not found in %s tunnel: leaving the interface(%v) == seen in the tunnel(%v)", tunnelType, helper.FlowsToString(flowsInnerTunnel), helper.FlowsToString(flowsBridge))
			}

			return nil
		},
	}

	RunTest(t, test)
}

func TestReplayCapture(t *testing.T) {
	test := &Test{
		setupCmds: []helper.Cmd{
			{"ovs-vsctl add-br br-rc", true},
		},

		setupFunction: func(c *TestContext) error {
			capture := c.captures[0]
			err := common.Retry(func() error {
				// Wait for the capture to be created and the PCAPSocket attribute to be set
				c.client.Get("capture", capture.UUID, capture)
				if capture.PCAPSocket == "" {
					return fmt.Errorf("Failed to retrieve PCAP socket for capture %s", capture.UUID)
				}

				return nil
			}, 5, time.Second)

			if err != nil {
				return err
			}

			return helper.SendPCAPFile("pcaptraces/eth-ip4-arp-dns-req-http-google.pcap", capture.PCAPSocket)
		},

		tearDownCmds: []helper.Cmd{
			{"ovs-vsctl del-br br-rc", true},
		},

		captures: []TestCapture{
			{gremlin: `G.V().Has('Name', 'br-rc', 'Type', 'ovsbridge')`, kind: "pcapsocket"},
		},

		check: func(c *TestContext) error {
			prefix := "g"
			if !c.time.IsZero() {
				prefix += fmt.Sprintf(".Context(%d)", common.UnixMillis(c.time))
			}

			gh := c.gh
			gremlin := prefix + ".V().Has('Name', 'br-rc', 'Type', 'ovsbridge')"
			node, err := gh.GetNode(gremlin)
			if err != nil {
				return err
			}

			gremlin = fmt.Sprintf(prefix+".Flows().Has('NodeTID', '%s')", node.Metadata()["TID"].(string))
			flows, err := gh.GetFlows(gremlin)
			if err != nil {
				return err
			}

			if len(flows) != 5 {
				return fmt.Errorf("Wrong number of flows. Expected 5, got %d", len(flows))
			}

			flows, err = gh.GetFlows(gremlin + ".Has('Application', 'DNS')")
			if err != nil {
				return err
			}

			if len(flows) != 2 {
				return fmt.Errorf("Wrong number of DNS flows. Expected 2, got %d", len(flows))
			}

			return nil
		},
	}

	RunTest(t, test)
}

func TestPcapInject(t *testing.T) {
	test := &Test{
		mode: OneShot,
		setupFunction: func(c *TestContext) error {
			file, err := os.Open("pcaptraces/eth-ip4-arp-dns-req-http-google.pcap")
			if err != nil {
				return err
			}
			defer file.Close()

			resp, err := c.client.Request("POST", "api/pcap", file)
			if err != nil {
				return err
			}

			if resp.StatusCode != 200 {
				return fmt.Errorf("Should get 200 status code, got %d", resp.StatusCode)
			}

			return nil
		},
		check: func(c *TestContext) error {
			flows, _ := c.gh.GetFlows(`G.Context(1454659514).Flows().Has('Application', 'DNS')`)
			if len(flows) != 2 {
				return fmt.Errorf("Wrong number of DNS flows. Expected 2, got %d", len(flows))
			}
			return nil
		},
	}

	RunTest(t, test)
}

func TestFlowVLANSegmentation(t *testing.T) {
	test := &Test{
		setupCmds: []helper.Cmd{
			{"sudo ovs-vsctl add-br br-vlan", true},

			{"sudo ip netns add vlan-vm1", true},
			{"sudo ip link add vlan-vm1-eth0 type veth peer name eth0 netns vlan-vm1", true},
			{"sudo ip link set vlan-vm1-eth0 up", true},

			{"sudo ip netns exec vlan-vm1 ip link set eth0 up", true},
			{"sudo ip netns exec vlan-vm1 ip link add link eth0 name vlan type vlan id 8", true},
			{"sudo ip netns exec vlan-vm1 ip address add 172.16.0.1/24 dev vlan", true},

			{"sudo ip netns add vlan-vm2", true},
			{"sudo ip link add vlan-vm2-eth0 type veth peer name eth0 netns vlan-vm2", true},
			{"sudo ip link set vlan-vm2-eth0 up", true},
			{"sudo ip netns exec vlan-vm2 ip link set eth0 up", true},
			{"sudo ip netns exec vlan-vm2 ip link add link eth0 name vlan type vlan id 8", true},
			{"sudo ip netns exec vlan-vm2 ip address add 172.16.0.2/24 dev vlan", true},

			{"sudo ovs-vsctl add-port br-vlan vlan-vm1-eth0", true},
			{"sudo ovs-vsctl add-port br-vlan vlan-vm2-eth0", true},

			{"sudo ip netns exec vlan-vm1 ip l set vlan up", true},

			{"sudo ip netns exec vlan-vm2 ip l set vlan up", true},
		},

		setupFunction: func(c *TestContext) error {
			helper.ExecCmds(t, helper.Cmd{Cmd: "ip netns exec vlan-vm1 ping -c 5 172.16.0.2", Check: true})
			return nil
		},

		tearDownCmds: []helper.Cmd{
			{"ip netns del vlan-vm1", true},
			{"ip netns del vlan-vm2", true},
			{"ovs-vsctl del-br br-vlan", true},
		},

		captures: []TestCapture{
			{gremlin: `G.V().Has('Name', 'vlan-vm1').Out().Has('Name', 'vlan')`},
			{gremlin: `G.V().Has('Name', 'vlan-vm2-eth0')`},
		},

		check: func(c *TestContext) error {
			prefix := "g"
			if !c.time.IsZero() {
				prefix += fmt.Sprintf(".Context(%d)", common.UnixMillis(c.time))
			}

			gh := c.gh
			flowsInnerTunnel, err := gh.GetFlows(prefix + `.V().Has('Name', 'vlan-vm1').Out().Has('Name', 'vlan').Flows().Has('LayersPath', 'Ethernet/IPv4/ICMPv4/Payload')`)
			if err != nil {
				return err
			}

			if len(flowsInnerTunnel) != 1 {
				return fmt.Errorf("We should have only one ICMPv4 flow in the tunnel %v", helper.FlowsToString(flowsInnerTunnel))
			}

			l3TrackingID := flowsInnerTunnel[0].L3TrackingID
			flowsBridge, err := gh.GetFlows(prefix + fmt.Sprintf(`.V().Has('Name', 'vlan-vm2-eth0').Flows().Has("L3TrackingID", "%s")`, l3TrackingID))
			if err != nil {
				return err
			}

			if len(flowsBridge) == 0 {
				return fmt.Errorf("L3TrackingID not found in VLANs: %v == %v", flowsInnerTunnel, flowsBridge)
			}

			return nil
		},
	}

	RunTest(t, test)
}

func TestSort(t *testing.T) {
	test := &Test{
		setupCmds: []helper.Cmd{
			{"ovs-vsctl add-br br-int", true},

			{"ip netns add dst-vm", true},
			{"ip link add dst-vm-eth0 type veth peer name dst-eth0 netns dst-vm", true},
			{"ip link set dst-vm-eth0 up", true},
			{"ip netns exec dst-vm ip link set dst-eth0 up", true},
			{"ip netns exec dst-vm ip address add 169.254.34.33/24 dev dst-eth0", true},

			{"ip netns add src-vm1", true},
			{"ip link add src-vm1-eth0 type veth peer name src1-eth0 netns src-vm1", true},
			{"ip link set src-vm1-eth0 up", true},
			{"ip netns exec src-vm1 ip link set src1-eth0 up", true},
			{"ip netns exec src-vm1 ip address add 169.254.34.34/24 dev src1-eth0", true},

			{"ip netns add src-vm2", true},
			{"ip link add src-vm2-eth0 type veth peer name src2-eth0 netns src-vm2", true},
			{"ip link set src-vm2-eth0 up", true},
			{"ip netns exec src-vm2 ip link set src2-eth0 up", true},
			{"ip netns exec src-vm2 ip address add 169.254.34.35/24 dev src2-eth0", true},

			{"ovs-vsctl add-port br-int dst-vm-eth0", true},
			{"ovs-vsctl add-port br-int src-vm1-eth0", true},
			{"ovs-vsctl add-port br-int src-vm2-eth0", true},
		},

		setupFunction: func(c *TestContext) (err error) {
			if err = ping(t, c, "G.V().Has('Name', 'src1-eth0')", "G.V().Has('Name', 'dst-eth0')", 10); err != nil {
				return
			}

			if err = ping(t, c, "G.V().Has('Name', 'src2-eth0')", "G.V().Has('Name', 'dst-eth0')", 20); err != nil {
				return
			}

			return
		},

		tearDownCmds: []helper.Cmd{
			{"ovs-vsctl del-br br-int", true},
			{"ip link del dst-vm-eth0", true},
			{"ip link del src-vm1-eth0", true},
			{"ip link del src-vm2-eth0", true},
			{"ip netns del dst-vm", true},
			{"ip netns del src-vm1", true},
			{"ip netns del src-vm2", true},
		},

		captures: []TestCapture{
			{gremlin: `G.V().Has('Name', 'dst-eth0', 'Type', 'veth')`},
		},

		check: func(c *TestContext) error {
			g := "g"
			if !c.time.IsZero() {
				g += fmt.Sprintf(".Context(%d, %d)", common.UnixMillis(c.time), 60)
			}
			gremlin := g + ".Flows().Has('Network', '169.254.34.33').Sort()"

			flows, err := c.gh.GetFlows(gremlin)
			if err != nil {
				return err
			}
			if len(flows) != 2 {
				flows, _ = c.gh.GetFlows(g + ".Flows()")
				return fmt.Errorf("Expected two flow, got %+v", flows)
			}
			//check is it in ascending order by Last field
			if flows[0].Last > flows[1].Last {
				return fmt.Errorf("Flows not in expected order, expected ASC got DESC")
			}

			gremlin = g + ".Flows().Has('Network', '169.254.34.33').Sort(DESC, 'Start')"

			flows, err = c.gh.GetFlows(gremlin)
			if err != nil {
				return err
			}
			if len(flows) != 2 {
				flows, _ = c.gh.GetFlows(g + ".Flows()")
				return fmt.Errorf("Expected two flow, got %+v", flows)
			}
			//check is it in descending order by Start field
			if flows[0].Start < flows[1].Start {
				return fmt.Errorf("Flows not in expected order, expected DESC got ASC")
			}
			return nil
		},
	}
	RunTest(t, test)
}

func TestFlowSumStep(t *testing.T) {
	test := &Test{
		setupCmds: []helper.Cmd{
			{"ovs-vsctl add-br br-sum", true},

			{"ip netns add vm1", true},
			{"ip link add vm1-eth0 type veth peer name intf1 netns vm1", true},
			{"ip link set vm1-eth0 up", true},
			{"ip netns exec vm1 ip link set intf1 up", true},
			{"ip netns exec vm1 ip address add 169.254.34.33/24 dev intf1", true},

			{"ip netns add vm2", true},
			{"ip link add vm2-eth0 type veth peer name intf2 netns vm2", true},
			{"ip link set vm2-eth0 up", true},
			{"ip netns exec vm2 ip link set intf2 up", true},
			{"ip netns exec vm2 ip address add 169.254.34.34/24 dev intf2", true},

			{"ip netns add vm3", true},
			{"ip link add vm3-eth0 type veth peer name intf3 netns vm3", true},
			{"ip link set vm3-eth0 up", true},
			{"ip netns exec vm3 ip link set intf3 up", true},
			{"ip netns exec vm3 ip address add 169.254.34.35/24 dev intf3", true},

			{"ovs-vsctl add-port br-sum vm1-eth0", true},
			{"ovs-vsctl add-port br-sum vm2-eth0", true},
			{"ovs-vsctl add-port br-sum vm3-eth0", true},
		},

		setupFunction: func(c *TestContext) (err error) {
			if err = ping(t, c, "G.V().Has('Name', 'intf1')", "G.V().Has('Name', 'intf2')", 3); err != nil {
				return
			}
			if err = ping(t, c, "G.V().Has('Name', 'intf2')", "G.V().Has('Name', 'intf3')", 4); err != nil {
				return
			}
			if err = ping(t, c, "G.V().Has('Name', 'intf1')", "G.V().Has('Name', 'intf3')", 3); err != nil {
				return
			}
			return
		},

		tearDownCmds: []helper.Cmd{
			{"ovs-vsctl del-br br-sum", true},
			{"ip link del vm1-eth0", true},
			{"ip link del vm2-eth0", true},
			{"ip link del vm3-eth0", true},
			{"ip netns del vm1", true},
			{"ip netns del vm2", true},
			{"ip netns del vm3", true},
		},

		captures: []TestCapture{
			{gremlin: `G.V().Has('Name', 'br-sum', 'Type', 'ovsbridge')`},
		},

		check: func(c *TestContext) error {
			gh := c.gh
			gremlin := fmt.Sprintf("g.Context(%d, %d)", common.UnixMillis(c.startTime), c.startTime.Unix()-c.setupTime.Unix()+5)
			gremlin += `.V().Has("Name", "br-sum", "Type", "ovsbridge").Flows().Has("LayersPath", "Ethernet/IPv4/ICMPv4").dedup().sum("Metric.ABPackets")`

			var s interface{}
			if err := gh.Query(gremlin, &s); err != nil {
				return fmt.Errorf("Error while retriving SUM: %v", err)
			}
			sum, _ := s.(json.Number).Int64()
			if sum != 10 {
				return fmt.Errorf("Got wrong sum value, Expected 10 got %v", sum)
			}
			return nil
		},
	}

	RunTest(t, test)
}
