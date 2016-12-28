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
	"fmt"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/logging"
	orient "github.com/skydive-project/skydive/topology/graph/orientdb"
)

type OrientDBStorage struct {
	client *orient.Client
}

func filterToExpression(f *flow.Filter, prefix string) string {
	if f.BoolFilter != nil {
		keyword := ""
		switch f.BoolFilter.Op {
		case flow.BoolFilterOp_NOT:
			// FIX not yet implemented for the orientdb backend
			// http://orientdb.com/docs/2.0/orientdb.wiki/SQL-Where.html
			return "NOT " + filterToExpression(f.BoolFilter.Filters[0], prefix)
		case flow.BoolFilterOp_OR:
			keyword = "OR"
		case flow.BoolFilterOp_AND:
			keyword = "AND"
		}
		var conditions []string
		for _, item := range f.BoolFilter.Filters {
			conditions = append(conditions, "("+filterToExpression(item, prefix)+")")
		}
		return strings.Join(conditions, " "+keyword+" ")
	}

	if f.TermStringFilter != nil {
		return fmt.Sprintf(`%s = "%s"`, prefix+f.TermStringFilter.Key, f.TermStringFilter.Value)
	}

	if f.TermInt64Filter != nil {
		return fmt.Sprintf(`%s = %d`, prefix+f.TermInt64Filter.Key, f.TermInt64Filter.Value)
	}

	if f.GtInt64Filter != nil {
		return fmt.Sprintf("%v > %v", prefix+f.GtInt64Filter.Key, f.GtInt64Filter.Value)
	}

	if f.LtInt64Filter != nil {
		return fmt.Sprintf("%v < %v", prefix+f.LtInt64Filter.Key, f.LtInt64Filter.Value)
	}

	if f.GteInt64Filter != nil {
		return fmt.Sprintf("%v >= %v", prefix+f.GteInt64Filter.Key, f.GteInt64Filter.Value)
	}

	if f.LteInt64Filter != nil {
		return fmt.Sprintf("%v <= %v", prefix+f.LteInt64Filter.Key, f.LteInt64Filter.Value)
	}

	if f.RegexFilter != nil {
		return fmt.Sprintf(`%s MATCHES "%s"`, prefix+f.RegexFilter.Key, f.RegexFilter.Value)
	}

	return ""
}

func metricToDocument(metric *flow.FlowMetric) orient.Document {
	return orient.Document{
		"@class":    "FlowMetric",
		"@type":     "d",
		"Start":     metric.Start,
		"Last":      metric.Last,
		"ABPackets": metric.ABPackets,
		"ABBytes":   metric.ABBytes,
		"BAPackets": metric.BAPackets,
		"BABytes":   metric.BABytes,
	}
}

func flowToDocument(flow *flow.Flow) orient.Document {
	linkLayer := orient.Document{
		"Protocol": flow.Link.Protocol,
		"A":        flow.Link.A,
		"B":        flow.Link.B,
	}

	metricDoc := metricToDocument(flow.Metric)

	flowDoc := orient.Document{
		"@class":       "Flow",
		"UUID":         flow.UUID,
		"TrackingID":   flow.TrackingID,
		"L3TrackingID": flow.L3TrackingID,
		"LayersPath":   flow.LayersPath,
		"Application":  flow.Application,
		"NodeTID":      flow.NodeTID,
		"ANodeTID":     flow.ANodeTID,
		"BNodeTID":     flow.BNodeTID,
		"Metric":       metricDoc,
		"LinkLayer":    linkLayer,
	}

	if flow.Network != nil {
		flowDoc["NetworkLayer"] = orient.Document{
			"Protocol": flow.Network.Protocol,
			"A":        flow.Network.A,
			"B":        flow.Network.B,
		}
	}

	if flow.Transport != nil {
		flowDoc["TransportLayer"] = orient.Document{
			"Protocol": flow.Transport.Protocol,
			"A":        flow.Transport.A,
			"B":        flow.Transport.B,
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

func documentToMetric(document orient.Document) (metric *flow.FlowMetric, err error) {
	if err = mapstructure.WeakDecode(document, &metric); err != nil {
		return nil, err
	}
	return
}

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

		if flow.LastUpdateMetric != nil {
			doc := metricToDocument(flow.LastUpdateMetric)
			doc["Flow"] = flowID
			if _, err := c.client.CreateDocument(doc); err != nil {
				logging.GetLogger().Errorf("Error while pushing metric %+v: %s\n", flow.LastUpdateMetric, err.Error())
				continue
			}
		}
	}

	return nil
}

func (c *OrientDBStorage) SearchFlows(fsq flow.FlowSearchQuery) (*flow.FlowSet, error) {
	interval := fsq.PaginationRange
	filter := fsq.Filter

	sql := "SELECT FROM Flow"
	if conditional := filterToExpression(filter, ""); conditional != "" {
		sql += " WHERE " + conditional
	}

	if interval != nil {
		sql += fmt.Sprintf(" LIMIT %d, %d", interval.To-interval.From, interval.From)
	}

	if fsq.Sort {
		sql += " ORDER BY " + fsq.SortBy
	}

	docs, err := c.client.Sql(sql)
	if err != nil {
		return nil, err
	}

	flowset := flow.NewFlowSet()
	for _, doc := range docs {
		flow, err := documentToFlow(doc)
		if err != nil {
			return nil, err
		}
		flowset.Flows = append(flowset.Flows, flow)
	}

	if fsq.Dedup {
		if err := flowset.Dedup(fsq.DedupBy); err != nil {
			return nil, err
		}
	}

	return flowset, nil
}

func (c *OrientDBStorage) SearchMetrics(fsq flow.FlowSearchQuery, metricFilter *flow.Filter) (map[string][]*flow.FlowMetric, error) {
	filter := fsq.Filter
	sql := "SELECT ABBytes, ABPackets, BABytes, BAPackets, Start, Last, Flow.UUID FROM FlowMetric"

	sql += " WHERE " + filterToExpression(metricFilter, "")

	if conditional := filterToExpression(filter, "Flow."); conditional != "" {
		sql += " AND " + conditional
	}

	sql += " ORDER BY Start"
	docs, err := c.client.Sql(sql)
	if err != nil {
		return nil, err
	}

	metrics := map[string][]*flow.FlowMetric{}
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

func (c *OrientDBStorage) Start() {
}

func (c *OrientDBStorage) Stop() {
}

func (c *OrientDBStorage) Close() {
}

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
				{Name: "Start", Type: "INTEGER", Mandatory: true, NotNull: true},
				{Name: "Last", Type: "INTEGER", Mandatory: true, NotNull: true},
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
				{Name: "Metric", Type: "EMBEDDED", LinkedClass: "FlowMetric"},
				{Name: "TrackingID", Type: "STRING", Mandatory: true, NotNull: true},
				{Name: "L3TrackingID", Type: "STRING"},
				{Name: "NodeTID", Type: "STRING"},
				{Name: "ANodeTID", Type: "STRING"},
				{Name: "BNodeTID", Type: "STRING"},
			},
			Indexes: []orient.Index{
				{Name: "Flow.UUID", Fields: []string{"UUID"}, Type: "UNIQUE"},
				{Name: "Flow.TrackingID", Fields: []string{"TrackingID"}, Type: "NOTUNIQUE"},
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
