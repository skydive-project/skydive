/*
 * Copyright (C) 2018 Red Hat, Inc.
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

package elasticsearch

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pierrec/xxHash/xxHash64"

	etcd "github.com/skydive-project/skydive/graffiti/etcd/client"
	"github.com/skydive-project/skydive/graffiti/logging"
)

var (
	// RollingRate delay between two potential rolling index
	RollingRate = time.Minute

	rollingRateLock sync.RWMutex
)

type rollIndexService struct {
	client   *Client
	config   Config
	indices  []Index
	quit     chan bool
	election etcd.MasterElection
}

func (r *rollIndexService) cleanup(index Index) {
	if r.config.IndicesLimit != 0 {
		resp, err := r.client.esClient.IndexGet(index.IndexWildcard(r.config.IndexPrefix)).Do(context.Background())
		if err != nil {
			logging.GetLogger().Errorf("Error while rolling index %s: %s", index.Alias(r.config.IndexPrefix), err)
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
	logging.GetLogger().Debugf("Start rolling indices (forced: %v)...", force)

	for _, index := range r.indices {
		ri := r.client.esClient.RolloverIndex(index.Alias(r.config.IndexPrefix))

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

		// Use the same mapping for the new created index
		indexMapping := map[string]interface{}{}
		json.Unmarshal([]byte(index.Mapping), &indexMapping)
		ri.Mappings(indexMapping)

		if needToRoll {
			alias := index.Alias(r.config.IndexPrefix)

			logging.GetLogger().Infof("Index %s rolling over", alias)

			logging.GetLogger().Debugf("Rolling over with: %+v", ri)

			resp, err := ri.Do(context.Background())
			if err != nil {
				logging.GetLogger().Errorf("Error while rolling index %s: %s", alias, err)
				continue
			}
			if resp.RolledOver {
				logging.GetLogger().Infof("Index %s rolled over", alias)

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
		if r.election == nil || r.election.IsMaster() {
			r.cleanup(index)
		}
	}

	for {
		select {
		case <-timer.C:
			if r.election == nil || r.election.IsMaster() {
				r.roll(false)
			}
		case <-r.quit:
			return
		}
	}
}

func (r *rollIndexService) start() {
	if r.election != nil {
		r.election.StartAndWait()
	}

	go r.run()
}

func (r *rollIndexService) stop() {
	r.quit <- true
}

// SetRollingRate override the default rolling index rate. Has to be called before client
// instantiation.
func SetRollingRate(rate time.Duration) {
	rollingRateLock.Lock()
	RollingRate = rate
	rollingRateLock.Unlock()
}

func newRollIndexService(client *Client, indices []Index, cfg Config, electionService etcd.MasterElectionService) *rollIndexService {
	hasher := xxHash64.New(0)
	for _, index := range indices {
		hasher.Write([]byte(index.Name))
	}
	hash := hex.EncodeToString(hasher.Sum(nil))[0:8]
	path := fmt.Sprintf("/elections/es-rolling-index:%s", hash)
	election := electionService.NewElection(path)

	return &rollIndexService{
		client:   client,
		config:   cfg,
		quit:     make(chan bool, 1),
		indices:  indices,
		election: election,
	}
}
