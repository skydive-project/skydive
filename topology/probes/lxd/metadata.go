//go:generate go run github.com/skydive-project/skydive/graffiti/gendecoder -package github.com/skydive-project/skydive/topology/probes/lxd
//go:generate go run github.com/mailru/easyjson/easyjson $GOFILE

/*
 * Copyright (C) 209 Red Hat, Inc.
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

package lxd

import (
	"encoding/json"
	"fmt"

	"github.com/skydive-project/skydive/graffiti/getter"
	"github.com/skydive-project/skydive/graffiti/graph"
)

// Metadata describes the informations stored for a LXD container
// easyjson:json
// gendecoder
type Metadata struct {
	Architecture string         `json:",omitempty"`
	Config       graph.Metadata `json:",omitempty" field:"Metadata"`
	CreatedAt    string         `json:",omitempty"`
	Description  string         `json:",omitempty"`
	Devices      graph.Metadata `json:",omitempty" field:"Metadata"`
	Ephemeral    string         `json:",omitempty"`
	Profiles     []string       `json:",omitempty"`
	Restore      string         `json:",omitempty"`
	Status       string         `json:",omitempty"`
	Stateful     string         `json:",omitempty"`
}

// MetadataDecoder implements a json message raw decoder
func MetadataDecoder(raw json.RawMessage) (getter.Getter, error) {
	var m Metadata
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("unable to unmarshal LXD metadata %s: %s", string(raw), err)
	}

	return &m, nil
}
