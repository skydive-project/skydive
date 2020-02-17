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

package flow

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	v "github.com/gima/govalid/v1"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/skydive-project/skydive/graffiti/filters"
	fl "github.com/skydive-project/skydive/flow/layers"
)

func TestFlowReflection(t *testing.T) {
	f := &Flow{}
	if strings.Contains(fmt.Sprintf("%v", f), "PANIC=") == true {
		t.Fail()
	}
}

func TestFlowMetric(t *testing.T) {
	flows := flowsFromPCAP(t, "pcaptraces/simple-tcpv4.pcap", layers.LinkTypeEthernet, nil)
	if len(flows) != 1 {
		t.Error("A single packet must generate 1 flow")
	}

	m := flows[0].Metric
	if m.Start == 0 || m.Last == 0 {
		t.Error("Start/Last empty")
	}

	e := &FlowMetric{
		ABPackets: 5,
		ABBytes:   344,
		BAPackets: 3,
		BABytes:   206,
		RTT:       33000,
		Start:     m.Start,
		Last:      m.Last,
	}

	if !reflect.DeepEqual(m, e) {
		t.Errorf("Expected metric %v not found, got: %v", e, m)
	}

	if flows[0].LastUpdateMetric != nil {
		t.Error("Shouldn't get LastUpdateMetric, as the flow table update didn't call")
	}
}

func TestFlowTruncatedMetric(t *testing.T) {
	flows := flowsFromPCAP(t, "pcaptraces/icmpv4-truncated.pcap", layers.LinkTypeEthernet, nil)
	if len(flows) != 1 {
		t.Error("A single packet must generate 1 flow")
	}

	m := flows[0].Metric
	if m.Start == 0 || m.Last == 0 {
		t.Error("Start/Last empty")
	}

	e := &FlowMetric{
		ABPackets: 1,
		ABBytes:   1066,
		BAPackets: 1,
		BABytes:   1066,
		RTT:       13104000,
		Start:     m.Start,
		Last:      m.Last,
	}

	if !reflect.DeepEqual(m, e) {
		t.Errorf("Expected metric %v not found, got: %v", e, m)
	}

	if flows[0].LastUpdateMetric != nil {
		t.Error("Shouldn't get LastUpdateMetric, as the flow table update didn't call")
	}
}

func TestFlowSimpleIPv4(t *testing.T) {
	flows := flowsFromPCAP(t, "pcaptraces/simple-tcpv4.pcap", layers.LinkTypeEthernet, nil)
	if len(flows) != 1 {
		t.Error("A single packet must generate 1 flow")
	}
	if flows[0].LayersPath != "Ethernet/IPv4/TCP" {
		t.Errorf("Flow LayersPath must be Ethernet/IPv4/TCP got : %s", flows[0].LayersPath)
	}
	if flows[0].Metric.RTT != 33000 {
		t.Errorf("Flow RTT must be 33000 got : %v", flows[0].Metric.RTT)
	}
}

func TestFlowsDNS(t *testing.T) {
	timeStr := []string{"2019-06-06T06:40:14.984235+03:00", "2019-06-06T06:40:24.62057+03:00", "2019-06-06T06:40:32.546622+03:00",
		"2019-06-06T06:40:45.385335+03:00", "2019-06-06T06:40:45.395144+03:00", "2019-06-06T06:40:45.40624+03:00"}
	times := make([]time.Time, 0, 6)
	for _, s := range timeStr {
		t, _ := time.Parse(time.RFC3339Nano, s)
		times = append(times, t)
	}
	opts := TableOpts{ExtraTCPMetric: true, ReassembleTCP: true, IPDefrag: true, ExtraLayers: ExtraLayers(2)}
	expected := []*Flow{
		{
			LayersPath:  "Ethernet/IPv4/UDP/DNS",
			Application: "DNS",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "14:4f:8a:d7:59:3d",
				B:        "40:9b:cd:d2:7b:11",
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "10.0.0.12",
				B:        "10.0.0.138",
			},
			Transport: &TransportLayer{
				Protocol: FlowProtocol_UDP,
				A:        41278,
				B:        53,
			},
			Metric: &FlowMetric{
				ABPackets: 2,
				ABBytes:   150,
				BAPackets: 2,
				BABytes:   683,
			},
			DNS: &fl.DNS{
				ID:           38125,
				QR:           true,
				OpCode:       "Query",
				RD:           true,
				RA:           true,
				ResponseCode: "No Error",
				QDCount:      1,
				ANCount:      2,
				NSCount:      1,
				Questions: []fl.DNSQuestion{
					{
						Name:  "c.go-mpulse.net",
						Type:  "AAAA",
						Class: "IN",
					},
				},
				Answers: []fl.DNSResourceRecord{
					{
						Name:       "c.go-mpulse.net",
						Type:       "CNAME",
						Class:      "IN",
						TTL:        728,
						DataLength: 33,
						CNAME:      "wildcard.go-mpulse.net.edgekey.net",
					},
					{
						Name:       "wildcard.go-mpulse.net.edgekey.net",
						Type:       "CNAME",
						Class:      "IN",
						TTL:        2924,
						DataLength: 21,
						CNAME:      "e4518.x.akamaiedge.net",
					},
				},
				Authorities: []fl.DNSResourceRecord{
					{
						Name:       "x.akamaiedge.net",
						Type:       "SOA",
						Class:      "IN",
						TTL:        5,
						DataLength: 49,
						SOA: &fl.DNSSOA{
							MName:   "n0x.akamaiedge.net",
							RName:   "hostmaster.akamai.com",
							Serial:  1559791419,
							Refresh: 1000,
							Retry:   1000,
							Expire:  1000,
							Minimum: 1800,
						},
					},
				},
				Timestamp: times[0],
			},
		},
		{
			LayersPath:  "Ethernet/IPv4/UDP/DNS",
			Application: "DNS",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "14:4f:8a:d7:59:3d",
				B:        "40:9b:cd:d2:7b:11",
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "10.0.0.12",
				B:        "10.0.0.138",
			},
			Transport: &TransportLayer{
				Protocol: FlowProtocol_UDP,
				A:        33823,
				B:        53,
			},
			Metric: &FlowMetric{
				ABPackets: 1,
				ABBytes:   77,
				BAPackets: 1,
				BABytes:   294,
			},
			DNS: &fl.DNS{
				ID:           40590,
				QR:           true,
				OpCode:       "Query",
				RD:           true,
				RA:           true,
				ResponseCode: "No Error",
				QDCount:      1,
				ANCount:      2,
				NSCount:      3,
				ARCount:      5,
				Questions: []fl.DNSQuestion{
					{
						Name:  "fedoraproject.org",
						Type:  "AAAA",
						Class: "IN",
					},
				},
				Answers: []fl.DNSResourceRecord{
					{
						Name:       "fedoraproject.org",
						Type:       "AAAA",
						Class:      "IN",
						TTL:        7,
						DataLength: 16,
						IP:         "2610:28:3090:3001:dead:beef:cafe:fed3",
					},
					{
						Name:       "fedoraproject.org",
						Type:       "AAAA",
						Class:      "IN",
						TTL:        7,
						DataLength: 16,
						IP:         "2604:1580:fe00:0:dead:beef:cafe:fed1",
					},
				},
				Authorities: []fl.DNSResourceRecord{
					{
						Name:       "fedoraproject.org",
						Type:       "NS",
						Class:      "IN",
						TTL:        67350,
						DataLength: 7,
						NS:         "ns04.fedoraproject.org",
					},
					{
						Name:       "fedoraproject.org",
						Type:       "NS",
						Class:      "IN",
						TTL:        67350,
						DataLength: 7,
						NS:         "ns02.fedoraproject.org",
					},
					{
						Name:       "fedoraproject.org",
						Type:       "NS",
						Class:      "IN",
						TTL:        67350,
						DataLength: 7,
						NS:         "ns05.fedoraproject.org",
					},
				},
				Additionals: []fl.DNSResourceRecord{
					{
						Name:       "ns02.fedoraproject.org",
						Type:       "A",
						Class:      "IN",
						TTL:        85167,
						DataLength: 4,
						IP:         "152.19.134.139",
					},
					{
						Name:       "ns04.fedoraproject.org",
						Type:       "A",
						Class:      "IN",
						TTL:        85167,
						DataLength: 4,
						IP:         "209.132.181.17",
					},
					{
						Name:       "ns05.fedoraproject.org",
						Type:       "A",
						Class:      "IN",
						TTL:        85167,
						DataLength: 4,
						IP:         "85.236.55.10",
					},
					{
						Name:       "ns02.fedoraproject.org",
						Type:       "AAAA",
						Class:      "IN",
						TTL:        85167,
						DataLength: 16,
						IP:         "2610:28:3090:3001:dead:beef:cafe:fed5",
					},
					{
						Name:       "ns05.fedoraproject.org",
						Type:       "AAAA",
						Class:      "IN",
						TTL:        85167,
						DataLength: 16,
						IP:         "2001:4178:2:1269:dead:beef:cafe:fed5",
					},
				},
				Timestamp: times[1],
			},
		},
		{
			LayersPath:  "Ethernet/IPv4/UDP/DNS",
			Application: "DNS",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "14:4f:8a:d7:59:3d",
				B:        "40:9b:cd:d2:7b:11",
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "10.0.0.12",
				B:        "10.0.0.138",
			},
			Transport: &TransportLayer{
				Protocol: FlowProtocol_UDP,
				A:        56656,
				B:        53,
			},
			Metric: &FlowMetric{
				ABPackets: 2,
				ABBytes:   148,
				BAPackets: 2,
				BABytes:   510,
			},
			DNS: &fl.DNS{
				ID:           28799,
				QR:           true,
				OpCode:       "Query",
				RD:           true,
				RA:           true,
				ResponseCode: "No Error",
				QDCount:      1,
				NSCount:      1,
				Questions: []fl.DNSQuestion{
					{
						Name:  "ipv4.adrta.com",
						Type:  "AAAA",
						Class: "IN",
					},
				},
				Authorities: []fl.DNSResourceRecord{
					{
						Name:       "adrta.com",
						Type:       "SOA",
						Class:      "IN",
						TTL:        900,
						DataLength: 69,
						SOA: &fl.DNSSOA{
							MName:   "ns-615.awsdns-12.net",
							RName:   "awsdns-hostmaster.amazon.com",
							Serial:  1,
							Refresh: 7200,
							Retry:   900,
							Expire:  1209600,
							Minimum: 86400,
						},
					},
				},
				Timestamp: times[2],
			},
		},
		{
			LayersPath:  "Ethernet/IPv4/UDP/DNS",
			Application: "DNS",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "14:4f:8a:d7:59:3d",
				B:        "40:9b:cd:d2:7b:11",
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "10.0.0.12",
				B:        "10.0.0.138",
			},
			Transport: &TransportLayer{
				Protocol: FlowProtocol_UDP,
				A:        54893,
				B:        53,
			},
			Metric: &FlowMetric{
				ABPackets: 1,
				ABBytes:   77,
				BAPackets: 1,
				BABytes:   354,
			},
			DNS: &fl.DNS{
				ID:           27181,
				QR:           true,
				OpCode:       "Query",
				RD:           true,
				RA:           true,
				ResponseCode: "No Error",
				QDCount:      1,
				ANCount:      9,
				NSCount:      3,
				ARCount:      4,
				Questions: []fl.DNSQuestion{
					{
						Name:  "fedoraproject.org",
						Type:  "A",
						Class: "IN",
					},
				},
				Answers: []fl.DNSResourceRecord{
					{
						Name:       "fedoraproject.org",
						Type:       "A",
						Class:      "IN",
						TTL:        44,
						DataLength: 4,
						IP:         "209.132.181.16",
					},
					{
						Name:       "fedoraproject.org",
						Type:       "A",
						Class:      "IN",
						TTL:        44,
						DataLength: 4,
						IP:         "209.132.181.15",
					},
					{
						Name:       "fedoraproject.org",
						Type:       "A",
						Class:      "IN",
						TTL:        44,
						DataLength: 4,
						IP:         "8.43.85.67",
					},
					{
						Name:       "fedoraproject.org",
						Type:       "A",
						Class:      "IN",
						TTL:        44,
						DataLength: 4,
						IP:         "67.219.144.68",
					},
					{
						Name:       "fedoraproject.org",
						Type:       "A",
						Class:      "IN",
						TTL:        44,
						DataLength: 4,
						IP:         "152.19.134.198",
					},
					{
						Name:       "fedoraproject.org",
						Type:       "A",
						Class:      "IN",
						TTL:        44,
						DataLength: 4,
						IP:         "209.132.190.2",
					},
					{
						Name:       "fedoraproject.org",
						Type:       "A",
						Class:      "IN",
						TTL:        44,
						DataLength: 4,
						IP:         "140.211.169.206",
					},
					{
						Name:       "fedoraproject.org",
						Type:       "A",
						Class:      "IN",
						TTL:        44,
						DataLength: 4,
						IP:         "185.141.165.254",
					},
					{
						Name:       "fedoraproject.org",
						Type:       "A",
						Class:      "IN",
						TTL:        44,
						DataLength: 4,
						IP:         "152.19.134.142",
					},
				},
				Authorities: []fl.DNSResourceRecord{
					{
						Name:       "fedoraproject.org",
						Type:       "NS",
						Class:      "IN",
						TTL:        67329,
						DataLength: 7,
						NS:         "ns04.fedoraproject.org",
					},
					{
						Name:       "fedoraproject.org",
						Type:       "NS",
						Class:      "IN",
						TTL:        67329,
						DataLength: 7,
						NS:         "ns02.fedoraproject.org",
					},
					{
						Name:       "fedoraproject.org",
						Type:       "NS",
						Class:      "IN",
						TTL:        67329,
						DataLength: 7,
						NS:         "ns05.fedoraproject.org",
					},
				},
				Additionals: []fl.DNSResourceRecord{
					{
						Name:       "ns02.fedoraproject.org",
						Type:       "A",
						Class:      "IN",
						TTL:        85146,
						DataLength: 4,
						IP:         "152.19.134.139",
					},
					{
						Name:       "ns04.fedoraproject.org",
						Type:       "A",
						Class:      "IN",
						TTL:        85146,
						DataLength: 4,
						IP:         "209.132.181.17",
					},
					{
						Name:       "ns05.fedoraproject.org",
						Type:       "A",
						Class:      "IN",
						TTL:        85146,
						DataLength: 4,
						IP:         "85.236.55.10",
					},
					{
						Name:       "ns02.fedoraproject.org",
						Type:       "AAAA",
						Class:      "IN",
						TTL:        85146,
						DataLength: 16,
						IP:         "2610:28:3090:3001:dead:beef:cafe:fed5",
					},
				},
				Timestamp: times[3],
			},
		},
		{
			LayersPath:  "Ethernet/IPv4/UDP/DNS",
			Application: "DNS",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "14:4f:8a:d7:59:3d",
				B:        "40:9b:cd:d2:7b:11",
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "10.0.0.12",
				B:        "10.0.0.138",
			},
			Transport: &TransportLayer{
				Protocol: FlowProtocol_UDP,
				A:        42992,
				B:        53,
			},
			Metric: &FlowMetric{
				ABPackets: 1,
				ABBytes:   74,
				BAPackets: 1,
				BABytes:   440,
			},
			DNS: &fl.DNS{
				ID:           43789,
				QR:           true,
				OpCode:       "Query",
				RD:           true,
				RA:           true,
				ResponseCode: "No Error",
				QDCount:      1,
				ANCount:      2,
				NSCount:      8,
				ARCount:      6,
				Questions: []fl.DNSQuestion{
					{
						Name:  "www.github.com",
						Type:  "A",
						Class: "IN",
					},
				},
				Answers: []fl.DNSResourceRecord{
					{
						Name:       "www.github.com",
						Type:       "CNAME",
						Class:      "IN",
						TTL:        2154,
						DataLength: 2,
						CNAME:      "github.com",
					},
					{
						Name:       "github.com",
						Type:       "A",
						Class:      "IN",
						TTL:        60,
						DataLength: 4,
						IP:         "140.82.118.4",
					},
				},
				Authorities: []fl.DNSResourceRecord{
					{
						Name:       "github.com",
						Type:       "NS",
						Class:      "IN",
						TTL:        154873,
						DataLength: 20,
						NS:         "ns3.p16.dynect.net",
					},
					{
						Name:       "github.com",
						Type:       "NS",
						Class:      "IN",
						TTL:        154873,
						DataLength: 6,
						NS:         "ns4.p16.dynect.net",
					},
					{
						Name:       "github.com",
						Type:       "NS",
						Class:      "IN",
						TTL:        154873,
						DataLength: 23,
						NS:         "ns-1283.awsdns-32.org",
					},
					{
						Name:       "github.com",
						Type:       "NS",
						Class:      "IN",
						TTL:        154873,
						DataLength: 22,
						NS:         "ns-421.awsdns-52.COM",
					},
					{
						Name:       "github.com",
						Type:       "NS",
						Class:      "IN",
						TTL:        154873,
						DataLength: 19,
						NS:         "ns-520.awsdns-01.net",
					},
					{
						Name:       "github.com",
						Type:       "NS",
						Class:      "IN",
						TTL:        154873,
						DataLength: 6,
						NS:         "ns2.p16.dynect.net",
					},
					{
						Name:       "github.com",
						Type:       "NS",
						Class:      "IN",
						TTL:        154873,
						DataLength: 25,
						NS:         "ns-1707.awsdns-21.co.uk",
					},
					{
						Name:       "github.com",
						Type:       "NS",
						Class:      "IN",
						TTL:        154873,
						DataLength: 6,
						NS:         "ns1.p16.dynect.net",
					},
				},
				Additionals: []fl.DNSResourceRecord{
					{
						Name:       "ns1.p16.dynect.net",
						Type:       "A",
						Class:      "IN",
						TTL:        42402,
						DataLength: 4,
						IP:         "208.78.70.16",
					},
					{
						Name:       "ns2.p16.dynect.net",
						Type:       "A",
						Class:      "IN",
						TTL:        71546,
						DataLength: 4,
						IP:         "204.13.250.16",
					},
					{
						Name:       "ns-421.awsdns-52.com",
						Type:       "A",
						Class:      "IN",
						TTL:        106592,
						DataLength: 4,
						IP:         "205.251.193.165",
					},
					{
						Name:       "ns-520.awsdns-01.net",
						Type:       "A",
						Class:      "IN",
						TTL:        66060,
						DataLength: 4,
						IP:         "205.251.194.8",
					},
					{
						Name:       "ns-1283.awsdns-32.org",
						Type:       "A",
						Class:      "IN",
						TTL:        38047,
						DataLength: 4,
						IP:         "205.251.197.3",
					},
					{
						Name:       "ns-1707.awsdns-21.co.uk",
						Type:       "A",
						Class:      "IN",
						TTL:        44919,
						DataLength: 4,
						IP:         "205.251.198.171",
					},
				},
				Timestamp: times[4],
			},
		},
		{
			LayersPath:  "Ethernet/IPv4/UDP/DNS",
			Application: "DNS",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "14:4f:8a:d7:59:3d",
				B:        "40:9b:cd:d2:7b:11",
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "10.0.0.12",
				B:        "10.0.0.138",
			},
			Transport: &TransportLayer{
				Protocol: FlowProtocol_UDP,
				A:        57048,
				B:        53,
			},
			Metric: &FlowMetric{
				ABPackets: 1,
				ABBytes:   70,
				BAPackets: 1,
				BABytes:   154,
			},
			DNS: &fl.DNS{
				ID:           64755,
				QR:           true,
				OpCode:       "Query",
				RD:           true,
				RA:           true,
				ResponseCode: "No Error",
				QDCount:      1,
				NSCount:      1,
				Questions: []fl.DNSQuestion{
					{
						Name:  "github.com",
						Type:  "AAAA",
						Class: "IN",
					},
				},
				Authorities: []fl.DNSResourceRecord{
					{
						Name:       "github.com",
						Type:       "SOA",
						Class:      "IN",
						TTL:        851,
						DataLength: 72,
						SOA: &fl.DNSSOA{
							MName:   "ns-1707.awsdns-21.co.uk",
							RName:   "awsdns-hostmaster.amazon.com",
							Serial:  1,
							Refresh: 7200,
							Retry:   900,
							Expire:  1209600,
							Minimum: 86400,
						},
					},
				},
				Timestamp: times[5],
			},
		},
	}

	validatePCAP(t, "pcaptraces/dns.pcap", layers.LinkTypeEthernet, nil, expected, opts)
}

func TestFlowSimpleIPv6(t *testing.T) {
	flows := flowsFromPCAP(t, "pcaptraces/simple-tcpv6.pcap", layers.LinkTypeEthernet, nil)
	if len(flows) != 1 {
		t.Error("A single packet must generate 1 flow")
	}
	if flows[0].LayersPath != "Ethernet/IPv6/TCP" {
		t.Errorf("Flow LayersPath must be Ethernet/IPv6/TCP got : %s", flows[0].LayersPath)
	}
	if flows[0].Metric.RTT != 28000 {
		t.Errorf("Flow RTT must be 28000 got : %v", flows[0].Metric.RTT)
	}
}

func TestFlowERSpanII(t *testing.T) {
	flows := flowsFromPCAP(t, "pcaptraces/erspanII.pcap", layers.LinkTypeEthernet, nil)
	if len(flows) != 2 {
		t.Error("ERSpan packet must generate 2 flows")
	}
	if flows[0].LayersPath != "Ethernet/IPv4/GRE" {
		t.Errorf("Flow LayersPath must be Ethernet/IPv4/GRE got : %s", flows[0].LayersPath)
	}
	if flows[1].LayersPath != "Ethernet/IPv4/ICMPv4" {
		t.Errorf("Flow LayersPath must be Ethernet/IPv4/ICMPv4 got : %s", flows[1].LayersPath)
	}
}

func TestFlowIPv4DefragDisabled(t *testing.T) {
	opt := TableOpts{ExtraTCPMetric: true, ReassembleTCP: true, IPDefrag: false}
	flows := flowsFromPCAP(t, "pcaptraces/ipv4-fragments.pcap", layers.LinkTypeEthernet, nil, opt)
	if len(flows) != 2 {
		t.Error("A fragmented packets must generate 2 flow", len(flows))
	}
	if flows[0].LayersPath != "Ethernet/IPv4/Fragment" {
		if flows[1].LayersPath != "Ethernet/IPv4/Fragment" {
			t.Errorf("Flow LayersPath must be Ethernet/IPv4/Fragment got : %s %s", flows[0].LayersPath, flows[1].LayersPath)
		}
	}
}

func TestFlowIPv4DefragEnabled(t *testing.T) {
	flows := flowsFromPCAP(t, "pcaptraces/ipv4-fragments.pcap", layers.LinkTypeEthernet, nil)
	if len(flows) != 1 {
		t.Error("A fragmented packets must generate 1 flow", len(flows))
	}
	if flows[0].LayersPath != "Ethernet/IPv4/Fragment/ICMPv4" {
		t.Errorf("Flow LayersPath must be Ethernet/IPv4/Fragment/ICMPv4 got : %s", flows[0].LayersPath)
	}
	if flows[0].IPMetric.FragmentErrors != 0 || flows[0].IPMetric.Fragments != 1 {
		t.Errorf("Flow IPMetric fragmentErrors %d fragments %d", flows[0].IPMetric.FragmentErrors, flows[0].IPMetric.Fragments)
	}
}

func TestFlowTCPSegments(t *testing.T) {
	flows := flowsFromPCAP(t, "pcaptraces/eth-ipv4-tcp-http-ooo.pcap", layers.LinkTypeEthernet, nil)
	if len(flows) != 1 {
		t.Error("A out of order tcp packets must generate 1 flow", len(flows))
	}
	if flows[0].LayersPath != "Ethernet/IPv4/TCP" {
		t.Errorf("Flow LayersPath must be Ethernet/IPv4/TCP got : %s", flows[0].LayersPath)
	}
	m := flows[0].TCPMetric
	if !(m.ABSegmentOutOfOrder == 0 && m.BASegmentOutOfOrder == 3) {
		t.Errorf("Flow SegmentOutOfOrder do not match, got : %d %d", m.ABSegmentOutOfOrder, m.BASegmentOutOfOrder)
	}
}

func TestBPFFilter(t *testing.T) {
	bpf, err := NewBPF(layers.LinkTypeEthernet, DefaultCaptureLength, "port 53 or port 80")
	if err != nil {
		t.Error(err)
	}

	flows := flowsFromPCAP(t, "pcaptraces/eth-ip4-arp-dns-req-http-google.pcap", layers.LinkTypeEthernet, bpf)
	if len(flows) != 4 {
		t.Errorf("A single packet must generate 1 flow got : %v", flows)
	}
}

func TestFlowJSON(t *testing.T) {
	f := Flow{
		UUID:       "uuid-1",
		LayersPath: "layerpath-1",
		Link: &FlowLayer{
			Protocol: FlowProtocol_ETHERNET,
			A:        "value-1",
			B:        "value-2",
		},
		ICMP: &ICMPLayer{
			Type: ICMPType_ECHO,
		},
		Start:            1111,
		Last:             222,
		LastUpdateMetric: &FlowMetric{},
		Metric: &FlowMetric{
			ABBytes:   33,
			ABPackets: 34,
			BABytes:   44,
			BAPackets: 55,
			Start:     1111111,
			Last:      2222222,
		},
		NodeTID: "probe-tid",
	}

	j, err := json.Marshal(f)
	if err != nil {
		t.Error(err)
	}

	schema := v.Object(
		v.ObjKV("UUID", v.String()),
		v.ObjKV("LayersPath", v.String()),
		v.ObjKV("NodeTID", v.String()),
		v.ObjKV("Start", v.Number()),
		v.ObjKV("Last", v.Number()),
		v.ObjKV("Link", v.Object(
			v.ObjKV("Protocol", v.String()),
			v.ObjKV("A", v.String()),
			v.ObjKV("B", v.String()),
		)),
		v.ObjKV("ICMP", v.Object(
			v.ObjKV("Type", v.String()),
		)),
		v.ObjKV("LastUpdateMetric", v.Object(
			v.ObjKV("ABPackets", v.Number()),
			v.ObjKV("ABBytes", v.Number()),
			v.ObjKV("BAPackets", v.Number()),
			v.ObjKV("BABytes", v.Number()),
			v.ObjKV("Start", v.Number()),
			v.ObjKV("Last", v.Number()),
		)),
		v.ObjKV("Metric", v.Object(
			v.ObjKV("ABPackets", v.Number()),
			v.ObjKV("ABBytes", v.Number()),
			v.ObjKV("BAPackets", v.Number()),
			v.ObjKV("BABytes", v.Number()),
			v.ObjKV("Start", v.Number()),
			v.ObjKV("Last", v.Number()),
		),
		))

	var data interface{}
	if err := json.Unmarshal(j, &data); err != nil {
		t.Fatal("JSON parsing failed. Err =", err)
	}

	if path, err := schema.Validate(data); err != nil {
		t.Fatalf("Validation failed at %s. Error (%s)", path, err)
	}

	var e Flow
	if err := json.Unmarshal(j, &e); err != nil {
		t.Fatal("JSON parsing failed. Err =", err)
	}

	if !reflect.DeepEqual(f, e) {
		t.Fatal("Unmarshalled flow not equal to the original")
	}
}

func compareFlowLayer(expected, tested *FlowLayer) bool {
	if tested == nil {
		return false
	}

	return expected.Protocol == tested.Protocol && expected.A == tested.A && expected.B == tested.B && expected.ID == tested.ID
}

func compareTransportLayer(expected, tested *TransportLayer) bool {
	if tested == nil {
		return false
	}

	return expected.Protocol == tested.Protocol && expected.A == tested.A && expected.B == tested.B && expected.ID == tested.ID
}

func compareFlowMetric(expected, tested *FlowMetric) bool {
	if tested == nil {
		return false
	}

	return expected.ABBytes == tested.ABBytes && expected.ABPackets == tested.ABPackets &&
		expected.BABytes == tested.BABytes && expected.BAPackets == tested.BAPackets
}

func compareDNSSOA(expected, tested *fl.DNSSOA) bool {
	if expected == nil && tested == nil {
		return true
	}
	if expected == nil || tested == nil {
		return false
	}

	return expected.MName == tested.MName && expected.RName == tested.RName && expected.Serial == tested.Serial &&
		expected.Refresh == tested.Refresh && expected.Retry == tested.Retry && expected.Expire == tested.Expire &&
		expected.Minimum == tested.Minimum
}

func compareDNSSRV(expected, tested *fl.DNSSRV) bool {
	if expected == nil && tested == nil {
		return true
	}
	if expected == nil || tested == nil {
		return false
	}

	return expected.Priority == tested.Priority && expected.Weight == tested.Weight && expected.Port == tested.Port &&
		expected.Name == tested.Name
}

func compareDNSMX(expected, tested *fl.DNSMX) bool {
	if expected == nil && tested == nil {
		return true
	}
	if expected == nil || tested == nil {
		return false
	}

	return expected.Preference == tested.Preference && expected.Name == tested.Name
}

func compareDNSTXTs(expected, tested []string) bool {
	if expected == nil && tested == nil {
		return true
	}
	if expected == nil || tested == nil {
		return false
	}
	if len(tested) != len(expected) {
		return false
	}

	for _, e := range expected {
		found := false
		for _, t := range tested {
			if e == t {
				found = true
			}
		}
		if !found {
			return false
		}
	}

	return true
}

func compareDNSQuestions(expected, tested []fl.DNSQuestion) bool {
	if expected == nil && tested == nil {
		return true
	}
	if expected == nil || tested == nil {
		return false
	}
	if len(tested) != len(expected) {
		return false
	}
	for _, e := range expected {
		found := false
		for _, t := range tested {
			if e.Name == t.Name && e.Type == t.Type && e.Class == t.Class {
				found = true
			}
		}
		if !found {
			return false
		}
	}

	return true
}

func compareDNSResourceRecords(expected, tested []fl.DNSResourceRecord) bool {
	if expected == nil && tested == nil {
		return true
	}
	if expected == nil || tested == nil {
		return false
	}
	if len(tested) != len(expected) {
		return false
	}
	for _, e := range expected {
		found := false
		for _, t := range tested {
			if e.Name == t.Name && e.Type == t.Type && e.Class == t.Class && e.TTL == t.TTL &&
				e.DataLength == t.DataLength && e.IP == t.IP && e.NS == t.NS && e.CNAME == t.CNAME &&
				e.PTR == t.PTR && compareDNSTXTs(e.TXTs, t.TXTs) && compareDNSSOA(e.SOA, t.SOA) &&
				compareDNSSRV(e.SRV, t.SRV) && compareDNSMX(e.MX, t.MX) {
				found = true
			}
		}
		if !found {
			return false
		}
	}

	return true
}

func compareDNSLayer(expected, tested *fl.DNS) bool {
	if expected == nil && tested == nil {
		return true
	}
	if expected == nil || tested == nil {
		return false
	}

	if expected.ID != tested.ID || expected.QR != tested.QR || expected.OpCode != tested.OpCode ||
		expected.RD != tested.RD || expected.RA != tested.RA || expected.Z != tested.Z || expected.ResponseCode != tested.ResponseCode ||
		expected.QDCount != tested.QDCount || expected.ANCount != tested.ANCount || expected.NSCount != tested.NSCount ||
		expected.ARCount != tested.ARCount || expected.Timestamp.UTC() != tested.Timestamp.UTC() {
		return false
	}

	if (expected.Questions != nil && tested.Questions == nil) || (expected.Questions == nil && tested.Questions != nil) {
		return false
	}
	if expected.Questions != nil && !compareDNSQuestions(expected.Questions, tested.Questions) {
		return false
	}

	if (expected.Answers != nil && tested.Answers == nil) || (expected.Answers == nil && tested.Answers != nil) {
		return false
	}
	if expected.Answers != nil && !compareDNSResourceRecords(expected.Answers, tested.Answers) {
		return false
	}

	if (expected.Authorities != nil && tested.Authorities == nil) || (expected.Authorities == nil && tested.Authorities != nil) {
		return false
	}
	if expected.Authorities != nil && !compareDNSResourceRecords(expected.Authorities, tested.Authorities) {
		return false
	}

	if (expected.Additionals != nil && tested.Additionals == nil) || (expected.Additionals == nil && tested.Additionals != nil) {
		return false
	}
	if expected.Additionals != nil && !compareDNSResourceRecords(expected.Additionals, tested.Additionals) {
		return false
	}

	return true
}

func compareFlow(expected, tested *Flow) bool {
	if expected.LayersPath != "" && expected.LayersPath != tested.LayersPath {
		return false
	}
	if expected.Application != "" && expected.Application != tested.Application {
		return false
	}
	if expected.TrackingID != "" && expected.TrackingID != tested.TrackingID {
		return false
	}
	if expected.ParentUUID != "" && expected.ParentUUID != tested.ParentUUID {
		return false
	}
	if expected.Link != nil && !compareFlowLayer(expected.Link, tested.Link) {
		return false
	}
	if expected.Network != nil && !compareFlowLayer(expected.Network, tested.Network) {
		return false
	}
	if expected.Transport != nil && !compareTransportLayer(expected.Transport, tested.Transport) {
		return false
	}
	if expected.Metric != nil && !compareFlowMetric(expected.Metric, tested.Metric) {
		return false
	}
	if expected.LastUpdateMetric != nil && !compareFlowMetric(expected.LastUpdateMetric,
		tested.LastUpdateMetric) {
		return false
	}
	if (expected.DNS == nil && tested.DNS != nil) || (expected.DNS != nil && tested.DNS == nil) {
		return false
	}
	if expected.DNS != nil && !compareDNSLayer(expected.DNS, tested.DNS) {
		return false
	}

	return true
}

func checkGoPacketSanity(t *testing.T, p gopacket.Packet) {
	if errLayer := p.ErrorLayer(); errLayer != nil {
		if l := p.Layer(gopacket.LayerTypeDecodeFailure); l != nil {
			df := l.(*gopacket.DecodeFailure)
			if len(df.Dump()) > 0 {
				t.Fatalf("packet made gopacket panic : %s %s", p.Dump(), df.Dump())
			}
		}
	}
}

func fillTableFromPCAP(t *testing.T, table *Table, filename string, linkType layers.LinkType, bpf *BPF) {
	handleRead, err := pcap.OpenOffline(filename)
	if err != nil {
		t.Fatal("PCAP OpenOffline error (handle to read packet): ", err)
	}
	defer handleRead.Close()

	for {
		data, ci, err := handleRead.ReadPacketData()
		if err != nil && err != io.EOF {
			t.Fatal("PCAP OpenOffline error (handle to read packet): ", err)
		} else if err == io.EOF {
			break
		} else {
			p := gopacket.NewPacket(data, linkType, gopacket.Default)
			p.Metadata().CaptureInfo = ci
			checkGoPacketSanity(t, p)

			ps := PacketSeqFromGoPacket(p, 0, bpf, table.IPDefragger())
			table.processPacketSeq(ps)
		}
	}
}

func getFlowChain(t *testing.T, table *Table, uuid string, flowChain map[string]*Flow) {
	// lookup for the parent
	searchQuery := &filters.SearchQuery{
		Filter: filters.NewTermStringFilter("UUID", uuid),
	}

	flows := table.getFlows(searchQuery).GetFlows()
	if len(flows) != 1 {
		t.Errorf("Should return only one flow got : %+v", flows)
	}
	fl := flows[0]

	flowChain[fl.UUID] = fl

	if fl.ParentUUID != "" {
		getFlowChain(t, table, fl.ParentUUID, flowChain)
	}
}

func validateAllParentChains(t *testing.T, table *Table) {
	searchQuery := &filters.SearchQuery{
		Filter: filters.NewNotNullFilter("ParentUUID"),
	}

	flowChain := make(map[string]*Flow, 0)

	flows := table.getFlows(searchQuery).GetFlows()
	for _, fl := range flows {
		flowChain[fl.UUID] = fl

		if fl.ParentUUID != "" {
			getFlowChain(t, table, fl.UUID, flowChain)
		}
	}

	// we should have touch all the flow
	flows = table.getFlows(&filters.SearchQuery{}).GetFlows()
	if len(flows) != len(flowChain) {
		t.Errorf("Flow parent chain is incorrect : %+v", flows)
	}
}

func flowsFromPCAP(t *testing.T, filename string, linkType layers.LinkType, bpf *BPF, opts ...TableOpts) []*Flow {
	opt := TableOpts{ExtraTCPMetric: true, ReassembleTCP: true, IPDefrag: true}
	if len(opts) > 0 {
		opt = opts[0]
	}

	table := NewTable(time.Second, time.Second, &fakeMessageSender{}, UUIDs{}, opt)
	fillTableFromPCAP(t, table, filename, linkType, bpf)
	validateAllParentChains(t, table)

	return table.getFlows(&filters.SearchQuery{}).Flows
}

func validatePCAP(t *testing.T, filename string, linkType layers.LinkType, bpf *BPF, expected []*Flow, opts ...TableOpts) {
	flows := flowsFromPCAP(t, filename, linkType, bpf, opts...)
	for _, e := range expected {
		found := false
		for _, f := range flows {
			if compareFlow(e, f) {
				found = true
			}

			// check timestamp > 0
			if f.GetStart() < 0 || f.GetLast() < 0 {
				f, _ := json.MarshalIndent(flows, "", "\t")
				t.Errorf("Wrong timestamps: %s", string(f))
			}
		}
		if !found {
			je, _ := json.MarshalIndent(e, "", "\t")
			f, _ := json.MarshalIndent(flows, "", "\t")
			t.Errorf("Flows mismatch, \nexpected %s\ngot  %s\n", string(je), string(f))
		}
	}
}

func TestPCAP1(t *testing.T) {
	expected := []*Flow{
		{
			LayersPath:  "Ethernet/ARP",
			Application: "ARP",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "fa:16:3e:29:e0:82",
				B:        "ff:ff:ff:ff:ff:ff",
			},
			Metric: &FlowMetric{
				ABPackets: 1,
				ABBytes:   42,
				BAPackets: 0,
				BABytes:   0,
			},
		},
		{
			LayersPath:  "Ethernet/IPv4/UDP/DNS",
			Application: "DNS",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "fa:16:3e:29:e0:82",
				B:        "fa:16:3e:96:06:e8",
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "192.168.0.5",
				B:        "8.8.8.8",
			},
			Transport: &TransportLayer{
				Protocol: FlowProtocol_UDP,
				A:        37686,
				B:        53,
			},
			Metric: &FlowMetric{
				ABPackets: 2,
				ABBytes:   148,
				BAPackets: 2,
				BABytes:   256,
			},
		},
		{
			LayersPath:  "Ethernet/IPv4/TCP",
			Application: "TCP",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "fa:16:3e:29:e0:82",
				B:        "fa:16:3e:96:06:e8",
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "192.168.0.5",
				B:        "173.194.40.147",
			},
			Transport: &TransportLayer{
				Protocol: FlowProtocol_TCP,
				A:        47838,
				B:        80,
			},
			Metric: &FlowMetric{
				ABPackets: 6,
				ABBytes:   516,
				BAPackets: 4,
				BABytes:   760,
			},
		},
		{
			LayersPath:  "Ethernet/IPv4/UDP/DNS",
			Application: "DNS",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "fa:16:3e:29:e0:82",
				B:        "fa:16:3e:96:06:e8",
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "192.168.0.5",
				B:        "8.8.8.8",
			},
			Transport: &TransportLayer{
				Protocol: FlowProtocol_UDP,
				A:        33553,
				B:        53,
			},
			Metric: &FlowMetric{
				ABPackets: 2,
				ABBytes:   146,
				BAPackets: 2,
				BABytes:   190,
			},
		},
		{
			LayersPath:  "Ethernet/IPv4/TCP",
			Application: "TCP",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "fa:16:3e:29:e0:82",
				B:        "fa:16:3e:96:06:e8",
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "192.168.0.5",
				B:        "216.58.211.67",
			},
			Transport: &TransportLayer{
				Protocol: FlowProtocol_TCP,
				A:        54785,
				B:        80,
			},
			Metric: &FlowMetric{
				ABPackets: 20,
				ABBytes:   1475,
				BAPackets: 18,
				BABytes:   21080,
			},
		},
	}

	validatePCAP(t, "pcaptraces/eth-ip4-arp-dns-req-http-google.pcap", layers.LinkTypeEthernet, nil, expected)
}

func TestEmptyParentUUIDExported(t *testing.T) {
	flow := &Flow{}

	m, err := json.Marshal(&flow)
	if err != nil {
		t.Fatal(err)
	}

	var i map[string]interface{}
	if err = json.Unmarshal(m, &i); err != nil {
		t.Fatal(err)
	}

	if _, ok := i["ParentUUID"]; !ok {
		t.Fatal("ParentUUID field should always be exported")
	}
}

// This trace contains two packets sniffed by one capture, on one
// interface. One packet layerpath is
// Ethernet/IPv4/UDP/MPLS/IPv4/ICMPv4 while the other one is
// Ethernet/IPv4/UDP/MPLS/Ethernet/IPv4/TCP
//
// Contrail can remove the ethernet header of the packet generated by
// a VM when it pushes it into the tunnel. In particular, if source
// and destination IPs  don't belong to the same network, Contrail
// remove the ethernet header. So, the MPLS payload can be ethernet or
// IP.
func TestPCAPMplsContrail(t *testing.T) {
	expected := []*Flow{
		{
			LayersPath:  "IPv4/ICMPv4",
			Application: "ICMPv4",
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "10.65.65.5",
				B:        "10.35.35.3",
			},
			Metric: &FlowMetric{
				ABPackets: 1,
				ABBytes:   104,
			},
		},
		{
			LayersPath:  "Ethernet/IPv4/UDP/MPLS",
			Application: "MPLS",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "d4:ae:52:9e:5d:2f",
				B:        "90:b1:1c:0f:bb:b8",
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "10.11.0.56",
				B:        "10.11.0.55",
			},
			Transport: &TransportLayer{
				Protocol: FlowProtocol_UDP,
				A:        53580,
				B:        51234,
			},
			Metric: &FlowMetric{
				ABPackets: 1,
				ABBytes:   120,
			},
		},
		{
			LayersPath:  "Ethernet/IPv4/TCP",
			Application: "TCP",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "02:47:10:72:2d:5b",
				B:        "02:70:7f:82:12:ab",
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "10.65.65.4",
				B:        "10.65.65.5",
			},
			Transport: &TransportLayer{
				Protocol: FlowProtocol_TCP,
				A:        52477,
				B:        80,
			},
			Metric: &FlowMetric{
				ABPackets: 1,
				ABBytes:   74,
			},
		},
		{
			LayersPath:  "Ethernet/IPv4/UDP/MPLS",
			Application: "MPLS",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "90:b1:1c:0f:bb:b8",
				B:        "d4:ae:52:9e:5d:2f",
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "10.11.0.55",
				B:        "10.11.0.56",
			},
			Transport: &TransportLayer{
				Protocol: FlowProtocol_UDP,
				A:        51822,
				B:        51234,
			},
			Metric: &FlowMetric{
				ABPackets: 1,
				ABBytes:   150,
			},
		},
	}

	layers.RegisterUDPPortLayerType(layers.UDPPort(51234), layers.LayerTypeMPLS)
	validatePCAP(t, "pcaptraces/contrail-udp-mpls-eth-and-ipv4.pcap", layers.LinkTypeEthernet, nil, expected)
}

func TestPCAPL3TrackingID(t *testing.T) {
	var l3TrackingID string

	flows := flowsFromPCAP(t, "pcaptraces/ping-with-without-ethernet.pcap", layers.LinkTypeEthernet, nil)
	for _, flow := range flows {
		if flow.Application == "ICMPv4" {
			if l3TrackingID == "" {
				l3TrackingID = flow.L3TrackingID
			} else {
				if l3TrackingID != flow.L3TrackingID {
					t.Errorf("L3TrackingID are not equal: %s != %s\n", l3TrackingID, flow.L3TrackingID)
				}
			}
		}
	}
}

// This trace contains two packets sniffed by one capture, on one
// interface. There are 4 pings running in parallel (10 echo, 10 reply) each
//
// Ethernet/VLAN/VLAN/VLAN/VLAN/IPv4/ICMPv4
// Ethernet/VLAN/VLAN/VLAN/IPv4/ICMPv4
// Ethernet/VLAN/VLAN/IPv4/ICMPv4
// Ethernet/VLAN/IPv4/ICMPv4
//
func TestVlansQinQ(t *testing.T) {
	expected := []*Flow{
		{
			LayersPath:  "Ethernet/Dot1Q/IPv4/ICMPv4",
			Application: "ICMPv4",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "92:b6:d9:98:93:bb",
				B:        "f2:74:63:a0:e3:7f",
				ID:       8,
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "172.16.0.2",
				B:        "172.16.0.1",
			},
			Metric: &FlowMetric{
				ABPackets: 10,
				ABBytes:   1020,
				BAPackets: 8,
				BABytes:   816,
			},
		},
		{
			LayersPath:  "Ethernet/Dot1Q/Dot1Q/IPv4/ICMPv4",
			Application: "ICMPv4",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "92:b6:d9:98:93:bb",
				B:        "f2:74:63:a0:e3:7f",
				ID:       40968,
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "172.16.10.2",
				B:        "172.16.10.1",
			},
			Metric: &FlowMetric{
				ABPackets: 10,
				ABBytes:   1060,
				BAPackets: 10,
				BABytes:   1060,
			},
		},
		{
			LayersPath:  "Ethernet/Dot1Q/Dot1Q/Dot1Q/IPv4/ICMPv4",
			Application: "ICMPv4",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "92:b6:d9:98:93:bb",
				B:        "f2:74:63:a0:e3:7f",
				ID:       335585288,
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "172.16.20.2",
				B:        "172.16.20.1",
			},
			Metric: &FlowMetric{
				ABPackets: 10,
				ABBytes:   1100,
				BAPackets: 9,
				BABytes:   990,
			},
		},
		{
			LayersPath:  "Ethernet/Dot1Q/Dot1Q/Dot1Q/Dot1Q/IPv4/ICMPv4",
			Application: "ICMPv4",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "92:b6:d9:98:93:bb",
				B:        "f2:74:63:a0:e3:7f",
				ID:       2061919887368,
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "172.16.30.2",
				B:        "172.16.30.1",
			},
			Metric: &FlowMetric{
				ABPackets: 10,
				ABBytes:   1140,
				BAPackets: 8,
				BABytes:   912,
			},
		},
	}

	validatePCAP(t, "pcaptraces/icmpv4-4vlanQinQ-id-8-10-20-30.pcap", layers.LinkTypeEthernet, nil, expected)
}

func TestGREEthernet(t *testing.T) {
	expected := []*Flow{
		{
			LayersPath:  "Ethernet/IPv4/GRE",
			Application: "GRE",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "00:0f:fe:dd:22:42",
				B:        "00:1b:d5:ff:54:d9",
				ID:       0,
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "72.205.54.70",
				B:        "86.106.164.150",
				ID:       0,
			},
			Metric: &FlowMetric{
				ABPackets: 5,
				ABBytes:   810,
				BAPackets: 5,
				BABytes:   810,
			},
		},
		{
			LayersPath:  "IPv4/GRE",
			Application: "GRE",
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "10.10.11.2",
				B:        "10.10.13.2",
				ID:       0,
			},
			Metric: &FlowMetric{
				ABPackets: 5,
				ABBytes:   620,
				BAPackets: 5,
				BABytes:   620,
			},
		},
		{
			LayersPath:  "IPv4/ICMPv4",
			Application: "ICMPv4",
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "10.10.25.1",
				B:        "192.168.1.2",
				ID:       0,
			},
			Metric: &FlowMetric{
				ABPackets: 5,
				ABBytes:   500,
				BAPackets: 5,
				BABytes:   500,
			},
		},
	}

	validatePCAP(t, "pcaptraces/gre-gre-icmpv4.pcap", layers.LinkTypeEthernet, nil, expected)
}

func TestL2L3EqualSrcDst(t *testing.T) {
	expected := []*Flow{
		{
			LayersPath:  "Ethernet/IPv4/TCP",
			Application: "TCP",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "00:00:00:00:00:00",
				B:        "00:00:00:00:00:00",
				ID:       0,
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "127.0.0.1",
				B:        "127.0.0.1",
				ID:       0,
			},
			Metric: &FlowMetric{
				ABPackets: 10,
				ABBytes:   668,
				BAPackets: 13,
				BABytes:   263010,
			},
		},
	}

	validatePCAP(t, "pcaptraces/iperf-same-L2L3.pcap", layers.LinkTypeEthernet, nil, expected)
}

func benchmarkPacketParsing(b *testing.B, filename string, linkType layers.LinkType) {
	handleRead, err := pcap.OpenOffline(filename)
	if err != nil {
		b.Fatal("PCAP OpenOffline error (handle to read packet): ", err)
	}
	defer handleRead.Close()

	data, ci, err := handleRead.ReadPacketData()
	if err != nil {
		b.Fatal("PCAP OpenOffline error (handle to read packet): ", err)
	}
	p := gopacket.NewPacket(data, linkType, gopacket.Default)
	p.Metadata().CaptureInfo = ci

	for n := 0; n != b.N; n++ {
		ps := PacketSeqFromGoPacket(p, 0, nil, nil)
		if ps == nil {
			b.Fatal("Failed to get PacketSeq: ", err)
		}
		for _, packet := range ps.Packets {
			NewFlowFromGoPacket(packet.GoPacket, "", &UUIDs{}, &Opts{})
		}
	}
}

func getgopacket(b *testing.B, handleRead *pcap.Handle, linkType layers.LinkType) (gopacket.Packet, error) {
	data, ci, err := handleRead.ReadPacketData()
	if err != nil {
		if err == io.EOF {
			return nil, err
		}
		b.Fatal("PCAP OpenOffline error (handle to read packet): ", err)
	}
	p := gopacket.NewPacket(data, linkType, gopacket.Default)
	p.Metadata().CaptureInfo = ci
	return p, nil
}

func benchPacketList(b *testing.B, filename string, linkType layers.LinkType, callback func(b *testing.B, table *Table, packetsList []gopacket.Packet)) (packets int, table *Table) {
	bulkNbPackets := 100000

	handleRead, err := pcap.OpenOffline(filename)
	if err != nil {
		b.Fatal("PCAP OpenOffline error (handle to read packet): ", err)
	}
	ft := NewTable(time.Hour, time.Hour, nil, UUIDs{}, TableOpts{})
	packets = 0

	for {
		var packetsList []gopacket.Packet
		ended := false
		for {
			p, err := getgopacket(b, handleRead, linkType)
			if err != nil {
				ended = true
				break
			}
			packetsList = append(packetsList, p)

			packets++
			if (packets % bulkNbPackets) == 0 {
				break
			}
		}

		b.StartTimer()
		callback(b, ft, packetsList)
		b.StopTimer()

		if ended {
			break
		}
	}
	handleRead.Close()

	return packets, ft
}

// Bench creation of a new flow for each packets
func benchmarkPacketsParsing(b *testing.B, filename string, linkType layers.LinkType) {
	b.ResetTimer()
	for n := 0; n != b.N; n++ {
		benchPacketList(b, filename, linkType,
			func(b *testing.B, table *Table, packetsList []gopacket.Packet) {
				for _, p := range packetsList {
					ps := PacketSeqFromGoPacket(p, 0, nil, nil)
					if ps == nil {
						b.Fatal("Failed to get PacketSeq")
					}
					for _, packet := range ps.Packets {
						NewFlowFromGoPacket(packet.GoPacket, "", &UUIDs{}, &Opts{})
					}
				}
			})
	}
}

// Bench creation of flow and connection tracking, via FlowTable
func benchmarkPacketsFlowTable(b *testing.B, filename string, linkType layers.LinkType) {
	b.ResetTimer()
	b.StopTimer()
	b.ReportAllocs()

	var ft *Table
	packets := 0
	for n := 0; n != b.N; n++ {
		packets, ft = benchPacketList(b, filename, linkType,
			func(b *testing.B, table *Table, packetsList []gopacket.Packet) {
				for _, p := range packetsList {
					ps := PacketSeqFromGoPacket(p, 0, nil, nil)
					if ps == nil {
						b.Fatal("Failed to get PacketSeq")
					}
					table.processPacketSeq(ps)
				}
			})
	}

	b.Logf("packets %d flows %d", packets, ft.table.Len())
	b.Logf("packets per flows %d", packets/ft.table.Len())
	fset := ft.getFlows(&filters.SearchQuery{
		Filter: filters.NewTermStringFilter("Network.Protocol", "IPV4"),
	})
	nbFlows := len(fset.Flows)
	b.Logf("IPv4 flows %d", nbFlows)
	fset = ft.getFlows(&filters.SearchQuery{
		Filter: filters.NewTermStringFilter("Network.Protocol", "IPV6"),
	})
	b.Logf("IPv6 flows %d", len(fset.Flows))

	if packets != 4679031 {
		b.Fail()
	}
	if ft.table.Len() != 9088 || nbFlows != 9088 {
		b.Fail()
	}
}

func BenchmarkPacketParsing1(b *testing.B) {
	benchmarkPacketParsing(b, "pcaptraces/gre-gre-icmpv4.pcap", layers.LinkTypeEthernet)
}

func BenchmarkPacketParsing2(b *testing.B) {
	benchmarkPacketParsing(b, "pcaptraces/simple-tcpv4.pcap", layers.LinkTypeEthernet)
}

func BenchmarkPacketsParsing(b *testing.B) {
	benchmarkPacketsParsing(b, "pcaptraces/201801011400.small.pcap", layers.LinkTypeEthernet)
}

func BenchmarkPacketsFlowTable(b *testing.B) {
	benchmarkPacketsFlowTable(b, "pcaptraces/201801011400.small.pcap", layers.LinkTypeEthernet)
}

// Bench creation of flow and connection tracking, via FlowTable
func BenchmarkQueryFlowTable(b *testing.B) {
	t := NewTable(time.Hour, time.Hour, &fakeMessageSender{}, UUIDs{})

	for i := 0; i != 10; i++ {
		f := &Flow{
			ICMP: &ICMPLayer{
				ID: uint32(i),
			},
		}
		t.table.Add(strconv.Itoa(i), f)
	}

	query := &filters.SearchQuery{
		Filter: filters.NewTermInt64Filter("ICMP.ID", 4444),
	}

	for n := 0; n != b.N; n++ {
		t.getFlows(query)
	}
}

func TestGREMPLS(t *testing.T) {
	expected := []*Flow{
		{
			LayersPath:  "Ethernet/IPv4/GRE/MPLS",
			Application: "MPLS",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "1e:1a:51:f4:45:46",
				B:        "ce:6e:35:76:f0:ab",
				ID:       0,
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "172.16.0.1",
				B:        "172.16.0.2",
				ID:       0,
			},
			Metric: &FlowMetric{
				ABPackets: 2,
				ABBytes:   252,
				BAPackets: 0,
				BABytes:   0,
			},
		},
		{
			LayersPath:  "IPv4/ICMPv4",
			Application: "ICMPv4",
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "172.16.0.1",
				B:        "192.168.0.2",
				ID:       0,
			},
			Metric: &FlowMetric{
				ABPackets: 2,
				ABBytes:   168,
				BAPackets: 0,
				BABytes:   0,
			},
		},
	}

	validatePCAP(t, "pcaptraces/gre-mpls-icmpv4.pcap", layers.LinkTypeEthernet, nil, expected)
}

func TestFlowSimpleSynFin(t *testing.T) {
	flows := flowsFromPCAP(t, "pcaptraces/simple-tcpv4.pcap", layers.LinkTypeEthernet, nil)
	// In test pcap SYNs happen at 2017-03-21 10:58:23.768977 +0200 IST
	synTimestamp := int64(1490086703768)
	// In test pcap FINs happen at 2017-03-21 10:58:27.507679 +0200 IST
	finTimestamp := int64(1490086707507)
	synTTL := uint32(64)

	if len(flows) != 1 {
		t.Error("A single packet must generate 1 flow")
	}
	if flows[0].TCPMetric == nil {
		t.Errorf("Flow SYN/FIN is disabled")
		return
	}
	if flows[0].TCPMetric.ABSynStart != synTimestamp {
		t.Errorf("In the flow AB-SYN must start at: %d, received at %d", synTimestamp, flows[0].TCPMetric.ABSynStart)
	}
	if flows[0].TCPMetric.BASynStart != synTimestamp {
		t.Errorf("In the flow BA-SYN must start at: %d, received at %d", synTimestamp, flows[0].TCPMetric.BASynStart)
	}
	if flows[0].TCPMetric.ABSynTTL != synTTL {
		t.Errorf("In flow AB-SYN TTL is: %d, supposed to be: %d", flows[0].TCPMetric.ABSynTTL, synTTL)
	}
	if flows[0].TCPMetric.BASynTTL != synTTL {
		t.Errorf("In flow BA-SYN TTL is: %d, supposed to be: %d", flows[0].TCPMetric.BASynTTL, synTTL)
	}
	if flows[0].TCPMetric.ABFinStart != finTimestamp {
		t.Errorf("In the flow AB-FIN must start at: %d, received at %d", finTimestamp, flows[0].TCPMetric.ABFinStart)
	}
	if flows[0].TCPMetric.BAFinStart != finTimestamp {
		t.Errorf("In the flow BA-FIN must start at: %d, received at %d", finTimestamp, flows[0].TCPMetric.BAFinStart)
	}
}

func TestGetFieldsXXX(t *testing.T) {
	f := &Flow{}

	for _, k := range f.GetFieldKeys() {
		if strings.HasPrefix(k, "XXX_") {
			t.Error("XXX_ private field exposed")
		}
	}
}

func TestGetFieldInterface(t *testing.T) {
	f := &Flow{}

	field, err := f.GetField("Metric")
	if err != nil {
		t.Error(err)
	}

	if field == nil {
		t.Error("Should return a Metric struct")
	}
}

func TestVxlanIcmpv4Truncated(t *testing.T) {
	expected := []*Flow{
		{
			LayersPath:  "Ethernet/IPv4/UDP/VXLAN",
			Application: "VXLAN",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "fa:36:71:46:76:19",
				B:        "3a:07:fe:34:45:8e",
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "172.16.0.1",
				B:        "172.16.0.2",
				ID:       10,
			},
			Transport: &TransportLayer{
				Protocol: FlowProtocol_UDP,
				A:        55091,
				B:        4789,
			},
			Metric: &FlowMetric{
				ABPackets: 1,
				ABBytes:   1116,
				BAPackets: 1,
				BABytes:   1116,
			},
		},
		{
			LayersPath:  "Ethernet/IPv4/ICMPv4",
			Application: "ICMPv4",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "f2:98:72:99:56:08",
				B:        "26:09:b6:98:f9:64",
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "192.168.0.1",
				B:        "192.168.0.2",
			},
			ICMP: &ICMPLayer{
				Type: ICMPType_ECHO,
				Code: 0,
				ID:   10222,
			},
			Metric: &FlowMetric{
				ABPackets: 1,
				ABBytes:   1066,
				BAPackets: 1,
				BABytes:   1066,
			},
		},
	}

	validatePCAP(t, "pcaptraces/vxlan-icmpv4-truncated.pcap", layers.LinkTypeEthernet, nil, expected)
}

func TestNTPCorrupted(t *testing.T) {
	expected := []*Flow{
		{
			LayersPath:  "Ethernet/Dot1Q/IPv4/UDP",
			Application: "UDP",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "00:1c:0f:5c:a2:83",
				B:        "00:1c:0f:09:00:10",
				ID:       202,
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "89.46.101.31",
				B:        "196.95.70.83",
			},
			Transport: &TransportLayer{
				Protocol: FlowProtocol_UDP,
				A:        40820,
				B:        123,
			},
			Metric: &FlowMetric{
				ABPackets: 1,
				ABBytes:   50,
				BAPackets: 0,
				BABytes:   0,
			},
		},
	}

	validatePCAP(t, "pcaptraces/ntp-corrupted.pcap", layers.LinkTypeEthernet, nil, expected)
}

func TestLinkType12(t *testing.T) {
	expected := []*Flow{
		{
			LayersPath:  "IPv4/TCP",
			Application: "TCP",
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "10.171.169.243",
				B:        "192.168.255.1",
				ID:       0,
			},
			Transport: &TransportLayer{
				Protocol: FlowProtocol_TCP,
				A:        10250,
				B:        41252,
				ID:       0,
			},
		},
	}

	validatePCAP(t, "pcaptraces/link-type-12.pcap", layers.LinkTypeRaw, nil, expected)
}

func TestVxlanSrcPort(t *testing.T) {
	expected := []*Flow{
		{
			LayersPath:  "Ethernet/IPv4/UDP/VXLAN",
			Application: "VXLAN",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "fa:36:71:46:76:19",
				B:        "3a:07:fe:34:45:8e",
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "172.16.0.1",
				B:        "172.16.0.2",
				ID:       10,
			},
			Transport: &TransportLayer{
				Protocol: FlowProtocol_UDP,
				A:        51031,
				B:        4789,
			},
			Metric: &FlowMetric{
				ABPackets: 2,
				ABBytes:   208,
				BAPackets: 0,
				BABytes:   0,
			},
		},
		{
			LayersPath:  "Ethernet/IPv4/TCP",
			Application: "TCP",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "f2:98:72:99:56:08",
				B:        "26:09:b6:98:f9:64",
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "192.168.0.1",
				B:        "192.168.0.2",
			},
			Transport: &TransportLayer{
				Protocol: FlowProtocol_TCP,
				A:        1468,
				B:        8080,
			},
			Metric: &FlowMetric{
				ABPackets: 1,
				ABBytes:   54,
				BAPackets: 0,
				BABytes:   0,
			},
		},
		{
			LayersPath:  "Ethernet/IPv4/TCP",
			Application: "TCP",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "f2:98:72:99:56:08",
				B:        "26:09:b6:98:f9:64",
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "192.168.0.1",
				B:        "192.168.0.2",
			},
			Transport: &TransportLayer{
				Protocol: FlowProtocol_TCP,
				A:        1890,
				B:        8080,
			},
			Metric: &FlowMetric{
				ABPackets: 1,
				ABBytes:   54,
				BAPackets: 0,
				BABytes:   0,
			},
		},
	}

	validatePCAP(t, "pcaptraces/vxlan-src-port.pcap", layers.LinkTypeEthernet, nil, expected)
}

func TestGeneve(t *testing.T) {
	expected := []*Flow{
		{
			LayersPath:  "Ethernet/IPv4/UDP/Geneve",
			Application: "Geneve",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "00:1b:21:3c:ab:64",
				B:        "00:1b:21:3c:ac:30",
				ID:       0,
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "20.0.0.1",
				B:        "20.0.0.2",
				ID:       10,
			},
			Transport: &TransportLayer{
				Protocol: FlowProtocol_UDP,
				A:        12618,
				B:        6081,
				ID:       0,
			},
			Metric: &FlowMetric{
				ABPackets: 19,
				ABBytes:   5027,
				BAPackets: 0,
				BABytes:   0,
			},
		},
		{
			LayersPath:  "Ethernet/IPv4/UDP/Geneve",
			Application: "Geneve",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "00:1b:21:3c:ac:30",
				B:        "00:1b:21:3c:ab:64",
				ID:       0,
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "20.0.0.2",
				B:        "20.0.0.1",
				ID:       11,
			},
			Transport: &TransportLayer{
				Protocol: FlowProtocol_UDP,
				A:        50525,
				B:        6081,
				ID:       0,
			},
			Metric: &FlowMetric{
				ABPackets: 20,
				ABBytes:   4253,
				BAPackets: 0,
				BABytes:   0,
			},
		},
		{
			LayersPath:  "Ethernet/IPv4/ICMPv4",
			Application: "ICMPv4",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "fe:71:d8:83:72:4f",
				B:        "b6:9e:d2:49:51:48",
				ID:       0,
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "30.0.0.2",
				B:        "30.0.0.1",
				ID:       0,
			},
			ICMP: &ICMPLayer{
				Type: ICMPType_ECHO,
				Code: 0,
				ID:   10578,
			},
			Metric: &FlowMetric{
				ABPackets: 3,
				ABBytes:   294,
				BAPackets: 0,
				BABytes:   0,
			},
		},
		{
			LayersPath:  "Ethernet/IPv4/ICMPv4",
			Application: "ICMPv4",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "b6:9e:d2:49:51:48",
				B:        "fe:71:d8:83:72:4f",
				ID:       0,
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "30.0.0.1",
				B:        "30.0.0.2",
				ID:       0,
			},
			ICMP: &ICMPLayer{
				Type: ICMPType_ECHO,
				Code: 0,
				ID:   10578,
			},
			Metric: &FlowMetric{
				ABPackets: 3,
				ABBytes:   294,
				BAPackets: 0,
				BABytes:   0,
			},
		},
		{
			LayersPath:  "Ethernet/IPv4/TCP",
			Application: "TCP",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "fe:71:d8:83:72:4f",
				B:        "b6:9e:d2:49:51:48",
				ID:       0,
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "30.0.0.2",
				B:        "30.0.0.1",
				ID:       0,
			},
			Transport: &TransportLayer{
				Protocol: FlowProtocol_TCP,
				A:        51225,
				B:        22,
				ID:       0,
			},
			Metric: &FlowMetric{
				ABPackets: 17,
				ABBytes:   2959,
				BAPackets: 0,
				BABytes:   0,
			},
		},
	}

	validatePCAP(t, "pcaptraces/geneve.pcap", layers.LinkTypeEthernet, nil, expected)
}

func TestLayerKeyMode(t *testing.T) {
	expected := []*Flow{
		{
			LayersPath:  "Ethernet/IPv4/ICMPv4",
			Application: "ICMPv4",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "f2:45:25:5b:3a:bc",
				B:        "2e:b9:2d:76:24:07",
				ID:       0,
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "192.168.0.2",
				B:        "192.168.0.1",
				ID:       0,
			},
			ICMP: &ICMPLayer{
				Type: ICMPType_ECHO,
				Code: 0,
				ID:   23604,
			},
			Metric: &FlowMetric{
				ABPackets: 2,
				ABBytes:   196,
				BAPackets: 2,
				BABytes:   196,
			},
			TrackingID:   "9e42a86914a043b8",
			L3TrackingID: "9e42a86914a043b8",
		},
	}

	validatePCAP(t, "pcaptraces/layer-key-mode.pcap", layers.LinkTypeEthernet, nil, expected, TableOpts{LayerKeyMode: L3PreferredKeyMode})

	expected = []*Flow{
		{
			LayersPath:  "Ethernet/IPv4/ICMPv4",
			Application: "ICMPv4",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "f2:45:25:5b:3a:ac",
				B:        "2e:b9:2d:76:24:07",
				ID:       0,
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "192.168.0.2",
				B:        "192.168.0.1",
				ID:       0,
			},
			ICMP: &ICMPLayer{
				Type: ICMPType_ECHO,
				Code: 0,
				ID:   23604,
			},
			Metric: &FlowMetric{
				ABPackets: 1,
				ABBytes:   98,
				BAPackets: 1,
				BABytes:   98,
			},
			TrackingID:   "6952cad8cd38c63c",
			L3TrackingID: "9e42a86914a043b8",
		},
		{
			LayersPath:  "Ethernet/IPv4/ICMPv4",
			Application: "ICMPv4",
			Link: &FlowLayer{
				Protocol: FlowProtocol_ETHERNET,
				A:        "f2:45:25:5b:3a:bc",
				B:        "2e:b9:2d:76:24:07",
				ID:       0,
			},
			Network: &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        "192.168.0.2",
				B:        "192.168.0.1",
				ID:       0,
			},
			ICMP: &ICMPLayer{
				Type: ICMPType_ECHO,
				Code: 0,
				ID:   23604,
			},
			Metric: &FlowMetric{
				ABPackets: 1,
				ABBytes:   98,
				BAPackets: 1,
				BABytes:   98,
			},
			TrackingID:   "a25d427a9f9dcdb8",
			L3TrackingID: "9e42a86914a043b8",
		},
	}

	validatePCAP(t, "pcaptraces/layer-key-mode.pcap", layers.LinkTypeEthernet, nil, expected, TableOpts{LayerKeyMode: L2KeyMode})
}

func TestVlanBPF(t *testing.T) {
	bpf, err := NewBPF(layers.LinkTypeEthernet, 256, "icmp or (vlan and icmp)")
	if err != nil {
		t.Error(err)
	}

	handleRead, err := pcap.OpenOffline("pcaptraces/icmp-vlan.pcap")
	if err != nil {
		t.Fatal("PCAP OpenOffline error (handle to read packet): ", err)
	}
	defer handleRead.Close()

	for {
		data, _, err := handleRead.ReadPacketData()
		if err != nil && err != io.EOF {
			t.Fatal("PCAP OpenOffline error (handle to read packet): ", err)
		} else if err == io.EOF {
			break
		} else {
			if !bpf.Matches(data) {
				p := gopacket.NewPacket(data, layers.LinkTypeEthernet, gopacket.Default)
				t.Errorf("expected packet not matched, got: %+v", p)
			}
		}
	}
}
