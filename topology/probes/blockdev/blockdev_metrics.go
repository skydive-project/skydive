//go:generate go run github.com/skydive-project/skydive/graffiti/gendecoder -package github.com/skydive-project/skydive/topology/probes/blockdev
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

	"github.com/skydive-project/skydive/graffiti/getter"
)

// BlockMetric container for the blockdevice IO stats
// easyjson:json
// gendecoder
type BlockMetric struct {
	ReadsPerSec        int64 `json:"ReadsPerSec,omitempty"`
	WritesPerSec       int64 `json:"WritesPerSec,omitempty"`
	ReadsKBPerSec      int64 `json:"ReadsKBPerSec,omitempty"`
	WritesKBPerSec     int64 `json:"WritesKBPerSec,omitempty"`
	ReadsMergedPerSec  int64 `json:"ReadsMergedPerSec,omitempty"`
	WritesMergedPerSec int64 `json:"WritesMergedPerSec,omitempty"`
	ReadsMerged        int64 `json:"ReadsMerged,omitempty"`
	WritesMerged       int64 `json:"WritesMerged,omitempty"`
	ReadServiceTime    int64 `json:"ReadServiceTime,omitempty"`
	WriteServiceTime   int64 `json:"WriteServiceTime,omitempty"`
	AverageQueueSize   int64 `json:"AverageQueueSize,omitempty"`
	AverageReadSize    int64 `json:"AverageReadSize,omitempty"`
	AverageWriteSize   int64 `json:"AverageWriteSize,omitempty"`
	ServiceTime        int64 `json:"ServiceTime,omitempty"`
	Utilization        int64 `json:"Utilization,omitempty"`
	Start              int64 `json:"Start,omitempty"`
	Last               int64 `json:"Last,omitempty"`
}

// IOMetric used for parsing the output of iostat
type IOMetric struct {
	ReadsPerSec        int64 `mapstructure:"r/s,omitempty"`
	WritesPerSec       int64 `mapstructure:"w/s,omitempty"`
	ReadsKBPerSec      int64 `mapstructure:"rkB/s,omitempty"`
	WritesKBPerSec     int64 `mapstructure:"wkB/s,omitempty"`
	ReadsMergedPerSec  int64 `mapstructure:"rrqm/s,omitempty"`
	WritesMergedPerSec int64 `mapstructure:"wrqm/s,omitempty"`
	ReadsMerged        int64 `mapstructure:"rrqm,omitempty"`
	WritesMerged       int64 `mapstructure:"wrqm,omitempty"`
	ReadServiceTime    int64 `mapstructure:"r_await,omitempty"`
	WriteServiceTime   int64 `mapstructure:"w_await,omitempty"`
	AverageQueueSize   int64 `mapstructure:"aqu-sz,omitempty"`
	AverageReadSize    int64 `mapstructure:"rareq-sz,omitempty"`
	AverageWriteSize   int64 `mapstructure:"wareq-sz,omitempty"`
	ServiceTime        int64 `mapstructure:"svctm,omitempty"`
	Utilization        int64 `mapstructure:"util,omitempty"`
	Start              int64 `mapstructure:"Start,omitempty"`
	Last               int64 `mapstructure:"Last,omitempty"`
}

// MakeCopy is used to copy an IOMetric to a Metric
func (im *IOMetric) MakeCopy() *BlockMetric {
	return &BlockMetric{
		ReadsPerSec:        im.ReadsPerSec,
		WritesPerSec:       im.WritesPerSec,
		ReadsKBPerSec:      im.ReadsKBPerSec,
		WritesKBPerSec:     im.WritesKBPerSec,
		ReadsMergedPerSec:  im.ReadsMergedPerSec,
		WritesMergedPerSec: im.WritesMergedPerSec,
		ReadsMerged:        im.ReadsMerged,
		WritesMerged:       im.WritesMerged,
		ReadServiceTime:    im.ReadServiceTime,
		WriteServiceTime:   im.WriteServiceTime,
		AverageQueueSize:   im.AverageQueueSize,
		AverageReadSize:    im.AverageReadSize,
		AverageWriteSize:   im.AverageWriteSize,
		ServiceTime:        im.ServiceTime,
		Utilization:        im.Utilization,
		Start:              im.Start,
		Last:               im.Last,
	}
}

// MetricDecoder implements a json message raw decoder
func MetricDecoder(raw json.RawMessage) (getter.Getter, error) {
	var metric BlockMetric
	if err := json.Unmarshal(raw, &metric); err != nil {
		return nil, err
	}

	return &metric, nil
}
