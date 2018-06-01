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
	"bytes"
	"net"
	"time"

	proto "github.com/huin/mqtt"
	"github.com/jeffallen/mqtt"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"
)

// MQTTProbe describes a mqtt probe
type Collectd struct {
	common.RWMutex
	Graph  *graph.Graph
	client *mqtt.ClientConn
}

func (c *Collectd) run() {
	for {
		// TODO improve with user & pass
		if err := c.client.Connect("", ""); err != nil {
			logging.GetLogger().Errorf("MQTT Collectd error: %s", err)
			time.Sleep(time.Second)
			continue
		}
		break
	}
	topic := proto.TopicQos{Topic: "collectd/#"}
	c.client.Subscribe([]proto.TopicQos{topic})

	for m := range c.client.Incoming {
		if m.TopicName == "collectd/localhost/memory/memory-used" {
			node := c.Graph.LookupFirstNode(graph.Metadata{"Type": "host"})
			if node != nil {
				var data bytes.Buffer
				m.Payload.WritePayload(&data)

				c.Graph.Lock()
				c.Graph.AddMetadata(node, "Memory.used", data.String())
				c.Graph.Unlock()
			}
		}
	}
}

func (c *Collectd) Start() {
	go c.run()
}

func (c *Collectd) Stop() {
}

func NewCollectd(g *graph.Graph) (*Collectd, error) {
	listen := config.GetString("agent.topology.mqtt.listen")

	sa, err := common.ServiceAddressFromString(listen)
	if err != nil {
		return nil, err
	}

	conn, err := net.Dial("tcp", sa.String())
	if err != nil {
		return nil, err
	}
	cc := mqtt.NewClientConn(conn)

	return &Collectd{
		Graph:  g,
		client: cc,
	}, nil
}
