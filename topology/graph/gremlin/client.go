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
	"fmt"
	"net/url"
	"reflect"
	"strconv"
)

type GremlinClient struct {
	Endpoint string
	client   client
}

type client interface {
	connect(endpoint string) error
	close()
	query(q string) ([]byte, error)
	queryElements(q string) ([]GremlinElement, error)
}

type GremlinPropertiesEncoder struct {
	bytes.Buffer
}

type GremlinProperty struct {
	ID    GremlinID   `json:"id"`
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
	return c.client.queryElements(q)
}

func (c *GremlinClient) Query(q string) ([]byte, error) {
	return c.client.query(q)
}

func (c *GremlinClient) Connect() error {
	return c.client.connect(c.Endpoint)
}

func (c *GremlinClient) Close() {
	c.client.close()
}

func NewClient(endpoint string) (*GremlinClient, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("Unable to parse the Endpoint %s: %s", endpoint, err.Error())
	}

	var client client
	switch u.Scheme {
	case "ws":
		client = &wsclient{}
	case "http":
		client = &restclient{}
	default:
		return nil, fmt.Errorf("Endpoint not supported %s", endpoint)
	}

	return &GremlinClient{
		Endpoint: endpoint,
		client:   client,
	}, nil
}
