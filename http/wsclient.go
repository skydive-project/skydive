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

package http

import (
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"github.com/redhat-cip/skydive/logging"
)

type WSClientEventHandler interface {
	OnMessage(m WSMessage)
	OnConnected()
	OnDisconnected()
}

type DefaultWSClientEventHandler struct {
}

type WSAsyncClient struct {
	Addr          string
	Port          int
	Path          string
	AuthClient    *AuthenticationClient
	host          string
	messages      chan string
	read          chan []byte
	quit          chan bool
	wg            sync.WaitGroup
	wsConn        *websocket.Conn
	eventHandlers []WSClientEventHandler
	connected     atomic.Value
	running       atomic.Value
}

func (d *DefaultWSClientEventHandler) OnMessage(m WSMessage) {
}

func (d *DefaultWSClientEventHandler) OnConnected() {
}

func (d *DefaultWSClientEventHandler) OnDisconnected() {
}

func (c *WSAsyncClient) sendMessage(m string) {
	if !c.IsConnected() {
		return
	}

	c.messages <- m
}

func (c *WSAsyncClient) SendWSMessage(m WSMessage) {
	c.sendMessage(m.String())
}

func (c *WSAsyncClient) IsConnected() bool {
	return c.connected.Load() == true
}

func (c *WSAsyncClient) send(msg string) error {
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

func (c *WSAsyncClient) sendHello() {
	m := WSMessage{
		Namespace: Namespace,
		Type:      "Hello",
		Obj:       c.host,
	}
	c.sendMessage(m.String())
}

func (c *WSAsyncClient) connect() {
	host := c.Addr + ":" + strconv.FormatInt(int64(c.Port), 10)

	conn, err := net.Dial("tcp", host)
	if err != nil {
		logging.GetLogger().Errorf("Connection to the WebSocket server failed: %s", err.Error())
		return
	}

	endpoint := "ws://" + host + c.Path
	u, err := url.Parse(endpoint)
	if err != nil {
		logging.GetLogger().Errorf("Unable to parse the WebSocket Endpoint %s: %s", endpoint, err.Error())
		conn.Close()
		return
	}

	headers := http.Header{"Origin": {endpoint}}
	if c.AuthClient != nil {
		if err := c.AuthClient.Authenticate(); err != nil {
			logging.GetLogger().Errorf("Unable to create a WebSocket connection %s : %s", endpoint, err.Error())
			conn.Close()
			return
		}
		c.AuthClient.SetHeaders(headers)
	}

	c.wsConn, _, err = websocket.NewClient(conn, u, headers, 1024, 1024)
	if err != nil {
		logging.GetLogger().Errorf("Unable to create a WebSocket connection %s : %s", endpoint, err.Error())
		conn.Close()
		return
	}
	defer c.wsConn.Close()
	c.wsConn.SetPingHandler(nil)

	c.connected.Store(true)
	logging.GetLogger().Infof("Connected to %s", endpoint)

	c.wg.Add(1)
	defer c.wg.Done()

	c.sendHello()

	// notify connected
	for _, l := range c.eventHandlers {
		l.OnConnected()
	}

	go func() {
		for c.running.Load() == true {
			_, m, err := c.wsConn.ReadMessage()
			if err != nil {
				break
			}

			c.read <- m
		}
		c.quit <- true
	}()

	for c.running.Load() == true {
		select {
		case msg := <-c.messages:
			err := c.send(msg)
			if err != nil {
				logging.GetLogger().Errorf("Error while writing to the WebSocket: %s", err.Error())
			}
		case m := <-c.read:
			msg, err := UnmarshalWSMessage(m)
			if err != nil {
				logging.GetLogger().Errorf("Error while decoding WSMessage %s", err.Error())
			} else {
				for _, e := range c.eventHandlers {
					e.OnMessage(msg)
				}
			}
		case <-c.quit:
			return
		}
	}
}

func (c *WSAsyncClient) Connect() {
	go func() {
		for c.running.Load() == true {
			c.connect()

			wasConnected := c.connected.Load()
			c.connected.Store(false)

			if wasConnected == true {
				for _, l := range c.eventHandlers {
					l.OnDisconnected()
				}
			}

			if c.running.Load() == true {
				time.Sleep(1 * time.Second)
			}
		}
	}()
}

func (c *WSAsyncClient) AddEventHandler(h WSClientEventHandler) {
	c.eventHandlers = append(c.eventHandlers, h)
}

func (c *WSAsyncClient) Disconnect() {
	c.running.Store(false)
	if c.connected.Load() == true {
		c.quit <- true
		c.wg.Wait()
	}
}

func NewWSAsyncClient(addr string, port int, path string, authClient *AuthenticationClient) (*WSAsyncClient, error) {
	host, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	c := &WSAsyncClient{
		Addr:       addr,
		Port:       port,
		Path:       path,
		AuthClient: authClient,
		host:       host,
		messages:   make(chan string, 500),
		read:       make(chan []byte, 500),
		quit:       make(chan bool),
	}
	c.connected.Store(false)
	c.running.Store(true)
	return c, nil
}
