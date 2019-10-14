//go:generate go run github.com/skydive-project/skydive/scripts/gendecoder -package github.com/skydive-project/skydive/topology/probes/vpp
//go:generate go run github.com/safchain/easyjson/easyjson $GOFILE

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

package vpp

import (
	"encoding/json"
	"fmt"

	"github.com/skydive-project/skydive/common"
)

// Metadata describes the metadata for a VPP interface
// easyjson:json
// gendecoder
type Metadata struct {
	SocketFilename string
	ID             int64
	SocketID       int64
	Master         bool
	Mode           string
	RingSize       int64
	BufferSize     int64
	LinkUpDown     bool
}

// MetadataDecoder implements a json message raw decoder
func MetadataDecoder(raw json.RawMessage) (common.Getter, error) {
	var m Metadata
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("unable to unmarshal VPP metadata %s: %s", string(raw), err)
	}

	return &m, nil
}
