package tests

import (
	"fmt"
	"testing"

	"github.com/skydive-project/skydive/api/client"
	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/common"
	shttp "github.com/skydive-project/skydive/http"
)

func TestStartupCapture(t *testing.T) {
	test := &Test{
		mode: OneShot,

		setupCmds: []Cmd{
			{"ovs-vsctl add-br br-startup", true},
			{"ovs-vsctl add-port br-startup startup-intf2 -- set interface startup-intf2 type=internal", true},
			{"ip netns add startup-vm2", true},
			{"ip link set startup-intf2 netns startup-vm2", true},
			{"ip netns exec startup-vm2 ip address add 169.254.33.34/24 dev startup-intf2", true},
			{"ip netns exec startup-vm2 ip link set startup-intf2 up", true},
		},

		tearDownCmds: []Cmd{
			{"ip netns del startup-vm2", true},
			{"ovs-vsctl del-br br-startup", true},
		},

		checks: []CheckFunction{func(c *CheckContext) error {
			//Get config captures
			var captures map[string]types.Capture
			cl, err := client.NewCrudClientFromConfig(&shttp.AuthenticationOpts{})
			if err != nil {
				return err
			}
			if err := cl.List("capture", &captures); err != nil {
				return err
			}

			gremlin := "g.V().Has('Name','startup-vm2')"
			var capture *types.Capture
			for _, tc := range captures {
				if gremlin == tc.GremlinQuery {
					capture = types.NewCapture(tc.GremlinQuery, tc.BPFFilter)
					break
				}
			}
			if capture == nil {
				return fmt.Errorf("Preconfigured capture wasn't created")
			}

			gh := client.NewGremlinQueryHelper(&shttp.AuthenticationOpts{})

			nodes, err := gh.GetNodes(capture.GremlinQuery)
			if err != nil {
				return err
			}

			if len(nodes) == 0 {
				return fmt.Errorf("No node matching capture %s", capture.GremlinQuery)
			}

			for _, node := range nodes {
				tp, err := node.GetFieldString("Type")
				if err != nil || !common.IsCaptureAllowed(tp) {
					continue
				}
				captureID, err := node.GetFieldString("Capture.ID")
				if err != nil {
					return fmt.Errorf("Node %+v matched the capture but capture is not enabled", node)
				}
				if captureID != capture.ID() {
					t.Errorf("Node %s matches multiple captures - %s", node.ID, node.String())
				}
			}
			return nil
		}},
	}
	RunTest(t, test)
}
