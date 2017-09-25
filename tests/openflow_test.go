package tests

import (
	"fmt"
	"testing"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/tests/helper"
)

type ruleCmd struct {
	rule string
	add  bool
}

func verify(c *CheckContext, expected []int) error {
	for i, e := range expected {
		gh := c.gh
		gremlin := "g"
		if !c.time.IsZero() {
			gremlin += fmt.Sprintf(".Context(%d)", common.UnixMillis(c.time))
		}

		gremlin += `.V().Has("Type", "ovsbridge", "Name", "br-test1")`
		gremlin += fmt.Sprintf(`.Out("Type", "ofrule").Has("actions","resubmit(;%d)")`, i+1)
		nodes, err := gh.GetNodes(gremlin)
		if err != nil {
			return err
		}
		l := len(nodes)
		if l != e {
			return fmt.Errorf("expected %d rules with 'resubmit(,%d)' but got %d - %v", e, i+1, l, nodes)
		}
	}
	return nil
}

func makeTest(t *testing.T, rules []ruleCmd, expected []int) {
	setupCmds := []helper.Cmd{
		{"ovs-vsctl add-br br-test1", true},
		{"ovs-vsctl add-port br-test1 intf1 -- set interface intf1 type=internal", true},
		{"ovs-vsctl add-port br-test1 intf2 -- set interface intf2 type=internal", true},
	}

	for _, ruleCmd := range rules {
		var cmd string
		if ruleCmd.add {
			cmd = fmt.Sprintf("ovs-ofctl add-flow br-test1 %s", ruleCmd.rule)
		} else {
			cmd = fmt.Sprintf("ovs-ofctl del-flows --strict br-test1 %s", ruleCmd.rule)
		}
		setupCmds = append(setupCmds, helper.Cmd{Cmd: cmd, Check: true})
	}

	test := &Test{
		setupCmds: setupCmds,

		tearDownCmds: []helper.Cmd{
			{"ovs-vsctl del-br br-test1", true},
		},

		checks: []CheckFunction{func(c *CheckContext) error {
			return verify(c, expected)
		}},
	}

	RunTest(t, test)
}

func TestAddOFRule(t *testing.T) {
	makeTest(
		t,
		[]ruleCmd{{"table=0,priority=0,in_port=1,actions=resubmit(,1)", true}},
		[]int{1})
}

func TestDelOFRule(t *testing.T) {
	makeTest(
		t,
		[]ruleCmd{
			{"table=0,priority=0,in_port=1,actions=resubmit(,1)", true},
			{"table=0,priority=0,in_port=1", false},
		},
		[]int{0})
}

func TestSuperimposedOFRule(t *testing.T) {
	makeTest(
		t,
		[]ruleCmd{
			{"table=0,priority=1,actions=resubmit(,1)", true},
			{"table=0,priority=2,actions=resubmit(,2)", true},
			{"table=0,priority=0,in_port=1,actions=resubmit(,3)", true},
			{"table=0,priority=3,actions=resubmit(,4)", true},
			{"table=0,priority=2", false},
		},
		[]int{1, 0, 1, 1})
}

func TestDelRuleWithBridge(t *testing.T) {
	setupCmds := []helper.Cmd{
		{"ovs-vsctl add-br br-test1", true},
		{"ovs-vsctl add-port br-test1 intf1 -- set interface intf1 type=internal", true},
		{"ovs-vsctl add-port br-test1 intf2 -- set interface intf2 type=internal", true},
		{"sleep 1", true},
		{"ovs-ofctl add-flow br-test1 table=0,priority=1,actions=resubmit(,1)", true},
		{"sleep 1", true},
		{"ovs-vsctl del-br br-test1", true},
	}

	test := &Test{
		setupCmds: setupCmds,

		tearDownCmds: []helper.Cmd{},

		checks: []CheckFunction{func(c *CheckContext) error { return verify(c, []int{0}) }},
	}

	RunTest(t, test)
}
