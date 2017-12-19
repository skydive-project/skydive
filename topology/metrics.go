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
	"github.com/vishvananda/netlink"
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
	Start             int64 `json:"Start"`
	Last              int64 `json:"Last"`
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

// Sub substracts two metrics and return a new Metrics object
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

// NewInterfaceMetricsFromNetlink returns a new InterfaceMetric object using
// values of netlink.
func NewInterfaceMetricsFromNetlink(link netlink.Link) *InterfaceMetric {
	statistics := link.Attrs().Statistics
	if statistics == nil {
		return nil
	}

	return &InterfaceMetric{
		Collisions:        int64(statistics.Collisions),
		Multicast:         int64(statistics.Multicast),
		RxBytes:           int64(statistics.RxBytes),
		RxCompressed:      int64(statistics.RxCompressed),
		RxCrcErrors:       int64(statistics.RxCrcErrors),
		RxDropped:         int64(statistics.RxDropped),
		RxErrors:          int64(statistics.RxErrors),
		RxFifoErrors:      int64(statistics.RxFifoErrors),
		RxFrameErrors:     int64(statistics.RxFrameErrors),
		RxLengthErrors:    int64(statistics.RxLengthErrors),
		RxMissedErrors:    int64(statistics.RxMissedErrors),
		RxOverErrors:      int64(statistics.RxOverErrors),
		RxPackets:         int64(statistics.RxPackets),
		TxAbortedErrors:   int64(statistics.TxAbortedErrors),
		TxBytes:           int64(statistics.TxBytes),
		TxCarrierErrors:   int64(statistics.TxCarrierErrors),
		TxCompressed:      int64(statistics.TxCompressed),
		TxDropped:         int64(statistics.TxDropped),
		TxErrors:          int64(statistics.TxErrors),
		TxFifoErrors:      int64(statistics.TxFifoErrors),
		TxHeartbeatErrors: int64(statistics.TxHeartbeatErrors),
		TxPackets:         int64(statistics.TxPackets),
		TxWindowErrors:    int64(statistics.TxWindowErrors),
	}
}
