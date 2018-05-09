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
	"fmt"

	"github.com/google/gopacket/layers"
	elastic "github.com/olivere/elastic"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/filters"
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
			"rtt": {
				"match": "RTT",
				"mapping": {
					"type": "long"
				}
			}
		},
		{
			"start": {
				"match": "*Start",
				"mapping": {
					"type": "date", "format": "epoch_millis"
				}
			}
		},
		{
			"last": {
				"match": "Last",
				"mapping": {
					"type": "date", "format": "epoch_millis"
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
					"type": "date", "format": "epoch_millis"
				}
			}
		},
		{
			"last": {
				"match": "Last",
				"mapping": {
					"type": "date", "format": "epoch_millis"
				}
			}
		}
	]
}`

const rawPacketMapping = `
{
	"_parent": {
		"type": "flow"
	},
	"dynamic_templates": [
		{
			"last": {
				"match": "Timestamp",
				"mapping": {
					"type": "date", "format": "epoch_millis"
				}
			}
		},
		{
			"bytes": {
				"match": "Index",
				"mapping": {
					"type": "long"
				}
			}
		}
	]
}`

// ElasticSearchStorage describes an ElasticSearch flow backend
type ElasticSearchStorage struct {
	client *esclient.ElasticSearchClient
}

func (c *ElasticSearchStorage) rollIndex(shouldRoll bool, err error) error {
	if err != nil {
		return fmt.Errorf("Error while indexing: %s", err.Error())
	}
	if shouldRoll {
		if err := c.client.RollIndex(); err != nil {
			return fmt.Errorf("Error while rolling index: %s", err.Error())
		}
	}
	return nil
}

// StoreFlows push a set of flows in the database
func (c *ElasticSearchStorage) StoreFlows(flows []*flow.Flow) error {
	if !c.client.Started() {
		return errors.New("ElasticSearchStorage is not yet started")
	}

	for _, f := range flows {
		if err := c.rollIndex(c.client.BulkIndex("flow", f.UUID, f)); err != nil {
			logging.GetLogger().Errorf(err.Error())
			continue
		}

		if f.LastUpdateMetric != nil {
			if err := c.rollIndex(c.client.BulkIndexChild("metric", f.UUID, "", f.LastUpdateMetric)); err != nil {
				logging.GetLogger().Errorf(err.Error())
				continue
			}
		}

		linkType, err := f.LinkType()
		if err != nil {
			logging.GetLogger().Errorf("Error while indexing: %s", err.Error())
			continue
		}
		for _, r := range f.LastRawPackets {
			rawpacket := map[string]interface{}{
				"LinkType":  linkType,
				"Timestamp": r.Timestamp,
				"Index":     r.Index,
				"Data":      r.Data,
			}
			if c.rollIndex(c.client.BulkIndexChild("rawpacket", f.UUID, "", rawpacket)) != nil {
				logging.GetLogger().Errorf(err.Error())
				continue
			}

		}
	}

	return nil
}

func (c *ElasticSearchStorage) sendRequest(docType string, esQuery elastic.Query, query filters.SearchQuery) (*elastic.SearchResult, error) {
	return c.client.Search(docType, esQuery, "", query)
}

// SearchRawPackets searches flow raw packets matching filters in the database
func (c *ElasticSearchStorage) SearchRawPackets(fsq filters.SearchQuery, packetFilter *filters.Filter) (map[string]*flow.RawPackets, error) {
	if !c.client.Started() {
		return nil, errors.New("ElasticSearchStorage is not yet started")
	}

	// do not escape flow as ES use sub object in that case
	flowQuery := c.client.FormatFilter(fsq.Filter, "")
	mustQueries := []elastic.Query{elastic.NewHasParentQuery("flow", flowQuery)}

	if packetFilter != nil {
		mustQueries = append(mustQueries, c.client.FormatFilter(packetFilter, ""))
	}

	out, err := c.sendRequest("rawpacket", elastic.NewBoolQuery().Must(mustQueries...), fsq)
	if err != nil {
		return nil, err
	}

	rawpackets := make(map[string]*flow.RawPackets)
	if len(out.Hits.Hits) > 0 {
		for _, d := range out.Hits.Hits {
			obj := struct {
				LinkType  layers.LinkType
				Timestamp int64
				Index     int64
				Data      []byte
			}{}
			if err := json.Unmarshal([]byte(*d.Source), &obj); err != nil {
				return nil, err
			}

			r := &flow.RawPacket{
				Timestamp: obj.Timestamp,
				Index:     obj.Index,
				Data:      obj.Data,
			}

			if fr, ok := rawpackets[d.Parent]; ok {
				fr.RawPackets = append(fr.RawPackets, r)
			} else {
				rawpackets[d.Parent] = &flow.RawPackets{
					LinkType:   obj.LinkType,
					RawPackets: []*flow.RawPacket{r},
				}
			}
		}
	}

	return rawpackets, nil
}

// SearchMetrics searches flow metrics matching filters in the database
func (c *ElasticSearchStorage) SearchMetrics(fsq filters.SearchQuery, metricFilter *filters.Filter) (map[string][]common.Metric, error) {
	if !c.client.Started() {
		return nil, errors.New("ElasticSearchStorage is not yet started")
	}

	// do not escape flow as ES use sub object in that case
	flowQuery := c.client.FormatFilter(fsq.Filter, "")
	metricQuery := c.client.FormatFilter(metricFilter, "")
	esQuery := elastic.NewBoolQuery().Must(
		elastic.NewHasParentQuery("flow", flowQuery),
		metricQuery,
	)

	out, err := c.sendRequest("metric", esQuery, fsq)
	if err != nil {
		return nil, err
	}

	metrics := map[string][]common.Metric{}
	if len(out.Hits.Hits) > 0 {
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

// SearchFlows search flow matching filters in the database
func (c *ElasticSearchStorage) SearchFlows(fsq filters.SearchQuery) (*flow.FlowSet, error) {
	if !c.client.Started() {
		return nil, errors.New("ElasticSearchStorage is not yet started")
	}

	out, err := c.sendRequest("flow", c.client.FormatFilter(fsq.Filter, ""), fsq)
	if err != nil {
		return nil, err
	}

	flowset := flow.NewFlowSet()
	if len(out.Hits.Hits) > 0 {
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

// Start the Database client
func (c *ElasticSearchStorage) Start() {
	go c.client.Start()
}

// Stop the Database client
func (c *ElasticSearchStorage) Stop() {
	c.client.Stop()
}

// New creates a new ElasticSearch database client
func New(backend string) (*ElasticSearchStorage, error) {
	cfg := esclient.NewConfig(backend)
	mappings := esclient.Mappings{
		{"metric": []byte(metricMapping)},
		{"rawpacket": []byte(rawPacketMapping)},
		{"flow": []byte(flowMapping)},
	}
	client, err := esclient.NewElasticSearchClient("flows", mappings, cfg)
	if err != nil {
		return nil, err
	}

	return &ElasticSearchStorage{client: client}, nil
}
