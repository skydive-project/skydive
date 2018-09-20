// +build opencontrail_tests

package tests

import (
	"fmt"
	"testing"
)

func TestOpenContrailTopology(t *testing.T) {
	test := &Test{
		setupCmds: []Cmd{
			{"contrail-create-network.py default-domain:default-project:vn1", true},
			{"netns-daemon-start -n default-domain:default-project:vn1 vm1", true},
		},

		tearDownCmds: []Cmd{
			// We should delete the net
			{"netns-daemon-stop vm1", true},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			gremlin := c.gremlin.V().Has("Contrail")

			nodes, err := c.gh.GetNodes(gremlin)
			if err != nil {
				return err
			}

			if len(nodes) != 1 {
				return fmt.Errorf("Expected 1 node, got %+v", nodes)
			}

			return nil
		}},
	}

	RunTest(t, test)
}

func TestOpenContrailRoutingTable(t *testing.T) {
	test := &Test{
		setupCmds: []Cmd{
			{"contrail-create-network.py default-domain:default-project:vn1", true},
			{"netns-daemon-start -n default-domain:default-project:vn1 vm1", true},
		},
		tearDownCmds: []Cmd{
			// We should delete the net
			{"netns-daemon-stop vm1", true},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			gremlin := c.gremlin.V().Has("Contrail.RoutingTable.Prefix", "20.1.1.253/32")
			nodes, err := c.gh.GetNodes(gremlin)
			if err != nil {
				return err
			}

			if len(nodes) != 1 {
				return fmt.Errorf("Expected 1 node, got %+v", nodes)
			}
			return nil
		}},
	}

	RunTest(t, test)
}
