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

package tests

import (
	"net"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/skydive-project/skydive/tools"
)

type packetsTraceInfo struct {
	filename string
	packets  int
	bytes    []int
}

var packetsTraces = [...]packetsTraceInfo{
	{
		filename: "pcaptraces/eth-ip4-arp-dns-req-http-google.pcap",
		packets:  58,
		bytes:    []int{44, 44, 76, 76, 104, 156, 76, 76, 68, 180, 68, 556, 68, 76, 76, 92, 104, 76, 76, 68, 216, 68, 1416, 68, 1416, 68, 1416, 68, 1416, 68, 1416, 68, 1416, 68, 1416, 68, 1416, 68, 1416, 68, 1416, 68, 68, 1416, 68, 1416, 68, 1416, 68, 1416, 68, 1080, 68, 68, 68, 68, 68, 68},
	},
}

func sflowSetup(t *testing.T) (*net.UDPConn, error) {
	addr := net.UDPAddr{
		Port: 0,
		IP:   net.ParseIP("localhost"),
	}
	conn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		t.Errorf("Unable to listen on UDP %s", err.Error())
		return nil, err
	}
	return conn, nil
}

const (
	maxDgramSize = 16384
)

func asyncSflowListen(t *testing.T, wg *sync.WaitGroup, conn *net.UDPConn, trace *packetsTraceInfo) {
	defer wg.Done()

	var buf [maxDgramSize]byte
	t.Log("listen...")
	nbPackets := 0

	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	for {
		_, _, err := conn.ReadFromUDP(buf[:])
		if err != nil {
			neterr := err.(*net.OpError)
			if neterr.Timeout() == false {
				t.Error(err.Error())
			}
			break
		}

		p := gopacket.NewPacket(buf[:], layers.LayerTypeSFlow, gopacket.Default)
		sflowLayer := p.Layer(layers.LayerTypeSFlow)
		sflowPacket, ok := sflowLayer.(*layers.SFlowDatagram)
		if !ok {
			t.Fatal("not SFlowDatagram")
			break
		}

		if sflowPacket.SampleCount > 0 {
			for _, sample := range sflowPacket.FlowSamples {
				for _, rec := range sample.Records {
					record, ok := rec.(layers.SFlowRawPacketFlowRecord)
					if !ok {
						t.Fatal("1st layer is not SFlowRawPacketFlowRecord type")
						break
					}

					packet := record.Header
					nbPackets++
					packetSize := len(packet.Data())

					if nbPackets > len(trace.bytes) {
						t.Fatalf("Too much Packets, reference have only %d", len(trace.bytes))
					}
					if trace.bytes[nbPackets-1] != packetSize {
						t.Fatalf("Packet size don't match %d %d", trace.bytes[nbPackets-1], packetSize)
					}
				}
			}
		}
	}

	if trace.packets != nbPackets {
		t.Fatalf("NB Packets don't match %d %d", trace.packets, nbPackets)
	}
}

func TestPcap2SflowReplay(t *testing.T) {
	conn, err := sflowSetup(t)
	if err != nil {
		t.Fatal("SFlow setup failed", err.Error())
	}
	defer conn.Close()
	laddr, err := net.ResolveUDPAddr(conn.LocalAddr().Network(), conn.LocalAddr().String())
	if err != nil {
		t.Error("Can't read back the local address")
	}

	for _, trace := range packetsTraces {
		var wg sync.WaitGroup
		wg.Add(1)

		go asyncSflowListen(t, &wg, conn, &trace)

		fulltrace, _ := filepath.Abs(trace.filename)
		err := tools.PCAP2SFlowReplay("localhost", laddr.Port, fulltrace, 1000, 5)
		if err != nil {
			t.Fatalf("Error during the replay: %s", err.Error())
		}

		wg.Wait()
	}
}
