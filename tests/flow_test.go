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
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/skydive-project/skydive/flow/probes"

	"github.com/google/gopacket"
	"github.com/google/gopacket/pcapgo"
	"github.com/skydive-project/skydive/api/client"
	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/flow"
	g "github.com/skydive-project/skydive/gremlin"
	"github.com/skydive-project/skydive/logging"
)

// ipv6Supported returns true if the platform support IPv6
func ipv6Supported() bool {
	if _, err := os.Stat("/proc/net/if_inet6"); os.IsNotExist(err) {
		return false
	}

	data, err := ioutil.ReadFile("/proc/sys/net/ipv6/conf/all/disable_ipv6")
	if err != nil {
		return false
	}

	if strings.TrimSpace(string(data)) == "1" {
		return false
	}

	return true
}

func TestSFlowProbeNode(t *testing.T) {
	test := &Test{
		setupCmds: []Cmd{
			{"ovs-vsctl add-br br-spn", true},
			{"ovs-vsctl add-port br-spn spn-intf1 -- set interface spn-intf1 type=internal", true},
			{"ip address add 169.254.33.33/24 dev spn-intf1", true},
			{"ip link set spn-intf1 up", true},
		},

		injections: []TestInjection{{
			from:  g.G.V().Has("Name", "spn-intf1", "Type", "internal"),
			toIP:  "169.254.33.34",
			count: 5,
		}},

		tearDownCmds: []Cmd{
			{"ovs-vsctl del-br br-spn", true},
		},

		captures: []TestCapture{
			{gremlin: g.G.V().Has("Name", "br-spn", "Type", "ovsbridge"), samplingRate: 1, pollingInterval: 10},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			node, err := c.gh.GetNode(c.gremlin.V().Has("Name", "br-spn", "Type", "ovsbridge").HasKey("TID"))
			if err != nil {
				return err
			}

			flows, err := c.gh.GetFlows(c.gremlin.Flows().Has("NodeTID", node.Metadata["TID"], "LayersPath", "Ethernet/IPv4/ICMPv4"))
			if err != nil {
				return err
			}

			if len(flows) != 1 {
				return fmt.Errorf("Unable to find only one flow with the expected NodeTID %s", node.Metadata["TID"])
			}

			return nil
		}},
	}

	RunTest(t, test)
}

func TestSFlowNodeTIDOvsInternalNetNS(t *testing.T) {
	test := &Test{
		setupCmds: []Cmd{
			{"ovs-vsctl add-br br-sntoin", true},
			{"ovs-vsctl add-port br-sntoin sntoin-intf1 -- set interface sntoin-intf1 type=internal", true},
			{"ip netns add sntoin-vm1", true},
			{"ip link set sntoin-intf1 netns sntoin-vm1", true},
			{"ip netns exec sntoin-vm1 ip address add 169.254.33.33/24 dev sntoin-intf1", true},
			{"ip netns exec sntoin-vm1 ip link set sntoin-intf1 up", true},
		},

		injections: []TestInjection{{
			from:  g.G.V().Has("Name", "sntoin-vm1").Out().Has("Name", "sntoin-intf1"),
			toIP:  "169.254.33.34",
			count: 5,
		}},

		tearDownCmds: []Cmd{
			{"ip netns del sntoin-vm1", true},
			{"ovs-vsctl del-br br-sntoin", true},
		},

		captures: []TestCapture{
			{gremlin: g.G.V().Has("Name", "br-sntoin", "Type", "ovsbridge"), samplingRate: 1, pollingInterval: 10},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			node, err := c.gh.GetNode(c.gremlin.V().Has("Name", "br-sntoin", "Type", "ovsbridge").HasKey("TID"))
			if err != nil {
				return err
			}

			flows, err := c.gh.GetFlows(c.gremlin.Flows("NodeTID", node.Metadata["TID"], "LayersPath", "Ethernet/IPv4/ICMPv4"))
			if err != nil {
				return err
			}

			if len(flows) == 0 {
				return errors.New("Unable to find a flow with the expected NodeTID")
			}

			return nil
		}},
	}

	RunTest(t, test)
}

func TestSFlowTwoNodeTID(t *testing.T) {
	test := &Test{
		setupCmds: []Cmd{
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

		injections: []TestInjection{{
			from:  g.G.V().Has("Name", "stnt-vm1").Out().Has("Name", "stnt-intf1"),
			to:    g.G.V().Has("Name", "stnt-vm2").Out().Has("Name", "stnt-intf2"),
			count: 5,
		}},

		tearDownCmds: []Cmd{
			{"ip netns del stnt-vm1", true},
			{"ip netns del stnt-vm2", true},
			{"ovs-vsctl del-br br-stnt1", true},
			{"ovs-vsctl del-br br-stnt2", true},
			{"sleep 1", true},
			{"ip link set br-stnt-link down", true},
			{"brctl delbr br-stnt-link", true},
		},

		captures: []TestCapture{
			{gremlin: `G.V().Has('Name', 'br-stnt1', 'Type', 'ovsbridge')`, samplingRate: 1, pollingInterval: 10},
			{gremlin: `G.V().Has('Name', 'br-stnt2', 'Type', 'ovsbridge')`, samplingRate: 1, pollingInterval: 10},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			flows, err := c.gh.GetFlows(c.gremlin.V().Has("Type", "ovsbridge").Flows().Has("LayersPath", "Ethernet/IPv4/ICMPv4"))
			if err != nil {
				return err
			}

			if len(flows) != 2 {
				return fmt.Errorf("Should have 2 flow entries one per NodeTID got: %d", len(flows))
			}

			node1, err := c.gh.GetNode(c.gremlin.V().Has("Name", "br-stnt1", "Type", "ovsbridge").HasKey("TID"))
			if err != nil {
				return err
			}

			node2, err := c.gh.GetNode(c.gremlin.V().Has("Name", "br-stnt2", "Type", "ovsbridge").HasKey("TID"))
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

			if flows[0].TrackingID != flows[1].TrackingID {
				return fmt.Errorf("Both flows should have the same TrackingID: %v", flows)
			}

			if flows[0].UUID == flows[1].UUID {
				return fmt.Errorf("Both flows should have different UUID: %v", flows)
			}

			return nil
		}},
	}

	RunTest(t, test)
}

func TestBPF(t *testing.T) {
	test := &Test{
		setupCmds: []Cmd{
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

		injections: []TestInjection{{
			from:  g.G.V().Has("Name", "bpf-vm1", "Type", "netns").Out().Has("Name", "eth0"),
			to:    g.G.V().Has("Name", "bpf-vm2", "Type", "netns").Out().Has("Name", "eth0"),
			count: 5,
		}},

		tearDownCmds: []Cmd{
			{"ip link set br-bpf down", true},
			{"brctl delbr br-bpf", true},
			{"ip link del bpf-vm1-eth0", true},
			{"ip link del bpf-vm2-eth0", true},
			{"ip netns del bpf-vm1", true},
			{"ip netns del bpf-vm2", true},
		},

		captures: []TestCapture{
			{gremlin: g.G.V().Has("Name", "bpf-vm1-eth0"), bpf: "icmp and host 169.254.66.67"},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			flows, err := c.gh.GetFlows(c.gremlin.V().Has("Name", "bpf-vm1-eth0").Flows())
			if err != nil {
				return err
			}

			if len(flows) != 1 {
				return fmt.Errorf("Should only get icmp packets, got : %v", flows)
			}

			return nil
		}},
	}

	RunTest(t, test)
}

func TestPCAPProbe(t *testing.T) {
	test := &Test{
		setupCmds: []Cmd{
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

		injections: []TestInjection{{
			from:  g.G.V().Has("Name", "pp-vm1", "Type", "netns").Out().Has("Name", "eth0"),
			to:    g.G.V().Has("Name", "pp-vm2", "Type", "netns").Out().Has("Name", "eth0"),
			count: 5,
		}},

		tearDownCmds: []Cmd{
			{"ip link set br-pp down", true},
			{"brctl delbr br-pp", true},
			{"ip link del pp-vm1-eth0", true},
			{"ip link del pp-vm2-eth0", true},
			{"ip netns del pp-vm1", true},
			{"ip netns del pp-vm2", true},
		},

		captures: []TestCapture{
			{gremlin: g.G.V().Has("Name", "br-pp", "Type", "bridge"), kind: "pcap"},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			node, err := c.gh.GetNode(c.gremlin.V().Has("Name", "br-pp", "Type", "bridge").HasKey("TID"))
			if err != nil {
				return err
			}

			flows, err := c.gh.GetFlows(c.gremlin.Flows().Has("NodeTID", node.Metadata["TID"]))
			if err != nil {
				return err
			}

			if len(flows) == 0 {
				return fmt.Errorf("Unable to find a flow with the expected NodeTID: %v", flows)
			}

			return nil
		}},
	}

	RunTest(t, test)
}

func TestSFlowSrcDstPath(t *testing.T) {
	test := &Test{
		setupCmds: []Cmd{
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

		injections: []TestInjection{{
			from:  g.G.V().Has("Name", "ssdp-vm1", "Type", "netns").Out().Has("Name", "ssdp-intf1"),
			to:    g.G.V().Has("Name", "ssdp-vm2", "Type", "netns").Out().Has("Name", "ssdp-intf2"),
			count: 5,
		}},

		tearDownCmds: []Cmd{
			{"ip netns del ssdp-vm1", true},
			{"ip netns del ssdp-vm2", true},
			{"ovs-vsctl del-br br-ssdp", true},
		},

		captures: []TestCapture{
			{gremlin: `G.V().Has('Name', 'br-ssdp', 'Type', 'ovsbridge')`, samplingRate: 1, pollingInterval: 10},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			node1, err := c.gh.GetNode(c.gremlin.V().Has("Name", "ssdp-intf1", "Type", "internal").HasKey("TID"))
			if err != nil {
				return errors.New("ssdp-intf1 not found")
			}

			node2, err := c.gh.GetNode(c.gremlin.V().Has("Name", "ssdp-intf2", "Type", "internal").HasKey("TID"))
			if err != nil {
				return errors.New("ssdp-intf2 not found")
			}

			srcNode, err := c.gh.GetNode(c.gremlin.Flows().Has("Network.A", "169.254.33.33").In())
			if err != nil {
				return errors.New("Source node not found")
			}

			dstNode, err := c.gh.GetNode(c.gremlin.Flows().Has("Network.A", "169.254.33.33").Out())
			if err != nil {
				return errors.New("Destination node found")
			}

			tid1, _ := node1.GetFieldString("TID")
			tid2, _ := srcNode.GetFieldString("TID")
			if tid1 != tid2 {
				return fmt.Errorf("Source Nodes not matching")
			}

			tid3, _ := node2.GetFieldString("TID")
			tid4, _ := dstNode.GetFieldString("TID")
			if tid3 != tid4 {
				return fmt.Errorf("Destination nodes not matching")
			}

			return nil
		}},
	}

	RunTest(t, test)
}

func TestFlowGremlin(t *testing.T) {
	test := &Test{
		setupCmds: []Cmd{
			{"ovs-vsctl add-br br-fg", true},
			{"ovs-vsctl add-port br-fg fg-intf1 -- set interface fg-intf1 type=internal", true},
			{"ip address add 169.254.33.33/24 dev fg-intf1", true},
			{"ip link set fg-intf1 up", true},
		},

		injections: []TestInjection{{
			from: g.G.V().Has("Name", "fg-intf1", "Type", "internal"), toIP: "169.254.33.34", count: 5,
		}},

		tearDownCmds: []Cmd{
			{"ovs-vsctl del-br br-fg", true},
		},

		captures: []TestCapture{
			{gremlin: g.G.V().Has("Name", "br-fg", "Type", "ovsbridge"), samplingRate: 1, pollingInterval: 10},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			node, err := c.gh.GetNode(c.gremlin.V().Has("Name", "br-fg", "Type", "ovsbridge"))
			if err != nil {
				return err
			}

			count, err := c.gh.GetInt64(c.gremlin.V().Has("Name", "br-fg", "Type", "ovsbridge").Count())
			if err != nil || count != 1 {
				return fmt.Errorf("Should return 1, got: %d - %s", count, err)
			}

			tid, _ := node.GetFieldString("TID")
			if tid == "" {
				return errors.New("Node TID not Found")
			}

			flowsGremlin := c.gremlin.V().Has("Name", "br-fg", "Type", "ovsbridge").Flows().Has("LayersPath", "Ethernet/IPv4/ICMPv4", "NodeTID", tid)
			flows, _ := c.gh.GetFlows(flowsGremlin)
			if len(flows) == 0 {
				return fmt.Errorf("Should return at least 1 flow, got: %v", flows)
			}

			flowsOpt, _ := c.gh.GetFlows(c.gremlin.V().Has("Name", "br-fg", "Type", "ovsbridge").Flows("LayersPath", "Ethernet/IPv4/ICMPv4", "NodeTID", tid))
			if len(flowsOpt) != len(flows) {
				return fmt.Errorf("Should return the same number of flows that without reduce optimization, got: %v", flowsOpt)
			}

			nodes, _ := c.gh.GetNodes(flowsGremlin.Out())
			if len(nodes) != 0 {
				return fmt.Errorf("Should return no destination node, got %d", len(nodes))
			}

			nodes, _ = c.gh.GetNodes(flowsGremlin.Both().Dedup())
			if len(nodes) != 1 {
				return fmt.Errorf("Should return one node, got %d", len(nodes))
			}

			nodes, _ = c.gh.GetNodes(flowsGremlin.In().Dedup())
			if len(nodes) != 1 {
				return fmt.Errorf("Should return one source node, got %d", len(nodes))
			}

			count, err = c.gh.GetInt64(flowsGremlin.Count())
			if err != nil || int(count) != len(flows) {
				return fmt.Errorf("Gremlin count doesn't correspond to the number of flows, got: %v, expected: %v", len(flows), count)
			}

			return nil
		}},
	}

	RunTest(t, test)
}

func TestFlowMetrics(t *testing.T) {
	test := &Test{
		setupCmds: []Cmd{
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
		},

		injections: []TestInjection{{
			from:    g.G.V().Has("Name", "fm-vm1").Out().Has("Name", "fm-intf1"),
			to:      g.G.V().Has("Name", "fm-vm2").Out().Has("Name", "fm-intf2"),
			count:   1,
			payload: string(make([]byte, 1024)),
		}},

		tearDownCmds: []Cmd{
			{"ip netns del fm-vm1", true},
			{"ip netns del fm-vm2", true},
			{"ovs-vsctl del-br br-fm", true},
		},

		captures: []TestCapture{
			{gremlin: g.G.V().Has("Name", "br-fm", "Type", "ovsbridge"), samplingRate: 1, pollingInterval: 10},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			pings := int64(1)
			ovsGremlin := c.gremlin.V().Has("Name", "br-fm", "Type", "ovsbridge")
			if _, err := c.gh.GetNode(ovsGremlin); err != nil {
				return err
			}

			gremlin := ovsGremlin.Flows().Has("LayersPath", g.Regex(".*ICMPv4.*"))

			icmp, err := c.gh.GetFlows(gremlin)
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
			if icmp[0].LayersPath != "Ethernet/IPv4/ICMPv4" {
				return fmt.Errorf("Wrong layer path, should be 'Ethernet/IPv4/ICMPv4', got %s", icmp[0].LayersPath)
			}

			ethernet := icmp[0].Metric
			if ethernet.BAPackets != pings {
				return fmt.Errorf("Number of packets is wrong, got: %v", ethernet.BAPackets)
			}

			if ethernet.ABBytes < pings*1066 || ethernet.BABytes < pings*1066 {
				return fmt.Errorf("Number of bytes is wrong, got: %d, expected at least %d", ethernet.BABytes, pings*1066)
			}

			flows, err := c.gh.GetFlows(ovsGremlin.Flows().Has("LayersPath", "Ethernet/IPv4/ICMPv4", "Metric.ABPackets", pings))
			if len(flows) != 1 || flows[0].Metric.BAPackets != pings {
				return fmt.Errorf("Number of packets is wrong, got %d, flows: %v (error: %+v)", len(flows), flows, err)
			}

			ipv4 := icmp[0].Metric // FIXME double check protocol Network = IPv4/v6
			if flows[0].Metric.ABBytes < ipv4.ABBytes {
				return fmt.Errorf("Layers bytes error, got: %v", icmp)
			}

			getFirstFlowMetric := func(query interface{}) (*flow.FlowMetric, error) {
				flows, err := c.gh.GetFlows(query)
				if err != nil {
					return nil, err
				}
				if len(flows) == 0 {
					return nil, common.ErrNotFound
				}
				return flows[0].Metric, nil
			}

			pingLen := icmp[0].Metric.ABBytes
			metric, err := getFirstFlowMetric(gremlin.Has("Metric.ABBytes", g.Gt(pingLen-1)))
			if err != nil || metric.ABBytes < pingLen {
				return fmt.Errorf("Number of bytes is wrong, got: %v (error: %s)", metric, err)
			}

			metric, err = getFirstFlowMetric(gremlin.Has("Metric.ABBytes", g.Gt(pingLen)))
			if err != common.ErrNotFound {
				return fmt.Errorf("Wrong number of flow, should have none, got : %v", metric)
			}

			metric, err = getFirstFlowMetric(gremlin.Has("Metric.ABBytes", g.Gte(pingLen)))
			if err != nil || metric.ABBytes < pingLen {
				return fmt.Errorf("Number of bytes is wrong, got: %v", metric)
			}

			metric, err = getFirstFlowMetric(gremlin.Has("Metric.ABBytes", g.Gte(pingLen+1)))
			if err != common.ErrNotFound {
				return fmt.Errorf("Wrong number of flow, should have none, got : %v", metric)
			}

			metric, err = getFirstFlowMetric(gremlin.Has("Metric.ABBytes", g.Lt(pingLen+1)))
			if err != nil || metric.ABBytes > pingLen {
				return fmt.Errorf("Number of bytes is wrong, got: %v", metric)
			}

			metric, err = getFirstFlowMetric(gremlin.Has("Metric.ABBytes", g.Lt(pingLen)))
			if err != common.ErrNotFound {
				return fmt.Errorf("Wrong number of flow, should have none, got : %v", metric)
			}

			metric, err = getFirstFlowMetric(gremlin.Has("Metric.ABBytes", g.Lte(pingLen)))
			if err != nil || metric == nil || metric.ABBytes > pingLen {
				return fmt.Errorf("Number of bytes is wrong, got: %v", metric)
			}

			metric, err = getFirstFlowMetric(gremlin.Has("Metric.ABBytes", g.Lte(pingLen-1)))
			if err != common.ErrNotFound {
				return fmt.Errorf("Wrong number of flow, should have none, got : %v", metric)
			}

			metric, err = getFirstFlowMetric(gremlin.Has("Metric.ABBytes", g.Inside(pingLen-1, pingLen+1)))
			if err != nil || metric == nil || metric.ABBytes <= pingLen-1 || metric.ABBytes >= pingLen+1 {
				return fmt.Errorf("Number of bytes is wrong, got: %v", metric)
			}

			metric, err = getFirstFlowMetric(gremlin.Has("Metric.ABBytes", g.Inside(pingLen, pingLen+1)))
			if err != common.ErrNotFound {
				return fmt.Errorf("Wrong number of flow, should have none, got : %v", metric)
			}

			metric, err = getFirstFlowMetric(gremlin.Has("Metric.ABBytes", g.Between(pingLen, pingLen+1)))
			if err != nil || metric == nil || metric.ABBytes <= pingLen-1 || metric.ABBytes >= pingLen+1 {
				return fmt.Errorf("Number of bytes is wrong, got: %v", metric)
			}

			metric, err = getFirstFlowMetric(gremlin.Has("Metric.ABBytes", g.Between(pingLen, pingLen)))
			if err != common.ErrNotFound {
				return fmt.Errorf("Wrong number of flow, should have none, got : %v", metric)
			}

			metric, err = getFirstFlowMetric(gremlin.Has("Metric.RTT", g.Gt(0)))
			if err != nil || metric == nil {
				return fmt.Errorf("Wrong number of flow, should have none, got : %v", metric)
			}

			return nil
		}},
	}

	RunTest(t, test)
}

func TestFlowMetricsStep(t *testing.T) {
	test := &Test{
		setupCmds: []Cmd{
			{"ip netns add fms-vm1", true},
			{"ip netns add fms-vm2", true},
			{"ip link add name fms-intf1 type veth peer name fms-intf2", true},
			{"ip link set fms-intf1 netns fms-vm1", true},
			{"ip link set fms-intf2 netns fms-vm2", true},
			{"ip netns exec fms-vm1 ip link set fms-intf1 up", true},
			{"ip netns exec fms-vm1 ip address add 169.254.33.33/24 dev fms-intf1", true},
			{"ip netns exec fms-vm2 ip link set fms-intf2 up", true},
			{"ip netns exec fms-vm2 ip address add 169.254.33.34/24 dev fms-intf2", true},
		},

		injections: []TestInjection{{
			from:    g.G.V().Has("Name", "fms-vm1").Out().Has("Name", "fms-intf1"),
			to:      g.G.V().Has("Name", "fms-vm2").Out().Has("Name", "fms-intf2"),
			count:   15,
			payload: string(make([]byte, 1024)),
		}},

		tearDownCmds: []Cmd{
			{"ip netns del fms-vm1", true},
			{"ip netns del fms-vm2", true},
		},

		captures: []TestCapture{
			{gremlin: g.G.V().Has("Name", "fms-intf1", "Type", "veth")},
		},

		mode: OneShot,

		checks: []CheckFunction{func(c *CheckContext) error {
			time.Sleep(time.Second * 30)
			return nil
		}, func(c *CheckContext) error {
			gremlin := g.G.Context(c.startTime, c.startTime.Unix()-c.setupTime.Unix()+5).V().Has("Name", "fms-intf1", "Type", "veth").Flows()

			metric, err := c.gh.GetFlowMetric(gremlin.Has("LayersPath", "Ethernet/IPv4/ICMPv4").Dedup().Metrics().Sum())
			if err != nil {
				flows, _ := c.gh.GetFlows(gremlin.Has("LayersPath", "Ethernet/IPv4/ICMPv4").Dedup().Metrics())
				return fmt.Errorf("Could not find metrics (%+v) for flows %s", metric, flowsToString(flows))
			}

			if metric.ABPackets != 15 || metric.BAPackets != 15 || metric.ABBytes < 15360 || metric.BABytes < 15360 {
				flows, _ := c.gh.GetFlows(gremlin)
				return fmt.Errorf("Wrong metric returned, got : %+v for flows %+v, request: %s", metric, flowsToString(flows), gremlin.Has("LayersPath", "Ethernet/IPv4/ICMPv4").Dedup().Metrics().Sum())
			}

			checkMetricsOrder := func(metrics map[string][]*flow.FlowMetric) error {
				// check it's sorted
				var start int64
				for _, metricsOfID := range metrics {
					if len(metricsOfID) <= 1 {
						return fmt.Errorf("Metric array should have more than 1 element (%+v)", metricsOfID)
					}

					for _, m := range metricsOfID {
						if m.GetStart() < start {
							return fmt.Errorf("Metrics not correctly sorted (%+v)", metricsOfID)
						}
						start = m.GetStart()
					}
				}

				return nil
			}

			metrics, err := c.gh.GetFlowMetrics(gremlin.Has("LayersPath", "Ethernet/IPv4/ICMPv4").Dedup().Metrics())
			if err != nil || len(metrics) == 0 {
				return fmt.Errorf("Could not find metrics (%+v)", metrics)
			}

			if len(metrics) != 1 {
				return fmt.Errorf("Should return only one metric array (%+v)", metrics)
			}

			if err = checkMetricsOrder(metrics); err != nil {
				return err
			}

			metrics, err = c.gh.GetFlowMetrics(gremlin.Has("LayersPath", "Ethernet/IPv4/ICMPv4").Dedup().Metrics().Aggregates(10))
			if err != nil || len(metrics) == 0 {
				return fmt.Errorf("Could not find metrics (%+v)", metrics)
			}

			return checkMetricsOrder(metrics)
		}},
	}

	RunTest(t, test)
}

func TestFlowHops(t *testing.T) {
	test := &Test{
		setupCmds: []Cmd{
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
		},

		injections: []TestInjection{{
			from:    g.G.V().Has("Name", "fh-vm1").Out().Has("Name", "fh-intf1"),
			to:      g.G.V().Has("Name", "fh-vm2").Out().Has("Name", "fh-intf2"),
			count:   1,
			payload: string(make([]byte, 1024-8)),
		}},

		tearDownCmds: []Cmd{
			{"ip netns del fh-vm1", true},
			{"ip netns del fh-vm2", true},
			{"ovs-vsctl del-br br-fh", true},
		},

		captures: []TestCapture{
			{gremlin: g.G.V().Has("Name", "br-fh", "Type", "ovsbridge"), samplingRate: 1, pollingInterval: 10},
		},

		mode: Replay,

		// since the agent update ticker is about 10 sec according to the configuration
		// we should wait 11 sec to have the first update and the MetricRange filled
		checks: []CheckFunction{func(c *CheckContext) error {
			prefix := c.gremlin.V().Has("Name", "br-fh", "Type", "ovsbridge")
			gremlin := prefix.Flows().Has("LayersPath", "Ethernet/IPv4/ICMPv4")
			flows, err := c.gh.GetFlows(gremlin)
			if err != nil {
				return err
			}

			if len(flows) != 1 {
				return errors.New("We should receive only one ICMPv4 flow")
			}

			gremlin = prefix.Flows().Has("TrackingID", flows[0].TrackingID).Nodes()
			tnodes, err := c.gh.GetNodes(gremlin)
			if err != nil {
				return err
			}

			if len(tnodes) != 3 {
				return fmt.Errorf("We should have 3 nodes NodeTID,A,B got : %v for flows : %v", tnodes, flows)
			}

			gremlin = prefix.Flows().Has("LayersPath", "Ethernet/IPv4/ICMPv4").Hops()
			nodes, err := c.gh.GetNodes(gremlin)
			if err != nil {
				return err
			}

			if len(nodes) != 1 {
				return errors.New("We should have 1 node NodeTID")
			}

			found := false
			for _, n := range tnodes {
				if found = n.ID == nodes[0].ID; found {
					break
				}
			}
			if !found {
				return errors.New("We should found the Hops nodes in the TrackingID nodes")
			}

			return nil
		}},
	}

	RunTest(t, test)
}

func TestIPv6FlowHopsIPv6(t *testing.T) {
	if !ipv6Supported() {
		t.Skipf("Platform doesn't support IPv6")
	}

	filterIPv6AddrAnd := func(flows []*flow.Flow, A, B string) (r []*flow.Flow) {
		for _, f := range flows {
			if f.Network == nil || (f.Network.Protocol != flow.FlowProtocol_IPV6) {
				continue
			}
			if strings.HasPrefix(f.Network.A, A) && strings.HasPrefix(f.Network.B, B) {
				r = append(r, f)
			}
			if strings.HasPrefix(f.Network.A, B) && strings.HasPrefix(f.Network.B, A) {
				r = append(r, f)
			}
		}
		return r
	}

	test := &Test{
		setupCmds: []Cmd{
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
		},

		injections: []TestInjection{{
			from:  g.G.V().Has("Name", "ipv6fh-vm1").Out().Has("Name", "ipv6fh-intf1"),
			to:    g.G.V().Has("Name", "ipv6fh-vm2").Out().Has("Name", "ipv6fh-intf2"),
			count: 5,
			ipv6:  true,
		}},

		tearDownCmds: []Cmd{
			{"ip netns del ipv6fh-vm1", true},
			{"ip netns del ipv6fh-vm2", true},
			{"ovs-vsctl del-br br-ipv6fh", true},
		},

		captures: []TestCapture{
			{gremlin: g.G.V().Has("Name", "br-ipv6fh", "Type", "ovsbridge"), samplingRate: 1, pollingInterval: 10},
		},

		// since the agent update ticker is about 10 sec according to the configuration
		// we should wait 11 sec to have the first update and the MetricRange filled
		checks: []CheckFunction{func(c *CheckContext) error {
			prefix := c.gremlin.V().Has("Name", "br-ipv6fh", "Type", "ovsbridge")
			gremlin := prefix.Flows().Has("LayersPath", "Ethernet/IPv6/ICMPv6/ICMPv6Echo", "ICMP.Type", "ECHO")
			// filterIPv6AddrAnd() as we received multicast/broadcast from fresh registered interfaces announcement
			allFlows, err := c.gh.GetFlows(gremlin)
			if err != nil {
				return err
			}

			flows := filterIPv6AddrAnd(allFlows, "fd49:37c8:5229::1", "fd49:37c8:5229::2")
			if len(flows) != 1 {
				return errors.New("We should receive only one ICMPv6 flow")
			}

			gremlin = prefix.Flows().Has("TrackingID", flows[0].TrackingID).Nodes()
			tnodes, err := c.gh.GetNodes(gremlin)
			if err != nil {
				return err
			}

			if len(tnodes) != 3 {
				return fmt.Errorf("We should have 3 nodes NodeTID,A,B, got %+v", tnodes)
			}

			/* Dedup() here for same reason than above ^^^ */
			gremlin = prefix.Flows().Has("LayersPath", "Ethernet/IPv6/ICMPv6/ICMPv6Echo").Hops().Dedup()
			nodes, err := c.gh.GetNodes(gremlin)
			if err != nil {
				return err
			}

			if len(nodes) != 1 {
				return errors.New("We should have 1 node NodeTID")
			}

			found := false
			for _, n := range tnodes {
				if n.ID == nodes[0].ID {
					found = true
					break
				}
			}
			if !found {
				return errors.New("We should found the Hops nodes in the TrackingID nodes")
			}

			return nil
		}},
	}

	RunTest(t, test)
}

func TestICMP(t *testing.T) {
	if !ipv6Supported() {
		t.Skipf("Platform doesn't support IPv6")
	}

	test := &Test{
		setupCmds: []Cmd{
			{"ovs-vsctl add-br br-icmp", true},

			{"ovs-vsctl add-port br-icmp icmp-intf1 -- set interface icmp-intf1 type=internal", true},
			{"ip netns add icmp-vm1", true},
			{"ip link set icmp-intf1 netns icmp-vm1", true},
			{"ip netns exec icmp-vm1 ip address add 10.0.0.1/24 dev icmp-intf1", true},
			{"ip netns exec icmp-vm1 ip address add fd49:37c8:5229::1/48 dev icmp-intf1", true},
			{"ip netns exec icmp-vm1 ip link set icmp-intf1 up", true},

			{"ovs-vsctl add-port br-icmp icmp-intf2 -- set interface icmp-intf2 type=internal", true},
			{"ip netns add icmp-vm2", true},
			{"ip link set icmp-intf2 netns icmp-vm2", true},
			{"ip netns exec icmp-vm2 ip address add 10.0.0.2/24 dev icmp-intf2", true},
			{"ip netns exec icmp-vm2 ip address add fd49:37c8:5229::2/48 dev icmp-intf2", true},
			{"ip netns exec icmp-vm2 ip link set icmp-intf2 up", true},
		},

		injections: []TestInjection{
			{
				from:  g.G.V().Has("Name", "icmp-vm1").Out().Has("Name", "icmp-intf1"),
				to:    g.G.V().Has("Name", "icmp-vm2").Out().Has("Name", "icmp-intf2"),
				count: 3,
				id:    123,
				mode:  types.PIModeRandom,
			},
			{
				from:  g.G.V().Has("Name", "icmp-vm1").Out().Has("Name", "icmp-intf1", "Type", "internal"),
				to:    g.G.V().Has("Name", "icmp-vm2").Out().Has("Name", "icmp-intf2", "Type", "internal"),
				count: 1,
				id:    456,
				ipv6:  true,
			},
		},

		tearDownCmds: []Cmd{
			{"ip netns del icmp-vm1", true},
			{"ip netns del icmp-vm2", true},
			{"ovs-vsctl del-br br-icmp", true},
		},

		captures: []TestCapture{
			{gremlin: g.G.V().Has("Name", "br-icmp", "Type", "ovsbridge"), samplingRate: 1, pollingInterval: 10},
		},

		mode: Replay,

		// since the agent update ticker is about 10 sec according to the configuration
		// we should wait 11 sec to have the first update and the MetricRange filled
		checks: []CheckFunction{func(c *CheckContext) error {
			prefix := c.gremlin.V().Has("Name", "br-icmp", "Type", "ovsbridge")

			gremlin := prefix.Flows().Has("LayersPath", "Ethernet/IPv4/ICMPv4", "ICMP.ID", 123)
			icmpFlows, err := c.gh.GetFlows(gremlin)
			if err != nil {
				return err
			}

			if len(icmpFlows) != 1 {
				return fmt.Errorf("We should receive one ICMPv4 flow with ID 123, got %s", flowsToString(icmpFlows))
			}

			ipv4TrackingID := icmpFlows[0].TrackingID
			gremlin = prefix.Flows().Has("TrackingID", ipv4TrackingID)
			icmpFlows, err = c.gh.GetFlows(gremlin)
			if err != nil {
				return err
			}

			if len(icmpFlows) != 1 {
				return fmt.Errorf("We should receive one ICMPv4 flow with TrackingID %s, got %s", ipv4TrackingID, flowsToString(icmpFlows))
			}

			gremlin = prefix.Flows().Has("LayersPath", "Ethernet/IPv6/ICMPv6/ICMPv6Echo", "ICMP.ID", 456)
			icmpFlows, err = c.gh.GetFlows(gremlin)
			if err != nil {
				return err
			}

			if len(icmpFlows) != 1 {
				return fmt.Errorf("We should receive one ICMPv6 flow with ID 456, got %s", flowsToString(icmpFlows))
			}

			ipv6TrackingID := icmpFlows[0].TrackingID
			gremlin = prefix.Flows().Has("TrackingID", ipv6TrackingID)
			icmpFlows, err = c.gh.GetFlows(gremlin)
			if err != nil {
				return err
			}

			if len(icmpFlows) != 1 {
				return fmt.Errorf("We should receive one ICMPv6 flow with TrackingID %s, got %s", ipv6TrackingID, flowsToString(icmpFlows))
			}

			gremlin = prefix.Flows().Has("LayersPath", "Ethernet/IPv4/ICMPv4", "ICMP.Type", "ECHO")
			icmpFlows, err = c.gh.GetFlows(gremlin)
			if err != nil {
				return err
			}

			if len(icmpFlows) != 3 {
				return fmt.Errorf("We should receive 3 ICMP flows, got %s", flowsToString(icmpFlows))
			}

			return nil
		}},
	}

	RunTest(t, test)
}

func TestFlowGRETunnel(t *testing.T) {
	test := testFlowTunnel(t, "br-fgt", "gre", false, "192.168.0.1", "192.168.0.2",
		"172.16.0.1", "172.16.0.2", "192.168.0.0/24")
	RunTest(t, test)
}

func TestFlowVxlanTunnel(t *testing.T) {
	test := testFlowTunnel(t, "br-fvt", "vxlan", false, "192.168.0.1", "192.168.0.2",
		"172.16.0.1", "172.16.0.2", "192.168.0.0/24")
	RunTest(t, test)
}

func TestIPv6FlowGRETunnelIPv6(t *testing.T) {
	t.Skip("Fedora seems didn't support ip6gre tunnel for the moment")

	if !ipv6Supported() {
		t.Skipf("Platform doesn't support IPv6")
	}

	test := testFlowTunnel(t, "br-fgtv6", "gre", true, "fdfe:38b:489c::1", "fdfe:38b:489c::2",
		"fd49:37c8:5229::1", "fd49:37c8:5229::2", "fdfe:38b:489c::/48")
	RunTest(t, test)
}

func testFlowTunnel(t *testing.T, bridge string, tunnelType string, ipv6 bool, IP1, IP2, tunnelIP1, tunnelIP2, addrRange string) *Test {
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
		setupCmds: []Cmd{
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

		tearDownCmds: []Cmd{
			{"ip link del tunnel-vm1-eth0", true},
			{"ip link del tunnel-vm2-eth0", true},
			{"ip netns del tunnel-vm1", true},
			{"ip netns del tunnel-vm2", true},
			{"ovs-vsctl del-br " + bridge, true},
		},

		captures: []TestCapture{
			{gremlin: g.G.V().Has("Name", "tunnel-vm1").Out().Has("Name", "tunnel")},
			{gremlin: g.G.V().Has("Name", "tunnel-vm2-eth0", "Type", "veth")},
		},

		injections: []TestInjection{{
			from:  g.G.V().Has("Name", "tunnel-vm1").Out().Has("Name", "dummy0"),
			to:    g.G.V().Has("Name", "tunnel-vm2").Out().Has("Name", "dummy0"),
			intf:  g.G.V().Has("Name", "tunnel-vm1").Out().Has("Name", "tunnel"),
			count: 5,
			ipv6:  ipv6,
		}},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			nodes, err := c.gh.GetNodes(c.gremlin.V().Has("Name", "tunnel-vm1").Out().Has("Name", "tunnel"))
			if err != nil {
				return err
			}

			if len(nodes) == 0 {
				return errors.New("Found no node")
			}

			node := nodes[0]
			tid, err := node.GetFieldString("TID")
			if err != nil {
				return fmt.Errorf("Node %s has no TID", node.ID)
			}

			flowsInnerTunnel, err := c.gh.GetFlows(c.gremlin.Flows().Has("NodeTID", tid, "Application", icmpVersion))
			if err != nil {
				return err
			}

			if len(flowsInnerTunnel) != 1 {
				return fmt.Errorf("We should have only one %s flow in the tunnel %v", icmpVersion, flowsToString(flowsInnerTunnel))
			}

			trackingID := flowsInnerTunnel[0].TrackingID
			flowsBridge, err := c.gh.GetFlows(c.gremlin.V().Has("Name", "tunnel-vm2-eth0").Flows().Has("TrackingID", trackingID, "Application", icmpVersion).Dedup())
			if err != nil {
				return err
			}

			if len(flowsBridge) == 0 {
				fb, _ := c.gh.GetFlows(c.gremlin.V().Has("Name", "tunnel-vm2-eth0").Flows())
				return fmt.Errorf("TrackingID not found in %s tunnel: leaving the interface(%v) == seen in the tunnel(%v)", tunnelType, flowsToString(flowsInnerTunnel), flowsToString(fb))
			}

			return nil
		}},
	}
	return test
}

func TestReplayCapture(t *testing.T) {
	var pcapSocket string
	var capture *types.Capture

	sendPCAPFile := func(filename string, socket string) error {
		file, err := os.Open(filename)
		if err != nil {
			return fmt.Errorf("Failed to open file %s: %s", filename, err.Error())
		}

		stats, err := file.Stat()
		if err != nil {
			return fmt.Errorf("Failed to get informations for %s: %s", filename, err.Error())
		}

		tcpAddr, err := net.ResolveTCPAddr("tcp", socket)
		if err != nil {
			return fmt.Errorf("Failed to parse address %s: %s", tcpAddr.String(), err.Error())
		}

		conn, err := net.DialTCP("tcp", nil, tcpAddr)
		if err != nil {
			return fmt.Errorf("Failed to connect to TCP socket %s: %s", tcpAddr.String(), err.Error())
		}

		unixFile, err := conn.File()
		if err != nil {
			return fmt.Errorf("Failed to get file description from socket %s: %s", socket, err.Error())
		}
		defer unixFile.Close()

		dst := unixFile.Fd()
		src := file.Fd()

		_, err = syscall.Sendfile(int(dst), int(src), nil, int(stats.Size()))
		if err != nil {
			logging.GetLogger().Fatalf("Failed to send file %s to socket %s: %s", filename, socket, err.Error())
		}

		return nil
	}

	test := &Test{
		setupCmds: []Cmd{
			{"ovs-vsctl add-br br-rc", true},
		},

		settleFunction: func(c *TestContext) error {
			capture = c.captures[0]
			n, err := c.gh.GetNode(g.G.V().Has("Captures.ID", capture.UUID))
			if err != nil {
				return err
			}

			field, _ := n.GetField("Captures")
			captures := field.(*probes.Captures)
			for _, c := range *captures {
				if c.PCAPSocket == "" {
					return fmt.Errorf("Failed to retrieve PCAP socket for capture %s", capture.UUID)
				}
				pcapSocket = c.PCAPSocket
			}

			return nil
		},

		setupFunction: func(c *TestContext) error {
			return sendPCAPFile("pcaptraces/eth-ip4-arp-dns-req-http-google.pcap", pcapSocket)
		},

		tearDownCmds: []Cmd{
			{"ovs-vsctl del-br br-rc", true},
		},

		captures: []TestCapture{
			{gremlin: g.G.V().Has("Name", "br-rc", "Type", "ovsbridge"), kind: "pcapsocket"},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			gremlin := c.gremlin.V().Has("Name", "br-rc", "Type", "ovsbridge").HasKey("TID")
			node, err := c.gh.GetNode(gremlin)
			if err != nil {
				return err
			}

			gremlin = c.gremlin.Flows().Has("NodeTID", node.Metadata["TID"])
			flows, err := c.gh.GetFlows(gremlin)
			if err != nil {
				return err
			}

			if len(flows) != 6 {
				return fmt.Errorf("Wrong number of flows. Expected 6, got %d", len(flows))
			}

			flows, err = c.gh.GetFlows(gremlin.Has("Application", "DNS"))
			if err != nil {
				return err
			}

			if len(flows) != 2 {
				return fmt.Errorf("Wrong number of DNS flows. Expected 2, got %d", len(flows))
			}

			return nil
		}},
	}

	RunTest(t, test)
}

func TestPcapInject(t *testing.T) {
	test := &Test{
		setupFunction: func(c *TestContext) error {
			file, err := os.Open("pcaptraces/eth-ip4-arp-dns-req-http-google.pcap")
			if err != nil {
				return err
			}
			defer file.Close()

			resp, err := c.client.Request("POST", "pcap", file, nil)
			if err != nil {
				return err
			}

			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				return fmt.Errorf("Should get 2xx status code, got %d", resp.StatusCode)
			}

			return nil
		},

		mode: OneShot,

		checks: []CheckFunction{func(c *CheckContext) error {
			flows, _ := c.gh.GetFlows(c.gremlin.Context(1454659514).Flows().Has("Application", "DNS"))
			if len(flows) != 2 {
				return fmt.Errorf("Wrong number of DNS flows. Expected 2, got %d", len(flows))
			}
			return nil
		}},
	}

	RunTest(t, test)
}

func TestFlowVLANSegmentation(t *testing.T) {
	test := &Test{
		setupCmds: []Cmd{
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

			{"sudo ip netns exec vlan-vm1 ip l set vlan up", true},
			{"sudo ip netns exec vlan-vm2 ip l set vlan up", true},

			{"sudo ovs-vsctl add-port br-vlan vlan-vm1-eth0", true},
			{"sudo ovs-vsctl add-port br-vlan vlan-vm2-eth0", true},
		},

		injections: []TestInjection{{
			from:  g.G.V().Has("Name", "vlan-vm1").Out().Has("Name", "vlan"),
			to:    g.G.V().Has("Name", "vlan-vm2").Out().Has("Name", "vlan"),
			count: 5,
		}},

		tearDownCmds: []Cmd{
			{"sudo ip link del vlan-vm1-eth0", true},
			{"sudo ip link del vlan-vm2-eth0", true},
			{"sudo ip netns del vlan-vm1", true},
			{"sudo ip netns del vlan-vm2", true},
			{"sudo ovs-vsctl del-br br-vlan", true},
		},

		captures: []TestCapture{
			{gremlin: g.G.V().Has("Name", "vlan-vm1").Out().Has("Name", "vlan")},
			{gremlin: g.G.V().Has("Name", "vlan-vm2-eth0", "Type", "veth")},
		},

		checks: []CheckFunction{func(c *CheckContext) error {
			outFlows, err := c.gh.GetFlows(c.gremlin.V().Has("Name", "vlan-vm2-eth0").Flows().Has("LayersPath", "Ethernet/Dot1Q/IPv4/ICMPv4"))
			if err != nil {
				return err
			}

			if len(outFlows) != 1 {
				return fmt.Errorf("We should have only one ICMPv4 flow %v", flowsToString(outFlows))
			}

			if outFlows[0].GetLink().GetID() != 8 {
				return fmt.Errorf("Should have a Vlan ID equal to 8 got: %v", flowsToString(outFlows))
			}

			inFlows, err := c.gh.GetFlows(c.gremlin.V().Has("Name", "vlan").Flows().Has("LayersPath", "Ethernet/IPv4/ICMPv4"))
			if err != nil {
				return err
			}

			if len(inFlows) != 1 {
				return fmt.Errorf("We should have only one ICMPv4 flow %v", flowsToString(inFlows))
			}

			if inFlows[0].L3TrackingID != outFlows[0].L3TrackingID {
				return fmt.Errorf("Both flows should have the same L3TrackingID: :%s vs %v", flowsToString(outFlows), flowsToString(inFlows))
			}

			return nil
		}},
	}

	RunTest(t, test)
}

func TestSort(t *testing.T) {
	test := &Test{
		setupCmds: []Cmd{
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

		tearDownCmds: []Cmd{
			{"ovs-vsctl del-br br-int", true},
			{"ip link del dst-vm-eth0", true},
			{"ip link del src-vm1-eth0", true},
			{"ip link del src-vm2-eth0", true},
			{"ip netns del dst-vm", true},
			{"ip netns del src-vm1", true},
			{"ip netns del src-vm2", true},
		},

		captures: []TestCapture{
			{gremlin: g.G.V().Has("Name", "dst-eth0", "Type", "veth")},
		},

		injections: []TestInjection{
			{from: g.G.V().Has("Name", "src1-eth0"), to: g.G.V().Has("Name", "dst-eth0"), count: 10},
			{from: g.G.V().Has("Name", "src2-eth0"), to: g.G.V().Has("Name", "dst-eth0"), count: 20},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			prefix := g.G.Context(c.time, 60)
			gremlin := prefix.Flows().Has("Network", "169.254.34.33").Sort()

			flows, err := c.gh.GetFlows(gremlin)
			if err != nil {
				return err
			}
			if len(flows) != 2 {
				flows, _ = c.gh.GetFlows(prefix.Flows())
				return fmt.Errorf("Expected two flow, got %+v", flows)
			}
			//check is it in ascending order by Last field
			if flows[0].Last > flows[1].Last {
				return fmt.Errorf("Flows not in expected order, expected ASC got DESC")
			}

			gremlin = prefix.Flows().Has("Network", "169.254.34.33").Sort(g.DESC, "Start")

			flows, err = c.gh.GetFlows(gremlin)
			if err != nil {
				return err
			}
			if len(flows) != 2 {
				flows, _ = c.gh.GetFlows(prefix.Flows())
				return fmt.Errorf("Expected two flow, got %+v", flows)
			}
			//check is it in descending order by Start field
			if flows[0].Start < flows[1].Start {
				return fmt.Errorf("Flows not in expected order, expected DESC got ASC")
			}

			return nil
		}},
	}
	RunTest(t, test)
}

func TestFlowSumStep(t *testing.T) {
	test := &Test{
		setupCmds: []Cmd{
			{"ovs-vsctl add-br br-sum", true},

			{"ip netns add vm1", true},
			{"ip link add vm1-eth0 type veth peer name intf1 netns vm1", true},
			{"ip link set vm1-eth0 up", true},
			{"ip netns exec vm1 ip address add 169.254.34.33/24 dev intf1", true},
			{"ip netns exec vm1 ip link set intf1 up", true},

			{"ip netns add vm2", true},
			{"ip link add vm2-eth0 type veth peer name intf2 netns vm2", true},
			{"ip link set vm2-eth0 up", true},
			{"ip netns exec vm2 ip address add 169.254.34.34/24 dev intf2", true},
			{"ip netns exec vm2 ip link set intf2 up", true},

			{"ip netns add vm3", true},
			{"ip link add vm3-eth0 type veth peer name intf3 netns vm3", true},
			{"ip link set vm3-eth0 up", true},
			{"ip netns exec vm3 ip address add 169.254.34.35/24 dev intf3", true},
			{"ip netns exec vm3 ip link set intf3 up", true},

			{"ovs-vsctl add-port br-sum vm1-eth0", true},
			{"ovs-vsctl add-port br-sum vm2-eth0", true},
			{"ovs-vsctl add-port br-sum vm3-eth0", true},
		},

		settleFunction: func(c *TestContext) (err error) {
			nodes, err := c.gh.GetNodes(g.G.V().Has("Name", "br-sum").Out().Has("Type", "ovsport").Dedup())
			if err != nil {
				return err
			}

			if len(nodes) != 4 {
				return fmt.Errorf("There should be 4 ports in bridge br-sum, got %+v", nodes)
			}

			return nil
		},

		injections: []TestInjection{
			{from: g.G.V().Has("Name", "intf1"), to: g.G.V().Has("Name", "intf2"), count: 3},
			{from: g.G.V().Has("Name", "intf2"), to: g.G.V().Has("Name", "intf3"), count: 4},
			{from: g.G.V().Has("Name", "intf1"), to: g.G.V().Has("Name", "intf3"), count: 3},
		},

		tearDownCmds: []Cmd{
			{"ovs-vsctl del-br br-sum", true},
			{"ip link del vm1-eth0", true},
			{"ip link del vm2-eth0", true},
			{"ip link del vm3-eth0", true},
			{"ip netns del vm1", true},
			{"ip netns del vm2", true},
			{"ip netns del vm3", true},
		},

		captures: []TestCapture{
			{gremlin: g.G.V().Has("Name", "br-sum", "Type", "ovsbridge"), samplingRate: 1, pollingInterval: 10},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			gremlin := c.gremlin.V().Has("Name", "br-sum", "Type", "ovsbridge").Flows().Has("LayersPath", "Ethernet/IPv4/ICMPv4").Dedup().Sum("Metric.ABPackets")

			sum, err := c.gh.GetInt64(gremlin)
			if err != nil {
				return fmt.Errorf("Error while retriving SUM: %v", err)
			}
			if sum != 10 {
				return fmt.Errorf("Got wrong sum value, Expected 10 got %v", sum)
			}
			return nil
		}},
	}

	RunTest(t, test)
}

func TestFlowCaptureNodeStep(t *testing.T) {
	test := &Test{
		setupCmds: []Cmd{
			{"ovs-vsctl add-br br-fcn", true},

			{"ip netns add cns-vm1", true},
			{"ip link add cns-vm1-eth0 type veth peer name intf1 netns cns-vm1", true},
			{"ip link set cns-vm1-eth0 up", true},
			{"ip netns exec cns-vm1 ip link set intf1 up", true},
			{"ip netns exec cns-vm1 ip address add 169.254.38.33/24 dev intf1", true},

			{"ip netns add cns-vm2", true},
			{"ip link add cns-vm2-eth0 type veth peer name intf2 netns cns-vm2", true},
			{"ip link set cns-vm2-eth0 up", true},
			{"ip netns exec cns-vm2 ip link set intf2 up", true},
			{"ip netns exec cns-vm2 ip address add 169.254.38.34/24 dev intf2", true},

			{"ovs-vsctl add-port br-fcn cns-vm1-eth0", true},
			{"ovs-vsctl add-port br-fcn cns-vm2-eth0", true},
		},

		tearDownCmds: []Cmd{
			{"ovs-vsctl del-br br-fcn", true},
			{"ip link del cns-vm1-eth0", true},
			{"ip link del cns-vm2-eth0", true},
			{"ip netns del cns-vm1", true},
			{"ip netns del cns-vm2", true},
		},

		captures: []TestCapture{
			{gremlin: g.G.V().Has("Name", "br-fcn", "Type", "ovsbridge"), samplingRate: 1, pollingInterval: 10},
		},

		injections: []TestInjection{
			{from: g.G.V().Has("Name", "intf1"), to: g.G.V().Has("Name", "intf2"), count: 3},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			nodes, err := c.gh.GetNodes(c.gremlin.Flows().Has("Network", "169.254.38.33").Dedup().CaptureNode())
			if err != nil {
				return err
			}

			if len(nodes) != 1 {
				return fmt.Errorf("Expected one node, got %+v", nodes)
			}

			nodeName := nodes[0].Metadata["Name"].(string)
			if nodeName != "br-fcn" {
				return fmt.Errorf("we should get br-fcn node, got %s", nodeName)
			}

			return nil
		}},
	}
	RunTest(t, test)
}

func TestFlowsWithShortestPath(t *testing.T) {
	test := &Test{
		setupCmds: []Cmd{
			{"ovs-vsctl add-br br-spt", true},

			{"ip netns add src-vm", true},
			{"ip link add src-vm-eth0 type veth peer name spt-src-eth0 netns src-vm", true},
			{"ip link set src-vm-eth0 up", true},
			{"ip netns exec src-vm ip link set spt-src-eth0 up", true},
			{"ip netns exec src-vm ip address add 169.254.37.33/24 dev spt-src-eth0", true},

			{"ip netns add dst-vm", true},
			{"ip link add dst-vm-eth0 type veth peer name spt-dst-eth0 netns dst-vm", true},
			{"ip link set dst-vm-eth0 up", true},
			{"ip netns exec dst-vm ip link set spt-dst-eth0 up", true},
			{"ip netns exec dst-vm ip address add 169.254.37.34/24 dev spt-dst-eth0", true},

			{"ovs-vsctl add-port br-spt src-vm-eth0", true},
			{"ovs-vsctl add-port br-spt dst-vm-eth0", true},
		},

		tearDownCmds: []Cmd{
			{"ovs-vsctl del-br br-spt", true},
			{"ip link del dst-vm-eth0", true},
			{"ip link del src-vm-eth0", true},
			{"ip netns del src-vm", true},
			{"ip netns del dst-vm", true},
		},

		captures: []TestCapture{
			{gremlin: g.G.V().Has("Name", "spt-src-eth0").ShortestPathTo(g.Metadata("Name", "spt-dst-eth0"), g.Metadata("RelationType", "layer2"))},
		},

		injections: []TestInjection{
			{from: g.G.V().Has("Name", "spt-src-eth0"), to: g.G.V().Has("Name", "spt-dst-eth0"), count: 10},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			gremlin := c.gremlin.V().Has("Name", "spt-src-eth0").ShortestPathTo(g.Metadata("Name", "spt-dst-eth0"), g.Metadata("RelationType", "layer2")).Flows().Has("Network", "169.254.37.33").Dedup()
			flows, err := c.gh.GetFlows(gremlin)
			if err != nil {
				return err
			}
			if len(flows) != 1 {
				return fmt.Errorf("Expected one flow, got %+v", flows)
			}

			return nil
		}},
	}
	RunTest(t, test)
}

func printRawPackets(t *testing.T, gh *client.GremlinQueryHelper, query g.QueryString) error {
	header := make(http.Header)
	resp, err := gh.Request(query.String(), header)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("PCAP request error: %s", resp.Status)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	t.Logf("RawPackets: %s", string(data))

	return nil
}

func getRawPackets(gh *client.GremlinQueryHelper, query g.QueryString) ([]gopacket.Packet, error) {
	header := make(http.Header)
	header.Set("Accept", "application/vnd.tcpdump.pcap")

	resp, err := gh.Request(query.String(), header)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("PCAP request error: %s", resp.Status)
	}

	handle, err := pcapgo.NewReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("PCAP opening error: %s", err)
	}

	var packets []gopacket.Packet
	for {
		data, _, err := handle.ReadPacketData()
		if err != nil {
			if err != io.EOF {
				return nil, fmt.Errorf("PCAP reading error: %s", err)
			}
			break
		}
		packet := gopacket.NewPacket(data, handle.LinkType(), gopacket.DecodeOptions{NoCopy: true})
		packets = append(packets, packet)
	}

	return packets, nil
}

func TestRawPackets(t *testing.T) {
	test := &Test{
		setupCmds: []Cmd{
			{"brctl addbr br-rp", true},
			{"ip link set br-rp up", true},
			{"ip netns add rp-vm1", true},
			{"ip link add name rp-vm1-eth0 type veth peer name eth0 netns rp-vm1", true},
			{"ip link set rp-vm1-eth0 up", true},
			{"ip netns exec rp-vm1 ip link set eth0 up", true},
			{"ip netns exec rp-vm1 ip address add 169.254.122.66/24 dev eth0", true},
			{"brctl addif br-rp rp-vm1-eth0", true},

			{"ip netns add rp-vm2", true},
			{"ip link add name rp-vm2-eth0 type veth peer name eth0 netns rp-vm2", true},
			{"ip link set rp-vm2-eth0 up", true},
			{"ip netns exec rp-vm2 ip link set eth0 up", true},
			{"ip netns exec rp-vm2 ip address add 169.254.122.67/24 dev eth0", true},
			{"brctl addif br-rp rp-vm2-eth0", true},
		},

		tearDownCmds: []Cmd{
			{"ip link set br-rp down", true},
			{"brctl delbr br-rp", true},
			{"ip link del rp-vm1-eth0", true},
			{"ip link del rp-vm2-eth0", true},
			{"ip netns del rp-vm1", true},
			{"ip netns del rp-vm2", true},
		},

		captures: []TestCapture{
			{gremlin: `G.V().Has('Name', 'rp-vm1-eth0')`, rawPackets: 9, kind: "pcap"},
		},

		injections: []TestInjection{{
			from:  "G.V().Has('Name', 'rp-vm1', 'Type', 'netns').Out().Has('Name', 'eth0')",
			to:    "G.V().Has('Name', 'rp-vm2', 'Type', 'netns').Out().Has('Name', 'eth0')",
			count: 2,
		}},

		mode: OneShot,

		checks: []CheckFunction{func(c *CheckContext) error {
			node, err := c.gh.GetNode(g.G.At("-1s").V().Has("Name", "rp-vm1-eth0").HasKey("TID"))
			if err != nil {
				return err
			}

			query := g.G.At("-1s").Flows().Has("NodeTID", node.Metadata["TID"], "LayersPath", "Ethernet/IPv4/ICMPv4")

			flows, err := c.gh.GetFlows(query)
			if err != nil {
				return err
			}

			if len(flows) != 1 {
				return fmt.Errorf("Should get one ICMPv4 flow: %v", flows)
			}

			if flows[0].RawPacketsCaptured != 4 {
				packets, err := getRawPackets(c.gh, query.RawPackets())
				if err != nil {
					return err
				}
				return fmt.Errorf("Should get 4 raw packets 2 echo/reply: %v, %+v", flows, packets)
			}

			if err = printRawPackets(t, c.gh, query.RawPackets()); err != nil {
				return err
			}

			packets, err := getRawPackets(c.gh, query.RawPackets())
			if err != nil {
				return err
			}

			if len(packets) != 4 {
				return fmt.Errorf("Should get 4 pcap raw packets 2 echo/reply: %v, %+v", flows, packets)
			}

			replyPackets, err := getRawPackets(c.gh, query.RawPackets().BPF("icmp[icmptype] == 0"))
			if err != nil {
				return err
			}

			if len(replyPackets) != 2 {
				return fmt.Errorf("Should get 2 echo reply raw packets: %v, %+v", flows, replyPackets)
			}

			return nil
		}},
	}

	RunTest(t, test)
}

func TestFlowsWithIpv4Range(t *testing.T) {
	test := &Test{
		setupCmds: []Cmd{
			{"ovs-vsctl add-br br-ipr", true},

			{"ip netns add src-ipr", true},
			{"ip link add src-ipr-eth0 type veth peer name ipr-src-eth0 netns src-ipr", true},
			{"ip link set src-ipr-eth0 up", true},
			{"ip netns exec src-ipr ip link set ipr-src-eth0 up", true},
			{"ip netns exec src-ipr ip address add 169.254.40.33/24 dev ipr-src-eth0", true},

			{"ip netns add dst-ipr", true},
			{"ip link add dst-ipr-eth0 type veth peer name ipr-dst-eth0 netns dst-ipr", true},
			{"ip link set dst-ipr-eth0 up", true},
			{"ip netns exec dst-ipr ip link set ipr-dst-eth0 up", true},
			{"ip netns exec dst-ipr ip address add 169.254.40.34/24 dev ipr-dst-eth0", true},

			{"ovs-vsctl add-port br-ipr src-ipr-eth0", true},
			{"ovs-vsctl add-port br-ipr dst-ipr-eth0", true},
		},

		tearDownCmds: []Cmd{
			{"ovs-vsctl del-br br-ipr", true},
			{"ip link del dst-ipr-eth0", true},
			{"ip link del src-ipr-eth0", true},
			{"ip netns del src-ipr", true},
			{"ip netns del dst-ipr", true},
		},

		captures: []TestCapture{
			{gremlin: g.G.V().Has("Name", "ipr-src-eth0")},
		},

		injections: []TestInjection{{
			from:  g.G.V().Has("Name", "ipr-src-eth0"),
			to:    g.G.V().Has("Name", "ipr-dst-eth0"),
			count: 10,
		}},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			gremlin := c.gremlin.Flows().Has("Network", g.Ipv4Range("169.254.40.0/24"))
			flows, err := c.gh.GetFlows(gremlin)
			if err != nil {
				return err
			}
			if len(flows) != 1 {
				return fmt.Errorf("Expected one flow, got %+v", flows)
			}
			return nil
		}},
	}
	RunTest(t, test)
}

func TestOvsMirror(t *testing.T) {
	test := &Test{
		setupCmds: []Cmd{
			{"ovs-vsctl add-br br-omir", true},
			{"ovs-vsctl add-port br-omir omir-if1 -- set interface omir-if1 type=internal", true},
			{"ip address add 169.254.93.33/24 dev omir-if1", true},
			{"ip link set omir-if1 up", true},
		},

		tearDownCmds: []Cmd{
			{"ovs-vsctl del-br br-omir", true},
		},

		captures: []TestCapture{
			{gremlin: g.G.V().Has("Name", "omir-if1", "Type", "ovsport")},
		},

		injections: []TestInjection{{
			from:  g.G.V().Has("Name", "omir-if1", "Type", "internal"),
			toIP:  "169.254.33.34",
			toMAC: "11:22:33:44:55:66",
			count: 5,
		}},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			orig, err := c.gh.GetNode(c.gremlin.V().Has("Name", "omir-if1", "Type", "ovsport"))
			if err != nil {
				return fmt.Errorf("Unable to find the expected ovsport: %s", err)
			}

			node, err := c.gh.GetNode(c.gremlin.V().Has("Name", g.Regex("mir.*"), "Type", "internal").HasKey("TID"))
			if err != nil {
				return fmt.Errorf("Unable to find the expected Mirror interface: %s", err)
			}

			field, err := node.GetField("Captures")
			if err != nil {
				return err
			}

			captures, ok := field.(*probes.Captures)
			if !ok || len(*captures) == 0 {
				return fmt.Errorf("Failed to retrieve captures on %s", node.ID)
			}
			mirrorOf := (*captures)[0].MirrorOf

			if mirrorOf != string(orig.ID) {
				aa, err := c.gh.GetNode(c.gremlin.V(mirrorOf))
				if err != nil {
					return fmt.Errorf("Unable to find the expected ovsport: %s", err)
				}
				return fmt.Errorf("Unable to find expected Mirror information of %v on mirror node %v != %v", orig, node, aa)
			}

			flows, err := c.gh.GetFlows(c.gremlin.Flows("NodeTID", node.Metadata["TID"], "LayersPath", "Ethernet/IPv4/ICMPv4"))
			if err != nil {
				return err
			}

			if len(flows) == 0 {
				return errors.New("Unable to find a flow with the expected NodeTID")
			}

			return nil
		}},
	}

	RunTest(t, test)
}

func TestSFlowCapture(t *testing.T) {
	test := &Test{
		setupCmds: []Cmd{
			{"ovs-vsctl add-br br-sfct", true},

			{"ovs-vsctl add-port br-sfct sfct-intf1 -- set interface sfct-intf1 type=internal", true},
			{"ip netns add sfct-vm1", true},
			{"ip link set sfct-intf1 netns sfct-vm1", true},
			{"ip netns exec sfct-vm1 ip address add 169.254.29.11/24 dev sfct-intf1", true},
			{"ip netns exec sfct-vm1 ip link set sfct-intf1 up", true},

			{"ovs-vsctl add-port br-sfct sfct-intf2 -- set interface sfct-intf2 type=internal", true},
			{"ip netns add sfct-vm2", true},
			{"ip link set sfct-intf2 netns sfct-vm2", true},
			{"ip netns exec sfct-vm2 ip address add 169.254.29.12/24 dev sfct-intf2", true},
			{"ip netns exec sfct-vm2 ip link set sfct-intf2 up", true},

			{"ovs-vsctl --id=@sflow create sflow agent=lo target=\\\"127.0.0.1:6343\\\" header=128 sampling=1 polling=10 -- set bridge br-sfct sflow=@sflow", true},
		},

		injections: []TestInjection{{
			from:  g.G.V().Has("Name", "sfct-vm1").Out().Has("Name", "sfct-intf1"),
			to:    g.G.V().Has("Name", "sfct-vm2").Out().Has("Name", "sfct-intf2"),
			count: 1,
		}},

		tearDownCmds: []Cmd{
			{"ip netns del sfct-vm1", true},
			{"ip netns del sfct-vm2", true},
			{"ovs-vsctl del-br br-sfct", true},
		},

		captures: []TestCapture{
			{gremlin: g.G.V().Has("Type", "host").Out().Has("Name", "lo", "Type", "device"), kind: "sflow", port: 6343},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			node, err := c.gh.GetNode(c.gremlin.V().Has("Type", "host").Out().Has("Name", "lo", "Type", "device"))
			if err != nil {
				return err
			}

			flows, err := c.gh.GetFlows(c.gremlin.Flows("NodeTID", node.Metadata["TID"]).Has("Network.A", "169.254.29.11"))
			if err != nil {
				return err
			}

			if len(flows) != 1 {
				return fmt.Errorf("We should receive only one flow, got: %d", len(flows))
			}

			return nil
		}},
	}

	RunTest(t, test)
}

func TestERSpanV1Target(t *testing.T) {
	test := &Test{
		setupCmds: []Cmd{
			{"brctl addbr br-erspan", true},
			{"ip link set br-erspan up", true},
			{"ip netns add erspan-vm1", true},
			{"ip link add name erspan-vm1-eth0 type veth peer name eth0 netns erspan-vm1", true},
			{"ip link set erspan-vm1-eth0 up", true},
			{"ip netns exec erspan-vm1 ip link set eth0 up", true},
			{"ip netns exec erspan-vm1 ip address add 169.254.66.68/24 dev eth0", true},
			{"brctl addif br-erspan erspan-vm1-eth0", true},

			{"ip netns add erspan-vm2", true},
			{"ip link add name erspan-vm2-eth0 type veth peer name eth0 netns erspan-vm2", true},
			{"ip link set erspan-vm2-eth0 up", true},
			{"ip netns exec erspan-vm2 ip link set eth0 up", true},
			{"ip netns exec erspan-vm2 ip address add 169.254.66.69/24 dev eth0", true},
			{"brctl addif br-erspan erspan-vm2-eth0", true},

			{"ip netns add erspan-end", true},
			{"ip link add name erspan-end-eth0 type veth peer name eth0 netns erspan-end", true},
			{"ip link set erspan-end-eth0 up", true},
			{"ip address add 169.254.67.1/24 dev erspan-end-eth0", true},
			{"ip netns exec erspan-end ip link set eth0 up", true},
			{"ip netns exec erspan-end ip address add 169.254.67.2/24 dev eth0", true},
		},

		injections: []TestInjection{{
			from:  g.G.V().Has("Name", "erspan-vm1", "Type", "netns").Out().Has("Name", "eth0"),
			to:    g.G.V().Has("Name", "erspan-vm2", "Type", "netns").Out().Has("Name", "eth0"),
			count: 5,
		}},

		tearDownCmds: []Cmd{
			{"ip link set br-erspan down", true},
			{"brctl delbr br-erspan", true},
			{"ip link del erspan-vm1-eth0", true},
			{"ip link del erspan-vm2-eth0", true},
			{"ip link del erspan-end-eth0", true},
			{"ip netns del erspan-vm1", true},
			{"ip netns del erspan-vm2", true},
			{"ip netns del erspan-end", true},
		},

		captures: []TestCapture{
			{gremlin: g.G.V().Has("Name", "erspan-vm1-eth0"), target: "169.254.67.2:0", targetType: "erspanv1", bpf: "icmp"},
			{gremlin: g.G.V().Has("Name", "erspan-end-eth0")},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			flows, err := c.gh.GetFlows(c.gremlin.Flows().Has("Application", "GRE", "Network.B", "169.254.67.2"))
			if err != nil {
				return err
			}

			if len(flows) != 1 || flows[0].Metric.ABPackets != 10 {
				return fmt.Errorf("Should get 1 gre flow with 10 packets, one per icmp echo/reply, got : %v", flows)
			}

			return nil
		}},
	}

	RunTest(t, test)
}

func TestNetFlowV5Target(t *testing.T) {
	test := &Test{
		setupCmds: []Cmd{
			{"brctl addbr br-netflow", true},
			{"ip link set br-netflow up", true},
			{"ip netns add netflow-vm1", true},
			{"ip link add name netflo-vm1-eth0 type veth peer name eth0 netns netflow-vm1", true},
			{"ip link set netflo-vm1-eth0 up", true},
			{"ip netns exec netflow-vm1 ip link set eth0 up", true},
			{"ip netns exec netflow-vm1 ip address add 169.254.66.68/24 dev eth0", true},
			{"brctl addif br-netflow netflo-vm1-eth0", true},

			{"ip netns add netflow-vm2", true},
			{"ip link add name netflo-vm2-eth0 type veth peer name eth0 netns netflow-vm2", true},
			{"ip link set netflo-vm2-eth0 up", true},
			{"ip netns exec netflow-vm2 ip link set eth0 up", true},
			{"ip netns exec netflow-vm2 ip address add 169.254.66.69/24 dev eth0", true},
			{"brctl addif br-netflow netflo-vm2-eth0", true},

			{"ip netns add netflow-end", true},
			{"ip link add name netflo-end-eth0 type veth peer name eth0 netns netflow-end", true},
			{"ip link set netflo-end-eth0 up", true},
			{"ip address add 169.254.67.1/24 dev netflo-end-eth0", true},
			{"ip netns exec netflow-end ip link set eth0 up", true},
			{"ip netns exec netflow-end ip address add 169.254.67.2/24 dev eth0", true},
		},

		injections: []TestInjection{{
			from:  g.G.V().Has("Name", "netflow-vm1", "Type", "netns").Out().Has("Name", "eth0"),
			to:    g.G.V().Has("Name", "netflow-vm2", "Type", "netns").Out().Has("Name", "eth0"),
			count: 5,
		}},

		tearDownCmds: []Cmd{
			{"ip link set br-netflow down", true},
			{"brctl delbr br-netflow", true},
			{"ip link del netflo-vm1-eth0", true},
			{"ip link del netflo-vm2-eth0", true},
			{"ip link del netflo-end-eth0", true},
			{"ip netns del netflow-vm1", true},
			{"ip netns del netflow-vm2", true},
			{"ip netns del netflow-end", true},
		},

		captures: []TestCapture{
			{gremlin: g.G.V().Has("Name", "netflo-vm1-eth0"), target: "169.254.67.2:8989", targetType: "netflowv5", bpf: "icmp"},
			{gremlin: g.G.V().Has("Name", "netflo-end-eth0")},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			flows, err := c.gh.GetFlows(c.gremlin.Flows().Has("Application", "UDP", "Transport", 8989))
			if err != nil {
				return err
			}

			if len(flows) != 1 {
				return fmt.Errorf("Should get 1 udp flow, got : %v", flows)
			}

			return nil
		}},
	}

	RunTest(t, test)
}

func TestFlowsHashCnx(t *testing.T) {
	test := testFlowsHashCnx(t)
	RunTest(t, test)
}

func testFlowsHashCnx(t *testing.T) *Test {
	test := &Test{
		setupCmds: []Cmd{
			{"ovs-vsctl add-br br-hash", true},

			{"ip netns add src-vm", true},
			{"ip link add src-vm-eth0 type veth peer name hash-src-eth0 netns src-vm", true},
			{"ip link set src-vm-eth0 up", true},
			{"ip netns exec src-vm ip link set hash-src-eth0 up", true},
			{"ip netns exec src-vm ip address add 169.254.107.33/24 dev hash-src-eth0", true},

			{"ip netns add dst-vm", true},
			{"ip link add dst-vm-eth0 type veth peer name hash-dst-eth0 netns dst-vm", true},
			{"ip link set dst-vm-eth0 up", true},
			{"ip netns exec dst-vm ip link set hash-dst-eth0 up", true},
			{"ip netns exec dst-vm ip address add 169.254.107.34/24 dev hash-dst-eth0", true},

			{"ovs-vsctl add-port br-hash src-vm-eth0", true},
			{"ovs-vsctl add-port br-hash dst-vm-eth0", true},
		},

		injections: []TestInjection{
			{
				protocol: "tcp",

				from:     g.G.V().Has("Name", "src-vm").Out().Has("Name", "hash-src-eth0"),
				fromPort: 12345,
				to:       g.G.V().Has("Name", "dst-vm").Out().Has("Name", "hash-dst-eth0"),
				toPort:   54321,
				count:    10,
			},
			{
				protocol: "tcp",

				from:     g.G.V().Has("Name", "src-vm").Out().Has("Name", "hash-src-eth0"),
				fromPort: 54321,
				to:       g.G.V().Has("Name", "dst-vm").Out().Has("Name", "hash-dst-eth0"),
				toPort:   12345,
				count:    10,
			},
		},

		tearDownCmds: []Cmd{
			{"ovs-vsctl del-br br-hash", true},
			{"ip link del dst-vm-eth0", true},
			{"ip link del src-vm-eth0", true},
			{"ip netns del src-vm", true},
			{"ip netns del dst-vm", true},
		},

		captures: []TestCapture{
			{gremlin: g.G.V().Has("Name", "hash-src-eth0")},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			gremlin := c.gremlin.Flows().Has("Network", "169.254.107.33", "LayersPath", "Ethernet/IPv4/TCP").Dedup()
			flows, err := c.gh.GetFlows(gremlin)
			if err != nil {
				return err
			}
			if len(flows) != 2 {
				return fmt.Errorf("Expected two flows, got %+v", flows)
			}
			foundAB := false
			foundBA := false
			for _, f := range flows {
				if f.Metric.ABPackets != 10 || f.Metric.BAPackets != 10 {
					return fmt.Errorf("Expected 10 packets each way, got %+v", flows)
				}
				if f.Transport.A == 12345 && f.Transport.B == 54321 {
					foundAB = true
				}
				if f.Transport.B == 12345 && f.Transport.A == 54321 {
					foundBA = true
				}
			}
			if !foundAB || !foundBA {
				return fmt.Errorf("Expected two flows, with ports swapped, got %+v", flows)
			}
			return nil
		}},
	}
	return test
}
