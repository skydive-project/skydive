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

package gremlin

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"

	"github.com/go-gremlin/gremlin"
	"github.com/gorilla/websocket"

	"github.com/redhat-cip/skydive/logging"
)

type wsclient struct {
	sync.RWMutex
	wsConn *websocket.Conn
}

func (c *wsclient) queryElements(q string) ([]GremlinElement, error) {
	result, err := c.query(q)
	if err != nil {
		logging.GetLogger().Errorf("Gremlin query error: %s, %s", q, err.Error())
		return nil, err
	}

	if len(result) == 0 {
		return make([]GremlinElement, 0), nil
	}

	els, err := resultToGremlinElements(result)
	if err != nil {
		logging.GetLogger().Errorf("Gremlin query error: %s, %s", q, err.Error())
		return nil, err
	}

	return els, nil
}

func (c *wsclient) query(q string) ([]byte, error) {
	m, err := json.Marshal(gremlin.Query(q))
	if err != nil {
		return nil, fmt.Errorf("Gremlin request error, %s: %s", q, err.Error())
	}

	c.Lock()
	defer c.Unlock()

	if err = c.sendMessage(string(m)); err != nil {
		return []byte{}, fmt.Errorf("Gremlin request error, %s: %s", q, err.Error())
	}

	b, err := gremlin.ReadResponse(c.wsConn)
	if err != nil {
		return []byte{}, fmt.Errorf("Gremlin request error, %s: %s", q, err.Error())
	}

	return b, nil
}

func (c *wsclient) sendMessage(msg string) error {
	w, err := c.wsConn.NextWriter(websocket.TextMessage)
	if err != nil {
		return err
	}
	if _, err = io.WriteString(w, msg); err != nil {
		return err
	}

	err = w.Close()

	return err
}

func (c *wsclient) connect(endpoint string) error {
	u, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("Unable to parse the WebSocket Endpoint %s: %s", endpoint, err.Error())
	}

	conn, err := net.Dial("tcp", u.Host)
	if err != nil {
		return fmt.Errorf("Connection to the WebSocket server failed: %s", err.Error())
	}

	wsConn, _, err := websocket.NewClient(conn, u, http.Header{}, 0, 4096)
	if err != nil {
		return fmt.Errorf("Unable to create a WebSocket connection %s : %s", endpoint, err.Error())
	}

	c.wsConn = wsConn

	return nil
}

func (c *wsclient) close() {
	c.wsConn.Close()
}
