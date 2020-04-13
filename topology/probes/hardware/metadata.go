//go:generate go run github.com/skydive-project/skydive/graffiti/gendecoder -package github.com/skydive-project/skydive/topology/probes/hardware
//go:generate go run github.com/mailru/easyjson/easyjson $GOFILE

/*
 * Copyright (C) 2020 Sylvain Afchain
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

package hardware

import (
	json "encoding/json"
	"fmt"

	"github.com/skydive-project/skydive/graffiti/getter"
)

// easyjson:json
// gendecoder
type CPUInfos []*CPUInfo

// CPUInfo defines host information
// easyjson:json
// gendecoder
type CPUInfo struct {
	CPU        int64  `json:"CPU,omitempty"`
	VendorID   string `json:"VendorID,omitempty"`
	Family     string `json:"Family,omitempty"`
	Model      string `json:"Model,omitempty"`
	Stepping   int64  `json:"Stepping,omitempty"`
	PhysicalID string `json:"PhysicalID,omitempty"`
	CoreID     string `json:"CoreID,omitempty"`
	Cores      int64  `json:"Cores,omitempty"`
	ModelName  string `json:"ModelName,omitempty"`
	Mhz        int64  `json:"Mhz,omitempty"`
	CacheSize  int64  `json:"CacheSize,omitempty"`
	Microcode  string `json:"Microcode,omitempty"`
}

// MetadataDecoder implements a json message raw decoder
func MetadataDecoder(raw json.RawMessage) (getter.Getter, error) {
	var c CPUInfos
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("unable to unmarshal Hardware metadata %s: %s", string(raw), err)
	}

	return &c, nil
}
