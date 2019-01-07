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
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/go-test/deep"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/graffiti/graph"
)

type ruleCmd struct {
	rule string
	add  bool
}

func verify(c *CheckContext, bridge string, expected []int) error {
	l := len(expected)
	found := make([]int, l)
	gremlin := c.gremlin.V().Has("Type", "ovsbridge", "Name", bridge).Out("Type", "ofrule").Sort()
	nodes, _ := c.gh.GetNodes(gremlin)
	for _, node := range nodes {
		metadata := node.Metadata
		actions := metadata["Actions"].([]interface{})
		if len(actions) != 1 {
			continue
		}
		action := actions[0].(map[string]interface{})
		if f := action["Function"]; f != nil {
			if f.(string) != "resubmit" {
				continue
			}

			args := action["Arguments"].([]interface{})
			if len(args) != 2 {
				continue
			}

			arg := args[1].(map[string]interface{})
			target, err := strconv.Atoi(arg["Function"].(string))
			if err == nil && target <= l {
				found[target-1] = found[target-1] + 1
			}
		} else if f = action["Type"]; f != "" {
			if f.(string) != "resubmit" {
				continue
			}

			args := action["Arguments"].(map[string]interface{})
			target := args["Table"].(int64)
			if target <= int64(l) {
				found[target-1] = found[target-1] + 1
			}
		}
	}
	for i, e := range expected {
		f := found[i]
		if f != e {
			return fmt.Errorf("expected %d rules with '%d' but got %d - %v", e, i, f, nodes)
		}
	}
	return nil
}

func makeTest(t *testing.T, suffix string, rules []ruleCmd, expected []int) {
	bridge := "br-of" + suffix
	intf1 := "intf1-" + suffix
	intf2 := "intf2-" + suffix
	setupCmds := []Cmd{
		{"ovs-vsctl --if-exists del-br " + bridge, true},
		{"ovs-vsctl add-br " + bridge, true},
		{"ovs-vsctl add-port " + bridge + " " + intf1 + " -- set interface " + intf1 + " type=internal", true},
		{"ovs-vsctl add-port " + bridge + " " + intf2 + " -- set interface " + intf2 + " type=internal", true},
		{"sleep 2", true},
	}

	for _, ruleCmd := range rules {
		var cmd string
		if ruleCmd.add {
			cmd = fmt.Sprintf("ovs-ofctl add-flow %s %s", bridge, ruleCmd.rule)
		} else {
			cmd = fmt.Sprintf("ovs-ofctl del-flows --strict %s %s", bridge, ruleCmd.rule)
		}
		setupCmds = append(setupCmds, Cmd{Cmd: cmd, Check: true})
	}

	test := &Test{
		setupCmds: setupCmds,

		tearDownCmds: []Cmd{
			{"ovs-vsctl del-br " + bridge, true},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			return verify(c, bridge, expected)
		}},
	}

	RunTest(t, test)
}

func TestAddOFRule(t *testing.T) {
	makeTest(
		t,
		"1",
		[]ruleCmd{{"table=0,priority=0,in_port=1,actions=resubmit(,1)", true}},
		[]int{1})
}

func TestDelOFRule(t *testing.T) {
	makeTest(
		t,
		"2",
		[]ruleCmd{
			{"table=0,priority=0,in_port=1,actions=resubmit(,1)", true},
			{"table=0,priority=0,in_port=1", false},
		},
		[]int{0})
}

func TestSuperimposedOFRule(t *testing.T) {
	makeTest(
		t,
		"3",
		[]ruleCmd{
			{"table=0,priority=1,actions=resubmit(,1)", true},
			{"table=0,priority=2,actions=resubmit(,2)", true},
			{"table=0,priority=0,in_port=1,actions=resubmit(,3)", true},
			{"table=0,priority=3,actions=resubmit(,4)", true},
			{"table=0,priority=2", false},
		},
		[]int{1, 0, 1, 1})
}

func TestDelRuleWithBridgeOFRule(t *testing.T) {
	setupCmds := []Cmd{
		{"ovs-vsctl --if-exists del-br br-of4", true},
		{"ovs-vsctl add-br br-of4", true},
		{"ovs-vsctl add-port br-of4 intf1-4 -- set interface intf1-4 type=internal", true},
		{"ovs-vsctl add-port br-of4 intf2-4 -- set interface intf2-4 type=internal", true},
		{"sleep 1", true},
		{"ovs-ofctl add-flow br-of4 table=0,priority=1,actions=resubmit(,1)", true},
		{"sleep 1", true},
		{"ovs-vsctl del-br br-of4", true},
	}

	test := &Test{
		setupCmds: setupCmds,

		tearDownCmds: []Cmd{},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error { return verify(c, "br-4", []int{0}) }},
	}

	RunTest(t, test)
}

func TestModOFRule(t *testing.T) {
	makeTest(
		t,
		"5",
		[]ruleCmd{{"table=0,priority=0,in_port=1,actions=resubmit(,1)", true},
			{"table=0,priority=0,in_port=1,actions=resubmit(,2)", true}},
		[]int{0, 1})
}

func verifyGroup(c *CheckContext, bridge string, expected int) error {
	retry := func() error {
		gremlin := c.gremlin.V().Has("Type", "ovsbridge", "Name", bridge).Out("Type", "ofgroup")
		nodes, err := c.gh.GetNodes(gremlin)
		if err != nil {
			return err
		}
		l := len(nodes)
		if l != expected {
			return fmt.Errorf("expected %d groups - %v", l, expected)
		}
		for _, node := range nodes {
			m := node.Metadata
			if m["GroupId"].(int64) != 123 {
				return fmt.Errorf("Group_id set to %d", m["GroupId"])
			}
			if m["GroupType"].(string) != "all" {
				return fmt.Errorf("Group type set to %s", m["GroupType"])
			}
			buckets := m["Buckets"].([]interface{})
			found := false
		loop:
			for _, rawbucket := range buckets {
				bucket := rawbucket.(map[string]interface{})
				actions := bucket["Actions"].([]interface{})
				for _, rawaction := range actions {
					action := rawaction.(map[string]interface{})
					if action["Function"] == "resubmit" || action["Type"] == "resubmit" {
						found = true
						break loop
					}
				}
			}
			if !found {
				return fmt.Errorf("Problem with contents: %v", buckets)
			}
		}
		return nil
	}
	return common.Retry(retry, 10, time.Second)
}

func TestAddOFGroup(t *testing.T) {
	setupCmds := []Cmd{
		{"ovs-vsctl --if-exists del-br br-of6", true},
		{"ovs-vsctl add-br br-of6", true},
		{"ovs-vsctl set bridge br-of6 protocols=OpenFlow10,OpenFlow14,OpenFlow15", true},
		{"ovs-vsctl add-port br-of6 intf1-6 -- set interface intf1-6 type=internal", true},
		{"ovs-vsctl add-port br-of6 intf2-6 -- set interface intf2-6 type=internal", true},
		{"sleep 1", true},
		{"ovs-ofctl -O OpenFlow14 add-group br-of6 group_id=123,type=all,bucket=actions=1,bucket=actions=resubmit(,1)", true},
		{"sleep 3", true},
	}
	test := &Test{
		setupCmds: setupCmds,
		tearDownCmds: []Cmd{
			{"ovs-vsctl del-br br-of6", true},
		},
		mode:   Replay,
		checks: []CheckFunction{func(c *CheckContext) error { return verifyGroup(c, "br-of6", 1) }},
	}

	RunTest(t, test)
}

func TestModOFGroup(t *testing.T) {
	setupCmds := []Cmd{
		{"ovs-vsctl --if-exists del-br br-of7", true},
		{"ovs-vsctl add-br br-of7", true},
		{"ovs-vsctl set bridge br-of7 protocols=OpenFlow10,OpenFlow14,OpenFlow15", true},
		{"ovs-vsctl add-port br-of7 intf1-7 -- set interface intf1-7 type=internal", true},
		{"ovs-vsctl add-port br-of7 intf2-7 -- set interface intf2-7 type=internal", true},
		{"ovs-ofctl -O OpenFlow15 add-group br-of7 group_id=123,type=all,bucket=actions=1", true},
		{"ovs-ofctl -O OpenFlow15 insert-buckets br-of7 group_id=123,command_bucket_id=last,bucket=bucket_id=2,actions=resubmit(,1)", true},
		{"sleep 3", true},
	}
	test := &Test{
		setupCmds: setupCmds,
		tearDownCmds: []Cmd{
			{"ovs-vsctl del-br br-of7", true},
		},
		mode:   Replay,
		checks: []CheckFunction{func(c *CheckContext) error { return verifyGroup(c, "br-of7", 1) }},
	}

	RunTest(t, test)
}

func TestDelOFGroup(t *testing.T) {
	setupCmds := []Cmd{
		{"ovs-vsctl --if-exists del-br br-of4", true},
		{"ovs-vsctl add-br br-of8", true},
		{"ovs-vsctl set bridge br-of8 protocols=OpenFlow10,OpenFlow14,OpenFlow15", true},
		{"ovs-vsctl add-port br-of8 intf1-8 -- set interface intf1-8 type=internal", true},
		{"ovs-vsctl add-port br-of8 intf2-8 -- set interface intf2-8 type=internal", true},
		{"sleep 1", true},
		{"ovs-ofctl -O OpenFlow14 add-group br-of8 group_id=123,type=all,bucket=actions=1", true},
		{"ovs-ofctl -O OpenFlow14 del-groups br-of8", true},
		{"sleep 3", true},
	}
	test := &Test{
		setupCmds: setupCmds,
		tearDownCmds: []Cmd{
			{"ovs-vsctl del-br br-of8", true},
		},
		mode:   Replay,
		checks: []CheckFunction{func(c *CheckContext) error { return verifyGroup(c, "br-of8", 0) }},
	}

	RunTest(t, test)
}

func testOVSRule(t *testing.T, kind, rule string, checkRule func(node *graph.Node) error) {
	if !ovsOflowNative {
		t.Skipf("Skipping test as we're not using native OpenFlow mode")
	}

	nodeType := "of" + kind
	if kind == "flow" {
		nodeType = "ofrule"
	}

	test := &Test{
		setupCmds: []Cmd{
			Cmd{"ovs-vsctl add-br br-test", true},
			Cmd{"ovs-ofctl del-flows br-test", true},
			Cmd{"ovs-vsctl set bridge br-test protocols=OpenFlow10,OpenFlow14", true},
			Cmd{"ovs-vsctl set-controller br-test ptcp:16633", true},
			Cmd{fmt.Sprintf("ovs-ofctl add-%s br-test %s", kind, rule), true},
		},

		tearDownCmds: []Cmd{
			{"ovs-vsctl del-br br-test", true},
		},

		retries: 10,

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			gremlin := c.gremlin.V().Has("Type", "ovsbridge", "Name", "br-test").Out("Type", nodeType).Sort("Priority")
			nodes, err := c.gh.GetNodes(gremlin)
			if err != nil {
				return err
			}

			if len(nodes) != 1 {
				return fmt.Errorf("Expected 1 %s, got %d", nodeType, len(nodes))
			}

			return checkRule(nodes[0])
		}},
	}

	RunTest(t, test)
}

func TestOVSOXMCsState(t *testing.T) {
	testOVSRule(t, "flow", "ct_state=+new-rel,actions=", func(node *graph.Node) error {
		filters, _ := node.GetField("Filters")
		expectedFilters := []interface{}{
			map[string]interface{}{
				"Type": "conn_tracking_state_masked",
				"Value": map[string]interface{}{
					"New":         true,
					"Established": true,
				},
			},
		}
		if err := deep.Equal(filters, expectedFilters); err != nil {
			return errors.New(spew.Sdump(err))
		}
		return nil
	})
}

func TestOVSOXMSimple(t *testing.T) {
	testOVSRule(t, "flow", "in_port=1,ip,actions=resubmit(,1)", func(node *graph.Node) error {
		filters, _ := node.GetField("Filters")
		expectedFilters := []interface{}{
			map[string]interface{}{
				"Type":  "in_port",
				"Value": int64(1),
			},
			map[string]interface{}{
				"Type":  "eth_type",
				"Value": "ip",
			},
		}
		if err := deep.Equal(filters, expectedFilters); err != nil {
			return errors.New(spew.Sdump(err))
		}
		return nil
	})
}

/*
func TestOVSOXM15(t *testing.T) {
	testOVSRule(t, "nw_src=192.168.200.0/24,actions=resubmit(,1)", func(node *graph.Node) error {
		filters, _ := node.GetField("Filters")
		expectedFilters := []interface{}{
			map[string]interface{}{
				"Type":  "packet_type",
				"Value": int64(0),
			},
		}
		if err := deep.Equal(filters, expectedFilters); err != nil {
			return errors.New(spew.Sdump(err))
		}
		return nil
	})
}
*/

func TestOVSRuleAttributes(t *testing.T) {
	testOVSRule(t, "flow", "-O OpenFlow14 reset_counts,priority=100,importance=200,actions=", func(node *graph.Node) error {
		if priority, _ := node.GetFieldInt64("Priority"); priority != 100 {
			return fmt.Errorf("Expected priority 100, got %d", priority)
		}
		if importance, _ := node.GetFieldInt64("Importance"); importance != 200 {
			return fmt.Errorf("Expected importance 200, got %d", importance)
		}
		flags, _ := node.GetField("Flags")
		expectedFlags := map[string]interface{}{
			"ResetCounts": true,
		}
		if err := deep.Equal(flags, expectedFlags); err != nil {
			return errors.New(spew.Sdump(err))
		}
		return nil
	})
}

func TestOVSActionDrop(t *testing.T) {
	testOVSRule(t, "flow", "priority=32768,actions=drop", func(node *graph.Node) error {
		if priority, _ := node.GetFieldInt64("Priority"); priority != 32768 {
			return fmt.Errorf("Expected priority 32768, got %d", priority)
		}
		actions, _ := node.GetField("Actions")
		expectedActions := []interface{}{
			map[string]interface{}{"Type": "drop"},
		}
		if err := deep.Equal(actions, expectedActions); err != nil {
			return errors.New(spew.Sdump(err))
		}
		return nil
	})
}

func TestOVSActionController(t *testing.T) {
	testOVSRule(t, "flow", "actions=controller(userdata=11.11.11.11)", func(node *graph.Node) error {
		actions, _ := node.GetField("Actions")
		expectedActions := []interface{}{
			map[string]interface{}{
				"Type": "nx_controller2",
				"Arguments": map[string]interface{}{
					"Properties": []interface{}{
						map[string]interface{}{
							"Type":     int64(3),
							"Length":   int64(8),
							"Userdata": base64.StdEncoding.EncodeToString([]byte{0x11, 0x11, 0x11, 0x11}),
						},
					},
				},
			},
		}
		if err := deep.Equal(actions, expectedActions); err != nil {
			return errors.New(spew.Sdump(err))
		}
		return nil
	})
}

func TestOVSActionSetField(t *testing.T) {
	testOVSRule(t, "flow", "priority=32768,actions=mod_nw_src=10.0.0.1", func(node *graph.Node) error {
		actions, _ := node.GetField("Actions")
		expectedActions := []interface{}{
			map[string]interface{}{
				"Type": "set_field",
				"Arguments": map[string]interface{}{
					"Field": map[string]interface{}{
						"Type":  "ipv4_src",
						"Value": "10.0.0.1",
					},
				},
			},
		}
		if err := deep.Equal(actions, expectedActions); err != nil {
			return errors.New(spew.Sdump(err))
		}
		return nil
	})
}

func TestOVSActionPopMPLS(t *testing.T) {
	testOVSRule(t, "flow", "priority=32768,actions=pop_mpls:1234", func(node *graph.Node) error {
		actions, _ := node.GetField("Actions")
		expectedActions := []interface{}{
			map[string]interface{}{
				"Type": "pop_mpls",
				"Arguments": map[string]interface{}{
					"Ethertype": int64(1234),
				},
			},
		}
		if err := deep.Equal(actions, expectedActions); err != nil {
			return errors.New(spew.Sdump(err))
		}
		return nil
	})
}

func TestOVSActionPushPop(t *testing.T) {
	testOVSRule(t, "flow", "ip,actions=push:NXM_OF_IP_DST[],pop:NXM_OF_IP_SRC[]", func(node *graph.Node) error {
		actions, _ := node.GetField("Actions")
		expectedActions := []interface{}{
			map[string]interface{}{
				"Type": "nx_stack_push",
				"Arguments": map[string]interface{}{
					"Field":  "ip_dst",
					"NBits":  int64(32),
					"Offset": int64(0),
				},
			},
			map[string]interface{}{
				"Type": "nx_stack_pop",
				"Arguments": map[string]interface{}{
					"Field":  "ip_src",
					"NBits":  int64(32),
					"Offset": int64(0),
				},
			},
		}
		if err := deep.Equal(actions, expectedActions); err != nil {
			return errors.New(spew.Sdump(err))
		}
		return nil
	})
}

func TestOVSActionEnqueue(t *testing.T) {
	testOVSRule(t, "flow", "actions=enqueue:123:456", func(node *graph.Node) error {
		actions, _ := node.GetField("Actions")
		expectedActions := []interface{}{
			map[string]interface{}{
				"Type": "set_queue",
				"Arguments": map[string]interface{}{
					"QueueId": int64(456),
				},
			},
			map[string]interface{}{
				"Type": "output",
				"Arguments": map[string]interface{}{
					"MaxLen": int64(65535),
					"Port":   int64(123),
				},
			},
			map[string]interface{}{
				"Type": "nx_pop_queue",
			},
		}
		if err := deep.Equal(actions, expectedActions); err != nil {
			return errors.New(spew.Sdump(err))
		}
		return nil
	})
}

func TestOVSActionBundleLoad(t *testing.T) {
	testOVSRule(t, "flow", "actions=bundle_load(eth_src,50,active_backup,ofport,reg0,slaves:1)", func(node *graph.Node) error {
		if priority, _ := node.GetFieldInt64("Priority"); priority != 32768 {
			return fmt.Errorf("Expected priority 32768, got %d", priority)
		}
		actions, _ := node.GetField("Actions")
		expectedActions := []interface{}{
			map[string]interface{}{
				"Type": "nx_bundle_load_in_port",
				"Arguments": map[string]interface{}{
					"Algorithm": "active_backup",
					"Basis":     int64(50),
					"Dst":       "reg0",
					"Fields":    "eth_src",
					"InPorts": []interface{}{
						map[string]interface{}{
							"Port": int64(65536),
						},
					},
					"NSlaves":   int64(1),
					"OfsNbits":  int64(31),
					"SlaveType": "in_port",
				},
			},
		}
		if err := deep.Equal(actions, expectedActions); err != nil {
			return errors.New(spew.Sdump(err))
		}
		return nil
	})
}

func TestOVSActionMultipath(t *testing.T) {
	testOVSRule(t, "flow", "actions=multipath(eth_src,50,modulo_n,1,0,NXM_NX_REG0[7])", func(node *graph.Node) error {
		if priority, _ := node.GetFieldInt64("Priority"); priority != 32768 {
			return fmt.Errorf("Expected priority 32768, got %d", priority)
		}
		actions, _ := node.GetField("Actions")
		expectedActions := []interface{}{
			map[string]interface{}{
				"Type": "nx_multipath",
				"Arguments": map[string]interface{}{
					"Algorithm": "modulo_n",
					"Arg":       int64(0),
					"Basis":     int64(50),
					"Dst":       "reg0",
					"Fields":    "eth_src",
					"MaxLink":   int64(0),
					"OfsNbits":  int64(448),
				},
			},
		}
		if err := deep.Equal(actions, expectedActions); err != nil {
			return errors.New(spew.Sdump(err))
		}
		return nil
	})
}

func TestOVSActionLearn(t *testing.T) {
	testOVSRule(t, "flow", "actions=learn(table=1,hard_timeout=60,NXM_OF_VLAN_TCI[0..11],NXM_OF_ETH_DST[]=NXM_OF_ETH_SRC[],output:NXM_OF_IN_PORT[])", func(node *graph.Node) error {
		if priority, _ := node.GetFieldInt64("Priority"); priority != 32768 {
			return fmt.Errorf("Expected priority 32768, got %d", priority)
		}
		actions, _ := node.GetField("Actions")
		expectedActions := []interface{}{
			map[string]interface{}{
				"Type": "nx_learn",
				"Arguments": map[string]interface{}{
					"Cookie":         int64(0),
					"FinHardTimeout": int64(0),
					"FinIdleTimeout": int64(0),
					"Flags":          int64(0),
					"FlowMods": []interface{}{
						map[string]interface{}{
							"SrcDst": int64(0),
							"NBits":  int64(12),
							"Src":    "vlan_tci",
							"SrcOfs": int64(0),
							"Dst":    "vlan_tci",
							"DstOfs": int64(0),
						},
						map[string]interface{}{
							"SrcDst": int64(0),
							"NBits":  int64(48),
							"Src":    "eth_src",
							"SrcOfs": int64(0),
							"Dst":    "eth_dst",
							"DstOfs": int64(0),
						},
						map[string]interface{}{
							"SrcDst": int64(16),
							"NBits":  int64(16),
							"Src":    "in_port",
							"SrcOfs": int64(0),
						},
					},
					"HardTimeout": int64(60),
					"IdleTimeout": int64(0),
					"Priority":    int64(32768),
					"TableId":     int64(1),
				},
			},
		}
		if err := deep.Equal(actions, expectedActions); err != nil {
			return errors.New(spew.Sdump(err))
		}
		return nil
	})
}

func TestOVSActionMove(t *testing.T) {
	testOVSRule(t, "flow", "actions=move:eth_dst->eth_src[]", func(node *graph.Node) error {
		actions, _ := node.GetField("Actions")
		expectedActions := []interface{}{
			map[string]interface{}{
				"Type": "nx_reg_move",
				"Arguments": map[string]interface{}{
					"NBits":  int64(48),
					"Src":    "eth_dst",
					"SrcOfs": int64(0),
					"Dst":    "eth_src",
					"DstOfs": int64(0),
				},
			},
		}
		if err := deep.Equal(actions, expectedActions); err != nil {
			return errors.New(spew.Sdump(err))
		}
		return nil
	})
}

func TestOVSActionCtNat(t *testing.T) {
	testOVSRule(t, "flow", "in_port=1,ip,action=ct(commit,zone=1,nat(src=10.1.1.240-10.1.1.255)),2", func(node *graph.Node) error {
		actions, _ := node.GetField("Actions")
		expectedActions := []interface{}{
			map[string]interface{}{
				"Type": "nx_ct",
				"Arguments": map[string]interface{}{
					"Alg": int64(0),
					"Flags": map[string]interface{}{
						"Commit": true,
					},
					"RecircTable": int64(255),
					"Value":       int64(1),
					"ZoneSrc":     "",
					"Actions": []interface{}{
						map[string]interface{}{
							"Type": "nx_nat",
							"Arguments": map[string]interface{}{
								"Flags":    int64(1),
								"Ipv4Min":  "10.1.1.240",
								"Ipv4Max":  "10.1.1.255",
								"Ipv6Min":  "",
								"Ipv6Max":  "",
								"ProtoMin": int64(0),
								"ProtoMax": int64(0),
								"RangePresent": map[string]interface{}{
									"Ipv4Min": true,
									"Ipv4Max": true,
								},
							},
						},
					},
				},
			},
			map[string]interface{}{
				"Type": "output",
				"Arguments": map[string]interface{}{
					"MaxLen": int64(0),
					"Port":   int64(2),
				},
			},
		}
		if err := deep.Equal(actions, expectedActions); err != nil {
			return errors.New(spew.Sdump(err))
		}
		return nil
	})
}

func TestOVSActionCtZone(t *testing.T) {
	testOVSRule(t, "flow", "ip,actions=ct(commit,zone=NXM_NX_REG4[0..15],exec(move:NXM_NX_REG3[]->NXM_NX_CT_MARK[],move:NXM_NX_REG1[]->NXM_NX_CT_LABEL[96..127]))", func(node *graph.Node) error {
		actions, _ := node.GetField("Actions")
		expectedActions := []interface{}{
			map[string]interface{}{
				"Type": "nx_ct",
				"Arguments": map[string]interface{}{
					"Alg": int64(0),
					"Flags": map[string]interface{}{
						"Commit": true,
					},
					"RecircTable": int64(255),
					"Value":       int64(15),
					"ZoneSrc":     "reg4",
					"Actions": []interface{}{
						map[string]interface{}{
							"Type": "nx_reg_move",
							"Arguments": map[string]interface{}{
								"Dst":    "conn_tracking_mark",
								"DstOfs": int64(0),
								"NBits":  int64(32),
								"Src":    "reg3",
								"SrcOfs": int64(0),
							},
						},
						map[string]interface{}{
							"Type": "nx_reg_move",
							"Arguments": map[string]interface{}{
								"Dst":    "conn_tracking_label",
								"DstOfs": int64(96),
								"NBits":  int64(32),
								"Src":    "reg1",
								"SrcOfs": int64(0),
							},
						},
					},
				},
			},
		}
		if err := deep.Equal(actions, expectedActions); err != nil {
			return errors.New(spew.Sdump(err))
		}
		return nil
	})
}

func TestOVSActionSample(t *testing.T) {
	testOVSRule(t, "flow", "actions=sample(probability=65535,collector_set_id=1,obs_domain_id=0,obs_point_id=0,sampling_port=1)", func(node *graph.Node) error {
		actions, _ := node.GetField("Actions")
		expectedActions := []interface{}{
			map[string]interface{}{
				"Type": "nx_sample2",
				"Arguments": map[string]interface{}{
					"CollectorSetId": int64(1),
					"Direction":      int64(0),
					"ObsDomainId":    int64(0),
					"ObsPointId":     int64(0),
					"Probability":    int64(65535),
					"SamplingPort":   int64(1),
				},
			},
		}
		if err := deep.Equal(actions, expectedActions); err != nil {
			return errors.New(spew.Sdump(err))
		}
		return nil
	})
}

func TestOVSActionClone(t *testing.T) {
	testOVSRule(t, "flow", "ip,actions=clone(set_field:192.168.3.3->ip_src)", func(node *graph.Node) error {
		actions, _ := node.GetField("Actions")
		expectedActions := []interface{}{
			map[string]interface{}{
				"Type": "nx_clone",
				"Arguments": map[string]interface{}{
					"Actions": []interface{}{
						map[string]interface{}{
							"Type": "set_field",
							"Arguments": map[string]interface{}{
								"Field": map[string]interface{}{
									"Type":  "ipv4_src",
									"Value": "192.168.3.3",
								},
							},
						},
					},
				},
			},
		}
		if err := deep.Equal(actions, expectedActions); err != nil {
			return errors.New(spew.Sdump(err))
		}
		return nil
	})
}

func TestOVSActionLoad(t *testing.T) {
	testOVSRule(t, "flow", "actions=load:0x1->NXM_NX_REG13[],load:0x2->OXM_OF_METADATA[]", func(node *graph.Node) error {
		actions, _ := node.GetField("Actions")
		expectedActions := []interface{}{
			map[string]interface{}{
				"Type": "nx_reg_load",
				"Arguments": map[string]interface{}{
					"OfsNbits": int64(31),
					"SrcField": "reg13",
					"Value":    int64(1),
				},
			},
			map[string]interface{}{
				"Type": "nx_reg_load",
				"Arguments": map[string]interface{}{
					"OfsNbits": int64(63),
					"SrcField": "metadata",
					"Value":    int64(2),
				},
			},
		}
		if err := deep.Equal(actions, expectedActions); err != nil {
			return errors.New(spew.Sdump(err))
		}
		return nil
	})
}

func TestOVSActionEncap(t *testing.T) {
	testOVSRule(t, "flow", "-O OpenFlow14 actions=encap(nsh(md_type=2,tlv(0x1000,10,0x12345678),tlv(0x2000,20,0xfedcba9876543210))),encap(ethernet)", func(node *graph.Node) error {
		actions, _ := node.GetField("Actions")
		expectedActions := []interface{}{
			map[string]interface{}{
				"Type": "nx_encap",
				"Arguments": map[string]interface{}{
					"HdrSize":    int64(0),
					"PacketType": "nsh",
					"Props": []interface{}{
						map[string]interface{}{
							"PropClass": int64(4),
							"Type":      int64(1),
							"Len":       int64(5),
							"MdType":    int64(2),
						},
						map[string]interface{}{
							"PropClass": int64(4),
							"Type":      int64(2),
							"Len":       int64(16),
							"TlvClass":  int64(4096),
							"TlvType":   int64(10),
							"TlvLen":    int64(4),
							"Value":     "EjRWeA==",
						},
						map[string]interface{}{
							"PropClass": int64(4),
							"Type":      int64(2),
							"Len":       int64(16),
							"TlvClass":  int64(8192),
							"TlvType":   int64(20),
							"TlvLen":    int64(8),
							"Value":     "/ty6mHZUMhA=",
						},
					},
				},
			},
			map[string]interface{}{
				"Type": "nx_encap",
				"Arguments": map[string]interface{}{
					"HdrSize":    int64(0),
					"PacketType": "ethernet",
					"Props":      nil,
				},
			},
		}
		if err := deep.Equal(actions, expectedActions); err != nil {
			return errors.New(spew.Sdump(err))
		}
		return nil
	})
}

func TestOVSActionWriteActions(t *testing.T) {
	testOVSRule(t, "flow", "-O OpenFlow14 actions=write_actions(output:6)", func(node *graph.Node) error {
		actions, _ := node.GetField("WriteActions")
		expectedActions := []interface{}{
			map[string]interface{}{
				"Type": "output",
				"Arguments": map[string]interface{}{
					"MaxLen": int64(0),
					"Port":   int64(6),
				},
			},
		}
		if err := deep.Equal(actions, expectedActions); err != nil {
			return errors.New(spew.Sdump(err))
		}
		return nil
	})
}

func TestOVSGroupAdd(t *testing.T) {
	testOVSRule(t, "group", "-O OpenFlow14 group_id=1,type=all,bucket=bucket_id:0,actions=set_field:00:00:00:11:11:11->eth_src,set_field:00:00:00:22:22:22->eth_dst,output:2,bucket=bucket_id:1,actions=set_field:00:00:00:11:11:11->eth_src,set_field:00:00:00:22:22:22->eth_dst,output:3", func(node *graph.Node) error {
		metadata := map[string]interface{}(node.Metadata)
		delete(metadata, "Metric")

		expectedMetadata := map[string]interface{}{
			"GroupId":   int64(1),
			"GroupType": "all",
			"Type":      "ofgroup",
			"Buckets": []interface{}{
				map[string]interface{}{
					"Len":        int64(0),
					"Weight":     int64(0),
					"WatchPort":  int64(4294967295),
					"WatchGroup": int64(4294967295),
					"Actions": []interface{}{
						map[string]interface{}{
							"Type": "set_field",
							"Arguments": map[string]interface{}{
								"Field": map[string]interface{}{
									"Type":  "eth_src",
									"Value": "00:00:00:11:11:11",
								},
							},
						},
						map[string]interface{}{
							"Type": "set_field",
							"Arguments": map[string]interface{}{
								"Field": map[string]interface{}{
									"Type":  "eth_dst",
									"Value": "00:00:00:22:22:22",
								},
							},
						},
						map[string]interface{}{
							"Type": "output",
							"Arguments": map[string]interface{}{
								"MaxLen": int64(0),
								"Port":   int64(2),
							},
						},
					},
				},
				map[string]interface{}{
					"Len":        int64(0),
					"Weight":     int64(0),
					"WatchPort":  int64(4294967295),
					"WatchGroup": int64(4294967295),
					"Actions": []interface{}{
						map[string]interface{}{
							"Type": "set_field",
							"Arguments": map[string]interface{}{
								"Field": map[string]interface{}{
									"Type":  "eth_src",
									"Value": "00:00:00:11:11:11",
								},
							},
						},
						map[string]interface{}{
							"Type": "set_field",
							"Arguments": map[string]interface{}{
								"Field": map[string]interface{}{
									"Type":  "eth_dst",
									"Value": "00:00:00:22:22:22",
								},
							},
						},
						map[string]interface{}{
							"Type": "output",
							"Arguments": map[string]interface{}{
								"MaxLen": int64(0),
								"Port":   int64(3),
							},
						},
					},
				},
			},
		}
		if err := deep.Equal(metadata, expectedMetadata); err != nil {
			return errors.New(spew.Sdump(err))
		}
		return nil
	})
}
