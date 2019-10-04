package mod

import (
	"testing"
)

func newTestResolveRunc(t *testing.T) Resolver {
	return &resolveRunc{gremlinClient: newLocalGremlinQueryHelper(newRuncTopologyGraph(t))}
}

func TestResolveRuncShouldFindContainerName(t *testing.T) {
	r := newTestResolveRunc(t)
	expected := "0_0_my-container-name-5bbc557665-h66vq_0"
	actual, err := r.IPToName("172.30.149.34", "ce2ed4fb-1340-57b1-796f-5d648665aed7")
	if err != nil {
		t.Fatalf("IPToName failed: %v", err)
	}
	if actual != expected {
		t.Errorf("Expected: %v, got: %v", expected, actual)
	}
}

func TestResolveRuncShouldNotFindNameOfNonExistingIP(t *testing.T) {
	r := newTestResolveRunc(t)
	actual, err := r.IPToName("11.22.33.44", "ce2ed4fb-1340-57b1-796f-5d648665aed7")
	if err == nil {
		t.Errorf("Expected error but got none")
	}
	if actual != "" {
		t.Errorf("Expected empty response, got: %v", actual)
	}
}

func TestResolveRuncShouldFindTIDType(t *testing.T) {
	r := newTestResolveRunc(t)
	expected := "netns"
	actual, err := r.TIDToType("ce2ed4fb-1340-57b1-796f-5d648665aed7")
	if err != nil {
		t.Fatalf("TIDToType failed: %v", err)
	}
	if actual != expected {
		t.Errorf("Expected: %v, got: %v", expected, actual)
	}
}

func TestResolveRuncShouldNotFindTIDTypeOfNonExistingTID(t *testing.T) {
	r := newTestResolveRunc(t)
	actual, err := r.TIDToType("11111111-1111-1111-1111-111111111111")
	if err == nil {
		t.Error("Expected error but got none")
	}
	if actual != "" {
		t.Errorf("Expected empty response, got: %v", actual)
	}
}
