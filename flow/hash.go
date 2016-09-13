/*
 * Copyright (C) 2015 Red Hat, Inc.
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

package flow

import (
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"net"
	"strconv"

	"github.com/google/gopacket/layers"
)

func var8bin(v []byte) []byte {
	r := make([]byte, 8)
	skip := 8 - len(v)
	for i, b := range v {
		r[i+skip] = b
	}
	return r
}

func HashFromValues(ab interface{}, ba interface{}) []byte {
	var vab, vba uint64
	var binab, binba []byte

	hasher := sha1.New()
	switch ab.(type) {
	case net.HardwareAddr:
		binab = ab.(net.HardwareAddr)
		binba = ba.(net.HardwareAddr)
		vab = binary.BigEndian.Uint64(var8bin(binab))
		vba = binary.BigEndian.Uint64(var8bin(binba))
	case net.IP:
		// IP can be IPV4 or IPV6
		binab = ab.(net.IP).To16()
		binba = ba.(net.IP).To16()

		vab = binary.BigEndian.Uint64(var8bin(binab[:8]))
		vba = binary.BigEndian.Uint64(var8bin(binba[:8]))
		if vab == vba {
			vab = binary.BigEndian.Uint64(var8bin(binab[8:]))
			vba = binary.BigEndian.Uint64(var8bin(binba[8:]))
		}
	case layers.TCPPort:
		binab = make([]byte, 2)
		binba = make([]byte, 2)
		binary.BigEndian.PutUint16(binab, uint16(ab.(layers.TCPPort)))
		binary.BigEndian.PutUint16(binba, uint16(ba.(layers.TCPPort)))
		vab = uint64(ab.(layers.TCPPort))
		vba = uint64(ba.(layers.TCPPort))
	case layers.UDPPort:
		binab = make([]byte, 2)
		binba = make([]byte, 2)
		binary.BigEndian.PutUint16(binab, uint16(ab.(layers.UDPPort)))
		binary.BigEndian.PutUint16(binba, uint16(ba.(layers.UDPPort)))
		vab = uint64(ab.(layers.UDPPort))
		vba = uint64(ba.(layers.UDPPort))
	case layers.SCTPPort:
		binab = make([]byte, 2)
		binba = make([]byte, 2)
		binary.BigEndian.PutUint16(binab, uint16(ab.(layers.SCTPPort)))
		binary.BigEndian.PutUint16(binba, uint16(ba.(layers.SCTPPort)))
		vab = uint64(ab.(layers.SCTPPort))
		vba = uint64(ba.(layers.SCTPPort))
	}

	if vab < vba {
		hasher.Write(binab)
		hasher.Write(binba)
	} else {
		hasher.Write(binba)
		hasher.Write(binab)
	}
	return hasher.Sum(nil)
}

func (fl *FlowLayer) Hash() []byte {
	if fl == nil {
		return []byte{}
	}
	if fl.Protocol == FlowProtocol_ETHERNET {
		amac, err := net.ParseMAC(fl.A)
		if err != nil {
			panic(err)
		}
		bmac, err := net.ParseMAC(fl.B)
		if err != nil {
			panic(err)
		}
		return HashFromValues(amac, bmac)
	}
	if fl.Protocol == FlowProtocol_IPV4 || fl.Protocol == FlowProtocol_IPV6 {
		aip := net.ParseIP(fl.A)
		bip := net.ParseIP(fl.B)
		return HashFromValues(aip, bip)
	}
	if fl.Protocol == FlowProtocol_TCPPORT {
		aTCPPort, err := strconv.ParseUint(fl.A, 10, 16)
		if err != nil {
			panic(err)
		}
		bTCPPort, err := strconv.ParseUint(fl.B, 10, 16)
		if err != nil {
			panic(err)
		}
		return HashFromValues(layers.TCPPort(aTCPPort), layers.TCPPort(bTCPPort))
	}
	if fl.Protocol == FlowProtocol_UDPPORT {
		aUDPPort, err := strconv.ParseUint(fl.A, 10, 16)
		if err != nil {
			panic(err)
		}
		bUDPPort, err := strconv.ParseUint(fl.B, 10, 16)
		if err != nil {
			panic(err)
		}
		return HashFromValues(layers.UDPPort(aUDPPort), layers.UDPPort(bUDPPort))
	}
	if fl.Protocol == FlowProtocol_SCTPPORT {
		aSCTPPort, err := strconv.ParseUint(fl.A, 10, 16)
		if err != nil {
			panic(err)
		}
		bSCTPPort, err := strconv.ParseUint(fl.B, 10, 16)
		if err != nil {
			panic(err)
		}
		return HashFromValues(layers.SCTPPort(aSCTPPort), layers.SCTPPort(bSCTPPort))
	}
	return nil
}

func (fl *FlowLayer) HashStr() string {
	return hex.EncodeToString(fl.Hash())
}
