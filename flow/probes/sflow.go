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

package probes

import (
	"net"
	"strconv"
	"strings"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"github.com/redhat-cip/skydive/analyzer"
	"github.com/redhat-cip/skydive/flow"
	"github.com/redhat-cip/skydive/flow/mappings"
	"github.com/redhat-cip/skydive/logging"
)

const (
	maxDgramSize = 1500
)

type SFlowProbe struct {
	Addr string
	Port int

	AnalyzerClient  *analyzer.Client
	MappingPipeline *mappings.MappingPipeline
}

func (probe *SFlowProbe) GetTarget() string {
	target := []string{probe.Addr, strconv.FormatInt(int64(probe.Port), 10)}
	return strings.Join(target, ":")
}

func (probe *SFlowProbe) Start() error {
	var buf [maxDgramSize]byte

	addr := net.UDPAddr{
		Port: probe.Port,
		IP:   net.ParseIP(probe.Addr),
	}
	conn, err := net.ListenUDP("udp", &addr)
	defer conn.Close()
	if err != nil {
		logging.GetLogger().Error("Unable to listen on port %d: %s", probe.Port, err.Error())
		return err
	}

	for {
		_, _, err := conn.ReadFromUDP(buf[:])
		if err != nil {
			continue
		}

		p := gopacket.NewPacket(buf[:], layers.LayerTypeSFlow, gopacket.Default)
		sflowLayer := p.Layer(layers.LayerTypeSFlow)
		sflowPacket, ok := sflowLayer.(*layers.SFlowDatagram)
		if !ok {
			continue
		}

		if sflowPacket.SampleCount > 0 {
			for _, sample := range sflowPacket.FlowSamples {
				flows := flow.FLowsFromSFlowSample(sflowPacket.AgentAddress.String(), &sample)

				logging.GetLogger().Debug("%d flows captured", len(flows))

				if probe.MappingPipeline != nil {
					probe.MappingPipeline.Enhance(flows)
				}

				if probe.AnalyzerClient != nil {
					probe.AnalyzerClient.SendFlows(flows)
				}
			}
		}
	}

	return nil
}

func (probe *SFlowProbe) SetAnalyzerClient(a *analyzer.Client) {
	probe.AnalyzerClient = a
}

func (probe *SFlowProbe) SetMappingPipeline(p *mappings.MappingPipeline) {
	probe.MappingPipeline = p
}

func NewSFlowProbe(addr string, port int) *SFlowProbe {
	return &SFlowProbe{Addr: addr, Port: port}
}
