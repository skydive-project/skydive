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
	"strings"

	"github.com/skydive-project/skydive/common"
)

// RoutingTables describes a list of routing table
// easyjson:json
type RoutingTables []*RoutingTable

// RoutingTable describes a list of Routes
// easyjson:json
type RoutingTable struct {
	ID     int64    `json:"ID"`
	Src    net.IP   `json:"Src"`
	Routes []*Route `json:"Routes"`
}

// Prefix describes prefix
type Prefix struct {
	net.IPNet
}

// Route describes a route
// easyjson:json
type Route struct {
	Protocol int64      `json:"Protocol"`
	Prefix   Prefix     `json:"Prefix"`
	NextHops []*NextHop `json:"NextHops"`
}

// NextHop describes a next hop
// easyjson:json
type NextHop struct {
	Priority int64  `json:"Priority"`
	IP       net.IP `json:"IP,omitempty"`
	IfIndex  int64  `json:"IfIndex"`
}

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
	s := p.String()
	return s == ipv4DefaultRouteStr || s == ipv6DefaultRouteStr
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
	*p = Prefix{*cidr}

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
		Prefix:   Prefix{prefix},
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

func (n *NextHop) getFieldString(key string) (string, error) {
	if key == "IP" {
		if n.IP != nil {
			return n.IP.String(), nil
		}
		return "", nil
	}

	return "", common.ErrFieldNotFound
}

func (n *NextHop) getFieldInt64(key string) (int64, error) {
	switch key {
	case "Priority":
		return n.Priority, nil
	case "IfIndex":
		return n.IfIndex, nil
	}

	return 0, common.ErrFieldNotFound
}

func (r *Route) getFieldString(keys ...string) ([]string, error) {
	if len(keys) == 0 {
		return nil, common.ErrFieldNotFound
	}

	if len(keys) == 1 && keys[0] == "Prefix" {
		return []string{r.Prefix.String()}, nil
	}

	if len(keys) < 2 {
		return nil, common.ErrFieldNotFound
	}

	var result []string

	switch keys[0] {
	case "NextHops":
		for _, nh := range r.NextHops {
			v, err := nh.getFieldString(keys[1])
			if err != nil {
				return nil, err
			}
			if v != "" {
				result = append(result, v)
			}
		}
	default:
		return nil, common.ErrFieldNotFound
	}

	return result, nil
}

func (r *Route) getFieldInt64(keys ...string) ([]int64, error) {
	if len(keys) == 0 {
		return nil, common.ErrFieldNotFound
	}

	if len(keys) == 1 && keys[0] == "Protocol" {
		return []int64{r.Protocol}, nil
	}

	var result []int64

	if len(keys) < 2 {
		return nil, common.ErrFieldNotFound
	}

	switch keys[0] {
	case "NextHops":
		for _, nh := range r.NextHops {
			v, err := nh.getFieldInt64(keys[1])
			if err != nil {
				return nil, err
			}
			result = append(result, v)
		}
	default:
		return nil, common.ErrFieldNotFound
	}

	return result, nil
}

func (rt *RoutingTable) getFieldString(key string) ([]string, error) {
	if key == "Src" {
		if rt.Src != nil {
			return []string{rt.Src.String()}, nil
		}
		return []string{}, nil
	}

	fields := strings.Split(key, ".")
	if len(fields) < 2 {
		return nil, common.ErrFieldNotFound
	}

	var result []string

	switch fields[0] {
	case "Routes":
		for _, r := range rt.Routes {
			v, err := r.getFieldString(fields[1:]...)
			if err != nil {
				return nil, err
			}
			if len(v) > 0 {
				result = append(result, v...)
			}
		}
	default:
		return nil, common.ErrFieldNotFound
	}

	return result, nil
}

// GetFieldString implements Getter interface
func (rt *RoutingTable) getFirstFieldString(key string) (string, error) {
	r, err := rt.getFieldString(key)
	if err != nil {
		return "", err
	}

	if len(r) == 0 {
		return "", nil
	}

	return r[0], nil
}

// GetFieldString implements Getter interface
func (rts *RoutingTables) GetFieldString(key string) (string, error) {
	for _, rt := range *rts {
		v, err := rt.getFirstFieldString(key)
		if err != nil {
			return "", err
		}
		return v, nil
	}

	return "", nil
}

func (rt *RoutingTable) getFieldInt64(key string) ([]int64, error) {
	if key == "ID" {
		return []int64{rt.ID}, nil
	}

	fields := strings.Split(key, ".")
	if len(fields) < 2 {
		return nil, common.ErrFieldNotFound
	}

	var result []int64

	switch fields[0] {
	case "Routes":
		for _, r := range rt.Routes {
			v, err := r.getFieldInt64(fields[1:]...)
			if err != nil {
				return nil, err
			}
			result = append(result, v...)
		}
	default:
		return nil, common.ErrFieldNotFound
	}

	return result, nil
}

// GetFieldInt64 implements Getter interface
func (rt *RoutingTable) getFirstFieldInt64(key string) (int64, error) {
	r, err := rt.getFieldInt64(key)
	if err != nil {
		return 0, err
	}

	if len(r) == 0 {
		return 0, nil
	}

	return r[0], nil
}

// GetFieldInt64 implements Getter interface
func (rts *RoutingTables) GetFieldInt64(key string) (int64, error) {
	for _, rt := range *rts {
		v, err := rt.getFirstFieldInt64(key)
		if err != nil {
			return 0, err
		}
		return v, nil
	}

	return 0, common.ErrFieldNotFound
}

func (rt *RoutingTable) getFieldInterface(key string) ([]interface{}, error) {
	switch key {
	case "Routes":
		var result []interface{}
		for _, r := range rt.Routes {
			result = append(result, r)
		}
		return result, nil
	case "Routes.NextHops":
		var result []interface{}
		for _, r := range rt.Routes {
			result = append(result, r.NextHops)
		}
		return result, nil
	}

	return nil, common.ErrFieldNotFound
}

// GetField returns the value of a field
func (rt *RoutingTable) getField(field string) (interface{}, error) {
	if i, err := rt.getFieldInterface(field); err == nil {
		return i, nil
	}

	if i, err := rt.getFieldInt64(field); err == nil {
		return i, nil
	}

	return rt.getFieldString(field)
}

// GetField implements Getter interface
func (rts *RoutingTables) GetField(field string) (interface{}, error) {
	var result []interface{}

	for _, rt := range *rts {
		v, err := rt.getField(field)

		if err != nil {
			return nil, err
		}
		switch v.(type) {
		case []int64:
			for _, i := range v.([]int64) {
				result = append(result, i)
			}
		case []string:
			for _, i := range v.([]string) {
				result = append(result, i)
			}
		default:
			result = append(result, v)
		}
	}

	return result, nil
}

// GetFieldKeys returns the list of valid field of a Flow
func (rts *RoutingTables) GetFieldKeys() []string {
	return rtFields
}

var rtFields []string

func init() {
	rtFields = common.StructFieldKeys(RoutingTable{})
}
