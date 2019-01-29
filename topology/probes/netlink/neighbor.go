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
	json "encoding/json"
	"fmt"
	"net"

	"github.com/skydive-project/skydive/common"
)

// Neighbors describes a list of neighbors
// easyjson:json
type Neighbors []*Neighbor

// Neighbor describes a member of the forwarding database
// easyjson:json
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

// GetFieldString implements Getter interface
func (nbs *Neighbors) GetFieldString(key string) (string, error) {
	for _, nb := range *nbs {
		switch key {
		case "Flags":
			if len(nb.Flags) == 0 {
				return "", nil
			}
			return nb.Flags[0], nil
		case "MAC":
			return nb.MAC, nil
		case "IP":
			if nb.IP != nil {
				return nb.IP.String(), nil
			}
			return "", nil
		case "State":
			if len(nb.State) == 0 {
				return "", nil
			}
			return nb.State[0], nil
		}
	}
	return "", common.ErrFieldNotFound
}

// GetFieldInt64 implements Getter interface
func (nbs *Neighbors) GetFieldInt64(key string) (int64, error) {
	for _, nb := range *nbs {
		switch key {
		case "Vlan":
			return nb.Vlan, nil
		case "VNI":
			return nb.VNI, nil
		case "IfIndex":
			return nb.IfIndex, nil
		}
	}
	return 0, common.ErrFieldNotFound
}

// GetField implements Getter interface
func (nbs *Neighbors) GetField(field string) (interface{}, error) {
	var result []interface{}

	for _, nb := range *nbs {
		switch field {
		case "Vlan":
			result = append(result, nb.Vlan)
		case "VNI":
			result = append(result, nb.VNI)
		case "IfIndex":
			result = append(result, nb.IfIndex)
		case "Flags":
			result = append(result, nb.Flags)
		case "MAC":
			result = append(result, nb.MAC)
		case "IP":
			result = append(result, nb.IP)
		case "State":
			result = append(result, nb.State)
		default:
			return result, common.ErrFieldNotFound
		}
	}

	return result, nil
}

// GetFieldKeys returns the list of valid field of a Flow
func (nbs *Neighbors) GetFieldKeys() []string {
	return nbFields
}

var nbFields []string

func init() {
	nbFields = common.StructFieldKeys(Neighbor{})
}
