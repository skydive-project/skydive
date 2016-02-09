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
	"time"

	"github.com/gorilla/websocket"

	"github.com/redhat-cip/skydive/logging"
)

type EventListener interface {
	OnConnected()
	OnDisconnected()
}

type AsyncClient struct {
	sync.RWMutex
	Addr      string
	Port      int
	Path      string
	messages  chan string
	quit      chan bool
	wg        sync.WaitGroup
	listeners []EventListener
	connected bool
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
	c.Lock()
	defer c.Unlock()

	return c.connected
}

func (c *AsyncClient) sendWSMessage(conn *websocket.Conn, msg string) error {
	w, err := conn.NextWriter(websocket.TextMessage)
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
		logging.GetLogger().Error("Connection to the WebSocket server failed: %s", err.Error())
		return
	}
	defer conn.Close()

	endpoint := "ws://" + host + c.Path
	u, err := url.Parse(endpoint)
	if err != nil {
		logging.GetLogger().Error("Unable to parse the WebSocket Endpoint %s: %s", endpoint, err.Error())
		return
	}

	wsConn, _, err := websocket.NewClient(conn, u, http.Header{"Origin": {endpoint}}, 1024, 1024)
	if err != nil {
		logging.GetLogger().Error("Unable to create a WebSocket connection %s : %s", endpoint, err.Error())
		return
	}
	defer wsConn.Close()
	wsConn.SetPingHandler(nil)

	logging.GetLogger().Info("Connected to %s", endpoint)

	c.wg.Add(1)
	defer c.wg.Done()

	c.Lock()
	c.connected = true
	c.Unlock()

	// notify connected
	for _, l := range c.listeners {
		l.OnConnected()
	}

	go func() {
		for {
			if _, _, err := wsConn.NextReader(); err != nil {
				c.quit <- true
				return
			}
		}
	}()

	var msg string
Loop:
	for {
		select {
		case msg = <-c.messages:
			err := c.sendWSMessage(wsConn, msg)
			if err != nil {
				logging.GetLogger().Error("Error while writing to the WebSocket: %s", err.Error())
				break Loop
			}
		case <-c.quit:
			break Loop
		}
	}
}

func (c *AsyncClient) Connect() {
	go func() {
		for {
			c.connect()

			c.Lock()
			connected := c.connected
			c.connected = false
			c.Unlock()

			if connected {
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
	c.quit <- true
	c.wg.Wait()
}

// Create new chat client.
func NewAsyncClient(addr string, port int, path string) *AsyncClient {
	return &AsyncClient{
		Addr:      addr,
		Port:      port,
		Path:      path,
		messages:  make(chan string, 500),
		quit:      make(chan bool, 1),
		connected: false,
	}
}
