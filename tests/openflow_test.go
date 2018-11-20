package tests

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/skydive-project/skydive/common"
)

type ruleCmd struct {
	rule string
	add  bool
}

func checkTest(t *testing.T) {
	if noOFTests {
		t.Skip("Don't run OpenFlows test as /usr/bin/ovs-ofctl didn't exist in agent process namespace")
	}
}

func verify(c *CheckContext, bridge string, expected []int) error {
	l := len(expected)
	found := make([]int, l)
	gremlin := c.gremlin.V().Has("Type", "ovsbridge", "Name", bridge).Out("Type", "ofrule")
	nodes, _ := c.gh.GetNodes(gremlin)
	for _, node := range nodes {
		metadata := node.Metadata
		actions := metadata["Actions"].([]interface{})
		if len(actions) != 1 {
			continue
		}
		action := actions[0].(map[string]interface{})
		f := action["Function"].(string)
		if f != "resubmit" {
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
	checkTest(t)
	bridge := "br-of" + suffix
	intf1 := "intf1-" + suffix
	intf2 := "intf2-" + suffix
	setupCmds := []Cmd{
		{"ovs-vsctl --if-exists del-br " + bridge, true},
		{"ovs-vsctl add-br " + bridge, true},
		{"ovs-vsctl add-port " + bridge + " " + intf1 + " -- set interface " + intf1 + " type=internal", true},
		{"ovs-vsctl add-port " + bridge + " " + intf2 + " -- set interface " + intf2 + " type=internal", true},
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
	checkTest(t)
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
					if action["Function"] == "resubmit" {
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
	checkTest(t)
	setupCmds := []Cmd{
		{"ovs-vsctl --if-exists del-br br-of6", true},
		{"ovs-vsctl add-br br-of6", true},
		{"ovs-vsctl set bridge br-of6 protocols=OpenFlow10,OpenFlow15", true},
		{"ovs-vsctl add-port br-of6 intf1-6 -- set interface intf1-6 type=internal", true},
		{"ovs-vsctl add-port br-of6 intf2-6 -- set interface intf2-6 type=internal", true},
		{"sleep 1", true},
		{"ovs-ofctl -O OpenFlow15 add-group br-of6 group_id=123,type=all,bucket=actions=1,bucket=actions=resubmit(,1)", true},
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
	checkTest(t)
	setupCmds := []Cmd{
		{"ovs-vsctl --if-exists del-br br-of7", true},
		{"ovs-vsctl add-br br-of7", true},
		{"ovs-vsctl set bridge br-of7 protocols=OpenFlow10,OpenFlow15", true},
		{"ovs-vsctl add-port br-of7 intf1-7 -- set interface intf1-7 type=internal", true},
		{"ovs-vsctl add-port br-of7 intf2-7 -- set interface intf2-7 type=internal", true},
		{"ovs-ofctl -O OpenFlow15 add-group br-of7 group_id=123,type=all,bucket=actions=1", true},
		{"sleep 1", true},
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
	checkTest(t)
	setupCmds := []Cmd{
		{"ovs-vsctl --if-exists del-br br-of4", true},
		{"ovs-vsctl add-br br-of8", true},
		{"ovs-vsctl set bridge br-of8 protocols=OpenFlow10,OpenFlow15", true},
		{"ovs-vsctl add-port br-of8 intf1-8 -- set interface intf1-8 type=internal", true},
		{"ovs-vsctl add-port br-of8 intf2-8 -- set interface intf2-8 type=internal", true},
		{"sleep 1", true},
		{"ovs-ofctl -O OpenFlow15 add-group br-of8 group_id=123,type=all,bucket=actions=1", true},
		{"ovs-ofctl -O OpenFlow15 del-groups br-of8", true},
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
