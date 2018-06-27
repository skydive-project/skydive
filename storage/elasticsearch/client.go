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
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	elastic "github.com/olivere/elastic"
	esconfig "github.com/olivere/elastic/config"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/filters"
	"github.com/skydive-project/skydive/logging"
)

const indexVersion = 11
const indexPrefix = "skydive"
const indexAllAlias = "all"

// Config describes configuration for elasticsearch
type Config struct {
	ElasticHost  string
	MaxConns     int
	RetrySeconds int
	BulkMaxDocs  int
	BulkMaxDelay int
	EntriesLimit int
	AgeLimit     int
	IndicesLimit int
}

func NewConfig(name ...string) Config {
	cfg := Config{}

	path := "storage."
	if len(name) > 0 {
		path += name[0]
	} else {
		path += "elasticsearch"
	}

	cfg.ElasticHost = config.GetString(path + ".host")
	cfg.MaxConns = config.GetInt(path + ".maxconns")
	cfg.RetrySeconds = config.GetInt(path + ".retry")
	cfg.BulkMaxDocs = config.GetInt(path + ".bulk_maxdocs")
	cfg.BulkMaxDelay = config.GetInt(path + ".bulk_maxdelay")

	cfg.EntriesLimit = config.GetInt(path + ".index_entries_limit")
	cfg.AgeLimit = 0
	// TODO: read the AgeLimit from the configuration. At this stage we are setting statically to zero since
	// TODO: the feature of reading the index creation date is not supported.
	// TODO: the code that need to happen when we will
	// TODO: be ready:: cfg.AgeLimit = config.GetInt(path + ".index_age_limit")
	cfg.IndicesLimit = config.GetInt(path + ".indices_to_keep")

	return cfg
}

// ElasticSearchClientInterface describes the mechanism API of ElasticSearch database client
type ElasticSearchClientInterface interface {
	FormatFilter(filter *filters.Filter, mapKey string) elastic.Query
	RollIndex() error
	Index(obj string, id string, data interface{}) (bool, error)
	BulkIndex(obj string, id string, data interface{}) (bool, error)
	IndexChild(obj string, parent string, id string, data interface{}) (bool, error)
	BulkIndexChild(obj string, parent string, id string, data interface{}) (bool, error)
	Update(obj string, id string, data interface{}) error
	BulkUpdate(obj string, id string, data interface{}) error
	UpdateWithPartialDoc(obj string, id string, data interface{}) error
	BulkUpdateWithPartialDoc(obj string, id string, data interface{}) error
	Get(obj string, id string) (*elastic.GetResult, error)
	Delete(obj string, id string) (*elastic.DeleteResponse, error)
	BulkDelete(obj string, id string)
	Search(obj string, query elastic.Query, index string, pagination filters.SearchQuery) (*elastic.SearchResult, error)
	Start()
	GetIndexAlias() string
	GetIndexAllAlias() string
}

// Mappings describes the mappings of the clinet connection
type Mappings []map[string][]byte

// ElasticIndex describes an ElasticSearch index and its current status
type ElasticIndex struct {
	sync.Mutex
	entriesCounter int
	path           string
	timeCreated    time.Time
}

// ElasticSearchClient describes a ElasticSearch client connection
type ElasticSearchClient struct {
	client        *elastic.Client
	bulkProcessor *elastic.BulkProcessor
	started       atomic.Value
	quit          chan bool
	wg            sync.WaitGroup
	name          string
	mappings      Mappings
	cfg           Config
	index         *ElasticIndex
}

// ErrBadConfig error bad configuration file
var ErrBadConfig = errors.New("elasticsearch : Config file is misconfigured, check elasticsearch key format")

func (e *ElasticIndex) increaseEntries() {
	e.entriesCounter++
}

func getTimeNow() string {
	t := time.Now()
	return fmt.Sprintf("%d-%02d-%02d_%02d-%02d-%02d",
		t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second())
}

func (c *ElasticSearchClient) getIndexPath() string {
	var suffix string
	if c.cfg.EntriesLimit != 0 || c.cfg.AgeLimit != 0 {
		suffix = "_" + getTimeNow()
	}

	return fmt.Sprintf("%s_%s_v%d%s", indexPrefix, c.name, indexVersion, suffix)
}

// GetIndexAlias returns the rolling alias which points to the currently active index
func (c *ElasticSearchClient) GetIndexAlias() string {
	return fmt.Sprintf("%s_%s", indexPrefix, c.name)
}

// GetIndexAllAlias return the alias which points to all Skydive indices
func (c *ElasticSearchClient) GetIndexAllAlias() string {
	return fmt.Sprintf("%s_%s_%s", indexPrefix, c.name, indexAllAlias)
}

func (c *ElasticSearchClient) countEntries() int {
	count, _ := c.client.Count(c.index.path).Do(context.Background())
	logging.GetLogger().Debugf("%s real entries in %s is %d", c.name, c.index.path, count)
	return int(count)
}

func (c *ElasticSearchClient) createAlias() error {
	newAlias := c.GetIndexAlias()
	allAlias := c.GetIndexAllAlias()
	aliasServer := c.client.Alias()

	aliasResult, err := c.client.Aliases().Do(context.Background())
	if err != nil {
		return err
	}

	for k := range aliasResult.Indices {
		if strings.HasPrefix(k, newAlias) {
			aliasServer.Remove(k, newAlias)
		}
	}

	aliasServer.Add(c.index.path, newAlias)
	aliasServer.Add(c.index.path, allAlias)

	if _, err := aliasServer.Do(context.Background()); err != nil {
		return err
	}

	return nil
}

func (c *ElasticSearchClient) addMappings() error {
	for _, document := range c.mappings {
		for obj, mapping := range document {
			if _, err := c.client.PutMapping().Index(c.index.path).Type(obj).BodyString(string(mapping)).Do(context.Background()); err != nil {
				return fmt.Errorf("Unable to create %s mapping: %s", obj, err.Error())
			}
		}
	}
	return nil
}

func (c *ElasticSearchClient) timeCreated() time.Time {
	//TODO: Get the index creation time (so we can compare to current time and decide if we need
	//TODO: to roll the index by time. Make sure this supports restart of the Analyzer
	return time.Now()
}

func (c *ElasticSearchClient) createIndex() error {
	c.index.path = c.getIndexPath()

	if exists, _ := c.client.IndexExists(c.index.path).Do(context.Background()); !exists {
		if _, err := c.client.CreateIndex(c.index.path).Do(context.Background()); err != nil {
			return errors.New("Unable to create the skydive index: " + err.Error())
		}
	}

	c.index.timeCreated = c.timeCreated()
	c.index.entriesCounter = c.countEntries()
	return c.addMappings()
}

func (c *ElasticSearchClient) start() error {
	c.index = &ElasticIndex{}
	if err := c.createIndex(); err != nil {
		logging.GetLogger().Errorf("Failed to create index %s", c.name)
		return err
	}

	if err := c.createAlias(); err != nil {
		logging.GetLogger().Errorf("Failed to create alias")
		return err
	}

	c.bulkProcessor.Start(context.Background())
	c.started.Store(true)

	logging.GetLogger().Infof("ElasticSearchStorage started with skydive index %s", c.name)

	return nil
}

// FormatFilter creates a ElasticSearch request based on filters
func (c *ElasticSearchClient) FormatFilter(filter *filters.Filter, mapKey string) elastic.Query {
	// TODO: remove all this and replace with olivere/elastic queries
	if filter == nil {
		return nil
	}

	prefix := mapKey
	if prefix != "" {
		prefix += "."
	}

	if f := filter.BoolFilter; f != nil {
		queries := make([]elastic.Query, len(f.Filters))
		for i, item := range f.Filters {
			queries[i] = c.FormatFilter(item, mapKey)
		}
		boolQuery := elastic.NewBoolQuery()
		switch f.Op {
		case filters.BoolFilterOp_NOT:
			return boolQuery.MustNot(queries...)
		case filters.BoolFilterOp_OR:
			return boolQuery.Should(queries...)
		case filters.BoolFilterOp_AND:
			return boolQuery.Must(queries...)
		default:
			return nil
		}
	}

	if f := filter.TermStringFilter; f != nil {
		return elastic.NewTermQuery(prefix+f.Key, f.Value)
	}
	if f := filter.TermInt64Filter; f != nil {
		return elastic.NewTermQuery(prefix+f.Key, f.Value)
	}

	if f := filter.RegexFilter; f != nil {
		// remove anchors as ES matches the whole string and doesn't support them
		value := strings.TrimPrefix(f.Value, "^")
		value = strings.TrimSuffix(value, "$")

		return elastic.NewRegexpQuery(prefix+f.Key, value)
	}

	if f := filter.IPV4RangeFilter; f != nil {
		// NOTE(safchain) as for now the IP fields are not typed as IP
		// use a regex

		// ignore the error at this point it should have been catched earlier
		regex, _ := common.IPV4CIDRToRegex(f.Value)

		// remove anchors as ES matches the whole string and doesn't support them
		value := strings.TrimPrefix(regex, "^")
		value = strings.TrimSuffix(value, "$")

		return elastic.NewRegexpQuery(prefix+f.Key, value)
	}

	if f := filter.GtInt64Filter; f != nil {
		return elastic.NewRangeQuery(prefix + f.Key).Gt(f.Value)
	}
	if f := filter.LtInt64Filter; f != nil {
		return elastic.NewRangeQuery(prefix + f.Key).Lt(f.Value)
	}
	if f := filter.GteInt64Filter; f != nil {
		return elastic.NewRangeQuery(prefix + f.Key).Gte(f.Value)
	}
	if f := filter.LteInt64Filter; f != nil {
		return elastic.NewRangeQuery(prefix + f.Key).Lte(f.Value)
	}
	if f := filter.NullFilter; f != nil {
		return elastic.NewBoolQuery().MustNot(elastic.NewExistsQuery(prefix + f.Key))
	}
	return nil
}

func (c *ElasticSearchClient) shouldRollIndexByCount() bool {
	if c.cfg.EntriesLimit == 0 {
		return false
	}
	logging.GetLogger().Debugf("%s entries counter is %d", c.name, c.index.entriesCounter)
	if c.index.entriesCounter < c.cfg.EntriesLimit {
		return false
	}
	c.bulkProcessor.Flush()
	c.client.Flush(c.name)

	c.index.entriesCounter = c.countEntries()
	if c.index.entriesCounter < c.cfg.EntriesLimit {
		return false
	}
	logging.GetLogger().Debugf("%s enough entries to roll", c.name)
	return true
}

func (c *ElasticSearchClient) shouldRollIndexByAge() bool {
	if c.cfg.AgeLimit == 0 {
		return false
	}
	age := int(time.Now().Sub(c.index.timeCreated).Seconds())
	logging.GetLogger().Debugf("%s age is %d", c.name, age)
	if age < c.cfg.AgeLimit {
		return false
	}
	logging.GetLogger().Debugf("%s old enough to roll", c.name)
	return true
}

func (c *ElasticSearchClient) shouldRollIndex() bool {
	return (c.shouldRollIndexByAge() || c.shouldRollIndexByCount())
}

func (c *ElasticSearchClient) ShouldRollIndex() bool {
	c.index.Lock()
	defer c.index.Unlock()
	return c.shouldRollIndex()
}

func (c *ElasticSearchClient) IndexPath() string {
	c.index.Lock()
	defer c.index.Unlock()
	return c.index.path
}

func (c *ElasticSearchClient) delIndices() {
	if c.cfg.IndicesLimit == 0 {
		return
	}

	indices, _ := c.client.IndexNames()
	sort.Strings(indices)

	numToDel := len(indices) - c.cfg.IndicesLimit
	if numToDel <= 0 {
		return
	}

	if _, err := c.client.DeleteIndex(indices[:numToDel]...).Do(context.Background()); err != nil {
		logging.GetLogger().Errorf("Error deleting indexes %+v: %s", indices, err.Error())
	}
}

func (c *ElasticSearchClient) rollIndex() error {
	c.index.Lock()
	defer c.index.Unlock()

	logging.GetLogger().Infof("Rolling indices for %s", c.name)

	if err := c.createIndex(); err != nil {
		return err
	}
	if err := c.createAlias(); err != nil {
		return err
	}

	logging.GetLogger().Infof("%s finished rolling indices", c.name)
	return nil
}

// RollIndex rolls the current elasticsearch index
func (c *ElasticSearchClient) RollIndex() error {
	if err := c.rollIndex(); err != nil {
		return err
	}
	c.delIndices()
	return nil
}

// Index returns the skydive index
func (c *ElasticSearchClient) Index(obj string, id string, data interface{}) (bool, error) {
	c.index.Lock()
	defer c.index.Unlock()

	if _, err := c.client.Index().Index(c.GetIndexAlias()).Type(obj).Id(id).BodyJson(data).Do(context.Background()); err != nil {
		return false, err
	}

	c.index.increaseEntries()
	return c.shouldRollIndex(), nil
}

// BulkIndex returns the bulk index from the indexer
func (c *ElasticSearchClient) BulkIndex(obj string, id string, data interface{}) (bool, error) {
	c.index.Lock()
	defer c.index.Unlock()

	req := elastic.NewBulkIndexRequest().Index(c.GetIndexAlias()).Type(obj).Id(id).Doc(data)
	c.bulkProcessor.Add(req)

	c.index.increaseEntries()
	return c.shouldRollIndex(), nil
}

// IndexChild index a child object
func (c *ElasticSearchClient) IndexChild(obj string, parent string, id string, data interface{}) (bool, error) {
	c.index.Lock()
	defer c.index.Unlock()

	if _, err := c.client.Index().Index(c.GetIndexAlias()).Type(obj).Id(id).Parent(parent).BodyJson(data).Do(context.Background()); err != nil {
		return false, err
	}

	c.index.increaseEntries()
	return c.shouldRollIndex(), nil
}

// BulkIndexChild index a while object with the indexer
func (c *ElasticSearchClient) BulkIndexChild(obj string, parent string, id string, data interface{}) (bool, error) {
	c.index.Lock()
	defer c.index.Unlock()

	req := elastic.NewBulkIndexRequest().Index(c.GetIndexAlias()).Type(obj).Id(id).Parent(parent).Doc(data)
	c.bulkProcessor.Add(req)

	c.index.increaseEntries()
	return c.shouldRollIndex(), nil
}

// Update an object
func (c *ElasticSearchClient) Update(obj string, id string, data interface{}) error {
	_, err := c.client.Update().Index(c.GetIndexAlias()).Type(obj).Id(id).Doc(data).Do(context.Background())
	return err
}

// BulkUpdate and object with the indexer
func (c *ElasticSearchClient) BulkUpdate(obj string, id string, data interface{}) error {
	req := elastic.NewBulkUpdateRequest().Index(c.GetIndexAlias()).Type(obj).Id(id).Doc(data)
	c.bulkProcessor.Add(req)

	return nil
}

// UpdateWithPartialDoc an object with partial data
func (c *ElasticSearchClient) UpdateWithPartialDoc(obj string, id string, data interface{}) error {
	// TODO: check this is right
	return c.Update(obj, id, data)
}

// BulkUpdateWithPartialDoc  an object with partial data using the indexer
func (c *ElasticSearchClient) BulkUpdateWithPartialDoc(obj string, id string, data interface{}) error {
	req := elastic.NewBulkUpdateRequest().Index(c.GetIndexAlias()).Type(obj).Id(id).Doc(data)
	c.bulkProcessor.Add(req)
	return nil
}

// Get an object
func (c *ElasticSearchClient) Get(obj string, id string) (*elastic.GetResult, error) {
	return c.client.Get().Index(c.GetIndexAlias()).Type(obj).Id(id).Do(context.Background())
}

// Delete an object
func (c *ElasticSearchClient) Delete(obj string, id string) (*elastic.DeleteResponse, error) {
	return c.client.Delete().Index(c.GetIndexAlias()).Type(obj).Id(id).Do(context.Background())
}

// BulkDelete an object with the indexer
func (c *ElasticSearchClient) BulkDelete(obj string, id string) {
	req := elastic.NewBulkDeleteRequest().Index(c.GetIndexAlias()).Type(obj).Id(id)
	c.bulkProcessor.Add(req)
}

// Search an object
func (c *ElasticSearchClient) Search(obj string, query elastic.Query, index string, opts filters.SearchQuery) (*elastic.SearchResult, error) {
	if index == "" {
		index = c.GetIndexAllAlias()
	}

	searchQuery := c.client.
		Search().
		Index(index).
		Type(obj).
		Query(query).
		Size(10000)

	if r := opts.PaginationRange; r != nil {
		if r.To < r.From {
			return nil, errors.New("Incorrect PaginationRange, To < From")
		}
		searchQuery = searchQuery.From(int(r.From)).Size(int(r.To - r.From))
	}

	if opts.Sort {
		searchQuery = searchQuery.SortWithInfo(elastic.SortInfo{
			Field:        opts.SortBy,
			Ascending:    common.SortOrder(opts.SortOrder) != common.SortDescending,
			UnmappedType: "date",
		})
	}

	return searchQuery.Do(context.Background())
}

// Start the Elasticsearch client background jobs
func (c *ElasticSearchClient) Start() {
	for {
		err := c.start()
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

		c.client.Stop()
	}
}

// Started is the client already started ?
func (c *ElasticSearchClient) Started() bool {
	return c.started.Load() == true
}

func urlFromHost(host string) (*url.URL, error) {
	urlStr := host
	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
		urlStr = "http://" + urlStr
	}

	url, err := url.Parse(urlStr)
	if err != nil || url.Port() == "" {
		return nil, ErrBadConfig
	}
	return url, nil
}

// GetClient returns the elastic client object
func (c *ElasticSearchClient) GetClient() *elastic.Client {
	return c.client
}

// NewElasticSearchClient creates a new ElasticSearch client based on configuration
func NewElasticSearchClient(name string, mappings Mappings, cfg Config) (*ElasticSearchClient, error) {
	url, err := urlFromHost(cfg.ElasticHost)
	if err != nil {
		return nil, err
	}

	if cfg.MaxConns == 0 {
		return nil, errors.New("maxconns has to be > 0")
	}

	esConfig, err := esconfig.Parse(url.String())
	if err != nil {
		return nil, err
	}

	esClient, err := elastic.NewClientFromConfig(esConfig)
	if err != nil {
		return nil, err
	}

	bulkProcessor, err := esClient.BulkProcessor().
		After(func(executionId int64, requests []elastic.BulkableRequest, response *elastic.BulkResponse, err error) {
			if err != nil {
				logging.GetLogger().Errorf("Failed to execute bulk query: %s", err)
				return
			}

			if response.Errors {
				logging.GetLogger().Errorf("Failed to insert %d entries", len(response.Failed()))
			}
		}).
		FlushInterval(time.Duration(cfg.BulkMaxDelay) * time.Second).
		Do(context.Background())
	if err != nil {
		return nil, err
	}

	client := &ElasticSearchClient{
		client:        esClient,
		bulkProcessor: bulkProcessor,
		quit:          make(chan bool, 1),
		index:         nil,
		name:          name,
		mappings:      mappings,
		cfg:           cfg,
	}

	client.started.Store(false)

	return client, nil
}
