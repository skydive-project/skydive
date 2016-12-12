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

package analyzer

import (
	"net"
	"strconv"
	"time"

	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/logging"
)

type Client struct {
	Addr string
	Port int

	connection *AgentAnalyzerClientConn
}

func (c *Client) connect() {
	strAddr := c.Addr + ":" + strconv.FormatInt(int64(c.Port), 10)
	srv, err := net.ResolveUDPAddr("udp", strAddr)
	if err != nil {
		logging.GetLogger().Errorf("Can't resolv address to %s", strAddr)
		time.Sleep(200 * time.Millisecond)
		return
	}
	connection, err := NewAgentAnalyzerClientConn(srv)
	if err != nil {
		logging.GetLogger().Errorf("Connection error to %s : %s", strAddr, err.Error())
		time.Sleep(200 * time.Millisecond)
		return
	}
	c.connection = connection
}

func (c *Client) SendFlow(f *flow.Flow) error {
	data, err := f.GetData()
	if err != nil {
		return err
	}

retry:
	_, err = c.connection.Write(data)
	if err != nil {
		logging.GetLogger().Errorf("flows connection to analyzer error %s : try to reconnect" + err.Error())
		c.connection.Close()
		c.connect()
		goto retry
	}

	return nil
}

func (c *Client) SendFlows(flows []*flow.Flow) {
	for _, flow := range flows {
		err := c.SendFlow(flow)
		if err != nil {
			logging.GetLogger().Errorf("Unable to send flow: %s", err.Error())
		}
	}
}

func NewClient(addr string, port int) (*Client, error) {
	client := &Client{Addr: addr, Port: port}
	client.connect()
	return client, nil
}
