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
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/logging"
	orient "github.com/skydive-project/skydive/topology/graph/orientdb"
)

type OrientDBStorage struct {
	client *orient.Client
}

func flowToDocument(flow *flow.Flow) orient.Document {
	var endpoints []orient.Document
	for _, endpoint := range flow.Statistics.Endpoints {
		flowEndpointStatisticsDocAB := orient.Document{
			"@class":  "FlowEndpointStatistics",
			"@type":   "d",
			"Value":   endpoint.GetAB().Value,
			"Packets": endpoint.GetAB().Packets,
			"Bytes":   endpoint.GetAB().Bytes,
		}

		flowEndpointStatisticsDocBA := orient.Document{
			"@class":  "FlowEndpointStatistics",
			"@type":   "d",
			"Value":   endpoint.GetBA().Value,
			"Packets": endpoint.GetBA().Packets,
			"Bytes":   endpoint.GetBA().Bytes,
		}

		endpointDoc := orient.Document{
			"@class": "Endpoint",
			"@type":  "d",
			"Type":   endpoint.Type,
			"AB":     flowEndpointStatisticsDocAB,
			"BA":     flowEndpointStatisticsDocBA,
		}

		endpoints = append(endpoints, endpointDoc)
	}

	statsDoc := orient.Document{
		"@class":    "Statistics",
		"@type":     "d",
		"Start":     flow.Statistics.Start,
		"Last":      flow.Statistics.Last,
		"Endpoints": endpoints,
	}

	flowDoc := orient.Document{
		"@class":        "Flow",
		"UUID":          flow.UUID,
		"LayersPath":    flow.LayersPath,
		"ProbeNodeUUID": flow.ProbeNodeUUID,
		"IfSrcNodeUUID": flow.IfSrcNodeUUID,
		"IfDstNodeUUID": flow.IfDstNodeUUID,
		"Statistics":    statsDoc,
	}

	return flowDoc
}

func documentToFlow(document orient.Document) (flow *flow.Flow, err error) {
	if err = mapstructure.WeakDecode(document, &flow); err != nil {
		return nil, err
	}
	return
}

func (c *OrientDBStorage) StoreFlows(flows []*flow.Flow) error {
	// TODO: use batch of operations
	for _, flow := range flows {
		if _, err := c.client.CreateDocument(flowToDocument(flow)); err != nil {
			logging.GetLogger().Errorf("Error while pushing flow %s: %s\n\n", flow.UUID, err.Error())
			return err
		}
	}
	return nil
}

func (c *OrientDBStorage) SearchFlows(filters *flow.Filters) ([]*flow.Flow, error) {
	sql := "SELECT FROM Flow"
	if len(filters.Range)+len(filters.Term.Terms) > 0 {
		var filterList []string
		for _, term := range filters.Term.Terms {
			marshal, err := json.Marshal(term.Value)
			if err != nil {
				return nil, err
			}
			filterList = append(filterList, fmt.Sprintf("%s = %s", term.Key, marshal))
		}
		sql += " WHERE "
		if filters.Term.Op == flow.AND {
			sql += strings.Join(filterList, " AND ")
		} else {
			sql += strings.Join(filterList, " OR ")
		}
	}
	sql += " ORDER BY Statistics.Last"
	docs, err := c.client.Sql(sql)
	if err != nil {
		return nil, err
	}

	flows := []*flow.Flow{}
	for _, doc := range docs {
		flow, err := documentToFlow(doc)
		if err != nil {
			return nil, err
		}
		flows = append(flows, flow)
	}

	return flows, nil
}

func (c *OrientDBStorage) Start() {
}

func (c *OrientDBStorage) Stop() {
}

func (c *OrientDBStorage) Close() {
}

func New() (*OrientDBStorage, error) {
	addr := config.GetConfig().GetString("orientdb.addr")
	database := config.GetConfig().GetString("orientdb.database")
	username := config.GetConfig().GetString("orientdb.username")
	password := config.GetConfig().GetString("orientdb.password")

	client, err := orient.NewClient(addr, database, username, password)
	if err != nil {
		return nil, err
	}

	if _, err := client.GetDocumentClass("FlowEndpointStatistics"); err != nil {
		class := orient.ClassDefinition{
			Name: "FlowEndpointStatistics",
			Properties: []orient.Property{
				{Name: "Bytes", Type: "INTEGER", Mandatory: true, NotNull: true},
				{Name: "Packets", Type: "INTEGER", Mandatory: true, NotNull: true},
				{Name: "Value", Type: "STRING", Mandatory: true, NotNull: true},
			},
			Indexes: []orient.Index{},
		}
		if err := client.CreateDocumentClass(class); err != nil {
			return nil, fmt.Errorf("Failed to register class FlowEndpointStatistics: %s", err.Error())
		}
	}

	if _, err := client.GetDocumentClass("FlowEndpointsStatistics"); err != nil {
		class := orient.ClassDefinition{
			Name: "FlowEndpointsStatistics",
			Properties: []orient.Property{
				{Name: "AB", Type: "EMBEDDED", LinkedClass: "FlowEndpointStatistics", Mandatory: true, NotNull: true},
				{Name: "BA", Type: "EMBEDDED", LinkedClass: "FlowEndpointStatistics", Mandatory: true, NotNull: true},
				{Name: "Type", Type: "INTEGER", Mandatory: true, NotNull: true},
			},
			Indexes: []orient.Index{},
		}
		if err := client.CreateDocumentClass(class); err != nil {
			return nil, fmt.Errorf("Failed to register class FlowEndpointsStatistics: %s", err.Error())
		}
	}

	if _, err := client.GetDocumentClass("FlowStatistics"); err != nil {
		class := orient.ClassDefinition{
			Name: "FlowStatistics",
			Properties: []orient.Property{
				{Name: "Endpoints", Type: "EMBEDDEDLIST", LinkedClass: "FlowEndpointsStatistics"},
				{Name: "Start", Type: "INTEGER"},
				{Name: "Last", Type: "INTEGER"},
			},
			Indexes: []orient.Index{},
		}
		if err := client.CreateDocumentClass(class); err != nil {
			return nil, fmt.Errorf("Failed to register class FlowStatistics: %s", err.Error())
		}
	}

	if _, err := client.GetDocumentClass("Flow"); err != nil {
		class := orient.ClassDefinition{
			Name: "Flow",
			Properties: []orient.Property{
				{Name: "UUID", Type: "STRING", Mandatory: true, NotNull: true},
				{Name: "LayersPath", Type: "STRING", Mandatory: true, NotNull: true},
				{Name: "Statistics", Type: "EMBEDDED", LinkedClass: "FlowStatistics"},
				{Name: "ProbeNodeUUID", Type: "STRING"},
				{Name: "IfSrcNodeUUID", Type: "STRING"},
				{Name: "IfDstNodeUUID", Type: "STRING"},
			},
			Indexes: []orient.Index{},
		}
		if err := client.CreateDocumentClass(class); err != nil {
			return nil, fmt.Errorf("Failed to register class Flow: %s", err.Error())
		}
	}

	return &OrientDBStorage{
		client: client,
	}, nil
}
