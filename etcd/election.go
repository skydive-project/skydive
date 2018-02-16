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
	"sync"
	"sync/atomic"
	"time"

	etcd "github.com/coreos/etcd/client"
	"golang.org/x/net/context"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/logging"
)

const (
	timeout = time.Second * 30
)

// MasterElectionListener describes the multi ETCD election mechanism
type MasterElectionListener interface {
	OnStartAsMaster()
	OnStartAsSlave()
	OnSwitchToMaster()
	OnSwitchToSlave()
}

// MasterElector describes an ETCD master elector
type MasterElector struct {
	sync.RWMutex
	EtcdKeyAPI etcd.KeysAPI
	Host       string
	path       string
	listeners  []MasterElectionListener
	cancel     context.CancelFunc
	master     bool
	state      int64
	wg         sync.WaitGroup
}

// TTL time to live
func (le *MasterElector) TTL() time.Duration {
	return timeout
}

func (le *MasterElector) holdLock(quit chan bool) {
	defer close(quit)

	tick := time.NewTicker(timeout / 2)
	defer tick.Stop()

	setOptions := &etcd.SetOptions{
		TTL:       timeout,
		PrevExist: etcd.PrevExist,
		PrevValue: le.Host,
	}

	ch := tick.C

	for {
		select {
		case <-ch:
			if _, err := le.EtcdKeyAPI.Set(context.Background(), le.path, le.Host, setOptions); err != nil {
				return
			}
		case <-quit:
			return
		}
	}
}

// IsMaster returns true if the current instance is master
func (le *MasterElector) IsMaster() bool {
	le.RLock()
	defer le.RUnlock()

	return le.master
}

// start starts the election process and send something to the chan when the first
// election is done
func (le *MasterElector) start(first chan struct{}) {
	// delete previous Lock
	le.EtcdKeyAPI.Delete(context.Background(), le.path, &etcd.DeleteOptions{PrevValue: le.Host})

	quit := make(chan bool)

	// try to get the lock
	setOptions := &etcd.SetOptions{
		TTL:       timeout,
		PrevExist: etcd.PrevNoExist,
	}

	if _, err := le.EtcdKeyAPI.Set(context.Background(), le.path, le.Host, setOptions); err == nil {
		logging.GetLogger().Infof("starting as the master for %s: %s", le.path, le.Host)

		le.Lock()
		le.master = true
		le.Unlock()

		go le.holdLock(quit)

		for _, listener := range le.listeners {
			listener.OnStartAsMaster()
		}
	} else {
		logging.GetLogger().Infof("starting as a follower for %s: %s", le.path, le.Host)
		for _, listener := range le.listeners {
			listener.OnStartAsSlave()
		}
	}

	if first != nil {
		first <- struct{}{}
	}

	// now watch for changes
	watcher := le.EtcdKeyAPI.Watcher(le.path, &etcd.WatcherOptions{})

	ctx, cancel := context.WithCancel(context.Background())
	le.cancel = cancel

	le.wg.Add(1)
	defer le.wg.Done()

	atomic.StoreInt64(&le.state, common.RunningState)
	for atomic.LoadInt64(&le.state) == common.RunningState {
		resp, err := watcher.Next(ctx)
		if err != nil {
			logging.GetLogger().Errorf("Error while watching etcd: %s", err.Error())

			time.Sleep(1 * time.Second)
			continue
		}

		switch resp.Action {
		case "expire", "delete", "compareAndDelete":
			_, err = le.EtcdKeyAPI.Set(context.Background(), le.path, le.Host, setOptions)
			if err == nil && !le.master {
				le.Lock()
				le.master = true
				le.Unlock()

				go le.holdLock(quit)

				logging.GetLogger().Infof("I'm now the master: %s", le.Host)
				for _, listener := range le.listeners {
					listener.OnSwitchToMaster()
				}
			}
		case "create", "update":
			le.RLock()
			master := le.master
			le.RUnlock()

			if !master {
				logging.GetLogger().Infof("The master is now: %s", resp.Node.Value)
				for _, listener := range le.listeners {
					listener.OnSwitchToSlave()
				}
			}
		}
	}

	// unlock before leaving so that another can take the lead
	le.EtcdKeyAPI.Delete(context.Background(), le.path, &etcd.DeleteOptions{PrevValue: le.Host})
}

// Start the master election mechanism
func (le *MasterElector) Start() {
	go le.start(nil)
}

// StartAndWait starts the election mechanism and wait for the first election
// before returning
func (le *MasterElector) StartAndWait() {
	first := make(chan struct{})
	defer close(first)

	go le.start(first)
	<-first
}

// Stop the election mechanism
func (le *MasterElector) Stop() {
	if atomic.CompareAndSwapInt64(&le.state, common.RunningState, common.StoppingState) {
		le.cancel()
		le.wg.Wait()
	}
}

// AddEventListener registers a new listener
func (le *MasterElector) AddEventListener(listener MasterElectionListener) {
	le.listeners = append(le.listeners, listener)
}

// NewMasterElector creates a new ETCD master elector
func NewMasterElector(host string, serviceType common.ServiceType, key string, etcdClient *Client) *MasterElector {
	return &MasterElector{
		EtcdKeyAPI: etcdClient.KeysAPI,
		Host:       host,
		path:       "/master-" + serviceType.String() + "-" + key,
		master:     false,
	}
}

// NewMasterElectorFromConfig creates a new ETCD master elector from configuration
func NewMasterElectorFromConfig(serviceType common.ServiceType, key string, etcdClient *Client) *MasterElector {
	host := config.GetString("host_id")
	return NewMasterElector(host, serviceType, key, etcdClient)
}
