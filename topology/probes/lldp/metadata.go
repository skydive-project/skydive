//go:generate go run github.com/skydive-project/skydive/graffiti/gendecoder -package github.com/skydive-project/skydive/topology/probes/lldp
//go:generate go run github.com/mailru/easyjson/easyjson $GOFILE

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

package lldp

import (
	json "encoding/json"
	"fmt"

	"github.com/skydive-project/skydive/graffiti/getter"
)

// Metadata describes the LLDP chassis metadata
// easyjson:json
// gendecoder
type Metadata struct {
	Description     string                   `json:"Description,omitempty"`
	ChassisID       string                   `json:"ChassisID,omitempty"`
	ChassisIDType   string                   `json:"ChassisIDType,omitempty"`
	SysName         string                   `json:"SysName,omitempty"`
	MgmtAddress     string                   `json:"MgmtAddress,omitempty"`
	PVID            int64                    `json:"PVID,omitempty"`
	VIDUsageDigest  int64                    `json:"VIDUsageDigest,omitempty"`
	ManagementVID   int64                    `json:"ManagementVID,omitempty"`
	PortID          string                   `json:"PortID,omitempty"`
	PortIDType      string                   `json:"PortIDType,omitempty"`
	LinkAggregation *LinkAggregationMetadata `json:"LinkAgggregation,omitempty"`
	VLANNames       []VLANNameMetadata       `json:"VLANNames,omitempty"`
	PPVIDs          []PPVIDMetadata          `json:"PPVIDs,omitempty"`
}

// LinkAggregationMetadata describes the LLDP link aggregation metadata
// easyjson:json
// gendecoder
type LinkAggregationMetadata struct {
	Enabled   bool  `json:"Enabled"`
	PortID    int64 `json:"PortID,omitempty"`
	Supported bool  `json:"Supported"`
}

// PPVIDMetadata describes the LLDP link PPVID metadata
// easyjson:json
// gendecoder
type PPVIDMetadata struct {
	Enabled   bool  `json:"Enabled"`
	ID        int64 `json:"ID"`
	Supported bool  `json:"Supported"`
}

// VLANNameMetadata describes the LLDP VLAN metadata
// easyjson:json
// gendecoder
type VLANNameMetadata struct {
	ID   int64  `json:"ID"`
	Name string `json:"Name"`
}

// MetadataDecoder implements the JSON raw decoder for LLDP metadata
func MetadataDecoder(raw json.RawMessage) (getter.Getter, error) {
	var metadata Metadata
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return nil, fmt.Errorf("unable to unmarshal LLDP metadata %s: %s", string(raw), err)
	}

	return &metadata, nil
}
