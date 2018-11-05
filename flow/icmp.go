/*
 * Copyright (C) 2018 Red Hat, Inc.
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

import "github.com/google/gopacket/layers"

// ICMPv4TypeToFlowICMPType converts an ICMP type to a Flow ICMPType
func ICMPv4TypeToFlowICMPType(kind uint8) ICMPType {
	switch kind {
	case layers.ICMPv4TypeEchoRequest, layers.ICMPv4TypeEchoReply:
		return ICMPType_ECHO
	case layers.ICMPv4TypeAddressMaskRequest, layers.ICMPv4TypeAddressMaskReply:
		return ICMPType_ADDRESS_MASK
	case layers.ICMPv4TypeDestinationUnreachable:
		return ICMPType_DESTINATION_UNREACHABLE
	case layers.ICMPv4TypeInfoRequest, layers.ICMPv4TypeInfoReply:
		return ICMPType_INFO
	case layers.ICMPv4TypeParameterProblem:
		return ICMPType_PARAMETER_PROBLEM
	case layers.ICMPv4TypeRedirect:
		return ICMPType_REDIRECT
	case layers.ICMPv4TypeRouterSolicitation, layers.ICMPv4TypeRouterAdvertisement:
		return ICMPType_ROUTER
	case layers.ICMPv4TypeSourceQuench:
		return ICMPType_SOURCE_QUENCH
	case layers.ICMPv4TypeTimeExceeded:
		return ICMPType_TIME_EXCEEDED
	case layers.ICMPv4TypeTimestampRequest, layers.ICMPv4TypeTimestampReply:
		return ICMPType_TIMESTAMP
	}

	return ICMPType_UNKNOWN
}

// ICMPv6TypeToFlowICMPType converts an ICMP type to a Flow ICMPType
func ICMPv6TypeToFlowICMPType(kind uint8) ICMPType {
	switch kind {
	case layers.ICMPv6TypeEchoRequest, layers.ICMPv6TypeEchoReply:
		return ICMPType_ECHO
	case layers.ICMPv6TypeNeighborSolicitation, layers.ICMPv6TypeNeighborAdvertisement:
		return ICMPType_NEIGHBOR
	case layers.ICMPv6TypeDestinationUnreachable:
		return ICMPType_DESTINATION_UNREACHABLE
	case layers.ICMPv6TypePacketTooBig:
		return ICMPType_PACKET_TOO_BIG
	case layers.ICMPv6TypeParameterProblem:
		return ICMPType_PARAMETER_PROBLEM
	case layers.ICMPv6TypeRedirect:
		return ICMPType_REDIRECT
	case layers.ICMPv6TypeRouterSolicitation, layers.ICMPv6TypeRouterAdvertisement:
		return ICMPType_ROUTER
	case layers.ICMPv6TypeTimeExceeded:
		return ICMPType_TIME_EXCEEDED
	}

	return ICMPType_UNKNOWN
}
