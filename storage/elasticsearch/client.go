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
	"sync/atomic"
	"time"

	elastigo "github.com/lebauce/elastigo/lib"

	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/logging"
)

const indexVersion = 3

type ElasticSearchClient struct {
	connection *elastigo.Conn
	indexer    *elastigo.BulkIndexer
	started    atomic.Value
}

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
	indexPath := fmt.Sprintf("/skydive_v%d", indexVersion)

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

func (c *ElasticSearchClient) Index(obj string, id string, data interface{}) error {
	_, err := c.connection.Index("skydive", obj, id, nil, data)
	return err
}

func (c *ElasticSearchClient) IndexChild(obj string, parent string, id string, data interface{}) error {
	_, err := c.connection.IndexWithParameters("skydive", obj, id, parent, 0, "", "", "", 0, "", "", false, nil, data)
	return err
}

func (c *ElasticSearchClient) Update(obj string, id string, data interface{}) error {
	_, err := c.connection.Update("skydive", obj, id, nil, data)
	return err
}

func (c *ElasticSearchClient) UpdateWithPartialDoc(obj string, id string, data interface{}) error {
	_, err := c.connection.UpdateWithPartialDoc("skydive", obj, id, nil, data, false)
	return err
}

func (c *ElasticSearchClient) Get(obj string, id string) (elastigo.BaseResponse, error) {
	return c.connection.Get("skydive", obj, id, nil)
}

func (c *ElasticSearchClient) Delete(obj string, id string) (elastigo.BaseResponse, error) {
	return c.connection.Delete("skydive", obj, id, nil)
}

func (c *ElasticSearchClient) Search(obj string, query string) (elastigo.SearchResult, error) {
	return c.connection.Search("skydive", obj, nil, query)
}

func (c *ElasticSearchClient) Start(mappings []map[string][]byte) {
	for {
		err := c.start(mappings)
		if err == nil {
			break
		}
		logging.GetLogger().Errorf("Unable to get connected to Elasticsearch: %s", err.Error())

		time.Sleep(1 * time.Second)
	}
}

func (c *ElasticSearchClient) Stop() {
	if c.started.Load() == true {
		c.indexer.Stop()
		c.connection.Close()
	}
}

func (c *ElasticSearchClient) Started() bool {
	return c.started.Load() == true
}

func NewElasticSearchClient(addr string, port string, maxConns int, retrySeconds int, bulkMaxDocs int) (*ElasticSearchClient, error) {
	c := elastigo.NewConn()

	c.Domain = addr
	c.Port = port

	indexer := c.NewBulkIndexerErrors(maxConns, retrySeconds)
	if bulkMaxDocs <= 0 {
		indexer.BulkMaxDocs = bulkMaxDocs
	}

	client := &ElasticSearchClient{
		connection: c,
		indexer:    indexer,
	}

	client.started.Store(false)
	return client, nil
}

func NewElasticSearchClientFromConfig() (*ElasticSearchClient, error) {
	elasticonfig := strings.Split(config.GetConfig().GetString("storage.elasticsearch.host"), ":")
	if len(elasticonfig) != 2 {
		return nil, ErrBadConfig
	}

	maxConns := config.GetConfig().GetInt("storage.elasticsearch.maxconns")
	retrySeconds := config.GetConfig().GetInt("storage.elasticsearch.retry")
	bulkMaxDocs := config.GetConfig().GetInt("storage.elasticsearch.bulk_maxdocs")

	return NewElasticSearchClient(elasticonfig[0], elasticonfig[1], maxConns, retrySeconds, bulkMaxDocs)
}
