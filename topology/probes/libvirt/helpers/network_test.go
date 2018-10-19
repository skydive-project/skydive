package helpers

import (
	"reflect"
	"testing"

	"github.com/skydive-project/skydive/topology/graph/realtime"
)

func TestFindInterfacesVmConnectedThrough(t *testing.T) {
	nodes, _ := FindInterfacesVMConnectedThrough(`
<domain type='xen' id='13'>
<devices>
    <interface type='bridge'>
      <target dev='target0'/>
    </interface>
</devices>
</domain>
`, "")
	nodesCompareTo := make(map[string]realtime.NodesConnectedToType)
	nodesCompareTo["target0"] = realtime.NodesConnectedToType{
		Name: "target0",
		Type: realtime.VLAYER_CONNECTION_TYPE,
		Metadata: map[string]string{
			"Libvirt.MAC":     "",
			"Libvirt.Address": ":...",
			"Libvirt.Alias":   "",
			"PeerIntfMAC":     "",
		},
	}

	if !reflect.DeepEqual(nodes, nodesCompareTo) {
		t.Errorf("nodes vm connected through not matched, expected %+v, got %+v", nodesCompareTo, nodes)
	}
}
