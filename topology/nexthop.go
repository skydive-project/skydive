//go:generate go run ../scripts/gendecoder.go -output nexthop_gendecoder.go

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

		for _, r := range t.Routes {
			ipnet := net.IPNet(r.Prefix)
			if r.Prefix.IsDefaultRoute() {
				defaultRouteIP = r.NextHops[0].IP
				defaultIfIndex = r.NextHops[0].IfIndex
			} else if ipnet.Contains(ip) {
				nextIP := r.NextHops[0].IP
				nh := &NextHop{IfIndex: r.NextHops[0].IfIndex}
				if nextIP != nil {
					nh.IP = nextIP
					if neighbors != nil {
						nh.MAC = neighbors.getMAC(nextIP)
					}
				}
				return nh, nil
			}
		}

		if defaultRouteIP != nil {
			nh := &NextHop{IP: defaultRouteIP, IfIndex: defaultIfIndex}
			if neighbors != nil {
				nh.MAC = neighbors.getMAC(defaultRouteIP)
			}
			return nh, nil
		}
	}

	return nil, common.ErrNotFound
}
