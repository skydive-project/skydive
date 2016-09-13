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

package elasticsearch

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/logging"
)

const flowMapping = `
{
	"dynamic_templates": [
		{
			"strings": {
				"match": "*",
				"match_mapping_type": "string",
				"mapping": {
					"type": "string", "index": "not_analyzed", "doc_values": false
				}
			}
		},
		{
			"packets": {
				"match": "*Packets",
				"mapping": {
					"type": "long"
				}
			}
		},
		{
			"bytes": {
				"match": "*Bytes",
				"mapping": {
					"type": "long"
				}
			}
		},
		{
			"start": {
				"match": "Start",
				"mapping": {
					"type": "date", "format": "epoch_second"
				}
			}
		},
		{
			"last": {
				"match": "Last",
				"mapping": {
					"type": "date", "format": "epoch_second"
				}
			}
		}
	]
}`

const metricMapping = `
{
	"_parent": {
		"type": "flow"
	},
	"dynamic_templates": [
		{
			"packets": {
				"match": "*Packets",
				"mapping": {
					"type": "long"
				}
			}
		},
		{
			"bytes": {
				"match": "*Bytes",
				"mapping": {
					"type": "long"
				}
			}
		},
		{
			"start": {
				"match": "Start",
				"mapping": {
					"type": "date", "format": "epoch_second"
				}
			}
		},
		{
			"last": {
				"match": "Last",
				"mapping": {
					"type": "date", "format": "epoch_second"
				}
			}
		}
	]
}`

type ElasticSearchStorage struct {
	client *ElasticSearchClient
}

func (c *ElasticSearchStorage) StoreFlows(flows []*flow.Flow) error {
	if !c.client.Started() {
		return errors.New("ElasticSearchStorage is not yet started")
	}

	for _, f := range flows {
		if err := c.client.Index("flow", f.UUID, f); err != nil {
			logging.GetLogger().Errorf("Error while indexing: %s", err.Error())
			continue
		}

		if f.Metric != nil {
			// TODO submit a pull request to add bulk request with parent supported
			if err := c.client.IndexChild("metric", f.UUID, "", f.Metric); err != nil {
				logging.GetLogger().Errorf("Error while indexing: %s", err.Error())
				continue
			}
		}
	}

	return nil
}

func (c *ElasticSearchStorage) formatFilter(filter *flow.Filter) map[string]interface{} {
	if f := filter.BoolFilter; f != nil {
		keyword := ""
		switch f.Op {
		case flow.BoolFilterOp_NOT:
			keyword = "must_not"
		case flow.BoolFilterOp_OR:
			keyword = "should"
		case flow.BoolFilterOp_AND:
			keyword = "must"
		}
		filters := []interface{}{}
		for _, item := range f.Filters {
			filters = append(filters, c.formatFilter(item))
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
				f.Key: f.Value,
			},
		}
	}
	if f := filter.TermInt64Filter; f != nil {
		return map[string]interface{}{
			"term": map[string]int64{
				f.Key: f.Value,
			},
		}
	}

	if f := filter.GtInt64Filter; f != nil {
		return map[string]interface{}{
			"range": map[string]interface{}{
				f.Key: &struct {
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
				f.Key: &struct {
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
				f.Key: &struct {
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
				f.Key: &struct {
					Lte interface{} `json:"lte,omitempty"`
				}{
					Lte: f.Value,
				},
			},
		}
	}
	return nil
}

func (c *ElasticSearchStorage) SearchFlows(filter *flow.Filter, interval *flow.Range) ([]*flow.Flow, error) {
	if !c.client.Started() {
		return nil, errors.New("ElasticSearchStorage is not yet started")
	}

	request := map[string]interface{}{
		"sort": map[string]interface{}{
			"Metric.Last": map[string]string{
				"order": "desc",
			},
		},
	}

	if interval != nil {
		request["from"] = interval.From
		request["size"] = interval.To - interval.From
	}

	if filter != nil {
		request["query"] = c.formatFilter(filter)
	}

	q, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	out, err := c.client.Search("flow", string(q))
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

func (c *ElasticSearchStorage) Start() {
	go c.client.Start([]map[string][]byte{
		{"metric": []byte(metricMapping)},
		{"flow": []byte(flowMapping)}},
	)
}

func (c *ElasticSearchStorage) Stop() {
	c.client.Stop()
}

func New() (*ElasticSearchStorage, error) {
	elasticonfig := strings.Split(config.GetConfig().GetString("storage.elasticsearch.host"), ":")
	if len(elasticonfig) != 2 {
		return nil, ErrBadConfig
	}

	maxConns := config.GetConfig().GetInt("storage.elasticsearch.maxconns")
	retrySeconds := config.GetConfig().GetInt("storage.elasticsearch.retry")
	if maxConns == 0 {
		maxConns = 10
	}
	if retrySeconds == 0 {
		retrySeconds = 60
	}

	client, err := NewElasticSearchClient(elasticonfig[0], elasticonfig[1], maxConns, retrySeconds)
	if err != nil {
		return nil, err
	}

	return &ElasticSearchStorage{client: client}, nil
}
