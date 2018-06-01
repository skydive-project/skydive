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

package mqtt

import (
	"net"

	"github.com/jeffallen/mqtt"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"
)

// MQTTProbe describes a mqtt probe
type MQTTProbe struct {
	Graph  *graph.Graph
	server *mqtt.Server
}

func (m *MQTTProbe) Start() {
	m.server.Start()

	collectd, err := NewCollectd(m.Graph)
	if err != nil {
		logging.GetLogger().Errorf("Collectd subscriber error: %s", err)
		return
	}
	collectd.Start()
}

func (m *MQTTProbe) Stop() {
	m.server.Done <- struct{}{}
}

func NewMQTTProbe(g *graph.Graph) (*MQTTProbe, error) {
	listen := config.GetString("agent.topology.mqtt.listen")

	sa, err := common.ServiceAddressFromString(listen)
	if err != nil {
		return nil, err
	}

	l, err := net.Listen("tcp", sa.String())
	if err != nil {
		return nil, err
	}

	return &MQTTProbe{
		Graph:  g,
		server: mqtt.NewServer(l),
	}, nil
}
