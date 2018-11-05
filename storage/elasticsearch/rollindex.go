/*
 * Copyright (C) 2018 Red Hat, Inc.
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

package elasticsearch

import (
	"context"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/spaolacci/murmur3"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/etcd"
	"github.com/skydive-project/skydive/logging"
)

var (
	// RollingRate delay between two potential rolling index
	RollingRate = time.Minute

	rollingRateLock sync.RWMutex
)

type rollIndexService struct {
	client      *Client
	config      Config
	indices     []Index
	triggerRoll chan bool
	quit        chan bool
	etcdClient  *etcd.Client
	elector     *etcd.MasterElector
}

func (r *rollIndexService) cleanup(index Index) {
	if r.config.IndicesLimit != 0 {
		resp, err := r.client.esClient.IndexGet(index.IndexWildcard()).Do(context.Background())
		if err != nil {
			logging.GetLogger().Errorf("Error while rolling index %s: %s", index.Alias(), err)
			return
		}

		if len(resp) <= r.config.IndicesLimit {
			return
		}
		logging.GetLogger().Infof("Start deleting indices to keep only %d", r.config.IndicesLimit)

		indices := make([]string, 0, len(resp))
		for k := range resp {
			indices = append(indices, k)
		}

		sort.Sort(sort.Reverse(sort.StringSlice(indices)))
		toDelete := indices[r.config.IndicesLimit:]

		// need to reindex first, thus won't delete directly but after the task finished
		if len(toDelete) > 0 {
			logging.GetLogger().Infof("Deleted indices %s", strings.Join(toDelete, ", "))
			if _, err := r.client.esClient.DeleteIndex(toDelete...).Do(context.Background()); err != nil {
				logging.GetLogger().Errorf("Error while deleting indices: %s", err)
			}
		}
	}
}

func (r *rollIndexService) roll(force bool) {
	logging.GetLogger().Debug("Start rolling indices...")

	for _, index := range r.indices {
		ri := r.client.esClient.RolloverIndex(index.Alias())

		needToRoll := false
		if force {
			needToRoll = true
		} else {
			if r.config.EntriesLimit != 0 {
				ri.AddMaxIndexDocsCondition(int64(r.config.EntriesLimit))
				needToRoll = true
			}
			min := int(time.Duration(r.config.AgeLimit).Minutes())
			if min != 0 {
				min := fmt.Sprintf("%dm", min)
				ri.AddMaxIndexAgeCondition(min)
				needToRoll = true
			}
		}

		if needToRoll {
			logging.GetLogger().Infof("Index %s rolling over", index.Alias())

			resp, err := ri.Do(context.Background())
			if err != nil {
				logging.GetLogger().Errorf("Error while rolling index %s: %s", index.Alias(), err)
				continue
			}
			if resp.RolledOver {
				logging.GetLogger().Infof("Index %s rolled over", index.Alias())

				r.cleanup(index)
			}
		}
	}

	logging.GetLogger().Infof("Rolling indices done")
}

func (r *rollIndexService) run() {
	rollingRateLock.RLock()
	timer := time.NewTicker(RollingRate)
	rollingRateLock.RUnlock()

	defer timer.Stop()

	// try to cleanup first
	for _, index := range r.indices {
		r.cleanup(index)
	}

	for {
		select {
		case <-r.triggerRoll:
			r.roll(true)
		case <-timer.C:
			if r.elector == nil || r.elector.IsMaster() {
				r.roll(false)
			}
		case <-r.quit:
			return
		}
	}
}

func (r *rollIndexService) indicesUUID() string {
	hasher := murmur3.New64()
	for _, index := range r.indices {
		hasher.Write([]byte(index.Name))
	}

	return hex.EncodeToString(hasher.Sum(nil))[0:8]
}

func (r *rollIndexService) start() {
	if r.etcdClient != nil {
		key := fmt.Sprintf("es-rolling-index:%s", r.indicesUUID())
		r.elector = etcd.NewMasterElectorFromConfig(common.AnalyzerService, key, r.etcdClient)
		r.elector.StartAndWait()
	}

	go r.run()
}

func (r *rollIndexService) stop() {
	r.quit <- true
}

// SetRollingRate override the default rolling index rate. Has to be called before client
// intantiation.
func SetRollingRate(rate time.Duration) {
	rollingRateLock.Lock()
	RollingRate = rate
	rollingRateLock.Unlock()
}

func newRollIndexService(client *Client, indices []Index, cfg Config, etcdClient *etcd.Client) *rollIndexService {
	return &rollIndexService{
		client:      client,
		config:      cfg,
		quit:        make(chan bool, 1),
		triggerRoll: make(chan bool, 1),
		indices:     indices,
		etcdClient:  etcdClient,
	}
}
