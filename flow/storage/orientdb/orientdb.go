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

package orientdb

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/filters"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/logging"
	orient "github.com/skydive-project/skydive/storage/orientdb"
)

// OrientDBStorage describes a OrientDB database client
type OrientDBStorage struct {
	client *orient.Client
}

func flowMetricToDocument(flow *flow.Flow, metric *flow.FlowMetric) orient.Document {
	return orient.Document{
		"@class":    "FlowMetric",
		"@type":     "d",
		"Start":     flow.LastUpdateStart,
		"Last":      flow.LastUpdateLast,
		"ABPackets": metric.ABPackets,
		"ABBytes":   metric.ABBytes,
		"BAPackets": metric.BAPackets,
		"BABytes":   metric.BABytes,
	}
}

func flowToDocument(flow *flow.Flow) orient.Document {
	metricDoc := flowMetricToDocument(flow, flow.Metric)
	lastMetricDoc := flowMetricToDocument(flow, flow.LastUpdateMetric)

	flowDoc := orient.Document{
		"@class":           "Flow",
		"UUID":             flow.UUID,
		"LayersPath":       flow.LayersPath,
		"Application":      flow.Application,
		"LastUpdateMetric": lastMetricDoc,
		"Metric":           metricDoc,
		"Start":            flow.Start,
		"Last":             flow.Last,
		"LastUpdateStart":  flow.LastUpdateStart,
		"LastUpdateLast":   flow.LastUpdateLast,
		"TrackingID":       flow.TrackingID,
		"L3TrackingID":     flow.L3TrackingID,
		"ParentUUID":       flow.ParentUUID,
		"NodeTID":          flow.NodeTID,
		"ANodeTID":         flow.ANodeTID,
		"BNodeTID":         flow.BNodeTID,
	}

	if flow.Link != nil {
		flowDoc["Link"] = orient.Document{
			"Protocol": flow.Link.Protocol.String(),
			"A":        flow.Link.A,
			"B":        flow.Link.B,
			"ID":       flow.Link.ID,
		}
	}

	if flow.Network != nil {
		flowDoc["Network"] = orient.Document{
			"Protocol": flow.Network.Protocol.String(),
			"A":        flow.Network.A,
			"B":        flow.Network.B,
			"ID":       flow.Network.ID,
		}
	}

	if flow.ICMP != nil {
		flowDoc["ICMP"] = orient.Document{
			"Type": flow.ICMP.Type.String(),
			"Code": flow.ICMP.Code,
			"ID":   flow.ICMP.ID,
		}
	}

	if flow.Transport != nil {
		flowDoc["Transport"] = orient.Document{
			"Protocol": flow.Transport.Protocol.String(),
			"A":        flow.Transport.A,
			"B":        flow.Transport.B,
			"ID":       flow.Transport.ID,
		}
	}

	return flowDoc
}

func documentToFlow(document orient.Document) (flow *flow.Flow, err error) {
	if err = mapstructure.WeakDecode(document, &flow); err != nil {
		return nil, err
	}
	return
}

func documentToMetric(document orient.Document) (*common.TimedMetric, error) {
	flowMetric := new(flow.FlowMetric)
	if err := mapstructure.WeakDecode(document, flowMetric); err != nil {
		return nil, err
	}

	start, err := document["Start"].(json.Number).Int64()
	if err != nil {
		return nil, err
	}

	last, err := document["Last"].(json.Number).Int64()
	if err != nil {
		return nil, err
	}

	return &common.TimedMetric{
		TimeSlice: common.TimeSlice{
			Start: start,
			Last:  last,
		},
		Metric: flowMetric,
	}, nil
}

// StoreFlows push a set of flows in the database
func (c *OrientDBStorage) StoreFlows(flows []*flow.Flow) error {
	// TODO: use batch of operations
	for _, flow := range flows {
		flowDoc, err := c.client.Upsert(flowToDocument(flow), "UUID")
		if err != nil {
			logging.GetLogger().Errorf("Error while pushing flow %s: %s\n", flow.UUID, err.Error())
			return err
		}

		flowID, ok := flowDoc["@rid"]
		if !ok {
			logging.GetLogger().Errorf("No @rid attribute for flow '%s'", flow.UUID)
			return err
		}

		if flow.LastUpdateStart != 0 {
			doc := flowMetricToDocument(flow, flow.LastUpdateMetric)
			doc["Flow"] = flowID
			if _, err := c.client.CreateDocument(doc); err != nil {
				logging.GetLogger().Errorf("Error while pushing metric %+v: %s\n", flow.LastUpdateMetric, err.Error())
				continue
			}
		}
	}

	return nil
}

// SearchFlows search flow matching filters in the database
func (c *OrientDBStorage) SearchFlows(fsq filters.SearchQuery) (*flow.FlowSet, error) {
	flowset := flow.NewFlowSet()

	err := c.client.Query("Flow", &fsq, &flowset.Flows)
	if err != nil {
		return nil, err
	}

	if fsq.Dedup {
		if err := flowset.Dedup(fsq.DedupBy); err != nil {
			return nil, err
		}
	}

	return flowset, nil
}

// SearchMetrics search flow metrics matching filters in the database
func (c *OrientDBStorage) SearchMetrics(fsq filters.SearchQuery, metricFilter *filters.Filter) (map[string][]*common.TimedMetric, error) {
	filter := fsq.Filter
	sql := "SELECT ABBytes, ABPackets, BABytes, BAPackets, Start, Last, Flow.UUID FROM FlowMetric"
	sql += " WHERE " + orient.FilterToExpression(metricFilter, nil)
	if conditional := orient.FilterToExpression(filter, func(s string) string { return "Flow." + s }); conditional != "" {
		sql += " AND " + conditional
	}

	if fsq.Sort {
		sql += " ORDER BY " + fsq.SortBy
		if fsq.SortOrder != "" {
			sql += " " + strings.ToUpper(fsq.SortOrder)
		}
	}

	docs, err := c.client.Search(sql)
	if err != nil {
		return nil, err
	}

	metrics := make(map[string][]*common.TimedMetric)
	for _, doc := range docs {
		metric, err := documentToMetric(doc)
		if err != nil {
			return nil, err
		}
		flowID := doc["Flow"].(string)
		metrics[flowID] = append(metrics[flowID], metric)
	}

	return metrics, nil
}

// Start the database client
func (c *OrientDBStorage) Start() {
}

// Stop the database client
func (c *OrientDBStorage) Stop() {
}

// Close the database client
func (c *OrientDBStorage) Close() {
}

// New creates a new OrientDB database client
func New() (*OrientDBStorage, error) {
	addr := config.GetConfig().GetString("storage.orientdb.addr")
	database := config.GetConfig().GetString("storage.orientdb.database")
	username := config.GetConfig().GetString("storage.orientdb.username")
	password := config.GetConfig().GetString("storage.orientdb.password")

	client, err := orient.NewClient(addr, database, username, password)
	if err != nil {
		return nil, err
	}

	if _, err := client.GetDocumentClass("FlowMetric"); err != nil {
		class := orient.ClassDefinition{
			Name: "FlowMetric",
			Properties: []orient.Property{
				{Name: "ABBytes", Type: "INTEGER", Mandatory: true, NotNull: true},
				{Name: "ABPackets", Type: "INTEGER", Mandatory: true, NotNull: true},
				{Name: "BABytes", Type: "INTEGER", Mandatory: true, NotNull: true},
				{Name: "BAPackets", Type: "INTEGER", Mandatory: true, NotNull: true},
				{Name: "Start", Type: "LONG", Mandatory: true, NotNull: true},
				{Name: "Last", Type: "LONG", Mandatory: true, NotNull: true},
			},
			Indexes: []orient.Index{
				{Name: "FlowMetric.TimeSpan", Fields: []string{"Start", "Last"}, Type: "NOTUNIQUE"},
			},
		}
		if err := client.CreateDocumentClass(class); err != nil {
			return nil, fmt.Errorf("Failed to register class FlowMetric: %s", err.Error())
		}
	}

	if _, err := client.GetDocumentClass("Flow"); err != nil {
		class := orient.ClassDefinition{
			Name: "Flow",
			Properties: []orient.Property{
				{Name: "UUID", Type: "STRING", Mandatory: true, NotNull: true},
				{Name: "LayersPath", Type: "STRING", Mandatory: true, NotNull: true},
				{Name: "Application", Type: "STRING"},
				{Name: "LastUpdateMetric", Type: "EMBEDDED", LinkedClass: "FlowMetric"},
				{Name: "Metric", Type: "EMBEDDED", LinkedClass: "FlowMetric"},
				{Name: "Start", Type: "LONG"},
				{Name: "Last", Type: "LONG"},
				{Name: "LastUpdateStart", Type: "LONG"},
				{Name: "LastUpdateLast", Type: "LONG"},
				{Name: "TrackingID", Type: "STRING", Mandatory: true, NotNull: true},
				{Name: "L3TrackingID", Type: "STRING"},
				{Name: "ParentUUID", Type: "STRING"},
				{Name: "NodeTID", Type: "STRING"},
				{Name: "ANodeTID", Type: "STRING"},
				{Name: "BNodeTID", Type: "STRING"},
			},
			Indexes: []orient.Index{
				{Name: "Flow.UUID", Fields: []string{"UUID"}, Type: "UNIQUE"},
				{Name: "Flow.TrackingID", Fields: []string{"TrackingID"}, Type: "NOTUNIQUE"},
				{Name: "Flow.TimeSpan", Fields: []string{"Start", "Last"}, Type: "NOTUNIQUE"},
			},
		}
		if err := client.CreateDocumentClass(class); err != nil {
			return nil, fmt.Errorf("Failed to register class Flow: %s", err.Error())
		}
	}

	flowProp := orient.Property{Name: "Flow", Type: "LINK", LinkedClass: "Flow", Mandatory: false, NotNull: true}
	client.CreateProperty("FlowMetric", flowProp)

	flowIndex := orient.Index{Name: "FlowMetric.Flow", Fields: []string{"Flow"}, Type: "NOTUNIQUE"}
	client.CreateIndex("FlowMetric", flowIndex)

	return &OrientDBStorage{
		client: client,
	}, nil
}
