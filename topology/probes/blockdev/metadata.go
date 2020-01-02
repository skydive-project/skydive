//go:generate go run github.com/skydive-project/skydive/scripts/gendecoder -package github.com/skydive-project/skydive/topology/probes/blockdev
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

package blockdev

import (
	"encoding/json"
	"fmt"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/graffiti/graph"
)

// Metadata describe the metadata of a block device
// easyjson:json
// gendecoder
type Metadata struct {
	Index        string
	Name         string
	Alignment    int64
	DiscAln      int64
	DiscGran     string
	DiscMax      string
	DiscZero     bool
	Fsavail      string
	Fssize       string
	Fstype       string
	FsusePercent string
	Fsused       string
	Group        string
	Hctl         string
	Hotplug      bool
	Kname        string
	Label        string
	LogSec       int64
	MajMin       string
	MinIo        int64
	Mode         string
	Model        string
	Mountpoint   string
	OptIo        int64
	Owner        string
	Partflags    string
	Partlabel    string
	Parttype     string
	Partuuid     string
	Path         string
	PhySec       int64
	Pkname       string
	Pttype       string
	Ptuuid       string
	Ra           int64
	Rand         bool
	Rev          string
	Rm           bool
	Ro           bool
	Rota         bool
	RqSize       int64
	Sched        string
	Serial       string
	Size         string
	State        string
	Subsystems   string
	Tran         string
	Type         string
	UUID         string
	Vendor       string
	Wsame        string
	WWN          string
	Labels       graph.Metadata `field:"Metadata"`
}

// MetadataDecoder implements a json message raw decoder
func MetadataDecoder(raw json.RawMessage) (common.Getter, error) {
	var m Metadata
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("unable to unmarshal block device metadata %s: %s", string(raw), err)
	}

	return &m, nil
}
