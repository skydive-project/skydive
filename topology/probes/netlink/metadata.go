//go:generate go run github.com/skydive-project/skydive/graffiti/gendecoder -package github.com/skydive-project/skydive/topology/probes/netlink
//go:generate go run github.com/mailru/easyjson/easyjson $GOFILE

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

	"github.com/skydive-project/skydive/graffiti/getter"
)

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
func VFSMetadataDecoder(raw json.RawMessage) (getter.Getter, error) {
	var vfs VFS
	if err := json.Unmarshal(raw, &vfs); err != nil {
		return nil, fmt.Errorf("unable to unmarshal routing table %s: %s", string(raw), err)
	}

	return &vfs, nil
}
