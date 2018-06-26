/*
 * Copyright (C) 2016 Red Hat, Inc.
 *
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 *
 */

package topology

import (
	"github.com/skydive-project/skydive/common"
)

// InterfaceMetric the interface packets counters
type InterfaceMetric struct {
	Collisions        int64 `json:"Collisions,omitempty"`
	Multicast         int64 `json:"Multicast,omitempty"`
	RxBytes           int64 `json:"RxBytes,omitempty"`
	RxCompressed      int64 `json:"RxCompressed,omitempty"`
	RxCrcErrors       int64 `json:"RxCrcErrors,omitempty"`
	RxDropped         int64 `json:"RxDropped,omitempty"`
	RxErrors          int64 `json:"RxErrors,omitempty"`
	RxFifoErrors      int64 `json:"RxFifoErrors,omitempty"`
	RxFrameErrors     int64 `json:"RxFrameErrors,omitempty"`
	RxLengthErrors    int64 `json:"RxLengthErrors,omitempty"`
	RxMissedErrors    int64 `json:"RxMissedErrors,omitempty"`
	RxOverErrors      int64 `json:"RxOverErrors,omitempty"`
	RxPackets         int64 `json:"RxPackets,omitempty"`
	TxAbortedErrors   int64 `json:"TxAbortedErrors,omitempty"`
	TxBytes           int64 `json:"TxBytes,omitempty"`
	TxCarrierErrors   int64 `json:"TxCarrierErrors,omitempty"`
	TxCompressed      int64 `json:"TxCompressed,omitempty"`
	TxDropped         int64 `json:"TxDropped,omitempty"`
	TxErrors          int64 `json:"TxErrors,omitempty"`
	TxFifoErrors      int64 `json:"TxFifoErrors,omitempty"`
	TxHeartbeatErrors int64 `json:"TxHeartbeatErrors,omitempty"`
	TxPackets         int64 `json:"TxPackets,omitempty"`
	TxWindowErrors    int64 `json:"TxWindowErrors,omitempty"`
	Start             int64 `json:"Start,omitempty"`
	Last              int64 `json:"Last,omitempty"`
}

// GetStart returns start time
func (im *InterfaceMetric) GetStart() int64 {
	return im.Start
}

// SetStart set start time
func (im *InterfaceMetric) SetStart(start int64) {
	im.Start = start
}

// GetLast returns last time
func (im *InterfaceMetric) GetLast() int64 {
	return im.Last
}

// SetLast set last tome
func (im *InterfaceMetric) SetLast(last int64) {
	im.Last = last
}

// GetFieldInt64 returns field by name
func (im *InterfaceMetric) GetFieldInt64(field string) (int64, error) {
	switch field {
	case "RxPackets":
		return im.RxPackets, nil
	case "TxPackets":
		return im.TxPackets, nil
	case "RxBytes":
		return im.RxBytes, nil
	case "TxBytes":
		return im.TxBytes, nil
	case "RxErrors":
		return im.RxErrors, nil
	case "TxErrors":
		return im.TxErrors, nil
	case "RxDropped":
		return im.RxDropped, nil
	case "TxDropped":
		return im.TxDropped, nil
	case "Multicast":
		return im.Multicast, nil
	case "Collisions":
		return im.Collisions, nil
	case "RxLengthErrors":
		return im.RxLengthErrors, nil
	case "RxOverErrors":
		return im.RxOverErrors, nil
	case "RxCrcErrors":
		return im.RxCrcErrors, nil
	case "RxFrameErrors":
		return im.RxFrameErrors, nil
	case "RxFifoErrors":
		return im.RxFifoErrors, nil
	case "RxMissedErrors":
		return im.RxMissedErrors, nil
	case "TxAbortedErrors":
		return im.TxAbortedErrors, nil
	case "TxCarrierErrors":
		return im.TxCarrierErrors, nil
	case "TxFifoErrors":
		return im.TxFifoErrors, nil
	case "TxHeartbeatErrors":
		return im.TxHeartbeatErrors, nil
	case "TxWindowErrors":
		return im.TxWindowErrors, nil
	case "RxCompressed":
		return im.RxCompressed, nil
	case "TxCompressed":
		return im.TxCompressed, nil
	}
	return 0, common.ErrFieldNotFound
}

// Add sum two metrics and return a new Metrics object
func (im *InterfaceMetric) Add(m common.Metric) common.Metric {
	om := m.(*InterfaceMetric)

	return &InterfaceMetric{
		Collisions:        im.Collisions + om.Collisions,
		Multicast:         im.Multicast + om.Multicast,
		RxBytes:           im.RxBytes + om.RxBytes,
		RxCompressed:      im.RxCompressed + om.RxCompressed,
		RxCrcErrors:       im.RxCrcErrors + om.RxCrcErrors,
		RxDropped:         im.RxDropped + om.RxDropped,
		RxErrors:          im.RxErrors + om.RxErrors,
		RxFifoErrors:      im.RxFifoErrors + om.RxFifoErrors,
		RxFrameErrors:     im.RxFrameErrors + om.RxFrameErrors,
		RxLengthErrors:    im.RxLengthErrors + om.RxLengthErrors,
		RxMissedErrors:    im.RxMissedErrors + om.RxMissedErrors,
		RxOverErrors:      im.RxOverErrors + om.RxOverErrors,
		RxPackets:         im.RxPackets + om.RxPackets,
		TxAbortedErrors:   im.TxAbortedErrors + om.TxAbortedErrors,
		TxBytes:           im.TxBytes + om.TxBytes,
		TxCarrierErrors:   im.TxCarrierErrors + om.TxCarrierErrors,
		TxCompressed:      im.TxCompressed + om.TxCompressed,
		TxDropped:         im.TxDropped + om.TxDropped,
		TxErrors:          im.TxErrors + om.TxErrors,
		TxFifoErrors:      im.TxFifoErrors + om.TxFifoErrors,
		TxHeartbeatErrors: im.TxHeartbeatErrors + om.TxHeartbeatErrors,
		TxPackets:         im.TxPackets + om.TxPackets,
		TxWindowErrors:    im.TxWindowErrors + om.TxWindowErrors,
		Start:             im.Start,
		Last:              im.Last,
	}
}

// Sub subtracts two metrics and return a new metrics object
func (im *InterfaceMetric) Sub(m common.Metric) common.Metric {
	om := m.(*InterfaceMetric)

	return &InterfaceMetric{
		Collisions:        im.Collisions - om.Collisions,
		Multicast:         im.Multicast - om.Multicast,
		RxBytes:           im.RxBytes - om.RxBytes,
		RxCompressed:      im.RxCompressed - om.RxCompressed,
		RxCrcErrors:       im.RxCrcErrors - om.RxCrcErrors,
		RxDropped:         im.RxDropped - om.RxDropped,
		RxErrors:          im.RxErrors - om.RxErrors,
		RxFifoErrors:      im.RxFifoErrors - om.RxFifoErrors,
		RxFrameErrors:     im.RxFrameErrors - om.RxFrameErrors,
		RxLengthErrors:    im.RxLengthErrors - om.RxLengthErrors,
		RxMissedErrors:    im.RxMissedErrors - om.RxMissedErrors,
		RxOverErrors:      im.RxOverErrors - om.RxOverErrors,
		RxPackets:         im.RxPackets - om.RxPackets,
		TxAbortedErrors:   im.TxAbortedErrors - om.TxAbortedErrors,
		TxBytes:           im.TxBytes - om.TxBytes,
		TxCarrierErrors:   im.TxCarrierErrors - om.TxCarrierErrors,
		TxCompressed:      im.TxCompressed - om.TxCompressed,
		TxDropped:         im.TxDropped - om.TxDropped,
		TxErrors:          im.TxErrors - om.TxErrors,
		TxFifoErrors:      im.TxFifoErrors - om.TxFifoErrors,
		TxHeartbeatErrors: im.TxHeartbeatErrors - om.TxHeartbeatErrors,
		TxPackets:         im.TxPackets - om.TxPackets,
		TxWindowErrors:    im.TxWindowErrors - om.TxWindowErrors,
		Start:             im.Start,
		Last:              im.Last,
	}
}

// IsZero returns true if all the values are equal to zero
func (im *InterfaceMetric) IsZero() bool {
	// sum as these numbers can't be <= 0
	return (im.Collisions +
		im.Multicast +
		im.RxBytes +
		im.RxCompressed +
		im.RxCrcErrors +
		im.RxDropped +
		im.RxErrors +
		im.RxFifoErrors +
		im.RxFrameErrors +
		im.RxLengthErrors +
		im.RxMissedErrors +
		im.RxOverErrors +
		im.RxPackets +
		im.TxAbortedErrors +
		im.TxBytes +
		im.TxCarrierErrors +
		im.TxCompressed +
		im.TxDropped +
		im.TxErrors +
		im.TxFifoErrors +
		im.TxHeartbeatErrors +
		im.TxPackets +
		im.TxWindowErrors) == 0
}

func (im *InterfaceMetric) applyRatio(ratio float64) *InterfaceMetric {
	return &InterfaceMetric{
		Collisions:        int64(float64(im.Collisions) * ratio),
		Multicast:         int64(float64(im.Multicast) * ratio),
		RxBytes:           int64(float64(im.RxBytes) * ratio),
		RxCompressed:      int64(float64(im.RxCompressed) * ratio),
		RxCrcErrors:       int64(float64(im.RxCrcErrors) * ratio),
		RxDropped:         int64(float64(im.RxDropped) * ratio),
		RxErrors:          int64(float64(im.RxErrors) * ratio),
		RxFifoErrors:      int64(float64(im.RxFifoErrors) * ratio),
		RxFrameErrors:     int64(float64(im.RxFrameErrors) * ratio),
		RxLengthErrors:    int64(float64(im.RxLengthErrors) * ratio),
		RxMissedErrors:    int64(float64(im.RxMissedErrors) * ratio),
		RxOverErrors:      int64(float64(im.RxOverErrors) * ratio),
		RxPackets:         int64(float64(im.RxPackets) * ratio),
		TxAbortedErrors:   int64(float64(im.TxAbortedErrors) * ratio),
		TxBytes:           int64(float64(im.TxBytes) * ratio),
		TxCarrierErrors:   int64(float64(im.TxCarrierErrors) * ratio),
		TxCompressed:      int64(float64(im.TxCompressed) * ratio),
		TxDropped:         int64(float64(im.TxDropped) * ratio),
		TxErrors:          int64(float64(im.TxErrors) * ratio),
		TxFifoErrors:      int64(float64(im.TxFifoErrors) * ratio),
		TxHeartbeatErrors: int64(float64(im.TxHeartbeatErrors) * ratio),
		TxPackets:         int64(float64(im.TxPackets) * ratio),
		TxWindowErrors:    int64(float64(im.TxWindowErrors) * ratio),
		Start:             im.Start,
		Last:              im.Last,
	}
}

// Slice splits a Metric into two parts
func (im *InterfaceMetric) Split(cut int64) (common.Metric, common.Metric) {
	if cut < im.Start {
		return nil, im
	} else if cut > im.Last {
		return im, nil
	} else if im.Start == im.Last {
		return im, nil
	} else if cut == im.Start {
		return nil, im
	} else if cut == im.Last {
		return im, nil
	}

	duration := float64(im.Last - im.Start)

	ratio1 := float64(cut-im.Start) / duration
	ratio2 := float64(im.Last-cut) / duration

	m1 := im.applyRatio(ratio1)
	m1.Last = cut

	m2 := im.applyRatio(ratio2)
	m2.Start = cut

	return m1, m2
}
