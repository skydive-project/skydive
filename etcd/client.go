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
	"fmt"
	"strconv"
	"time"

	"golang.org/x/net/context"

	etcd "github.com/coreos/etcd/client"

	"github.com/skydive-project/skydive/config"
)

type EtcdClient struct {
	Client  *etcd.Client
	KeysApi etcd.KeysAPI
}

func (client *EtcdClient) GetInt64(key string) (int64, error) {
	resp, err := client.KeysApi.Get(context.Background(), key, nil)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(resp.Node.Value, 10, 64)
}

func (client *EtcdClient) SetInt64(key string, value int64) error {
	_, err := client.KeysApi.Set(context.Background(), key, strconv.FormatInt(value, 10), nil)
	return err
}

func (client *EtcdClient) Stop() {
	if tr, ok := etcd.DefaultTransport.(interface {
		CloseIdleConnections()
	}); ok {
		tr.CloseIdleConnections()
	}
}

func NewEtcdClient(etcdServers []string, clientTimeout time.Duration) (*EtcdClient, error) {
	cfg := etcd.Config{
		Endpoints:               etcdServers,
		Transport:               etcd.DefaultTransport,
		HeaderTimeoutPerRequest: clientTimeout,
	}

	etcdClient, err := etcd.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to etcd: %s", err)
	}

	kapi := etcd.NewKeysAPI(etcdClient)

	return &EtcdClient{
		Client:  &etcdClient,
		KeysApi: kapi,
	}, nil
}

func NewEtcdClientFromConfig() (*EtcdClient, error) {
	etcdServers := config.GetEtcdServerAddrs()
	etcdTimeout := config.GetConfig().GetInt("etcd.client_timeout")
	switch etcdTimeout {
	case 0:
		etcdTimeout = 5 // Default timeout
	case -1:
		etcdTimeout = 0 // No timeout
	}

	return NewEtcdClient(etcdServers, time.Duration(etcdTimeout)*time.Second)
}
