package mod

import (
	"testing"
)

func newTestResolveDocker(t *testing.T) Resolver {
	return &resolveDocker{gremlinClient: newLocalGremlinQueryHelper(newDockerTopologyGraph(t))}
}

func TestResolveDockerShouldFindContainerNameWithMultipleNetworkInterfaces(t *testing.T) {
	r := newTestResolveDocker(t)
	expected := "0_0_pinger-container-1_0"
	nameTIDs := []string{
		"eac0f98c-2ab0-5b89-6490-9e8816f8cba3", // eth0 interface inside the container netns
		"38e2f253-2305-5e91-5af5-2bfcab208b1a", // veth interface
		"577f878a-1e7f-5b2d-60f0-efc7ff5da510", // docker0 bridge
	}
	for _, nameTID := range nameTIDs {
		actual, err := r.IPToName("172.17.0.3", nameTID)
		if err != nil {
			t.Fatalf("IPToName failed: %v", err)
		}
		if actual != expected {
			t.Errorf("Expected: %v, got: %v", expected, actual)
		}
	}
}

func TestResolveDockerShouldNotFindNameOfNonExistingIP(t *testing.T) {
	r := newTestResolveDocker(t)
	actual, err := r.IPToName("8.7.6.5", "eac0f98c-2ab0-5b89-6490-9e8816f8cba3")
	if err == nil {
		t.Errorf("Expected error but got none")
	}
	if actual != "" {
		t.Errorf("Expected empty response, got: %v", actual)
	}
}

func TestResolveDockerShouldFindTIDType(t *testing.T) {
	r := newTestResolveDocker(t)
	expected := "bridge"
	actual, err := r.TIDToType("577f878a-1e7f-5b2d-60f0-efc7ff5da510")
	if err != nil {
		t.Fatalf("TIDToType failed: %v", err)
	}
	if actual != expected {
		t.Errorf("Expected: %v, got: %v", expected, actual)
	}
}

func TestResolveDockerShouldNotFindTIDTypeOfNonExistingTID(t *testing.T) {
	r := newTestResolveDocker(t)
	actual, err := r.TIDToType("11111111-1111-1111-1111-111111111111")
	if err == nil {
		t.Error("Expected error but got none")
	}
	if actual != "" {
		t.Errorf("Expected empty response, got: %v", actual)
	}
}
