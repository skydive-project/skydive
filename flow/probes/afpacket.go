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

package probes

import (
	"time"

	"github.com/google/gopacket"
	//"github.com/google/gopacket/afpacket"
	"github.com/skydive-project/skydive/flow/probes/afpacket"
)

// AFPacketHandle describes a AF network kernel packets
type AFPacketHandle struct {
	tpacket *afpacket.TPacket
}

// ReadPacketData reads one packet
func (h *AFPacketHandle) ReadPacketData() ([]byte, gopacket.CaptureInfo, error) {
	return h.tpacket.ReadPacketData()
}

// Close the AF packet handle
func (h *AFPacketHandle) Close() {
	h.tpacket.Close()
}

// NewAFPacketHandle creates a new network AF packet probe
func NewAFPacketHandle(ifName string, snaplen int32) (*AFPacketHandle, error) {
	tpacket, err := afpacket.NewTPacket(
		afpacket.OptInterface(ifName),
		afpacket.OptFrameSize(snaplen),
		afpacket.OptPollTimeout(1*time.Second),
	)

	if err != nil {
		return nil, err
	}

	return &AFPacketHandle{tpacket: tpacket}, err
}
