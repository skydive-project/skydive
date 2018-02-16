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

package etcd

import (
	"context"
	"fmt"
	"strconv"
	"time"

	etcd "github.com/coreos/etcd/client"

	"github.com/skydive-project/skydive/config"
)

// Client describes a ETCD configuration client
type Client struct {
	client  *etcd.Client
	KeysAPI etcd.KeysAPI
}

// GetInt64 returns an int64 value from the configuration key
func (client *Client) GetInt64(key string) (int64, error) {
	resp, err := client.KeysAPI.Get(context.Background(), key, nil)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(resp.Node.Value, 10, 64)
}

// SetInt64 set an int64 value to the configuration key
func (client *Client) SetInt64(key string, value int64) error {
	_, err := client.KeysAPI.Set(context.Background(), key, strconv.FormatInt(value, 10), nil)
	return err
}

// Stop the client
func (client *Client) Stop() {
	if tr, ok := etcd.DefaultTransport.(interface {
		CloseIdleConnections()
	}); ok {
		tr.CloseIdleConnections()
	}
}

// NewClient creates a new ETCD client connection to ETCD servers
func NewClient(etcdServers []string, clientTimeout time.Duration) (*Client, error) {
	cfg := etcd.Config{
		Endpoints:               etcdServers,
		Transport:               etcd.DefaultTransport,
		HeaderTimeoutPerRequest: clientTimeout,
	}

	client, err := etcd.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to etcd: %s", err)
	}

	kapi := etcd.NewKeysAPI(client)

	return &Client{
		client:  &client,
		KeysAPI: kapi,
	}, nil
}

// NewClientFromConfig creates a new ETCD client from configuration
func NewClientFromConfig() (*Client, error) {
	etcdServers := config.GetEtcdServerAddrs()
	etcdTimeout := config.GetInt("etcd.client_timeout")
	switch etcdTimeout {
	case 0:
		etcdTimeout = 5 // Default timeout
	case -1:
		etcdTimeout = 0 // No timeout
	}

	return NewClient(etcdServers, time.Duration(etcdTimeout)*time.Second)
}
