/*
 * Copyright (C) 2018 Red Hat, Inc.
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

package traversal

import (
	"net"
	"strings"
	"testing"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
	"github.com/skydive-project/skydive/topology/probes/netlink"
)

func execNextHopQuery(t *testing.T, g *graph.Graph, query string) traversal.GraphTraversalStep {
	tr := traversal.NewGremlinTraversalParser()
	tr.AddTraversalExtension(NewNextHopTraversalExtension())

	ts, err := tr.Parse(strings.NewReader(query))
	if err != nil {
		t.Fatalf("%s: %s", query, err)
	}

	res, err := ts.Exec(g, false)
	if err != nil {
		t.Fatalf("%s: %s", query, err)
	}

	return res
}

/*This will test the nexthop in neighbors*/
func TestNextHopStep1(t *testing.T) {
	t.Skip("this usecase removed.")
	g := newGraph(t)
	var neighbors netlink.Neighbors
	neighbor := &netlink.Neighbor{
		IP:      net.ParseIP("192.168.0.2"),
		IfIndex: 2,
		MAC:     "fa:16:3e:c1:e8:d1",
	}
	neighbors = append(neighbors, neighbor)
	m1 := graph.Metadata{
		"Neighbors": &neighbors,
	}
	n, _ := g.NewNode(graph.GenID(), m1)
	res := execNextHopQuery(t, g, "g.v().NextHop('192.168.0.2')")

	if len(res.Values()) != 1 {
		t.Fatalf("Should return 1 result, returned: %v", res.Values())
	}

	nexthops := res.Values()[0].(map[string]*NextHop)
	nexthop, ok := nexthops[string(n.ID)]
	if !ok {
		t.Fatalf("Node entry not found")
	}
	if nexthop.IP != "192.168.0.2" {
		t.Fatalf("IP not matching")
	}
}

/*Find the nexthop IP in routing table and
find the MAC in neighbors*/
func TestNextHopStep2(t *testing.T) {
	g := newGraph(t)
	var neighbors netlink.Neighbors
	neighbor := &netlink.Neighbor{
		IP:      net.ParseIP("10.16.0.2"),
		IfIndex: 2,
		MAC:     "fa:16:3e:c1:e8:d1",
	}
	neighbors = append(neighbors, neighbor)

	var nhs []*netlink.NextHop
	nh := &netlink.NextHop{
		IP:      net.ParseIP("10.16.0.2"),
		IfIndex: 2,
	}
	nhs = append(nhs, nh)

	var routes []*netlink.Route
	_, cidr, _ := net.ParseCIDR("192.168.0.0/24")
	route := &netlink.Route{
		Prefix:   netlink.Prefix(*cidr),
		NextHops: nhs,
	}
	routes = append(routes, route)

	var routingtables netlink.RoutingTables
	routingtable := &netlink.RoutingTable{
		ID:     255,
		Routes: routes,
	}
	routingtables = append(routingtables, routingtable)

	m1 := graph.Metadata{
		"Neighbors":     &neighbors,
		"RoutingTables": &routingtables,
	}

	n, _ := g.NewNode(graph.GenID(), m1)
	res := execNextHopQuery(t, g, "g.v().NextHop('192.168.0.5')")

	if len(res.Values()) != 1 {
		t.Fatalf("Should return 1 result, returned: %v", res.Values())
	}

	nexthops := res.Values()[0].(map[string]*NextHop)
	nexthop, ok := nexthops[string(n.ID)]
	if !ok {
		t.Fatalf("Node entry not found")
	}
	if nexthop.IP != "10.16.0.2" {
		t.Fatalf("IP not matching, got: %s", nexthop.IP)
	}
}

/* return default nexthop*/
func TestNextHopStep3(t *testing.T) {
	g := newGraph(t)
	var neighbors netlink.Neighbors
	neighbor := &netlink.Neighbor{
		IP:      net.ParseIP("10.16.0.12"),
		IfIndex: 2,
		MAC:     "fa:16:3e:ce:e8:d1",
	}
	neighbors = append(neighbors, neighbor)

	var nhs []*netlink.NextHop
	nh := &netlink.NextHop{
		IP:      net.ParseIP("10.16.0.12"),
		IfIndex: 2,
	}
	nhs = append(nhs, nh)

	var routes []*netlink.Route
	route := &netlink.Route{
		Prefix:   netlink.Prefix(netlink.IPv4DefaultRoute),
		NextHops: nhs,
	}
	routes = append(routes, route)

	var routingtables netlink.RoutingTables
	routingtable := &netlink.RoutingTable{
		ID:     255,
		Routes: routes,
	}
	routingtables = append(routingtables, routingtable)

	m1 := graph.Metadata{
		"Neighbors":     &neighbors,
		"RoutingTables": &routingtables,
	}

	n, _ := g.NewNode(graph.GenID(), m1)
	res := execNextHopQuery(t, g, "g.v().NextHop('8.8.8.8')")

	if len(res.Values()) != 1 {
		t.Fatalf("Should return 1 result, returned: %v", res.Values())
	}

	nexthops := res.Values()[0].(map[string]*NextHop)
	nexthop, ok := nexthops[string(n.ID)]
	if !ok {
		t.Fatalf("Node entry not found")
	}
	if nexthop.IP != "10.16.0.12" {
		t.Fatalf("IP not matching, got: %s", nexthop.IP)
	}
}

/* select correct interface over default one*/
func TestNextHopStep4(t *testing.T) {
	g := newGraph(t)
	var neighbors netlink.Neighbors
	neighbor1 := &netlink.Neighbor{
		IP:      net.ParseIP("10.16.0.12"),
		IfIndex: 2,
		MAC:     "fa:16:3e:ce:e8:d1",
	}
	neighbor2 := &netlink.Neighbor{
		IP:      net.ParseIP("192.64.0.1"),
		IfIndex: 2,
		MAC:     "af:16:3e:de:e8:d3",
	}

	neighbors = append(neighbors, neighbor1, neighbor2)

	var nhs1 []*netlink.NextHop
	nhs1 = append(nhs1, &netlink.NextHop{
		IP:      net.ParseIP("10.16.0.12"),
		IfIndex: 2,
	})
	var nhs2 []*netlink.NextHop
	nhs2 = append(nhs2, &netlink.NextHop{
		IP:      net.ParseIP("192.64.0.1"),
		IfIndex: 2,
	})

	var routes []*netlink.Route
	routes = append(routes, &netlink.Route{
		NextHops: nhs1,
	})
	_, cidr, _ := net.ParseCIDR("10.16.0.0/24")
	routes = append(routes, &netlink.Route{
		NextHops: nhs2,
		Prefix:   netlink.Prefix(*cidr),
	})

	var routingtables netlink.RoutingTables
	routingtables = append(routingtables, &netlink.RoutingTable{
		ID:     255,
		Routes: routes,
	})

	m1 := graph.Metadata{
		"Neighbors":     &neighbors,
		"RoutingTables": &routingtables,
	}

	n, _ := g.NewNode(graph.GenID(), m1)
	res := execNextHopQuery(t, g, "g.v().NextHop('10.16.0.3')")

	if len(res.Values()) != 1 {
		t.Fatalf("Should return 1 result, returned: %v", res.Values())
	}

	nexthops := res.Values()[0].(map[string]*NextHop)
	nexthop, ok := nexthops[string(n.ID)]
	if !ok {
		t.Fatalf("Node entry not found")
	}
	if nexthop.IP != "192.64.0.1" {
		t.Fatalf("IP not matching, got: %s", nexthop.IP)
	}
}

/*Return interface index if nexthop doesn't have IP*/
func TestNextHopStep5(t *testing.T) {
	g := newGraph(t)

	var nhs []*netlink.NextHop
	nhs = append(nhs, &netlink.NextHop{
		IfIndex: 5,
	})

	var routes []*netlink.Route
	_, cidr, _ := net.ParseCIDR("10.60.0.0/24")
	routes = append(routes, &netlink.Route{
		NextHops: nhs,
		Prefix:   netlink.Prefix(*cidr),
	})

	var routingtables netlink.RoutingTables
	routingtables = append(routingtables, &netlink.RoutingTable{
		ID:     255,
		Routes: routes,
	})

	m1 := graph.Metadata{
		"RoutingTables": &routingtables,
	}

	n, _ := g.NewNode(graph.GenID(), m1)
	res := execNextHopQuery(t, g, "g.v().NextHop('10.60.0.5')")

	if len(res.Values()) != 1 {
		t.Fatalf("Should return 1 result, returned: %v", res.Values())
	}

	nexthops := res.Values()[0].(map[string]*NextHop)
	nexthop, ok := nexthops[string(n.ID)]
	if !ok {
		t.Fatalf("Node entry not found")
	}
	if nexthop.IP != "" {
		t.Fatalf("IP not matching, got: %s", nexthop.IP)
	}
	if nexthop.IfIndex != 5 {
		t.Fatalf("Interface index not matching, got: %d", nexthop.IfIndex)
	}
}
