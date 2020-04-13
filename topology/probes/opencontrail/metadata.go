//go:generate go run github.com/skydive-project/skydive/graffiti/gendecoder -package github.com/skydive-project/skydive/topology/probes/opencontrail
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

package opencontrail

import (
	json "encoding/json"
	"fmt"

	"github.com/skydive-project/skydive/graffiti/getter"
)

// Metadata defines the information stored about a Contrail interface
// easyjson:json
// gendecoder
type Metadata struct {
	UUID         string   `json:",omitempty"`
	MAC          string   `json:",omitempty"`
	VRF          string   `json:",omitempty"`
	VRFID        int64    `json:",omitempty"`
	LocalIP      string   `json:",omitempty"`
	RoutingTable []*Route `json:",omitempty"`
}

// Route is the skydive representation of a Contrail route
// easyjson:json
// gendecoder
type Route struct {
	Family   string `json:",omitempty"`
	Prefix   string `json:",omitempty"`
	NhID     int64  `json:"NhID,omitempty"`
	Protocol int64  `json:",omitempty"`
}

// RoutingTable describes a Contrail route.
// A VRF contains the list of interface that use this VRF in order to
// be able to garbage collect VRF: if a VRF is no longer associated to
// an interface, this VRF can be deleted.
// easyjson:json
// gendecoder
type RoutingTable struct {
	InterfacesUUID []string
	Routes         []*Route
}

// MetadataDecoder implements a json message raw decoder
func MetadataDecoder(raw json.RawMessage) (getter.Getter, error) {
	var m Metadata
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("unable to unmarshal opencontrail object %s: %s", string(raw), err)
	}

	return &m, nil
}
