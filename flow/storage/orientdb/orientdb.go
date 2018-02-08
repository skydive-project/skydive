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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/gopacket/layers"
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

func flowRawPacketToDocument(linkType layers.LinkType, rawpacket *flow.RawPacket) orient.Document {
	return orient.Document{
		"@class":    "FlowRawPacket",
		"@type":     "d",
		"LinkType":  linkType,
		"Timestamp": rawpacket.Timestamp,
		"Index":     rawpacket.Index,
		"Data":      rawpacket.Data,
	}
}

func flowMetricToDocument(flow *flow.Flow, metric *flow.FlowMetric) orient.Document {
	if metric != nil {
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
	return nil
}

func flowTCPMetricToDocument(flow *flow.Flow, tcp_metric *flow.TCPMetric) orient.Document {
	if tcp_metric != nil {
		return orient.Document{
			"@class":     "TCPMetric",
			"@type":      "d",
			"ABSynStart": tcp_metric.ABSynStart,
			"BASynStart": tcp_metric.BASynStart,
			"ABSynTTL":   tcp_metric.ABSynTTL,
			"BASynTTL":   tcp_metric.BASynTTL,
			"ABFinStart": tcp_metric.ABFinStart,
			"BAFinStart": tcp_metric.BAFinStart,
			"ABRstStart": tcp_metric.ABRstStart,
			"BARstStart": tcp_metric.BARstStart,
		}

	}
	return nil
}

func flowToDocument(flow *flow.Flow) orient.Document {
	metricDoc := flowMetricToDocument(flow, flow.Metric)
	lastMetricDoc := flowMetricToDocument(flow, flow.LastUpdateMetric)
	tcpMetricDoc := flowTCPMetricToDocument(flow, flow.TCPFlowMetric)
	var flowDoc orient.Document
	flowDoc = orient.Document{
		"@class":             "Flow",
		"UUID":               flow.UUID,
		"LayersPath":         flow.LayersPath,
		"Application":        flow.Application,
		"Metric":             metricDoc,
		"Start":              flow.Start,
		"Last":               flow.Last,
		"RTT":                flow.RTT,
		"TrackingID":         flow.TrackingID,
		"L3TrackingID":       flow.L3TrackingID,
		"ParentUUID":         flow.ParentUUID,
		"NodeTID":            flow.NodeTID,
		"ANodeTID":           flow.ANodeTID,
		"BNodeTID":           flow.BNodeTID,
		"RawPacketsCaptured": flow.RawPacketsCaptured,
	}

	if tcpMetricDoc != nil {
		flowDoc["TCPFlowMetric"] = tcpMetricDoc
	}

	if lastMetricDoc != nil {
		flowDoc["LastUpdateMetric"] = lastMetricDoc
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

func documentToMetric(document orient.Document) (common.Metric, error) {
	flowMetric := new(flow.FlowMetric)
	if err := mapstructure.WeakDecode(document, flowMetric); err != nil {
		return nil, err
	}

	return flowMetric, nil
}

func documentToRawPacket(document orient.Document) (*flow.RawPacket, layers.LinkType, error) {
	// decode base64 by hand as the json decoder used by orient db client just see a string
	// and can not know that this is a array of byte as it only see interface{} as value
	var err error
	if document["Data"], err = base64.StdEncoding.DecodeString(document["Data"].(string)); err != nil {
		return nil, layers.LinkType(0), err
	}

	rawpacket := new(flow.RawPacket)
	if err = mapstructure.WeakDecode(document, rawpacket); err != nil {
		return nil, layers.LinkType(0), err
	}

	l, err := document["LinkType"].(json.Number).Int64()
	if err != nil {
		return nil, layers.LinkType(0), err
	}

	return rawpacket, layers.LinkType(l), nil
}

// StoreFlows pushes a set of flows in the database
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
			doc := flowMetricToDocument(flow, flow.LastUpdateMetric)
			doc["Flow"] = flowID
			if _, err = c.client.CreateDocument(doc); err != nil {
				logging.GetLogger().Errorf("Error while pushing metric %+v: %s\n", flow.LastUpdateMetric, err.Error())
				continue
			}
		}

		linkType, err := flow.LinkType()
		if err != nil {
			logging.GetLogger().Errorf("Error while indexing: %s", err.Error())
			continue
		}
		for _, r := range flow.LastRawPackets {
			doc := flowRawPacketToDocument(linkType, r)
			doc["Flow"] = flowID
			if _, err = c.client.CreateDocument(doc); err != nil {
				logging.GetLogger().Errorf("Error while pushing raw packet %+v: %s\n", r, err.Error())
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

// SearchMetrics searches flow raw packets matching filters in the database
func (c *OrientDBStorage) SearchRawPackets(fsq filters.SearchQuery, packetFilter *filters.Filter) (map[string]*flow.RawPackets, error) {
	filter := fsq.Filter
	sql := "SELECT LinkType, Timestamp, Index, Data, Flow.UUID FROM FlowRawPacket"

	where := false
	if packetFilter != nil {
		sql += " WHERE " + orient.FilterToExpression(packetFilter, nil)
		where = true
	}
	if conditional := orient.FilterToExpression(filter, func(s string) string { return "Flow." + s }); conditional != "" {
		if where {
			sql += " AND " + conditional
		} else {
			sql += " WHERE " + conditional
			where = true
		}
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

	rawpackets := make(map[string]*flow.RawPackets)
	for _, doc := range docs {
		r, linkType, err := documentToRawPacket(doc)
		if err != nil {
			return nil, err
		}
		flowID := doc["Flow"].(string)

		if fr, ok := rawpackets[flowID]; ok {
			fr.RawPackets = append(fr.RawPackets, r)
		} else {
			rawpackets[flowID] = &flow.RawPackets{
				LinkType:   linkType,
				RawPackets: []*flow.RawPacket{r},
			}
		}
	}

	return rawpackets, nil
}

// SearchMetrics searches flow metrics matching filters in the database
func (c *OrientDBStorage) SearchMetrics(fsq filters.SearchQuery, metricFilter *filters.Filter) (map[string][]common.Metric, error) {
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

	metrics := make(map[string][]common.Metric)
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
	addr := config.GetString("storage.orientdb.addr")
	database := config.GetString("storage.orientdb.database")
	username := config.GetString("storage.orientdb.username")
	password := config.GetString("storage.orientdb.password")

	client, err := orient.NewClient(addr, database, username, password)
	if err != nil {
		return nil, err
	}

	if _, err := client.GetDocumentClass("FlowRawPacket"); err != nil {
		class := orient.ClassDefinition{
			Name: "FlowRawPacket",
			Properties: []orient.Property{
				{Name: "LinkType", Type: "INTEGER", Mandatory: true, NotNull: true},
				{Name: "Timestamp", Type: "LONG", Mandatory: true, NotNull: true},
				{Name: "Index", Type: "INTEGER", Mandatory: true, NotNull: true},
				{Name: "Data", Type: "BINARY", Mandatory: true, NotNull: true},
			},
			Indexes: []orient.Index{
				{Name: "FlowRawPacket.Timestamp", Fields: []string{"Timestamp"}, Type: "NOTUNIQUE"},
			},
		}
		if err := client.CreateDocumentClass(class); err != nil {
			return nil, fmt.Errorf("Failed to register class FlowRawPacket: %s", err.Error())
		}
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

	if _, err := client.GetDocumentClass("TCPMetric"); err != nil {
		class := orient.ClassDefinition{
			Name: "TCPMetric",
			Properties: []orient.Property{
				{Name: "ABSynStart", Type: "LONG", Mandatory: false, NotNull: true},
				{Name: "BASynStart", Type: "LONG", Mandatory: false, NotNull: true},
				{Name: "ABSynTTL", Type: "INTEGER", Mandatory: false, NotNull: true},
				{Name: "BASynTTL", Type: "INTEGER", Mandatory: false, NotNull: true},
				{Name: "ABFinStart", Type: "LONG", Mandatory: false, NotNull: true},
				{Name: "BAFinStart", Type: "LONG", Mandatory: false, NotNull: true},
				{Name: "ABRstStart", Type: "LONG", Mandatory: false, NotNull: true},
				{Name: "BARstStart", Type: "LONG", Mandatory: false, NotNull: true},
			},
			Indexes: []orient.Index{
				{Name: "TCPMetric.TimeSpan", Fields: []string{"ABSynStart", "ABFinStart"}, Type: "NOTUNIQUE"},
			},
		}
		if err := client.CreateDocumentClass(class); err != nil {
			return nil, fmt.Errorf("Failed to register class ExtFlowMetric: %s", err.Error())
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
				{Name: "TCPFlowMetric", Type: "EMBEDDED", LinkedClass: "TCPMetric"},
				{Name: "Start", Type: "LONG"},
				{Name: "Last", Type: "LONG"},
				{Name: "TrackingID", Type: "STRING", Mandatory: true, NotNull: true},
				{Name: "L3TrackingID", Type: "STRING"},
				{Name: "ParentUUID", Type: "STRING"},
				{Name: "NodeTID", Type: "STRING"},
				{Name: "ANodeTID", Type: "STRING"},
				{Name: "BNodeTID", Type: "STRING"},
				{Name: "RawPacketsCaptured", Type: "LONG"},
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

	client.CreateProperty("TCPMetric", flowProp)

	extFlowIndex := orient.Index{Name: "TCPMetric.Flow", Fields: []string{"Flow"}, Type: "NOTUNIQUE"}
	client.CreateIndex("TCPMetric", extFlowIndex)

	return &OrientDBStorage{
		client: client,
	}, nil
}
