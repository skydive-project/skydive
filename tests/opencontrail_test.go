// +build opencontrail

package tests

import (
	"fmt"
	"testing"

	g "github.com/skydive-project/skydive/gremlin"
	"github.com/skydive-project/skydive/tests/helper"
)

func TestOpenContrailTopology(t *testing.T) {
	test := &Test{
		setupCmds: []helper.Cmd{
			{"contrail-create-network.py default-domain:default-project:vn1", true},
			{"netns-daemon-start -n default-domain:default-project:vn1 vm1", true},
		},
		tearDownCmds: []helper.Cmd{
			// We should delete the net
			{"netns-daemon-stop vm1", true},
		},

		checks: []CheckFunction{func(c *CheckContext) error {
			gh := c.gh
			gremlin := g.G.V().Has("Contrail")

			nodes, err := gh.GetNodes(gremlin)
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
