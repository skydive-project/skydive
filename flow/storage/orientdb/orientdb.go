/*
 * Copyright (C) 2016 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy ofthe License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specificlanguage governing permissions and
 * limitations under the License.
 *
 */

package orientdb

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/gopacket/layers"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/filters"
	"github.com/skydive-project/skydive/flow"
	fl "github.com/skydive-project/skydive/flow/layers"
	"github.com/skydive-project/skydive/logging"
	orient "github.com/skydive-project/skydive/storage/orientdb"
)

// Storage describes a OrientDB database client
type Storage struct {
	client *orient.Client
}

// easyjson:json
type flowDoc struct {
	Class              string `json:"@class"`
	UUID               *string
	LayersPath         *string
	Application        *string
	Link               *flow.FlowLayer      `json:"Link,omitempty"`
	Network            *flow.FlowLayer      `json:"Network,omitempty"`
	Transport          *flow.TransportLayer `json:"Transport,omitempty"`
	ICMP               *flow.ICMPLayer      `json:"ICMP,omitempty"`
	Metric             *flow.FlowMetric     `json:"Metric,omitempty"`
	TCPMetric          *flow.TCPMetric      `json:"TCPMetric,omitempty"`
	IPMetric           *flow.IPMetric       `json:"IPMetric,omitempty"`
	DHCPv4             *fl.DHCPv4           `json:"DHCPv4,omitempty"`
	DNS                *fl.DNS              `json:"DNS,omitempty"`
	VRRPv2             *fl.VRRPv2           `json:"VRRPv2,omitempty"`
	RawPacketsCaptured int64
	TrackingID         *string
	L3TrackingID       *string
	ParentUUID         *string
	NodeTID            *string
	Start              int64
	Last               int64
}

// easyjson:json
type rawpacketDoc struct {
	Class     string `json:"@class"`
	Type      string `json:"@type"`
	Flow      string
	LinkType  layers.LinkType
	Timestamp int64
	Index     int64
	Data      []byte
}

// easyjson:json
type metricDoc struct {
	Class     string `json:"@class"`
	Type      string `json:"@type"`
	Flow      string
	ABPackets int64
	ABBytes   int64
	BAPackets int64
	BABytes   int64
	Start     int64
	Last      int64
}

func flowToDoc(f *flow.Flow) *flowDoc {
	return &flowDoc{
		Class:              "Flow",
		UUID:               &f.UUID,
		LayersPath:         &f.LayersPath,
		Application:        &f.Application,
		Link:               f.Link,
		Network:            f.Network,
		Transport:          f.Transport,
		ICMP:               f.ICMP,
		Metric:             f.Metric,
		TCPMetric:          f.TCPMetric,
		IPMetric:           f.IPMetric,
		DHCPv4:             f.DHCPv4,
		DNS:                f.DNS,
		VRRPv2:             f.VRRPv2,
		TrackingID:         &f.TrackingID,
		L3TrackingID:       &f.L3TrackingID,
		ParentUUID:         &f.ParentUUID,
		NodeTID:            &f.NodeTID,
		RawPacketsCaptured: f.RawPacketsCaptured,
		Start:              f.Start,
		Last:               f.Last,
	}
}

func metricToDoc(rid string, m *flow.FlowMetric) *metricDoc {
	return &metricDoc{
		Class:     "FlowMetric",
		Type:      "d",
		Flow:      rid,
		ABBytes:   m.ABBytes,
		ABPackets: m.ABPackets,
		BABytes:   m.BABytes,
		BAPackets: m.BAPackets,
		Start:     m.Start,
		Last:      m.Last,
	}
}

func (m *metricDoc) metric() *flow.FlowMetric {
	return &flow.FlowMetric{
		ABBytes:   m.ABBytes,
		ABPackets: m.ABPackets,
		BABytes:   m.BABytes,
		BAPackets: m.BAPackets,
		Start:     m.Start,
		Last:      m.Last,
	}
}

func rawpacketToDoc(rid string, linkType layers.LinkType, r *flow.RawPacket) *rawpacketDoc {
	return &rawpacketDoc{
		Class:     "FlowRawPacket",
		Type:      "d",
		Flow:      rid,
		LinkType:  linkType,
		Index:     r.Index,
		Timestamp: r.Timestamp,
		Data:      r.Data,
	}
}

func (r *rawpacketDoc) rawpacket() *flow.RawPacket {
	return &flow.RawPacket{
		Index:     r.Index,
		Timestamp: r.Timestamp,
		Data:      r.Data,
	}
}

// StoreFlows pushes a set of flows in the database
func (c *Storage) StoreFlows(flows []*flow.Flow) error {
	// TODO: use batch of operations
	for _, flow := range flows {
		fd := flowToDoc(flow)
		raw, err := json.Marshal(fd)
		if err != nil {
			return fmt.Errorf("Error while pushing flow %s: %s", flow.UUID, err)
		}

		result, err := c.client.Upsert("Flow", json.RawMessage(raw), "UUID", flow.UUID)
		if err != nil {
			return fmt.Errorf("Error while pushing flow %s: %s", flow.UUID, err)
		}

		data := struct {
			Result []struct {
				RID string `json:"@rid"`
			}
		}{}

		if err := json.Unmarshal(result.Body, &data); err != nil {
			return fmt.Errorf("Error while decoding flow %s: %s", flow.UUID, err)
		}

		if len(data.Result) == 0 {
			return fmt.Errorf("Error while decoding flow %s: no result", flow.UUID)
		}

		if flow.LastUpdateMetric != nil {
			md := metricToDoc(data.Result[0].RID, flow.LastUpdateMetric)
			raw, err := json.Marshal(md)
			if err != nil {
				return fmt.Errorf("Error while pushing metric %s: %s", flow.UUID, err)
			}

			if _, err = c.client.CreateDocument(json.RawMessage(raw)); err != nil {
				return fmt.Errorf("Error while pushing metric %+v: %s", flow.LastUpdateMetric, err)
			}
		}

		linkType, err := flow.LinkType()
		if err != nil {
			return fmt.Errorf("Error while indexing: %s", err)
		}
		for _, r := range flow.LastRawPackets {
			rd := rawpacketToDoc(data.Result[0].RID, linkType, r)
			raw, err := json.Marshal(rd)
			if err != nil {
				return fmt.Errorf("Error while pushing raw packet %s: %s", flow.UUID, err)
			}

			if _, err = c.client.CreateDocument(json.RawMessage(raw)); err != nil {
				return fmt.Errorf("Error while pushing raw packet %+v: %s", r, err)
			}
		}
	}

	return nil
}

// SearchFlows search flow matching filters in the database
func (c *Storage) SearchFlows(fsq filters.SearchQuery) (*flow.FlowSet, error) {
	result, err := c.client.Query("Flow", &fsq)
	if err != nil {
		return nil, err
	}

	data := struct {
		Result []*flow.Flow
	}{}

	if err := json.Unmarshal(result.Body, &data); err != nil {
		logging.GetLogger().Errorf("Error while decoding flows %s, %s", err, string(result.Body))
		return nil, err
	}
	flowset := flow.NewFlowSet()
	flowset.Flows = data.Result

	if fsq.Dedup {
		if err := flowset.Dedup(fsq.DedupBy); err != nil {
			return nil, err
		}
	}

	return flowset, nil
}

// SearchRawPackets searches flow raw packets matching filters in the database
func (c *Storage) SearchRawPackets(fsq filters.SearchQuery, packetFilter *filters.Filter) (map[string]*flow.RawPackets, error) {
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
		}
	}

	if fsq.Sort {
		sql += " ORDER BY " + fsq.SortBy
		if fsq.SortOrder != "" {
			sql += " " + strings.ToUpper(fsq.SortOrder)
		}
	}

	result, err := c.client.Search(sql)
	if err != nil {
		return nil, err
	}

	data := struct {
		Result []*rawpacketDoc
	}{}

	if err := json.Unmarshal(result.Body, &data); err != nil {
		logging.GetLogger().Errorf("Error while decoding flows %s, %s", err, string(result.Body))
		return nil, err
	}

	rawpackets := make(map[string]*flow.RawPackets)
	for _, doc := range data.Result {
		r := doc.rawpacket()
		if fr, ok := rawpackets[doc.Flow]; ok {
			fr.RawPackets = append(fr.RawPackets, r)
		} else {
			rawpackets[doc.Flow] = &flow.RawPackets{
				LinkType:   doc.LinkType,
				RawPackets: []*flow.RawPacket{r},
			}
		}
	}

	return rawpackets, nil
}

// SearchMetrics searches flow metrics matching filters in the database
func (c *Storage) SearchMetrics(fsq filters.SearchQuery, metricFilter *filters.Filter) (map[string][]common.Metric, error) {
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

	result, err := c.client.Search(sql)
	if err != nil {
		return nil, err
	}

	data := struct {
		Result []*metricDoc
	}{}

	if err := json.Unmarshal(result.Body, &data); err != nil {
		logging.GetLogger().Errorf("Error while decoding flows %s, %s", err, string(result.Body))
		return nil, err
	}

	metrics := make(map[string][]common.Metric)
	for _, md := range data.Result {
		metrics[md.Flow] = append(metrics[md.Flow], md.metric())
	}

	return metrics, nil
}

// Start the database client
func (c *Storage) Start() {
}

// Stop the database client
func (c *Storage) Stop() {
}

// Close the database client
func (c *Storage) Close() {
}

// New creates a new OrientDB database client
func New(backend string) (*Storage, error) {
	path := "storage." + backend
	addr := config.GetString(path + ".addr")
	database := config.GetString(path + ".database")
	username := config.GetString(path + ".username")
	password := config.GetString(path + ".password")

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
			return nil, fmt.Errorf("Failed to register class FlowRawPacket: %s", err)
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
			return nil, fmt.Errorf("Failed to register class FlowMetric: %s", err)
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
				{Name: "ABSegmentOutOfOrder", Type: "LONG", Mandatory: false, NotNull: true},
				{Name: "ABSegmentSkipped", Type: "LONG", Mandatory: false, NotNull: true},
				{Name: "ABSegmentSkippedBytes", Type: "LONG", Mandatory: false, NotNull: true},
				{Name: "ABPackets", Type: "LONG", Mandatory: false, NotNull: true},
				{Name: "ABBytes", Type: "LONG", Mandatory: false, NotNull: true},
				{Name: "ABSawStart", Type: "LONG", Mandatory: false, NotNull: true},
				{Name: "ABSawEnd", Type: "LONG", Mandatory: false, NotNull: true},
				{Name: "BASegmentOutOfOrder", Type: "LONG", Mandatory: false, NotNull: true},
				{Name: "BASegmentSkipped", Type: "LONG", Mandatory: false, NotNull: true},
				{Name: "BASegmentSkippedBytes", Type: "LONG", Mandatory: false, NotNull: true},
				{Name: "BAPackets", Type: "LONG", Mandatory: false, NotNull: true},
				{Name: "BABytes", Type: "LONG", Mandatory: false, NotNull: true},
				{Name: "BASawStart", Type: "LONG", Mandatory: false, NotNull: true},
				{Name: "BASawEnd", Type: "LONG", Mandatory: false, NotNull: true},
			},
			Indexes: []orient.Index{
				{Name: "TCPMetric.TimeSpan", Fields: []string{"ABSynStart", "ABFinStart"}, Type: "NOTUNIQUE"},
			},
		}
		if err := client.CreateDocumentClass(class); err != nil {
			return nil, fmt.Errorf("Failed to register class FlowMetric: %s", err)
		}
	}

	if _, err := client.GetDocumentClass("IPMetric"); err != nil {
		class := orient.ClassDefinition{
			Name: "IPMetric",
			Properties: []orient.Property{
				{Name: "Fragments", Type: "LONG", Mandatory: false, NotNull: true},
				{Name: "FragmentErrors", Type: "LONG", Mandatory: false, NotNull: true},
			},
		}
		if err := client.CreateDocumentClass(class); err != nil {
			return nil, fmt.Errorf("Failed to register class IPMetric: %s", err)
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
				{Name: "IPMetric", Type: "EMBEDDED", LinkedClass: "IPMetric"},
				{Name: "TCPMetric", Type: "EMBEDDED", LinkedClass: "TCPMetric"},
				{Name: "Start", Type: "LONG"},
				{Name: "Last", Type: "LONG"},
				{Name: "TrackingID", Type: "STRING", Mandatory: true, NotNull: true},
				{Name: "L3TrackingID", Type: "STRING"},
				{Name: "ParentUUID", Type: "STRING"},
				{Name: "NodeTID", Type: "STRING"},
				{Name: "RawPacketsCaptured", Type: "LONG"},
			},
			Indexes: []orient.Index{
				{Name: "Flow.UUID", Fields: []string{"UUID"}, Type: "UNIQUE"},
				{Name: "Flow.TrackingID", Fields: []string{"TrackingID"}, Type: "NOTUNIQUE"},
				{Name: "Flow.TimeSpan", Fields: []string{"Start", "Last"}, Type: "NOTUNIQUE"},
			},
		}
		if err := client.CreateDocumentClass(class); err != nil {
			return nil, fmt.Errorf("Failed to register class Flow: %s", err)
		}
	}

	flowProp := orient.Property{Name: "Flow", Type: "LINK", LinkedClass: "Flow", Mandatory: false, NotNull: true}

	client.CreateProperty("FlowMetric", flowProp)
	flowIndex := orient.Index{Name: "FlowMetric.Flow", Fields: []string{"Flow"}, Type: "NOTUNIQUE"}
	client.CreateIndex("FlowMetric", flowIndex)

	client.CreateProperty("TCPMetric", flowProp)
	tcpMetricFlowIndex := orient.Index{Name: "TCPMetric.Flow", Fields: []string{"Flow"}, Type: "NOTUNIQUE"}
	client.CreateIndex("TCPMetric", tcpMetricFlowIndex)

	client.CreateProperty("IPMetric", flowProp)
	ipMetricFlowIndex := orient.Index{Name: "IPMetric.Flow", Fields: []string{"Flow"}, Type: "NOTUNIQUE"}
	client.CreateIndex("IPMetric", ipMetricFlowIndex)

	return &Storage{
		client: client,
	}, nil
}
