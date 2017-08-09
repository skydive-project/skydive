package tests

import (
	"fmt"
	"testing"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/tests/helper"
)

func TestAddOFRule(t *testing.T) {
	test := &Test{
		setupCmds: []helper.Cmd{
			{"ovs-vsctl add-br br-test1", true},
			{"ovs-vsctl add-port br-test1 intf1 -- set interface intf1 type=internal", true},
			{"ovs-vsctl add-port br-test1 intf2 -- set interface intf2 type=internal", true},
			{"ovs-ofctl add-flow br-test1 table=0,priority=0,in_port=1,actions=drop", true},
		},

		tearDownCmds: []helper.Cmd{
			{"ovs-vsctl del-br br-test1", true},
		},

		check: func(c *TestContext) error {
			gh := c.gh
			gremlin := "g"
			if !c.time.IsZero() {
				gremlin += fmt.Sprintf(".Context(%d)", common.UnixMillis(c.time))
			}

			gremlin += `.V().Has("Type", "ovsbridge", "Name", "br-test1")`
			gremlin += `.Out("Type", "ofrule").Has("actions","drop")`

			nodes, err := gh.GetNodes(gremlin)
			if err != nil {
				return err
			}

			if len(nodes) != 1 {
				return fmt.Errorf("Expected 1 node, got %+v", nodes)
			}

			return nil
		},
	}

	RunTest(t, test)
}

func TestDelOFRule(t *testing.T) {
	test := &Test{
		setupCmds: []helper.Cmd{
			{"ovs-vsctl add-br br-test1", true},
			{"ovs-vsctl add-port br-test1 intf1 -- set interface intf1 type=internal", true},
			{"ovs-vsctl add-port br-test1 intf2 -- set interface intf2 type=internal", true},
			{"ovs-ofctl add-flow br-test1 table=0,priority=0,in_port=1,actions=drop", true},
			{"ovs-ofctl del-flows br-test1 table=0,in_port=1", true},
		},

		tearDownCmds: []helper.Cmd{
			{"ovs-vsctl del-br br-test1", true},
		},

		check: func(c *TestContext) error {
			gh := c.gh
			gremlin := "g"
			if !c.time.IsZero() {
				gremlin += fmt.Sprintf(".Context(%d)", common.UnixMillis(c.time))
			}

			gremlin += `.V().Has("Type", "ovsbridge", "Name", "br-test1")`
			gremlin += `.Out("Type", "ofrule").Has("actions","drop")`

			nodes, err := gh.GetNodes(gremlin)
			if err != nil {
				return err
			}

			if len(nodes) != 0 {
				return fmt.Errorf("Expected 1 node, got %+v", nodes)
			}

			return nil
		},
	}

	RunTest(t, test)
}
