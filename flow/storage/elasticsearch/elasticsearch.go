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

	"github.com/lebauce/elastigo/lib"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/logging"
	esclient "github.com/skydive-project/skydive/storage/elasticsearch"
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
	client *esclient.ElasticSearchClient
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

		if f.LastUpdateMetric != nil {
			// TODO submit a pull request to add bulk request with parent supported
			if err := c.client.IndexChild("metric", f.UUID, "", f.LastUpdateMetric); err != nil {
				logging.GetLogger().Errorf("Error while indexing: %s", err.Error())
				continue
			}
		}
	}

	return nil
}

func (c *ElasticSearchStorage) formatFilter(filter *flow.Filter) map[string]interface{} {
	if filter == nil {
		return map[string]interface{}{
			"match_all": map[string]interface{}{},
		}
	}

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

	if f := filter.RegexFilter; f != nil {
		return map[string]interface{}{
			"regexp": map[string]string{
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

func (c *ElasticSearchStorage) requestFromQuery(fsq flow.FlowSearchQuery) (map[string]interface{}, error) {
	request := map[string]interface{}{"size": 10000}

	if fsq.PaginationRange != nil {
		if fsq.PaginationRange.To < fsq.PaginationRange.From {
			return request, errors.New("Incorrect PaginationRange, To < From")
		}

		request["from"] = fsq.PaginationRange.From
		request["size"] = fsq.PaginationRange.To - fsq.PaginationRange.From
	}

	return request, nil
}

func (c *ElasticSearchStorage) sendRequest(docType string, request map[string]interface{}) (elastigo.SearchResult, error) {
	q, err := json.Marshal(request)
	if err != nil {
		return elastigo.SearchResult{}, err
	}
	return c.client.Search(docType, string(q))
}

func (c *ElasticSearchStorage) SearchMetrics(fsq flow.FlowSearchQuery, metricFilter *flow.Filter) (map[string][]*flow.FlowMetric, error) {
	if !c.client.Started() {
		return nil, errors.New("ElasticSearchStorage is not yet started")
	}

	request, err := c.requestFromQuery(fsq)
	if err != nil {
		return nil, err
	}

	flowQuery := c.formatFilter(fsq.Filter)
	musts := []map[string]interface{}{{
		"has_parent": map[string]interface{}{
			"type":  "flow",
			"query": flowQuery,
		},
	}}

	metricQuery := c.formatFilter(metricFilter)
	musts = append(musts, metricQuery)

	request["query"] = map[string]interface{}{
		"bool": map[string]interface{}{
			"must": musts,
		},
	}

	if fsq.Sort {
		request["sort"] = map[string]interface{}{
			fsq.SortBy: map[string]string{
				"order": "asc",
			},
		}
	}

	out, err := c.sendRequest("metric", request)
	if err != nil {
		return nil, err
	}

	metrics := map[string][]*flow.FlowMetric{}
	if out.Hits.Len() > 0 {
		for _, d := range out.Hits.Hits {
			m := new(flow.FlowMetric)
			if err := json.Unmarshal([]byte(*d.Source), m); err != nil {
				return nil, err
			}
			metrics[d.Parent] = append(metrics[d.Parent], m)
		}
	}

	return metrics, nil
}

func (c *ElasticSearchStorage) SearchFlows(fsq flow.FlowSearchQuery) (*flow.FlowSet, error) {
	if !c.client.Started() {
		return nil, errors.New("ElasticSearchStorage is not yet started")
	}

	request, err := c.requestFromQuery(fsq)
	if err != nil {
		return nil, err
	}

	var query map[string]interface{}
	if fsq.Filter != nil {
		query = c.formatFilter(fsq.Filter)
	}

	request["query"] = query

	if fsq.Sort {
		request["sort"] = map[string]interface{}{
			fsq.SortBy: map[string]string{
				"order": "desc",
			},
		}
	}

	out, err := c.sendRequest("flow", request)
	if err != nil {
		return nil, err
	}

	flowset := flow.NewFlowSet()
	if out.Hits.Len() > 0 {
		for _, d := range out.Hits.Hits {
			f := new(flow.Flow)
			if err := json.Unmarshal([]byte(*d.Source), f); err != nil {
				return nil, err
			}
			flowset.Flows = append(flowset.Flows, f)
		}
	}

	if fsq.Dedup {
		if err := flowset.Dedup(fsq.DedupBy); err != nil {
			return nil, err
		}
	}

	return flowset, nil
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
	client, err := esclient.NewElasticSearchClientFromConfig()
	if err != nil {
		return nil, err
	}

	return &ElasticSearchStorage{client: client}, nil
}
