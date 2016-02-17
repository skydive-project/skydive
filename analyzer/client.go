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

	"github.com/redhat-cip/skydive/flow"
	"github.com/redhat-cip/skydive/logging"
)

type Client struct {
	Addr string
	Port int

	connection net.Conn
}

func (c *Client) SendFlow(f *flow.Flow) error {
	data, err := f.GetData()
	if err != nil {
		return err
	}

	c.connection.Write(data)

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

func (c *Client) AsyncFlowsUpdate(ft *flow.FlowTable, every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		<-ticker.C
		flows := ft.FilterLast(every + (10 * time.Second))
		logging.GetLogger().Infof("Send %d Flows to the Analyzer", len(flows))
		c.SendFlows(flows)
	}
}

func NewClient(addr string, port int) (*Client, error) {
	client := &Client{Addr: addr, Port: port}

	srv, err := net.ResolveUDPAddr("udp", addr+":"+strconv.FormatInt(int64(port), 10))
	if err != nil {
		return nil, err
	}

	connection, err := net.DialUDP("udp", nil, srv)
	if err != nil {
		return nil, err
	}

	client.connection = connection

	return client, nil
}
