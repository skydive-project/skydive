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
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"time"

	"github.com/coreos/etcd/client"
	"github.com/coreos/etcd/embed"
	"github.com/coreos/etcd/pkg/osutil"
	"github.com/coreos/etcd/pkg/types"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/logging"
)

const (
	memberName   = "skydive"
	startTimeout = 10 * time.Second
)

// EmbeddedEtcd provides a single node etcd server.
type EmbeddedEtcd struct {
	Port    int
	config  *embed.Config
	etcd    *embed.Etcd
	dataDir string
}

// NewEmbeddedEtcd creates a new embedded ETCD server
func NewEmbeddedEtcd(name string, listen string, dataDir string, maxWalFiles, maxSnapFiles uint, debug bool) (*EmbeddedEtcd, error) {
	sa, err := common.ServiceAddressFromString(listen)
	if err != nil {
		return nil, err
	}

	cfg := embed.NewConfig()
	cfg.Name = name
	cfg.Debug = debug
	cfg.Dir = dataDir
	cfg.ClusterState = embed.ClusterStateFlagNew
	cfg.MaxWalFiles = maxWalFiles
	cfg.MaxSnapFiles = maxSnapFiles

	var listenClientURLs types.URLs
	var listenPeerURLs types.URLs
	if sa.Addr == "0.0.0.0" || sa.Addr == "::" {
		if listenClientURLs, err = interfaceURLs(sa.Port); err != nil {
			return nil, err
		}
		if listenPeerURLs, err = interfaceURLs(sa.Port + 1); err != nil {
			return nil, err
		}
	} else {
		listenClientURLs, _ = types.NewURLs([]string{fmt.Sprintf("http://%s:%d", sa.Addr, sa.Port)})
		listenPeerURLs, _ = types.NewURLs([]string{fmt.Sprintf("http://%s:%d", sa.Addr, sa.Port+1)})
	}

	cfg.LCUrls = listenClientURLs
	cfg.LPUrls = listenPeerURLs
	cfg.ACUrls = listenClientURLs // This probably won't work with proxy feature

	var advertisePeerUrls types.URLs
	peers := config.GetStringMapString("etcd.peers")
	if len(peers) != 0 {
		initialPeers, err := types.NewURLsMapFromStringMap(peers, ",")
		if err != nil {
			return nil, err
		}

		if advertisePeerUrls = initialPeers[name]; advertisePeerUrls == nil {
			return nil, fmt.Errorf("Unable to find Etcd name entry in the peers list: %s", name)
		}
		cfg.InitialCluster = initialPeers.String()
	}

	if advertisePeerUrls == nil {
		advertisePeerUrls, _ = types.NewURLs([]string{fmt.Sprintf("http://localhost:%d", sa.Port+1)})
		cfg.InitialCluster = types.URLsMap{name: advertisePeerUrls}.String()
	}

	cfg.APUrls = advertisePeerUrls

	etcd, err := embed.StartEtcd(cfg)
	if err != nil {
		return nil, err
	}

	osutil.RegisterInterruptHandler(etcd.Close)

	select {
	case <-etcd.Server.ReadyNotify():
		log.Printf("Server is ready!")
	case <-time.After(60 * time.Second):
		etcd.Server.Stop() // trigger a shutdown
		log.Printf("Server took too long to start!")
	}

	// Wait for etcd server to be ready
	t := time.Now().Add(startTimeout)

	clientConfig := client.Config{
		Endpoints:               listenClientURLs.StringSlice(),
		Transport:               client.DefaultTransport,
		HeaderTimeoutPerRequest: time.Second,
	}
	etcdClient, err := client.New(clientConfig)
	if err != nil {
		return nil, err
	}
	kapi := client.NewKeysAPI(etcdClient)

	for {
		if time.Now().After(t) {
			return nil, errors.New("Failed to start etcd")
		}
		if _, err := kapi.Set(context.Background(), "/skydive", "", nil); err == nil {
			logging.GetLogger().Debugf("Successfully started etcd")
			break
		}
		time.Sleep(time.Second)
	}

	return &EmbeddedEtcd{
		Port:   sa.Port,
		config: cfg,
		etcd:   etcd,
	}, nil
}

// NewEmbeddedEtcdFromConfig creates a new embedded ETCD server from configuration
func NewEmbeddedEtcdFromConfig() (*EmbeddedEtcd, error) {
	name := config.GetString("etcd.name")
	dataDir := config.GetString("etcd.data_dir")
	listen := config.GetString("etcd.listen")
	maxWalFiles := uint(config.GetInt("etcd.max_wal_files"))
	maxSnapFiles := uint(config.GetInt("etcd.max_snap_files"))
	debug := config.GetBool("etcd.debug")
	return NewEmbeddedEtcd(name, listen, dataDir, maxWalFiles, maxSnapFiles, debug)
}

// Stop the embedded server
func (se *EmbeddedEtcd) Stop() error {
	se.etcd.Close()
	return nil
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
		if !ok || (!ip.IP.IsGlobalUnicast() && !ip.IP.IsLoopback()) {
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
