/*
 * Copyright (C) 2018 Red Hat, Inc.
 *
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 *
 */

package netlink

import "net"

// RoutingTable describes a list of Routes
// easyjson:json
type RoutingTable struct {
	ID     int64    `json:"Id"`
	Src    net.IP   `json:"Src,omitempty"`
	Routes []*Route `json:"Routes,omitempty"`
}

// Route describes a route
// easyjson:json
type Route struct {
	Protocol int64      `json:"Protocol,omitempty"`
	Prefix   string     `json:"Prefix,omitempty"`
	Nexthops []*NextHop `json:"Nexthops,omitempty"`
}

// NextHop describes a next hop
// easyjson:json
type NextHop struct {
	Priority int64  `json:"Priority,omitempty"`
	IP       net.IP `json:"Src,omitempty"`
	IfIndex  int64  `json:"IfIndex,omitempty"`
}

// GetRoute returns route for the given protocol and prefix
func (rt *RoutingTable) GetRoute(protocol int64, prefix string) *Route {
	for _, r := range rt.Routes {
		if r.Protocol == protocol && r.Prefix == prefix {
			return r
		}
	}
	return nil
}

// GetOrCreateRoute creates if not existing a new route and returns it
func (rt *RoutingTable) GetOrCreateRoute(protocol int64, prefix string) *Route {
	if r := rt.GetRoute(protocol, prefix); r != nil {
		return r
	}
	r := &Route{
		Protocol: protocol,
		Prefix:   prefix,
	}
	rt.Routes = append(rt.Routes, r)
	return r
}

// GetNexthop returns the nexthop for the given ip and ifindex
func (r *Route) GetNexthop(ip net.IP, ifIndex int64) *NextHop {
	for _, n := range r.Nexthops {
		if n.IP.Equal(ip) && n.IfIndex == ifIndex {
			return n
		}
	}
	return nil
}

// GetOrCreateNexthop creates if not existing a new nexthop and returns it
func (r *Route) GetOrCreateNexthop(ip net.IP, ifIndex int64, priority int64) *NextHop {
	if n := r.GetNexthop(ip, ifIndex); n != nil {
		return n
	}
	nh := &NextHop{
		IP:       ip,
		IfIndex:  ifIndex,
		Priority: priority,
	}
	r.Nexthops = append(r.Nexthops, nh)
	return nh
}
