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

package common

// CaptureType describes a list of allowed and default captures probes
type CaptureType struct {
	Allowed []string
	Default string
}

var (
	// CaptureTypes contain all registred capture type and associated probes
	CaptureTypes = map[string]CaptureType{}
)

func initCaptureTypes() {
	// add ovs type
	CaptureTypes["ovsbridge"] = CaptureType{Allowed: []string{"ovssflow", "pcapsocket"}, Default: "ovssflow"}
	CaptureTypes["device"] = CaptureType{Allowed: []string{"afpacket", "pcap", "pcapsocket", "sflow"}, Default: "afpacket"}

	// anything else will be handled by gopacket
	types := []string{
		"internal", "veth", "tun", "bridge", "dummy", "gre",
		"bond", "can", "hsr", "ifb", "macvlan", "macvtap", "vlan", "vxlan",
		"gretap", "ip6gretap", "geneve", "ipoib", "vcan", "ipip", "ipvlan",
		"lowpan", "ip6tnl", "ip6gre", "sit",
	}

	for _, t := range types {
		CaptureTypes[t] = CaptureType{Allowed: []string{"afpacket", "pcap", "pcapsocket"}, Default: "afpacket"}
	}
}

// IsCaptureAllowed returns true if the node capture type exist
func IsCaptureAllowed(nodeType string) bool {
	_, ok := CaptureTypes[nodeType]
	return ok
}

func init() {
	initCaptureTypes()
}
