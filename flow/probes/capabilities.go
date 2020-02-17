/*
 * Copyright (C) 2016 Red Hat, Inc.
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

package probes

import (
	"fmt"
)

// CaptureType describes a list of allowed and default captures probes
type CaptureType struct {
	Allowed []string
	Default string
}

// ProbeCapability defines probe capability
type ProbeCapability int

const (
	// BPFCapability the probe is able to handle bpf filters
	BPFCapability ProbeCapability = 1 << 0
	// RawPacketsCapability the probe can capture raw packets
	RawPacketsCapability = 1 << 1
	// ExtraTCPMetricCapability the probe can report TCP metrics
	ExtraTCPMetricCapability = 1 << 2
	// MultipleOnSameNodeCapability is defined on probes that support multiple captures of the same type on one node
	MultipleOnSameNodeCapability = 1 << 3
)

var (
	// ProbeTypes returns a list of all the capture probes
	ProbeTypes = []string{"ovssflow", "pcapsocket", "ovsmirror", "dpdk", "afpacket", "pcap", "ebpf", "sflow", "ovsnetflow"}

	// CaptureTypes contains all registered capture type and associated probes
	CaptureTypes = map[string]CaptureType{}

	// ProbeCapabilities defines capability per probes
	ProbeCapabilities = map[string]ProbeCapability{}
)

func initCaptureTypes() {
	CaptureTypes["ovsbridge"] = CaptureType{Allowed: []string{"ovssflow", "pcapsocket", "ovsnetflow"}, Default: "ovssflow"}
	CaptureTypes["ovsport"] = CaptureType{Allowed: []string{"ovsmirror"}, Default: "ovsmirror"}
	CaptureTypes["dpdkport"] = CaptureType{Allowed: []string{"dpdk"}, Default: "dpdk"}

	// anything else will be handled by gopacket
	types := []string{
		"internal", "veth", "tun", "bridge", "dummy", "gre",
		"bond", "can", "hsr", "ifb", "macvlan", "macvtap", "vlan", "vxlan",
		"gretap", "ip6gretap", "geneve", "ipoib", "vcan", "ipip", "ipvlan",
		"lowpan", "ip6tnl", "ip6gre", "sit", "device",
	}

	for _, t := range types {
		CaptureTypes[t] = CaptureType{Allowed: []string{"afpacket", "pcap", "pcapsocket", "sflow", "ebpf"}, Default: "afpacket"}
	}
}

// IsCaptureAllowed returns true if the node capture type exist
func IsCaptureAllowed(nodeType string) bool {
	_, ok := CaptureTypes[nodeType]
	return ok
}

func initProbeCapabilities() {
	ProbeCapabilities["afpacket"] = BPFCapability | RawPacketsCapability | ExtraTCPMetricCapability | MultipleOnSameNodeCapability
	ProbeCapabilities["pcap"] = BPFCapability | RawPacketsCapability | ExtraTCPMetricCapability | MultipleOnSameNodeCapability
	ProbeCapabilities["pcapsocket"] = BPFCapability | RawPacketsCapability | ExtraTCPMetricCapability
	ProbeCapabilities["sflow"] = BPFCapability | RawPacketsCapability | ExtraTCPMetricCapability
	ProbeCapabilities["ovssflow"] = BPFCapability | RawPacketsCapability | ExtraTCPMetricCapability
	ProbeCapabilities["dpdk"] = BPFCapability | RawPacketsCapability | ExtraTCPMetricCapability
	ProbeCapabilities["ovsmirror"] = BPFCapability | RawPacketsCapability | ExtraTCPMetricCapability
	ProbeCapabilities["ebpf"] = ExtraTCPMetricCapability
	ProbeCapabilities["ovsnetflow"] = BPFCapability | RawPacketsCapability | ExtraTCPMetricCapability
}

// CheckProbeCapabilities checks that a probe supports given capabilities
func CheckProbeCapabilities(probeType string, capability ProbeCapability) bool {
	if c, ok := ProbeCapabilities[probeType]; ok {
		if (c & capability) > 0 {
			return true
		}
	}
	return false
}

// ProbeTypeForNode returns the appropriate probe type for the given node type
// and capture type.
func ProbeTypeForNode(nodeType string, captureType string) (string, error) {
	probeType := ""
	if captureType != "" {
		types := CaptureTypes[nodeType].Allowed
		for _, t := range types {
			if t == captureType {
				probeType = t
				break
			}
		}
		if probeType == "" {
			return "", fmt.Errorf("Capture type %s not allowed on this node type: %s", captureType, nodeType)
		}
	} else {
		// no capture type defined for this type of node
		c, ok := CaptureTypes[nodeType]
		if !ok {
			return "", nil
		}
		probeType = c.Default
	}
	return probeType, nil
}

func init() {
	initCaptureTypes()
	initProbeCapabilities()
}
