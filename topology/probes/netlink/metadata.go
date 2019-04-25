//go:generate go run ../../../scripts/gendecoder.go -package github.com/skydive-project/skydive/topology/probes/netlink

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

package netlink

import (
	json "encoding/json"
	"fmt"
	"net"

	"github.com/skydive-project/skydive/common"
)

// Neighbors describes a list of neighbors
// easyjson:json
// gendecoder
type Neighbors []*Neighbor

// Neighbor describes a member of the forwarding database
// easyjson:json
// gendecoder
type Neighbor struct {
	Flags   []string `json:"Flags,omitempty"`
	MAC     string
	IP      net.IP   `json:"IP,omitempty"`
	State   []string `json:"State,omitempty"`
	Vlan    int64    `json:"Vlan,omitempty"`
	VNI     int64    `json:"VNI,omitempty"`
	IfIndex int64
}

// NeighborMetadataDecoder implements a json message raw decoder
func NeighborMetadataDecoder(raw json.RawMessage) (common.Getter, error) {
	var nbs Neighbors
	if err := json.Unmarshal(raw, &nbs); err != nil {
		return nil, fmt.Errorf("unable to unmarshal routing table %s: %s", string(raw), err)
	}

	return &nbs, nil
}

// VFS describes a list of virtual functions
// easyjson:json
// gendecoder
type VFS []*VF

// VF describes the metadata of a virtual function
// easyjson:json
// gendecoder
type VF struct {
	ID        int64  `json:",omitempty"`
	LinkState int64  `json:",omitempty"`
	MAC       string `json:",omitempty"`
	Qos       int64  `json:",omitempty"`
	Spoofchk  bool   `json:",omitempty"`
	TxRate    int64  `json:",omitempty"`
	Vlan      int64  `json:",omitempty"`
}

// VFSMetadataDecoder implements a json message raw decoder
func VFSMetadataDecoder(raw json.RawMessage) (common.Getter, error) {
	var vfs VFS
	if err := json.Unmarshal(raw, &vfs); err != nil {
		return nil, fmt.Errorf("unable to unmarshal routing table %s: %s", string(raw), err)
	}

	return &vfs, nil
}

// RoutingTables describes a list of routing table
// easyjson:json
// gendecoder
type RoutingTables []*RoutingTable

// RoutingTable describes a list of Routes
// easyjson:json
// gendecoder
type RoutingTable struct {
	ID     int64    `json:"ID"`
	Src    net.IP   `json:"Src"`
	Routes []*Route `json:"Routes"`
}

// Prefix describes prefix
type Prefix net.IPNet

// Route describes a route
// easyjson:json
// gendecoder
type Route struct {
	Protocol int64      `json:"Protocol"`
	Prefix   Prefix     `json:"Prefix"`
	NextHops []*NextHop `json:"NextHops"`
}

// NextHop describes a next hop
// easyjson:json
// gendecoder
type NextHop struct {
	Priority int64  `json:"Priority"`
	IP       net.IP `json:"IP,omitempty"`
	IfIndex  int64  `json:"IfIndex"`
}
