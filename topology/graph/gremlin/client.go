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

package gremlin

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"sync"

	"github.com/go-gremlin/gremlin"
	"github.com/gorilla/websocket"

	"github.com/redhat-cip/skydive/logging"
)

type GremlinClient struct {
	sync.RWMutex
	Addr   string
	Port   int
	wsConn *websocket.Conn
}

type GremlinPropertiesEncoder struct {
	bytes.Buffer
}

type GremlinProperty struct {
	ID    string      `json:"id"`
	Value interface{} `json:"value"`
}

type GremlinProperties map[string][]GremlinProperty

type GremlinID string

type GremlinElement struct {
	ID         GremlinID         `json:"id"`
	Label      string            `json:"label"`
	Type       string            `json:"type"`
	Properties GremlinProperties `json:"properties"`
}

func (i *GremlinID) UnmarshalJSON(b []byte) error {
	var j int64

	if err := json.Unmarshal(b, &j); err == nil {
		*i = GremlinID(strconv.FormatInt(j, 10))
		return nil
	}

	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*i = GremlinID(s)
		return nil
	}

	return errors.New("cannot unmarshal gremlin id: " + string(b))
}

// avoid loop in UnmarshalJSON
type jproperties GremlinProperties

func (p *GremlinProperties) UnmarshalJSON(b []byte) error {
	jprops := jproperties{}
	if err := json.Unmarshal(b, &jprops); err == nil {
		*p = GremlinProperties(jprops)
		return nil
	}

	hm := make(map[string]interface{})
	if err := json.Unmarshal(b, &hm); err == nil {
		props := GremlinProperties{}

		for k, v := range hm {
			prop := GremlinProperty{
				ID:    "",
				Value: v,
			}

			props[k] = []GremlinProperty{prop}
		}
		*p = props
		return nil
	}

	return errors.New("cannot unmarshal properties: " + string(b))
}

func (p *GremlinPropertiesEncoder) EncodeString(s string) error {
	p.WriteByte('"')
	p.WriteString(s)
	p.WriteByte('"')

	return nil
}

func (p *GremlinPropertiesEncoder) EncodeInt64(i int64) error {
	p.WriteString(strconv.FormatInt(i, 10))

	return nil
}

func (p *GremlinPropertiesEncoder) EncodeUint64(i uint64) error {
	p.WriteString(strconv.FormatUint(i, 10))

	return nil
}

func (p *GremlinPropertiesEncoder) EncodeKVPair(k string, v interface{}) error {
	p.EncodeString(k)
	p.WriteByte(',')
	return p.Encode(v)
}

func (p *GremlinPropertiesEncoder) EncodeMap(m map[string]interface{}) error {
	i := 0
	for k, v := range m {
		if i > 0 {
			p.WriteByte(',')
		}
		err := p.EncodeKVPair(k, v)
		if err != nil {
			return err
		}
		i++
	}

	return nil
}

func (p *GremlinPropertiesEncoder) Encode(v interface{}) error {
	switch v.(type) {
	case string:
		return p.EncodeString(v.(string))
	case int:
		return p.EncodeInt64(int64(v.(int)))
	case int32:
		return p.EncodeInt64(int64(v.(int32)))
	case int64:
		return p.EncodeInt64(v.(int64))
	case uint:
		return p.EncodeUint64(uint64(v.(uint)))
	case uint32:
		return p.EncodeUint64(uint64(v.(uint32)))
	case uint64:
		return p.EncodeUint64(v.(uint64))
	case float64:
		return p.EncodeInt64(int64(v.(float64)))
	case map[string]interface{}:
		return p.EncodeMap(v.(map[string]interface{}))
	}
	return errors.New("type unsupported: " + reflect.TypeOf(v).String())
}

func resultToGremlinElements(result []byte) ([]GremlinElement, error) {
	var els []GremlinElement
	err := json.Unmarshal(result, &els)
	if err != nil {
		return nil, err
	}

	return els, nil
}

func (c *GremlinClient) QueryElements(q string) ([]GremlinElement, error) {
	result, err := c.Query(q)
	if err != nil {
		logging.GetLogger().Error("Gremlin query error: %s, %s", q, err.Error())
		return nil, err
	}

	if len(result) == 0 {
		return make([]GremlinElement, 0), nil
	}

	els, err := resultToGremlinElements(result)
	if err != nil {
		logging.GetLogger().Error("Gremlin query error: %s, %s", q, err.Error())
		return nil, err
	}

	return els, nil
}

func (c *GremlinClient) Query(q string) ([]byte, error) {
	m, err := json.Marshal(gremlin.Query(q))
	if err != nil {
		return nil, err
	}

	c.Lock()
	defer c.Unlock()

	if err = c.sendMessage(string(m)); err != nil {
		return []byte{}, err
	}
	return gremlin.ReadResponse(c.wsConn)
}

func (c *GremlinClient) sendMessage(msg string) error {
	w, err := c.wsConn.NextWriter(websocket.TextMessage)
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, msg)
	if err != nil {
		return err
	}

	err = w.Close()
	if err != nil {
	}

	return err
}

func (c *GremlinClient) Connect() {
	host := c.Addr + ":" + strconv.FormatInt(int64(c.Port), 10)

	conn, err := net.Dial("tcp", host)
	if err != nil {
		logging.GetLogger().Error("Connection to the WebSocket server failed: %s", err.Error())
		return
	}

	endpoint := "ws://" + host
	u, err := url.Parse(endpoint)
	if err != nil {
		logging.GetLogger().Error("Unable to parse the WebSocket Endpoint %s: %s", endpoint, err.Error())
		return
	}

	wsConn, _, err := websocket.NewClient(conn, u, http.Header{}, 0, 4096)
	if err != nil {
		logging.GetLogger().Error("Unable to create a WebSocket connection %s : %s", endpoint, err.Error())
		return
	}

	logging.GetLogger().Info("Connected to gremlin server %s:%d", c.Addr, c.Port)

	c.wsConn = wsConn
}

func (c *GremlinClient) Close() {
	c.wsConn.Close()
}

func NewClient(addr string, port int) *GremlinClient {
	return &GremlinClient{
		Addr: addr,
		Port: port,
	}
}
