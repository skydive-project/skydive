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
	"errors"
	"fmt"
	"time"

	etcd "github.com/coreos/etcd/client"

	"github.com/redhat-cip/skydive/config"
)

type EtcdClient struct {
	Client  *etcd.Client
	KeysApi etcd.KeysAPI
}

func NewEtcdClient(etcdServers []string) (*EtcdClient, error) {
	cfg := etcd.Config{
		Endpoints: etcdServers,
		Transport: etcd.DefaultTransport,
		// set timeout per request to fail fast when the target endpoint is unavailable
		HeaderTimeoutPerRequest: time.Second,
	}

	etcdClient, err := etcd.New(cfg)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to connect to etcd: %s", err))
	}

	kapi := etcd.NewKeysAPI(etcdClient)

	return &EtcdClient{
		Client:  &etcdClient,
		KeysApi: kapi,
	}, nil
}

func NewEtcdClientFromConfig() (*EtcdClient, error) {
	etcdServers := config.GetConfig().GetStringSlice("etcd.servers")

	return NewEtcdClient(etcdServers)
}
