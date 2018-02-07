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
)

const indexVersion = 10

// ElasticSearchClientInterface describes the mechanism API of ElasticSearch database client
type ElasticSearchClientInterface interface {
	FormatFilter(filter *filters.Filter, mapKey string) map[string]interface{}
	Index(obj string, id string, data interface{}) error
	BulkIndex(obj string, id string, data interface{}) error
	IndexChild(obj string, parent string, id string, data interface{}) error
	BulkIndexChild(obj string, parent string, id string, data interface{}) error
	Update(obj string, id string, data interface{}) error
	BulkUpdate(obj string, id string, data interface{}) error
	UpdateWithPartialDoc(obj string, id string, data interface{}) error
	BulkUpdateWithPartialDoc(obj string, id string, data interface{}) error
	Get(obj string, id string) (elastigo.BaseResponse, error)
	Delete(obj string, id string) (elastigo.BaseResponse, error)
	BulkDelete(obj string, id string)
	Search(obj string, query string) (elastigo.SearchResult, error)
	Start(mappings []map[string][]byte)
}

// ElasticSearchClient describes a ElasticSearch client connection
type ElasticSearchClient struct {
	connection *elastigo.Conn
	indexer    *elastigo.BulkIndexer
	started    atomic.Value
	quit       chan bool
	wg         sync.WaitGroup
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

func (c *ElasticSearchClient) createAlias() error {
	aliases := `{"actions": [`

	code, data, _ := c.request("GET", "/_aliases", "", "")
	if code == http.StatusOK {
		var current map[string]interface{}

		err := json.Unmarshal(data, &current)
		if err != nil {
			return errors.New("Unable to parse aliases: " + err.Error())
		}

		for k := range current {
			if strings.HasPrefix(k, "skydive_") {
				remove := `{"remove":{"alias": "skydive", "index": "%s"}},`
				aliases += fmt.Sprintf(remove, k)
			}
		}
	}

	add := `{"add":{"alias": "skydive", "index": "skydive_v%d"}}]}`
	aliases += fmt.Sprintf(add, indexVersion)

	code, _, _ = c.request("POST", "/_aliases", "", aliases)
	if code != http.StatusOK {
		return errors.New("Unable to create an alias to the skydive index: " + strconv.FormatInt(int64(code), 10))
	}

	return nil
}

func (c *ElasticSearchClient) start(mappings []map[string][]byte) error {
	indexPath := fmt.Sprintf("skydive_v%d", indexVersion)

	if _, err := c.connection.OpenIndex(indexPath); err != nil {
		if _, err := c.connection.CreateIndex(indexPath); err != nil {
			return errors.New("Unable to create the skydive index: " + err.Error())
		}
	}

	for _, document := range mappings {
		for obj, mapping := range document {
			if err := c.connection.PutMappingFromJSON(indexPath, obj, []byte(mapping)); err != nil {
				return fmt.Errorf("Unable to create %s mapping: %s", obj, err.Error())
			}
		}
	}

	if err := c.createAlias(); err != nil {
		return err
	}

	c.indexer.Start()
	c.started.Store(true)

	logging.GetLogger().Infof("ElasticSearchStorage started")

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

// Index returns the skydive index
func (c *ElasticSearchClient) Index(obj string, id string, data interface{}) error {
	_, err := c.connection.Index("skydive", obj, id, nil, data)
	return err
}

// BulkIndex returns the bulk index from the indexer
func (c *ElasticSearchClient) BulkIndex(obj string, id string, data interface{}) error {
	return c.indexer.Index("skydive", obj, id, "", "", nil, data)
}

// IndexChild index a child object
func (c *ElasticSearchClient) IndexChild(obj string, parent string, id string, data interface{}) error {
	_, err := c.connection.IndexWithParameters("skydive", obj, id, parent, 0, "", "", "", 0, "", "", false, nil, data)
	return err
}

// BulkIndexChild index a while object with the indexer
func (c *ElasticSearchClient) BulkIndexChild(obj string, parent string, id string, data interface{}) error {
	return c.indexer.Index("skydive", obj, id, parent, "", nil, data)
}

// Update an object
func (c *ElasticSearchClient) Update(obj string, id string, data interface{}) error {
	_, err := c.connection.Update("skydive", obj, id, nil, data)
	return err
}

// BulkUpdate and object with the indexer
func (c *ElasticSearchClient) BulkUpdate(obj string, id string, data interface{}) error {
	return c.indexer.Update("skydive", obj, id, "", "", nil, data)
}

// UpdateWithPartialDoc an object with partial data
func (c *ElasticSearchClient) UpdateWithPartialDoc(obj string, id string, data interface{}) error {
	_, err := c.connection.UpdateWithPartialDoc("skydive", obj, id, nil, data, false)
	return err
}

// BulkUpdateWithPartialDoc  an object with partial data using the indexer
func (c *ElasticSearchClient) BulkUpdateWithPartialDoc(obj string, id string, data interface{}) error {
	return c.indexer.UpdateWithPartialDoc("skydive", obj, id, "", "", nil, data, false)
}

// Get an object
func (c *ElasticSearchClient) Get(obj string, id string) (elastigo.BaseResponse, error) {
	return c.connection.Get("skydive", obj, id, nil)
}

// Delete an object
func (c *ElasticSearchClient) Delete(obj string, id string) (elastigo.BaseResponse, error) {
	return c.connection.Delete("skydive", obj, id, nil)
}

// BulkDelete an object with the indexer
func (c *ElasticSearchClient) BulkDelete(obj string, id string) {
	c.indexer.Delete("skydive", obj, id)
}

// Search an object
func (c *ElasticSearchClient) Search(obj string, query string) (elastigo.SearchResult, error) {
	return c.connection.Search("skydive", obj, nil, query)
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
func (c *ElasticSearchClient) Start(mappings []map[string][]byte) {
	c.wg.Add(1)
	go c.errorReader()

	for {
		err := c.start(mappings)
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
