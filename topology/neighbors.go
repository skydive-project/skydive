//go:generate go run ../scripts/gendecoder.go -output neighbors_gendecoder.go

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
	MAC     string   `json:"MAC,omitempty"`
	IP      net.IP   `json:"IP,omitempty"`
	State   []string `json:"State,omitempty"`
	Vlan    int64    `json:"Vlan,omitempty"`
	VNI     int64    `json:"VNI,omitempty"`
	IfIndex int64
}

func (neighbors *Neighbors) getMAC(ip net.IP) string {
	for _, n := range *neighbors {
		if n.IP.Equal(ip) {
			return n.MAC
		}
	}
	return ""
}

// NeighborMetadataDecoder implements a json message raw decoder
func NeighborMetadataDecoder(raw json.RawMessage) (common.Getter, error) {
	var nbs Neighbors
	if err := json.Unmarshal(raw, &nbs); err != nil {
		return nil, fmt.Errorf("unable to unmarshal routing table %s: %s", string(raw), err)
	}

	return &nbs, nil
}
