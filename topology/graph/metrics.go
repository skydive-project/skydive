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

package graph

import "github.com/skydive-project/skydive/common"

// InterfaceMetric the interface packets counters
type InterfaceMetric struct {
	RxPackets         int64
	TxPackets         int64
	RxBytes           int64
	TxBytes           int64
	RxErrors          int64
	TxErrors          int64
	RxDropped         int64
	TxDropped         int64
	Multicast         int64
	Collisions        int64
	RxLengthErrors    int64
	RxOverErrors      int64
	RxCrcErrors       int64
	RxFrameErrors     int64
	RxFifoErrors      int64
	RxMissedErrors    int64
	TxAbortedErrors   int64
	TxCarrierErrors   int64
	TxFifoErrors      int64
	TxHeartbeatErrors int64
	TxWindowErrors    int64
	RxCompressed      int64
	TxCompressed      int64
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

// Add do a sum operation on interface metric
func (im *InterfaceMetric) Add(m common.Metric) common.Metric {
	im2 := m.(*InterfaceMetric)

	im.RxPackets += im2.RxPackets
	im.TxPackets += im2.TxPackets
	im.RxBytes += im2.RxBytes
	im.TxBytes += im2.TxBytes
	im.RxErrors += im2.RxErrors
	im.TxErrors += im2.TxErrors
	im.RxDropped += im2.RxDropped
	im.TxDropped += im2.TxDropped
	im.Multicast += im2.Multicast
	im.Collisions += im2.Collisions
	im.RxLengthErrors += im2.RxLengthErrors
	im.RxOverErrors += im2.RxOverErrors
	im.RxCrcErrors += im2.RxCrcErrors
	im.RxFrameErrors += im2.RxFrameErrors
	im.RxFifoErrors += im2.RxFifoErrors
	im.RxMissedErrors += im2.RxMissedErrors
	im.TxAbortedErrors += im2.TxAbortedErrors
	im.TxCarrierErrors += im2.TxCarrierErrors
	im.TxFifoErrors += im2.TxFifoErrors
	im.TxHeartbeatErrors += im2.TxHeartbeatErrors
	im.TxWindowErrors += im2.TxWindowErrors
	im.RxCompressed += im2.RxCompressed
	im.TxCompressed += im2.TxCompressed

	return im
}
