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

package elasticsearch

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	elastigo "github.com/mattbaird/elastigo/lib"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/filters"
	"github.com/skydive-project/skydive/logging"
	"sort"
)

const indexVersion = 11
const indexPrefix = "skydive"
const indexAllAlias = "all"

// ElasticSearchClientInterface describes the mechanism API of ElasticSearch database client
type ElasticSearchClientInterface interface {
	FormatFilter(filter *filters.Filter, mapKey string) map[string]interface{}
	RollIndex() error
	Index(obj string, id string, data interface{}) (bool, error)
	BulkIndex(obj string, id string, data interface{}) (bool, error)
	IndexChild(obj string, parent string, id string, data interface{}) (bool, error)
	BulkIndexChild(obj string, parent string, id string, data interface{}) (bool, error)
	Update(obj string, id string, data interface{}) error
	BulkUpdate(obj string, id string, data interface{}) error
	UpdateWithPartialDoc(obj string, id string, data interface{}) error
	BulkUpdateWithPartialDoc(obj string, id string, data interface{}) error
	Get(obj string, id string) (elastigo.BaseResponse, error)
	Delete(obj string, id string) (elastigo.BaseResponse, error)
	BulkDelete(obj string, id string)
	Search(obj string, query string, index string) (elastigo.SearchResult, error)
	Start(name string, mappings []map[string][]byte, entriesLimit int, ageLimit int, indicesLimit int)
	GetIndexAlias() string
	GetIndexAllAlias() string
}

// ElasticIndex describes an ElasticSearch index and its current status
type ElasticIndex struct {
	entriesCounter int
	mappings       []map[string][]byte
	name           string
	path           string
	timeCreated    time.Time
	entriesLimit   int
	ageLimit       int
	indicesLimit   int
	lock           sync.Mutex
}

// ElasticSearchClient describes a ElasticSearch client connection
type ElasticSearchClient struct {
	connection *elastigo.Conn
	indexer    *elastigo.BulkIndexer
	started    atomic.Value
	quit       chan bool
	wg         sync.WaitGroup
	index      *ElasticIndex
}

// ErrBadConfig error bad configuration file
var ErrBadConfig = errors.New("elasticsearch : Config file is misconfigured, check elasticsearch key format")

func (c *ElasticSearchClient) request(method string, path string, query string, body string) (int, []byte, error) {
	req, err := c.connection.NewRequest(method, path, query)
	if err != nil {
		return 503, nil, err
	}

	if body != "" {
		req.SetBodyString(body)
	}

	var response map[string]interface{}
	return req.Do(&response)
}

func (e *ElasticIndex) increaseEntries() {
	e.lock.Lock()
	e.entriesCounter++
	e.lock.Unlock()
}

func getIndexPath(name string) string {
	t := time.Now()
	return fmt.Sprintf("skydive_%s_v%d_%d-%02d-%02d_%02d-%02d-%02d",
		name, indexVersion, t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second())
}

// Get the rolling alias which points to the currently active index
func (c *ElasticSearchClient) GetIndexAlias() string {
	return fmt.Sprintf("%s_%s", indexPrefix, c.index.name)
}

// Get the alias which points to all Skydive indices
func (c *ElasticSearchClient) GetIndexAllAlias() string {
	return fmt.Sprintf("%s_%s", indexPrefix, indexAllAlias)
}

func (c *ElasticSearchClient) countEntries() int {
	curEntriesCount, _ := c.connection.Count(c.index.path, "", nil, "")
	logging.GetLogger().Debugf("%s real entries in %s is %d", c.index.name, c.index.path, curEntriesCount.Count)
	return curEntriesCount.Count
}

func (c *ElasticSearchClient) createAlias() error {
	newAlias := c.GetIndexAlias()
	allAlias := c.GetIndexAllAlias()
	aliases := `{"actions": [`

	code, data, _ := c.request("GET", "/_aliases", "", "")
	if code == http.StatusOK {
		var current map[string]interface{}

		err := json.Unmarshal(data, &current)
		if err != nil {
			return errors.New("Unable to parse aliases: " + err.Error())
		}

		for k := range current {
			if strings.HasPrefix(k, newAlias) {
				remove := `{"remove":{"alias": "%s", "index": "%s"}},`
				aliases += fmt.Sprintf(remove, newAlias, k)
			}
		}
	}

	add := `{"add":{"alias": "%s", "index": "%s"}}, {"add":{"alias": "%s", "index": "%s"}}]}`
	aliases += fmt.Sprintf(add, newAlias, c.index.path, allAlias, c.index.path)
	logging.GetLogger().Debugf("Creating aliases: %s", aliases)

	code, _, _ = c.request("POST", "/_aliases", "", aliases)
	if code != http.StatusOK {
		return errors.New("Unable to create an alias to the skydive index: " + strconv.FormatInt(int64(code), 10))
	}

	return nil
}

func (c *ElasticSearchClient) addMappings() error {
	for _, document := range c.index.mappings {
		for obj, mapping := range document {
			if err := c.connection.PutMappingFromJSON(c.index.path, obj, []byte(mapping)); err != nil {
				return fmt.Errorf("Unable to create %s mapping: %s", obj, err.Error())
			}
		}
	}
	return nil
}

func (c *ElasticSearchClient) createIndex(name string) error {
	if name == "" {
		name = c.index.name
	}
	c.index.path = getIndexPath(name)
	c.index.name = name
	c.index.timeCreated = time.Now()

	if _, err := c.connection.OpenIndex(c.index.path); err != nil {
		if _, err := c.connection.CreateIndex(c.index.path); err != nil {
			return errors.New("Unable to create the skydive index: " + err.Error())
		}
	}

	c.index.entriesCounter = c.countEntries()
	return c.addMappings()

}

func (c *ElasticSearchClient) start(name string, mappings []map[string][]byte, entriesLimit int, ageLimit int, indicesLimit int) error {
	c.index = &ElasticIndex{
		mappings:     mappings,
		entriesLimit: entriesLimit,
		ageLimit:     ageLimit,
		indicesLimit: indicesLimit,
		lock:         sync.Mutex{},
	}

	if err := c.createIndex(name); err != nil {
		logging.GetLogger().Errorf("Failed to create index %s", name)
		return err
	}

	if err := c.createAlias(); err != nil {
		logging.GetLogger().Errorf("Failed to create alias")
		return err
	}

	c.indexer.Start()
	c.started.Store(true)

	logging.GetLogger().Infof("ElasticSearchStorage started with skydive index %s", c.index.name)

	return nil
}

// FormatFilter creates a ElasticSearch request based on filters
func (c *ElasticSearchClient) FormatFilter(filter *filters.Filter, mapKey string) map[string]interface{} {
	if filter == nil {
		return nil
	}

	prefix := mapKey
	if prefix != "" {
		prefix += "."
	}

	if f := filter.BoolFilter; f != nil {
		keyword := ""
		switch f.Op {
		case filters.BoolFilterOp_NOT:
			keyword = "must_not"
		case filters.BoolFilterOp_OR:
			keyword = "should"
		case filters.BoolFilterOp_AND:
			keyword = "must"
		}
		filters := []interface{}{}
		for _, item := range f.Filters {
			filters = append(filters, c.FormatFilter(item, mapKey))
		}
		return map[string]interface{}{
			"bool": map[string]interface{}{
				keyword: filters,
			},
		}
	}

	if f := filter.TermStringFilter; f != nil {
		return map[string]interface{}{
			"term": map[string]string{
				prefix + f.Key: f.Value,
			},
		}
	}
	if f := filter.TermInt64Filter; f != nil {
		return map[string]interface{}{
			"term": map[string]int64{
				prefix + f.Key: f.Value,
			},
		}
	}

	if f := filter.RegexFilter; f != nil {
		// remove anchors as ES matches the whole string and doesn't support them
		value := strings.TrimPrefix(f.Value, "^")
		value = strings.TrimSuffix(value, "$")

		return map[string]interface{}{
			"regexp": map[string]string{
				prefix + f.Key: value,
			},
		}
	}

	if f := filter.IPV4RangeFilter; f != nil {
		// NOTE(safchain) as for now the IP fields are not typed as IP
		// use a regex

		// ignore the error at this point it should have been catched earlier
		regex, _ := common.IPV4CIDRToRegex(f.Value)

		// remove anchors as ES matches the whole string and doesn't support them
		value := strings.TrimPrefix(regex, "^")
		value = strings.TrimSuffix(value, "$")

		return map[string]interface{}{
			"regexp": map[string]string{
				prefix + f.Key: value,
			},
		}
	}

	if f := filter.GtInt64Filter; f != nil {
		return map[string]interface{}{
			"range": map[string]interface{}{
				prefix + f.Key: &struct {
					Gt interface{} `json:"gt,omitempty"`
				}{
					Gt: f.Value,
				},
			},
		}
	}
	if f := filter.LtInt64Filter; f != nil {
		return map[string]interface{}{
			"range": map[string]interface{}{
				prefix + f.Key: &struct {
					Lt interface{} `json:"lt,omitempty"`
				}{
					Lt: f.Value,
				},
			},
		}
	}
	if f := filter.GteInt64Filter; f != nil {
		return map[string]interface{}{
			"range": map[string]interface{}{
				prefix + f.Key: &struct {
					Gte interface{} `json:"gte,omitempty"`
				}{
					Gte: f.Value,
				},
			},
		}
	}
	if f := filter.LteInt64Filter; f != nil {
		return map[string]interface{}{
			"range": map[string]interface{}{
				prefix + f.Key: &struct {
					Lte interface{} `json:"lte,omitempty"`
				}{
					Lte: f.Value,
				},
			},
		}
	}
	if f := filter.NullFilter; f != nil {
		return map[string]interface{}{
			"bool": map[string]interface{}{
				"must_not": map[string]interface{}{
					"exists": map[string]interface{}{
						"field": prefix + f.Key,
					},
				},
			},
		}
	}
	return nil
}

func (c *ElasticSearchClient) shouldRollIndexByCount() bool {
	if c.index.entriesLimit == -1 {
		logging.GetLogger().Debugf("%s entries limit not set", c.index.name)
		return false
	}
	logging.GetLogger().Debugf("%s entries counter is %d", c.index.name, c.index.entriesCounter)
	if c.index.entriesCounter < c.index.entriesLimit {
		return false
	}
	c.indexer.Flush()
	time.Sleep(3 * time.Millisecond)

	c.index.entriesCounter = c.countEntries()
	if c.index.entriesCounter < c.index.entriesLimit {
		return false
	}
	logging.GetLogger().Debugf("%s enough entries to roll", c.index.name)
	return true
}

func (c *ElasticSearchClient) shouldRollIndexByAge() bool {
	if c.index.ageLimit == -1 {
		logging.GetLogger().Debugf("%s age limit not set", c.index.name)
		return false
	}
	age := int(time.Now().Sub(c.index.timeCreated).Seconds())
	logging.GetLogger().Debugf("%s age is %d", c.index.name, age)
	if age < c.index.ageLimit {
		return false
	}
	logging.GetLogger().Debugf("%s old enough to roll", c.index.name)
	return true
}

func (c *ElasticSearchClient) shouldRollIndex() bool {
	return (c.shouldRollIndexByCount() || c.shouldRollIndexByAge())
}

func (c *ElasticSearchClient) delIndices() {
	if c.index.indicesLimit == -1 {
		logging.GetLogger().Debugf("No indices limit specified for %s", c.index.name)
		return
	}

	indices := c.connection.GetCatIndexInfo(c.GetIndexAlias() + "_*")
	sort.Slice(indices, func(i, j int) bool {
		return indices[i].Name < indices[j].Name
	})

	numToDel := len(indices) - c.index.indicesLimit
	if numToDel <= 0 {
		return
	}

	for _, esIndex := range indices[:numToDel] {
		logging.GetLogger().Debugf("Deleting index of %s: %s", c.index.name, esIndex.Name)
		if _, err := c.connection.DeleteIndex(esIndex.Name); err != nil {
			logging.GetLogger().Errorf("Error deleting index %s: %s", esIndex.Name, err.Error())
		}
	}
}

// Roll the current elasticsearch index
func (c *ElasticSearchClient) RollIndex() error {
	c.indexer.Flush()
	time.Sleep(3 * time.Millisecond)
	logging.GetLogger().Infof("Rolling indices for %s", c.index.name)
	c.index.lock.Lock()

	if err := c.createIndex(""); err != nil {
		c.index.lock.Unlock()
		return err
	}
	if err := c.createAlias(); err != nil {
		c.index.lock.Unlock()
		return err
	}

	logging.GetLogger().Infof("%s finished rolling indices", c.index.name)
	c.index.lock.Unlock()
	c.delIndices()
	return nil
}

// Index returns the skydive index
func (c *ElasticSearchClient) Index(obj string, id string, data interface{}) (bool, error) {
	c.index.lock.Lock()
	if _, err := c.connection.Index(c.GetIndexAlias(), obj, id, nil, data); err != nil {
		c.index.lock.Unlock()
		return false, err
	}
	c.index.lock.Unlock()
	c.index.increaseEntries()
	return c.shouldRollIndex(), nil
}

// BulkIndex returns the bulk index from the indexer
func (c *ElasticSearchClient) BulkIndex(obj string, id string, data interface{}) (bool, error) {
	c.index.lock.Lock()
	if err := c.indexer.Index(c.GetIndexAlias(), obj, id, "", "", nil, data); err != nil {
		c.index.lock.Unlock()
		return false, err
	}
	c.index.lock.Unlock()
	c.index.increaseEntries()
	return c.shouldRollIndex(), nil
}

// IndexChild index a child object
func (c *ElasticSearchClient) IndexChild(obj string, parent string, id string, data interface{}) (bool, error) {
	c.index.lock.Lock()
	_, err := c.connection.IndexWithParameters(c.GetIndexAlias(), obj, id, parent, 0, "", "", "", 0, "", "", false, nil, data)
	if err != nil {
		c.index.lock.Unlock()
		return false, err
	}
	c.index.lock.Unlock()
	c.index.increaseEntries()
	return c.shouldRollIndex(), nil
}

// BulkIndexChild index a while object with the indexer
func (c *ElasticSearchClient) BulkIndexChild(obj string, parent string, id string, data interface{}) (bool, error) {
	c.index.lock.Lock()
	if err := c.indexer.Index(c.GetIndexAlias(), obj, id, parent, "", nil, data); err != nil {
		c.index.lock.Unlock()
		return false, err
	}
	c.index.lock.Unlock()
	c.index.increaseEntries()
	return c.shouldRollIndex(), nil
}

// Update an object
func (c *ElasticSearchClient) Update(obj string, id string, data interface{}) error {
	_, err := c.connection.Update(c.GetIndexAlias(), obj, id, nil, data)
	return err
}

// BulkUpdate and object with the indexer
func (c *ElasticSearchClient) BulkUpdate(obj string, id string, data interface{}) error {
	return c.indexer.Update(c.GetIndexAlias(), obj, id, "", "", nil, data)
}

// UpdateWithPartialDoc an object with partial data
func (c *ElasticSearchClient) UpdateWithPartialDoc(obj string, id string, data interface{}) error {
	_, err := c.connection.UpdateWithPartialDoc(c.GetIndexAlias(), obj, id, nil, data, false)
	return err
}

// BulkUpdateWithPartialDoc  an object with partial data using the indexer
func (c *ElasticSearchClient) BulkUpdateWithPartialDoc(obj string, id string, data interface{}) error {
	return c.indexer.UpdateWithPartialDoc(c.GetIndexAlias(), obj, id, "", "", nil, data, false)
}

// Get an object
func (c *ElasticSearchClient) Get(obj string, id string) (elastigo.BaseResponse, error) {
	return c.connection.Get(c.GetIndexAllAlias(), obj, id, nil)
}

// Delete an object
func (c *ElasticSearchClient) Delete(obj string, id string) (elastigo.BaseResponse, error) {
	return c.connection.Delete(c.GetIndexAlias(), obj, id, nil)
}

// BulkDelete an object with the indexer
func (c *ElasticSearchClient) BulkDelete(obj string, id string) {
	c.indexer.Delete(c.GetIndexAlias(), obj, id)
}

// Search an object
func (c *ElasticSearchClient) Search(obj string, query string, index string) (elastigo.SearchResult, error) {
	if index == "" {
		index = c.GetIndexAllAlias()
	}
	return c.connection.Search(index, obj, nil, query)
}

func (c *ElasticSearchClient) errorReader() {
	defer c.wg.Done()

	for {
		select {
		case err := <-c.indexer.ErrorChannel:
			logging.GetLogger().Errorf("Elasticsearch request error: %s, %v", err.Err.Error(), err.Buf)
		case <-c.quit:
			return
		}
	}
}

// Start the Elasticsearch client background jobs
func (c *ElasticSearchClient) Start(name string, mappings []map[string][]byte, entriesLimit int, ageLimit int, indicesLimit int) {
	c.wg.Add(1)
	go c.errorReader()

	for {
		err := c.start(name, mappings, entriesLimit, ageLimit, indicesLimit)
		if err == nil {
			break
		}
		logging.GetLogger().Errorf("Unable to get connected to Elasticsearch: %s", err.Error())

		time.Sleep(1 * time.Second)
	}
}

// Stop Elasticsearch background client
func (c *ElasticSearchClient) Stop() {
	if c.started.Load() == true {
		c.quit <- true
		c.wg.Wait()

		c.indexer.Stop()
		c.connection.Close()
	}
}

// Started is the client already started ?
func (c *ElasticSearchClient) Started() bool {
	return c.started.Load() == true
}

// NewElasticSearchClient creates a new ElasticSearch client
func NewElasticSearchClient(addr string, port string, maxConns int, retrySeconds int, bulkMaxDocs int, bulkMaxDelay int) (*ElasticSearchClient, error) {
	c := elastigo.NewConn()

	c.Domain = addr
	c.Port = port

	indexer := c.NewBulkIndexerErrors(maxConns, retrySeconds)
	if bulkMaxDocs > 0 {
		indexer.BulkMaxDocs = bulkMaxDocs

		// set chan to 80% of max doc
		if bulkMaxDocs > 100 {
			indexer.BulkChannel = make(chan []byte, int(float64(bulkMaxDocs)*0.8))
		}
	}
	// override the default error chan size
	indexer.ErrorChannel = make(chan *elastigo.ErrorBuffer, 100)

	if bulkMaxDelay > 0 {
		indexer.BufferDelayMax = time.Duration(bulkMaxDelay) * time.Second
	}

	client := &ElasticSearchClient{
		connection: c,
		indexer:    indexer,
		quit:       make(chan bool),
		index:      nil,
	}

	client.started.Store(false)

	return client, nil
}

// NewElasticSearchClientFromConfig creates a new ElasticSearch client based on configuration
func NewElasticSearchClientFromConfig() (*ElasticSearchClient, error) {
	elasticonfig := strings.Split(config.GetString("storage.elasticsearch.host"), ":")
	if len(elasticonfig) != 2 {
		return nil, ErrBadConfig
	}

	maxConns := config.GetInt("storage.elasticsearch.maxconns")
	if maxConns == 0 {
		return nil, errors.New("storage.elasticsearch.maxconns has to be > 0")
	}
	retrySeconds := config.GetInt("storage.elasticsearch.retry")
	bulkMaxDocs := config.GetInt("storage.elasticsearch.bulk_maxdocs")
	bulkMaxDelay := config.GetInt("storage.elasticsearch.bulk_maxdelay")

	return NewElasticSearchClient(elasticonfig[0], elasticonfig[1], maxConns, retrySeconds, bulkMaxDocs, bulkMaxDelay)
}
