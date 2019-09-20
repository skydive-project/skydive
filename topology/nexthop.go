//go:generate go run github.com/skydive-project/skydive/scripts/gendecoder -output nexthop_gendecoder.go
//go:generate go run github.com/safchain/easyjson/easyjson $GOFILE

/*
 * Copyright (C) 2019 Red Hat, Inc.
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

package topology

import (
	"net"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/graffiti/graph"
)

// NextHop describes a next hop
// easyjson:json
// gendecoder
type NextHop struct {
	Priority int64  `json:"Priority"`
	IP       net.IP `json:"IP,omitempty"`
	MAC      string `json:"MAC,omitempty"`
	IfIndex  int64  `json:"IfIndex"`
}

// GetNextHop returns the next hop to reach a specified IP
func GetNextHop(node *graph.Node, ip net.IP) (*NextHop, error) {
	var neighbors *Neighbors
	if n, err := node.GetField("Neighbors"); err == nil {
		neighbors = n.(*Neighbors)
	}

	tables, err := node.GetField("RoutingTables")
	if err != nil {
		return nil, err
	}

	// TODO: use policies to select routing tables
	rts := tables.(*RoutingTables)
	for _, t := range *rts {
		var defaultRouteIP net.IP
		var defaultIfIndex int64
		var nh *NextHop

		getNeighbor := func(ip net.IP) string {
			if neighbors != nil {
				return neighbors.getMAC(ip)
			}
			return ""
		}

		for _, r := range t.Routes {
			ipnet := net.IPNet(r.Prefix)
			if r.Prefix.IsDefaultRoute() {
				defaultRouteIP = r.NextHops[0].IP
				defaultIfIndex = r.NextHops[0].IfIndex
			} else if ipnet.Contains(ip) {
				nh = &NextHop{IfIndex: r.NextHops[0].IfIndex}

				if r.NextHops[0].IP != nil {
					nh.IP = r.NextHops[0].IP
					nh.MAC = getNeighbor(nh.IP)

					// dedicated NH so return here
					return nh, nil
				}

				// same network but maybe a dedicated route, keep checking
				nh.IP = ip
				nh.MAC = getNeighbor(nh.IP)
			}
		}

		// one route found
		if nh != nil {
			return nh, nil
		}

		// no route found try with the default
		if defaultRouteIP != nil {
			nh := &NextHop{IP: defaultRouteIP, IfIndex: defaultIfIndex}
			nh.MAC = getNeighbor(nh.IP)

			return nh, nil
		}
	}

	return nil, common.ErrNotFound
}
