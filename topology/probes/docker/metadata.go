//go:generate go run ../../../scripts/gendecoder.go -package github.com/skydive-project/skydive/topology/probes/docker

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

package docker

import (
	"encoding/json"
	"fmt"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/graffiti/graph"
)

// Metadata describe the metadata of a docker container
// easyjson:json
// gendecoder
type Metadata struct {
	ContainerID   string
	ContainerName string
	Labels        graph.Metadata `field:"Metadata"`
}

// MetadataDecoder implements a json message raw decoder
func MetadataDecoder(raw json.RawMessage) (common.Getter, error) {
	var m Metadata
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("unable to unmarshal docker metadata %s: %s", string(raw), err)
	}

	return &m, nil
}
