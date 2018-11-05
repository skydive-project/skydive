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

	"github.com/google/gopacket/layers"
	"github.com/olivere/elastic"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/etcd"
	"github.com/skydive-project/skydive/filters"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/logging"
	es "github.com/skydive-project/skydive/storage/elasticsearch"
)

const flowMapping = `
{
	"dynamic_templates": [
		{
			"strings": {
				"match": "*",
				"match_mapping_type": "string",
				"mapping": {
					"type": "keyword"
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
					"type": "date",
					"format": "epoch_millis"
				}
			}
		},
		{
			"last": {
				"match": "Last",
				"mapping": {
					"type": "date",
					"format": "epoch_millis"
				}
			}
		},
		{
			"last": {
				"match": "Timestamp",
				"mapping": {
					"type": "date",
					"format": "epoch_millis"
				}
			}
		}
	]
}`

var (
	flowIndex = es.Index{
		Name:      "flow",
		Type:      "flow",
		Mapping:   flowMapping,
		RollIndex: true,
	}
	metricIndex = es.Index{
		Name:      "metric",
		Type:      "metric",
		Mapping:   flowMapping,
		RollIndex: true,
	}
	rawpacketIndex = es.Index{
		Name:      "rawpacket",
		Type:      "rawpacket",
		Mapping:   flowMapping,
		RollIndex: true,
	}
)

// Storage describes an ElasticSearch flow backend
type Storage struct {
	client *es.Client
}

// easyjson:json
type embeddedFlow struct {
	UUID         *string
	LayersPath   *string
	Application  *string
	Link         *flow.FlowLayer      `json:"Link,omitempty"`
	Network      *flow.FlowLayer      `json:"Network,omitempty"`
	Transport    *flow.TransportLayer `json:"Transport,omitempty"`
	ICMP         *flow.ICMPLayer      `json:"ICMP,omitempty"`
	TrackingID   *string
	L3TrackingID *string
	ParentUUID   *string
	NodeTID      *string
	Start        *int64
	Last         *int64
}

func flowToEmbbedFlow(f *flow.Flow) *embeddedFlow {
	return &embeddedFlow{
		UUID:         &f.UUID,
		LayersPath:   &f.LayersPath,
		Application:  &f.Application,
		Link:         f.Link,
		Network:      f.Network,
		Transport:    f.Transport,
		ICMP:         f.ICMP,
		TrackingID:   &f.TrackingID,
		L3TrackingID: &f.L3TrackingID,
		ParentUUID:   &f.ParentUUID,
		NodeTID:      &f.NodeTID,
		Start:        &f.Start,
		Last:         &f.Last,
	}
}

// easyjson:json
type metricRecord struct {
	*flow.FlowMetric
	Flow *embeddedFlow `json:"Flow"`
}

// easyjson:json
type rawpacketRecord struct {
	LinkType layers.LinkType
	*flow.RawPacket
	Flow *embeddedFlow `json:"Flow"`
}

// StoreFlows push a set of flows in the database
func (c *Storage) StoreFlows(flows []*flow.Flow) error {
	if !c.client.Started() {
		return errors.New("Storage is not yet started")
	}

	for _, f := range flows {
		data, err := json.Marshal(f)
		if err != nil {
			logging.GetLogger().Error(err)
			continue
		}

		if err := c.client.BulkIndex(flowIndex, f.UUID, json.RawMessage(data)); err != nil {
			logging.GetLogger().Error(err)
			continue
		}

		eflow := flowToEmbbedFlow(f)

		if f.LastUpdateMetric != nil {
			record := &metricRecord{
				FlowMetric: f.LastUpdateMetric,
				Flow:       eflow,
			}

			data, err := json.Marshal(record)
			if err != nil {
				logging.GetLogger().Error(err)
				continue
			}

			if err := c.client.BulkIndex(metricIndex, "", json.RawMessage(data)); err != nil {
				logging.GetLogger().Error(err)
				continue
			}
		}

		linkType, err := f.LinkType()
		if err != nil {
			logging.GetLogger().Errorf("Error while indexing: %s", err)
			continue
		}
		for _, r := range f.LastRawPackets {
			record := &rawpacketRecord{
				LinkType:  linkType,
				RawPacket: r,
				Flow:      eflow,
			}

			data, err := json.Marshal(record)
			if err != nil {
				logging.GetLogger().Error(err)
				continue
			}

			if c.client.BulkIndex(rawpacketIndex, "", json.RawMessage(data)) != nil {
				logging.GetLogger().Error(err)
				continue
			}
		}
	}

	return nil
}

func (c *Storage) sendRequest(typ string, query elastic.Query, pagination filters.SearchQuery, indices ...string) (*elastic.SearchResult, error) {
	return c.client.Search(typ, query, pagination, indices...)
}

// SearchRawPackets searches flow raw packets matching filters in the database
func (c *Storage) SearchRawPackets(fsq filters.SearchQuery, packetFilter *filters.Filter) (map[string]*flow.RawPackets, error) {
	if !c.client.Started() {
		return nil, errors.New("Storage is not yet started")
	}

	// do not escape flow as ES use sub object in that case
	mustQueries := []elastic.Query{es.FormatFilter(fsq.Filter, "Flow")}

	if packetFilter != nil {
		mustQueries = append(mustQueries, es.FormatFilter(packetFilter, ""))
	}

	out, err := c.sendRequest("rawpacket", elastic.NewBoolQuery().Must(mustQueries...), fsq, rawpacketIndex.IndexWildcard())
	if err != nil {
		return nil, err
	}

	rawpackets := make(map[string]*flow.RawPackets)
	if len(out.Hits.Hits) > 0 {
		for _, d := range out.Hits.Hits {
			var record rawpacketRecord
			if err := json.Unmarshal([]byte(*d.Source), &record); err != nil {
				return nil, err
			}

			if fr, ok := rawpackets[*record.Flow.UUID]; ok {
				fr.RawPackets = append(fr.RawPackets, record.RawPacket)
			} else {
				rawpackets[*record.Flow.UUID] = &flow.RawPackets{
					LinkType:   record.LinkType,
					RawPackets: []*flow.RawPacket{record.RawPacket},
				}
			}
		}
	}

	return rawpackets, nil
}

// SearchMetrics searches flow metrics matching filters in the database
func (c *Storage) SearchMetrics(fsq filters.SearchQuery, metricFilter *filters.Filter) (map[string][]common.Metric, error) {
	if !c.client.Started() {
		return nil, errors.New("Storage is not yet started")
	}

	// do not escape flow as ES use sub object in that case
	flowQuery := es.FormatFilter(fsq.Filter, "Flow")
	metricQuery := es.FormatFilter(metricFilter, "")

	query := elastic.NewBoolQuery().Must(flowQuery, metricQuery)
	out, err := c.sendRequest("metric", query, fsq, metricIndex.IndexWildcard())
	if err != nil {
		return nil, err
	}

	metrics := make(map[string][]common.Metric)
	if len(out.Hits.Hits) > 0 {
		for _, d := range out.Hits.Hits {
			var record metricRecord
			if err := json.Unmarshal([]byte(*d.Source), &record); err != nil {
				return nil, err
			}

			if fm, ok := metrics[*record.Flow.UUID]; ok {
				metrics[*record.Flow.UUID] = append(fm, record.FlowMetric)
			} else {
				metrics[*record.Flow.UUID] = []common.Metric{record.FlowMetric}
			}
		}
	}

	return metrics, nil
}

// SearchFlows search flow matching filters in the database
func (c *Storage) SearchFlows(fsq filters.SearchQuery) (*flow.FlowSet, error) {
	if !c.client.Started() {
		return nil, errors.New("Storage is not yet started")
	}

	// TODO: dedup and sort in order to remove duplicate flow UUID due to rolling index
	out, err := c.sendRequest("flow", es.FormatFilter(fsq.Filter, ""), fsq, flowIndex.IndexWildcard())
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
func (c *Storage) Start() {
	go c.client.Start()
}

// Stop the Database client
func (c *Storage) Stop() {
	c.client.Stop()
}

// New creates a new ElasticSearch database client
func New(backend string, etcdClient *etcd.Client) (*Storage, error) {
	cfg := es.NewConfig(backend)

	indices := []es.Index{
		flowIndex,
		metricIndex,
		rawpacketIndex,
	}

	client, err := es.NewClient(indices, cfg, etcdClient)
	if err != nil {
		return nil, err
	}

	return &Storage{client: client}, nil
}
