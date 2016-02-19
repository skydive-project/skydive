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

package graph

import (
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"github.com/redhat-cip/skydive/logging"
)

type EventListener interface {
	OnConnected()
	OnDisconnected()
}

type AsyncClient struct {
	Addr      string
	Port      int
	Path      string
	messages  chan string
	quit      chan bool
	wg        sync.WaitGroup
	wsConn    *websocket.Conn
	listeners []EventListener
	connected atomic.Value
	running   atomic.Value
}

func (c *AsyncClient) sendMessage(m string) {
	if !c.IsConnected() {
		return
	}

	c.messages <- m
}

func (c *AsyncClient) SendWSMessage(m WSMessage) {
	c.sendMessage(m.String())
}

func (c *AsyncClient) IsConnected() bool {
	return c.connected.Load() == true
}

func (c *AsyncClient) sendWSMessage(msg string) error {
	w, err := c.wsConn.NextWriter(websocket.TextMessage)
	if err != nil {
		return err
	}

	_, err = io.WriteString(w, msg)
	if err != nil {
		return err
	}

	return w.Close()
}

func (c *AsyncClient) connect() {
	host := c.Addr + ":" + strconv.FormatInt(int64(c.Port), 10)

	conn, err := net.Dial("tcp", host)
	if err != nil {
		logging.GetLogger().Errorf("Connection to the WebSocket server failed: %s", err.Error())
		return
	}
	defer conn.Close()

	endpoint := "ws://" + host + c.Path
	u, err := url.Parse(endpoint)
	if err != nil {
		logging.GetLogger().Errorf("Unable to parse the WebSocket Endpoint %s: %s", endpoint, err.Error())
		return
	}

	c.wsConn, _, err = websocket.NewClient(conn, u, http.Header{"Origin": {endpoint}}, 1024, 1024)
	if err != nil {
		logging.GetLogger().Errorf("Unable to create a WebSocket connection %s : %s", endpoint, err.Error())
		return
	}
	c.wsConn.SetPingHandler(nil)

	logging.GetLogger().Infof("Connected to %s", endpoint)

	c.wg.Add(1)
	defer c.wg.Done()

	c.connected.Store(true)

	// notify connected
	for _, l := range c.listeners {
		l.OnConnected()
	}

	go func() {
		for {
			if _, _, err := c.wsConn.NextReader(); err != nil {
				break
			}
		}
		c.quit <- true
	}()

	for {
		select {
		case msg := <-c.messages:
			err := c.sendWSMessage(msg)
			if err != nil {
				logging.GetLogger().Errorf("Error while writing to the WebSocket: %s", err.Error())
				break
			}
		case <-c.quit:
			return
		}
	}

	if c.running.Load() == true {
		c.wsConn.Close()
	}
}

func (c *AsyncClient) Connect() {
	go func() {
		for c.running.Load() == true {
			c.connect()

			wasConnected := c.connected.Load()
			c.connected.Store(false)

			if wasConnected == true {
				for _, l := range c.listeners {
					l.OnDisconnected()
				}
			}

			time.Sleep(1 * time.Second)
		}
	}()
}

func (c *AsyncClient) AddListener(l EventListener) {
	c.listeners = append(c.listeners, l)
}

func (c *AsyncClient) Disconnect() {
	c.running.Store(false)
	if c.connected.Load() == true {
		c.wsConn.Close()
	}
	c.wg.Wait()
	close(c.quit)
}

func NewAsyncClient(addr string, port int, path string) *AsyncClient {
	c := &AsyncClient{
		Addr:     addr,
		Port:     port,
		Path:     path,
		messages: make(chan string, 500),
		quit:     make(chan bool),
	}
	c.connected.Store(false)
	c.running.Store(true)
	return c
}
