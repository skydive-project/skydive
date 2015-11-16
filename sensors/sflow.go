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

package sensors

import (
	"net"
	"strconv"
	"strings"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"github.com/redhat-cip/skydive/analyzer"
	"github.com/redhat-cip/skydive/flow"
	"github.com/redhat-cip/skydive/logging"
	"github.com/redhat-cip/skydive/mappings"
)

const (
	maxDgramSize = 1500
)

type SFlowSensor struct {
	Addr string
	Port int

	/* TODO(safchain) replace by analyzer client */
	AnalyzerClient *analyzer.Client
	FlowMapper     *mappings.FlowMapper
}

func (sensor *SFlowSensor) GetTarget() string {
	target := []string{sensor.Addr, strconv.FormatInt(int64(sensor.Port), 10)}
	return strings.Join(target, ":")
}

func (sensor *SFlowSensor) start() error {
	var buf [maxDgramSize]byte

	addr := net.UDPAddr{
		Port: sensor.Port,
		IP:   net.ParseIP(sensor.Addr),
	}
	conn, err := net.ListenUDP("udp", &addr)
	defer conn.Close()
	if err != nil {
		logging.GetLogger().Error("Unable to listen on port %d: %s", sensor.Port, err.Error())
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

				if sensor.FlowMapper != nil {
					sensor.FlowMapper.Enhance(flows)
				}

				if sensor.AnalyzerClient != nil {
					sensor.AnalyzerClient.SendFlows(flows)
				}
			}
		}
	}

	return nil
}

func (sensor *SFlowSensor) Start() {
	go sensor.start()
}

func (sensor *SFlowSensor) SetAnalyzerClient(a *analyzer.Client) {
	sensor.AnalyzerClient = a
}

func (sensor *SFlowSensor) SetFlowMapper(m *mappings.FlowMapper) {
	sensor.FlowMapper = m
}

func NewSFlowSensor(addr string, port int) SFlowSensor {
	sensor := SFlowSensor{Addr: addr, Port: port}
	return sensor
}
