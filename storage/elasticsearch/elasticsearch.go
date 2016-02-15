/*
 * Copyright (C) 2015 Red Hat, Inc.
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

package elasticseach

import (
	"encoding/json"
	"errors"
	"strconv"

	elastigo "github.com/mattbaird/elastigo/lib"

	"github.com/redhat-cip/skydive/config"
	"github.com/redhat-cip/skydive/flow"
	"github.com/redhat-cip/skydive/logging"
	"github.com/redhat-cip/skydive/storage"
)

type ElasticSearchStorage struct {
	connection *elastigo.Conn
	indexer    *elastigo.BulkIndexer
}

func (c *ElasticSearchStorage) StoreFlows(flows []*flow.Flow) error {
	for _, flow := range flows {
		err := c.indexer.Index("skydive", "flow", flow.ID(), "", "", nil, flow)
		if err != nil {
			logging.GetLogger().Error("Error while indexing: %s", err.Error())
			continue
		}
	}

	return nil
}

func (c *ElasticSearchStorage) SearchFlows(filters storage.Filters) ([]*flow.Flow, error) {
	query := map[string]interface{}{
		"sort": map[string]interface{}{
			"Statistics.Last": map[string]string{
				"order": "desc",
			},
		},
		"from": 0,
		"size": 5,
	}
	if len(filters) > 0 {
		query = map[string]interface{}{
			"query": map[string]interface{}{
				"term": filters,
			},
			"sort": map[string]interface{}{
				"Statistics.Last": map[string]string{
					"order": "desc",
				},
			},
			"from": 0,
			"size": 5,
		}
	}

	q, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}

	out, err := c.connection.Search("skydive", "flow", nil, string(q))
	if err != nil {
		return nil, err
	}

	flows := []*flow.Flow{}

	if out.Hits.Len() > 0 {
		for _, d := range out.Hits.Hits {
			f := new(flow.Flow)
			err := json.Unmarshal([]byte(*d.Source), f)
			if err != nil {
				return nil, err
			}

			flows = append(flows, f)
		}
	}

	return flows, nil
}

func (c *ElasticSearchStorage) initialize() error {
	c.indexer = c.connection.NewBulkIndexerErrors(10, 60)
	c.indexer.Start()

	req, err := c.connection.NewRequest("GET", "/skydive", "")
	if err != nil {
		return err
	}

	var response map[string]interface{}
	code, _, _ := req.Do(&response)
	if code == 200 {
		return nil
	}

	// template to remove the analyzer
	req, err = c.connection.NewRequest("PUT", "/skydive", "")
	if err != nil {
		return err
	}

	body := `{"mappings":{"flow":{"dynamic_templates":[`
	body += `{"notanalyzed_graph":{"match":"*Graph*","mapping":{"type":"string","index":"not_analyzed"}}},`
	body += `{"notanalyzed_graph":{"match":"*Layer*","mapping":{"type":"string","index":"not_analyzed"}}}`
	body += `]}}}`

	req.SetBodyString(body)

	code, _, err = req.Do(&response)
	if err != nil {
		return err
	}

	if code != 200 {
		return errors.New("Unable to create the skydive index: " + strconv.FormatInt(int64(code), 10))
	}

	return nil
}

var ErrBadConfig = errors.New("elasticseach : Config file is misconfigured, check elasticsearch key format")

func (c *ElasticSearchStorage) Close() {
	c.indexer.Stop()
	c.connection.Close()
}

func New() (*ElasticSearchStorage, error) {
	c := elastigo.NewConn()

	elasticonfig := config.GetConfig().Section("storage").Key("elasticsearch").Strings(":")
	if len(elasticonfig) != 2 {
		return nil, ErrBadConfig
	}
	c.Domain = elasticonfig[0]
	c.Port = elasticonfig[1]

	storage := &ElasticSearchStorage{connection: c}

	err := storage.initialize()
	if err != nil {
		return nil, err
	}

	return storage, nil
}
