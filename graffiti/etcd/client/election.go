/*
 * Copyright (C) 2016 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy ofthe License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specificlanguage governing permissions and
 * limitations under the License.
 *
 */

package client

import (
	"sync"
	"time"

	"github.com/safchain/insanelock"
	etcd "go.etcd.io/etcd/client/v3"
	"golang.org/x/net/context"

	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/graffiti/service"
)

const (
	timeout = time.Second * 30
)

// MasterElector describes an ETCD master elector
type MasterElector struct {
	insanelock.RWMutex
	Client    *etcd.Client
	HolderID  string
	path      string
	listeners []MasterElectionListener
	cancel    context.CancelFunc
	master    bool
	state     service.State
	wg        sync.WaitGroup
	logger    logging.Logger
}

// TTL time to live
func (le *MasterElector) TTL() time.Duration {
	return timeout
}

func (le *MasterElector) holdLock(quit chan bool) {
	defer close(quit)

	tick := time.NewTicker(timeout / 2)
	defer tick.Stop()

	leaseResp, err := le.Client.Grant(context.Background(), int64(timeout.Seconds()))
	if err != nil {
		panic(err)
	}

	ch := tick.C

	for {
		select {
		case <-ch:
			if _, err := le.Client.Txn(context.Background()).
				If(
					etcd.Compare(etcd.Value(le.path), "=", le.HolderID), // PrevValue
					etcd.Compare(etcd.CreateRevision(le.path), ">", 0),  // PrevExist (key must exist)
				).
				Then(
					etcd.OpPut(le.path, le.HolderID, etcd.WithLease(leaseResp.ID)), // Set the key with the lease (TTL)
				).
				Commit(); err != nil {
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
	if _, err := le.Client.Txn(context.Background()).
		If(
			etcd.Compare(etcd.Value(le.path), "=", le.HolderID),
		).
		Then(
			etcd.OpDelete(le.path),
		).
		Commit(); err != nil {
		panic(err)
	}
	quit := make(chan bool)

	leaseResp, err := le.Client.Grant(context.Background(), int64(timeout.Seconds()))
	if err != nil {
		panic(err)
	}

	if _, err := le.Client.Txn(context.Background()).
		If(
			etcd.Compare(etcd.CreateRevision(le.path), "=", 0), // Key does NOT exist
		).
		Then(
			etcd.OpPut(le.path, le.HolderID, etcd.WithLease(leaseResp.ID)), // Set with lease (TTL)
		).
		Commit(); err != nil {

		le.logger.Infof("starting as the master for %s: %s", le.path, le.HolderID)

		le.Lock()
		le.master = true
		le.Unlock()

		go le.holdLock(quit)

		for _, listener := range le.listeners {
			listener.OnStartAsMaster()
		}
	} else {
		le.logger.Infof("starting as a follower for %s: %s", le.path, le.HolderID)
		for _, listener := range le.listeners {
			listener.OnStartAsSlave()
		}
	}

	if first != nil {
		first <- struct{}{}
	}

	// now watch for changes
	watcher := le.Client.Watch(le.path, &etcd.WatcherOptions{})

	ctx, cancel := context.WithCancel(context.Background())
	le.cancel = cancel

	le.wg.Add(1)
	defer le.wg.Done()

	le.state.Store(service.RunningState)
	for le.state.Load() == service.RunningState {
		resp, err := watcher.Next(ctx)
		if err != nil {
			if err == context.Canceled {
				break
			}

			le.logger.Errorf("Error while watching etcd: %s", err.Error())

			time.Sleep(1 * time.Second)
			continue
		}

		switch resp.Action {
		case "expire", "delete", "compareAndDelete":
			_, err = le.EtcdKeyAPI.Set(context.Background(), le.path, le.HolderID, setOptions)
			if err == nil && !le.master {
				le.Lock()
				le.master = true
				le.Unlock()

				go le.holdLock(quit)

				le.logger.Infof("I'm now the master: %s", le.HolderID)
				for _, listener := range le.listeners {
					listener.OnSwitchToMaster()
				}
			}
		case "create", "update":
			le.RLock()
			master := le.master
			le.RUnlock()

			if !master {
				le.logger.Infof("The master is now: %s", resp.Node.Value)
				for _, listener := range le.listeners {
					listener.OnSwitchToSlave()
				}
			}
		}
	}

	// unlock before leaving so that another can take the lead
	le.EtcdKeyAPI.Delete(context.Background(), le.path, &etcd.DeleteOptions{PrevValue: le.HolderID})
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
	if le.state.CompareAndSwap(service.RunningState, service.StoppingState) {
		le.cancel()
		le.wg.Wait()
	}
}

// AddEventListener registers a new listener
func (le *MasterElector) AddEventListener(listener MasterElectionListener) {
	le.listeners = append(le.listeners, listener)
}

// NewMasterElector creates a new ETCD master elector
func NewMasterElector(etcdClient *Client, path string) *MasterElector {
	return &MasterElector{
		EtcdKeyAPI: etcdClient.KeysAPI,
		HolderID:   etcdClient.id,
		path:       path,
		master:     false,
		logger:     etcdClient.logger,
	}
}
