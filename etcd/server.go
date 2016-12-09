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
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	etcd "github.com/coreos/etcd/client"
	"github.com/coreos/etcd/etcdserver"
	"github.com/coreos/etcd/etcdserver/api/v2http"
	"github.com/coreos/etcd/pkg/osutil"
	"github.com/coreos/etcd/pkg/types"

	"github.com/skydive-project/skydive/config"

	"golang.org/x/net/context"
)

const (
	memberName   = "skydive"
	clusterName  = "skydive-cluster"
	startTimeout = 10 * time.Second
	// No peer URL exists but etcd doesn't allow the value to be empty.
	peerURL    = "http://localhost:2379"
	clusterCfg = memberName + "=" + peerURL
)

// EmbeddedEtcd provides a single node etcd server.
type EmbeddedEtcd struct {
	Port     int
	listener net.Listener
	server   *etcdserver.EtcdServer
	dataDir  string
}

func NewEmbeddedEtcd(port int, dataDir string) (*EmbeddedEtcd, error) {
	var err error
	se := &EmbeddedEtcd{Port: port}
	se.listener, err = net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
	}

	se.Port = se.listener.Addr().(*net.TCPAddr).Port
	clientURLs, err := interfaceURLs(se.Port)
	if err != nil {
		se.Stop()
		return nil, err
	}

	peerURLs, err := types.NewURLs([]string{peerURL})
	if err != nil {
		se.Stop()
		return nil, err
	}

	cfg := &etcdserver.ServerConfig{
		Name:       memberName,
		ClientURLs: clientURLs,
		PeerURLs:   peerURLs,
		DataDir:    dataDir,
		InitialPeerURLsMap: types.URLsMap{
			memberName: peerURLs,
		},
		NewCluster:    true,
		TickMs:        100,
		ElectionTicks: 10,
	}

	se.server, err = etcdserver.NewServer(cfg)
	if err != nil {
		return nil, err
	}

	se.server.Start()
	osutil.RegisterInterruptHandler(se.server.Stop)

	go http.Serve(se.listener,
		v2http.NewClientHandler(se.server, cfg.ReqTimeout()))

	// Wait for etcd server to be ready
	t := time.Now().Add(startTimeout)
	etcdClient, err := etcd.New(etcd.Config{
		Endpoints:               []string{fmt.Sprintf("http://localhost:%d", port)},
		Transport:               etcd.DefaultTransport,
		HeaderTimeoutPerRequest: time.Second,
	})
	if err != nil {
		return nil, err
	}
	kapi := etcd.NewKeysAPI(etcdClient)

	for {
		if time.Now().After(t) {
			return nil, errors.New("Failed to start etcd")
		}
		if _, err := kapi.Set(context.Background(), "/skydive", "", nil); err == nil {
			break
		}
		time.Sleep(time.Second)
	}

	return se, nil
}

func NewEmbeddedEtcdFromConfig() (*EmbeddedEtcd, error) {
	dataDir := config.GetConfig().GetString("etcd.data_dir")
	port := config.GetConfig().GetInt("etcd.port")

	return NewEmbeddedEtcd(port, dataDir)
}

func (se *EmbeddedEtcd) Stop() error {
	var err error
	firstErr := func(e error) {
		if e != nil && err == nil {
			err = e
		}
	}

	if se.listener != nil {
		firstErr(se.listener.Close())
	}

	if se.server != nil {
		se.server.Stop()
	}

	if se.dataDir != "" {
		firstErr(os.RemoveAll(se.dataDir))
	}

	return err
}

// Generate all publishable URLs for a given HTTP port.
func interfaceURLs(port int) (types.URLs, error) {
	allAddrs, err := net.InterfaceAddrs()
	if err != nil {
		return []url.URL{}, err
	}

	var allURLs types.URLs
	for _, a := range allAddrs {
		ip, ok := a.(*net.IPNet)
		if !ok || !ip.IP.IsGlobalUnicast() {
			continue
		}

		tcp := net.TCPAddr{
			IP:   ip.IP,
			Port: port,
		}

		u := url.URL{
			Scheme: "http",
			Host:   tcp.String(),
		}
		allURLs = append(allURLs, u)
	}

	if len(allAddrs) == 0 {
		return []url.URL{}, fmt.Errorf("no publishable addresses")
	}

	return allURLs, nil
}
