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

package netlink

import (
	"encoding/json"
	"fmt"
	"net"
	"reflect"

	"github.com/skydive-project/skydive/common"
)

var (
	// IPv4DefaultRoute default IPv4 route
	IPv4DefaultRoute    = net.IPNet{IP: net.IPv4zero, Mask: net.CIDRMask(0, 8*net.IPv4len)}
	ipv4DefaultRouteStr = IPv4DefaultRoute.String()
	// IPv6DefaultRoute default IPv6 route
	IPv6DefaultRoute    = net.IPNet{IP: net.IPv6zero, Mask: net.CIDRMask(0, 8*net.IPv6len)}
	ipv6DefaultRouteStr = IPv6DefaultRoute.String()
)

// IsDefaultRoute return whether the given cidr is a default route
func (p *Prefix) IsDefaultRoute() bool {
	ipnet := net.IPNet(*p)
	s := ipnet.String()
	return s == ipv4DefaultRouteStr || s == ipv6DefaultRouteStr
}

func (p *Prefix) String() string {
	ipnet := net.IPNet(*p)
	return ipnet.String()
}

// RoutingTablesMetadataDecoder implements a json message raw decoder
func RoutingTablesMetadataDecoder(raw json.RawMessage) (common.Getter, error) {
	var rt RoutingTables
	if err := json.Unmarshal(raw, &rt); err != nil {
		return nil, fmt.Errorf("unable to unmarshal routing table %s: %s", string(raw), err)
	}

	return &rt, nil
}

// MarshalJSON custom marshal function
func (p *Prefix) MarshalJSON() ([]byte, error) {
	return []byte(`"` + p.String() + `"`), nil
}

// UnmarshalJSON custom unmarshal function
func (p *Prefix) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	_, cidr, err := net.ParseCIDR(s)
	if err != nil {
		return err
	}
	*p = Prefix(*cidr)

	return nil
}

// GetRoute returns route for the given protocol and prefix
func (rt *RoutingTable) GetRoute(protocol int64, prefix net.IPNet) *Route {
	for _, r := range rt.Routes {
		if r.Protocol == protocol && reflect.DeepEqual(r.Prefix, Prefix{prefix}) {
			return r
		}
	}
	return nil
}

// GetOrCreateRoute creates if not existing a new route and returns it
func (rt *RoutingTable) GetOrCreateRoute(protocol int64, prefix net.IPNet) *Route {
	if r := rt.GetRoute(protocol, prefix); r != nil {
		return r
	}
	r := &Route{
		Protocol: protocol,
		Prefix:   Prefix(prefix),
	}
	rt.Routes = append(rt.Routes, r)
	return r
}

// GetNextHop returns the nexthop for the given ip and ifindex
func (r *Route) GetNextHop(ip net.IP, ifIndex int64) *NextHop {
	for _, n := range r.NextHops {
		if n.IP.Equal(ip) && n.IfIndex == ifIndex {
			return n
		}
	}
	return nil
}

// GetOrCreateNextHop creates if not existing a new nexthop and returns it
func (r *Route) GetOrCreateNextHop(ip net.IP, ifIndex int64, priority int64) *NextHop {
	if n := r.GetNextHop(ip, ifIndex); n != nil {
		return n
	}
	nh := &NextHop{
		IP:       ip,
		IfIndex:  ifIndex,
		Priority: priority,
	}
	r.NextHops = append(r.NextHops, nh)
	return nh
}
