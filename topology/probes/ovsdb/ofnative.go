/*
 * Copyright (C) 2018 Red Hat, Inc.
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

package ovsdb

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/avast/retry-go"
	"github.com/cnf/structhash"
	goloxi "github.com/skydive-project/goloxi"
	"github.com/skydive-project/goloxi/of14"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/openflow"
	"github.com/skydive-project/skydive/ovs/monitor"
	"github.com/skydive-project/skydive/topology"
	tp "github.com/skydive-project/skydive/topology/probes"
)

type ofHandler interface {
	openflow.Protocol
	OnMessage(msg goloxi.Message)
	NewGroupForwardRequest() (goloxi.Message, error)
	NewRoleRequest() (goloxi.Message, error)
	NewFlowStatsRequest(match Match) (goloxi.Message, error)
	NewGroupDescStatsRequest() (goloxi.Message, error)
}

type ofProbe struct {
	sync.Mutex
	Ctx              tp.Context
	bridge           string
	address          string
	tlsConfig        *tls.Config
	handler          ofHandler
	client           *openflow.Client
	monitor          *monitor.Monitor
	ctx              context.Context
	cancel           context.CancelFunc
	lastUpdateMetric time.Time
	rules            map[graph.Identifier]graph.Identifier
	requests         map[uint32]*ofRule
}

type ofFilter struct {
	Type      string
	Value     interface{}
	ValueMask interface{} `json:",omitempty"`
}

type ofAction struct {
	Type      string
	Arguments map[string]interface{}
}

func newOfAction(action goloxi.IAction) *ofAction {
	return &ofAction{
		Type:      action.GetActionName(),
		Arguments: action.GetActionFields(),
	}
}

type ofEnum interface {
	MarshalJSON() ([]byte, error)
}

type ofRule struct {
	monitorID    graph.Identifier
	Cookie       int64
	Table        int64
	Priority     int64
	IdleTimeout  int64
	HardTimeout  int64
	Importance   int64
	Flags        ofEnum
	Filters      []*ofFilter
	Actions      []*ofAction
	WriteActions []*ofAction
	GotoTable    int64
}

type ofBucket struct {
	Len        int64
	Weight     int64
	WatchPort  int64
	WatchGroup int64
	Actions    []*ofAction
}

func newOfBucket(weight, watchPort, watchGroup int64, actions []goloxi.IAction) *ofBucket {
	bucket := &ofBucket{
		WatchGroup: watchGroup,
		WatchPort:  watchPort,
		Weight:     int64(weight),
		Actions:    make([]*ofAction, len(actions)),
	}
	for i, action := range actions {
		bucket.Actions[i] = newOfAction(action)
	}
	return bucket
}

func (r *ofRule) GetMetadata() graph.Metadata {
	metadata := graph.Metadata{
		"Type":        "ofrule",
		"Cookie":      fmt.Sprintf("0x%x", r.Cookie),
		"Table":       r.Table,
		"Priority":    r.Priority,
		"IdleTimeout": r.IdleTimeout,
		"HardTimeout": r.HardTimeout,
		"Importance":  r.Importance,
		"Flags":       r.Flags,
	}

	normalizeValue := func(value interface{}) interface{} {
		switch t := value.(type) {
		case net.HardwareAddr:
			return t.String()
		case net.IP:
			return t.String()
		case uint64:
			return int64(t)
		case uint32:
			return int64(t)
		case uint16:
			return int64(t)
		case uint8:
			return int64(t)
		default:
			if s, ok := t.(fmt.Stringer); ok {
				return s.String()
			}
			return t
		}
	}

	filters := make([]interface{}, len(r.Filters))
	for i, filter := range r.Filters {
		f := map[string]interface{}{
			"Type":  filter.Type,
			"Value": normalizeValue(filter.Value),
		}
		if filter.ValueMask != nil {
			f["Mask"] = normalizeValue(filter.ValueMask)
		}
		filters[i] = f
	}
	metadata["Filters"] = filters

	actions := make([]interface{}, len(r.Actions))
	for i, action := range r.Actions {
		m := map[string]interface{}{
			"Type": action.Type,
		}
		if len(action.Arguments) > 0 {
			m["Arguments"] = action.Arguments
		}
		actions[i] = m
	}
	metadata["Actions"] = actions

	actions = make([]interface{}, len(r.WriteActions))
	for i, action := range r.WriteActions {
		m := map[string]interface{}{
			"Type": action.Type,
		}
		if len(action.Arguments) > 0 {
			m["Arguments"] = action.Arguments
		}
		actions[i] = m
	}
	metadata["WriteActions"] = actions

	if r.GotoTable != 0 {
		metadata["GotoTable"] = r.GotoTable
	}

	return metadata
}

func (r *ofRule) GetID(host, bridge string) graph.Identifier {
	hash, _ := structhash.Hash(struct {
		Host     string
		Bridge   string
		Cookie   int64
		Priority int64
		Table    int64
		Filters  []*ofFilter
	}{
		Host:     host,
		Bridge:   bridge,
		Cookie:   r.Cookie,
		Priority: r.Priority,
		Table:    r.Table,
		Filters:  r.Filters,
	}, 1)
	return graph.Identifier(hash)
}

func newOfRule(cookie uint64, table uint8, priority, idleTimeout, hardTimeout, importance uint16, flags ofEnum, match of14.IMatchV3, actions []goloxi.IAction, writeActions []goloxi.IAction, gotoTable uint8) (*ofRule, error) {
	rule := &ofRule{
		Cookie:       int64(cookie),
		Table:        int64(table),
		Priority:     int64(priority),
		IdleTimeout:  int64(idleTimeout),
		HardTimeout:  int64(hardTimeout),
		Importance:   int64(importance),
		Flags:        flags,
		Filters:      make([]*ofFilter, len(match.GetOxmList())),
		Actions:      make([]*ofAction, len(actions)),
		WriteActions: make([]*ofAction, len(writeActions)),
		GotoTable:    int64(gotoTable),
	}

	for i, entry := range match.GetOxmList() {
		rule.Filters[i] = &ofFilter{
			Type:  entry.GetOXMName(),
			Value: entry.GetOXMValue(),
		}
		if oxmMasked, ok := entry.(goloxi.IOxmMasked); ok {
			rule.Filters[i].ValueMask = oxmMasked.GetOXMValueMask()
		}
	}

	for i, action := range actions {
		rule.Actions[i] = newOfAction(action)
	}

	for i, action := range writeActions {
		rule.WriteActions[i] = newOfAction(action)
	}

	if len(actions) == 0 && len(writeActions) == 0 && gotoTable == 0 {
		rule.Actions = append(rule.Actions, &ofAction{Type: "drop"})
	}

	return rule, nil
}

type ofGroup struct {
	GroupType ofEnum
	ID        int64
	Buckets   []*ofBucket
}

func (g *ofGroup) GetMetadata() graph.Metadata {
	metadata := graph.Metadata{
		"Type":      "ofgroup",
		"GroupType": g.GroupType,
		"GroupId":   g.ID,
		"Buckets":   g.Buckets,
	}

	return metadata
}

func (probe *ofProbe) sendFlowStatsRequest(match Match) error {
	msg, err := probe.handler.NewFlowStatsRequest(match)
	if err != nil {
		return err
	}

	return probe.client.SendMessage(msg)
}

// sendGroupStatsDescRequest sends a group stats request, similar to ovs-ofctl dump-groups
func (probe *ofProbe) sendGroupStatsDescRequest() error {
	msg, err := probe.handler.NewGroupDescStatsRequest()
	if err != nil {
		return err
	}

	return probe.client.SendMessage(msg)
}

func (probe *ofProbe) Monitor(ctx context.Context) (err error) {
	probe.monitor, err = monitor.NewMonitor(probe.address, probe.tlsConfig)
	if err != nil {
		return err
	}

	versions := probe.Ctx.Config.GetStringSlice("ovs.oflow.openflow_versions")
	sort.Strings(versions)

	var protocols []openflow.Protocol
	for _, version := range versions {
		switch version {
		case "OpenFlow10":
			protocols = append(protocols, openflow.OpenFlow10)
		case "OpenFlow11":
			protocols = append(protocols, openflow.OpenFlow11)
		case "OpenFlow12":
			protocols = append(protocols, openflow.OpenFlow12)
		case "OpenFlow13":
			protocols = append(protocols, openflow.OpenFlow13)
		case "OpenFlow14":
			protocols = append(protocols, openflow.OpenFlow14)
		case "OpenFlow15":
			protocols = append(protocols, openflow.OpenFlow15)
		default:
			return fmt.Errorf("Unsupported version '%s", version)
		}
	}

	probe.client, err = openflow.NewClient(probe.address, probe.tlsConfig, protocols)
	if err != nil {
		return err
	}

	probe.lastUpdateMetric = time.Now().UTC()

	probe.monitor.RegisterListener(&of10Handler{probe: probe})

	if err = retry.Do(func() error { return probe.client.Start(ctx) }, retry.Attempts(8)); err != nil {
		return err
	}

	ofVersion := probe.client.GetProtocol().GetVersion()
	switch ofVersion {
	case goloxi.VERSION_1_2:
		probe.handler = &of12Handler{probe: probe}
	case goloxi.VERSION_1_3:
		probe.handler = &of13Handler{probe: probe}
	case goloxi.VERSION_1_4:
		probe.handler = &of14Handler{probe: probe}
	case goloxi.VERSION_1_5:
		probe.handler = &of15Handler{probe: probe}
	default:
		return fmt.Errorf("Unsupported version %d for bridge %s", ofVersion, probe.address)
	}
	probe.client.RegisterListener(probe.handler)

	if err = retry.Do(func() error { return probe.monitor.Start(ctx) }, retry.Attempts(8)); err != nil {
		return err
	}

	go func() {
		probe.queryStats(ctx)
	}()

	return nil
}

func (probe *ofProbe) queryStats(ctx context.Context) {
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	defer cancelFunc()

	probe.sendGroupStatsDescRequest()

	timer := time.NewTicker(time.Second * 10)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			probe.sendFlowStatsRequest(nil)
			probe.sendGroupStatsDescRequest()
		case <-cancelCtx.Done():
			return
		}
	}
}

func (probe *ofProbe) MonitorGroup() error {
	if probe.handler.GetVersion() < goloxi.VERSION_1_5 {
		return ErrGroupNotSupported
	}

	msg, err := probe.handler.NewRoleRequest()
	if err != nil {
		return err
	}

	if err := probe.client.SendMessage(msg); err != nil {
		return err
	}

	if err := probe.client.SendMessage(probe.handler.NewBarrierRequest()); err != nil {
		return err
	}

	msg, err = probe.handler.NewGroupForwardRequest()
	if err != nil {
		return err
	}

	if err := probe.client.SendMessage(msg); err != nil {
		return err
	}

	return probe.sendGroupStatsDescRequest()
}

func (probe *ofProbe) Cancel() {
	if probe.cancel != nil {
		probe.cancel()
	}
}

func (probe *ofProbe) syncNode(id graph.Identifier, m graph.Metadata, delete bool) *graph.Node {
	n := probe.Ctx.Graph.GetNode(id)
	if delete {
		if n != nil {
			if err := probe.Ctx.Graph.DelNode(n); err != nil {
				probe.Ctx.Logger.Error(err)
			}
		}
	} else {
		if n != nil {
			tr := probe.Ctx.Graph.StartMetadataTransaction(n)
			for k, v := range m {
				tr.AddMetadata(k, v)
			}
			tr.Commit()
		} else {
			m["Metric"] = &topology.InterfaceMetric{}
			newNode, err := probe.Ctx.Graph.NewNode(id, m)
			if err != nil {
				probe.Ctx.Logger.Error(err)
				return nil
			}
			n = newNode
			topology.AddOwnershipLink(probe.Ctx.Graph, probe.Ctx.RootNode, n, nil)
		}
	}
	return n
}

func (probe *ofProbe) handleGroup(group *ofGroup, delete bool) *graph.Node {
	id := graph.GenID(fmt.Sprintf("%d", group.ID), probe.bridge)
	return probe.syncNode(id, group.GetMetadata(), delete)
}

func (probe *ofProbe) handleFlowRule(rule *ofRule, delete bool) *graph.Node {
	id := rule.GetID(probe.Ctx.Graph.GetHost(), probe.bridge)
	return probe.syncNode(id, rule.GetMetadata(), delete)
}

// Match is the interface of an OpenFlow match
type Match interface {
	of14.IMatchV3
}

// Stats holds the statistics of an OpenFlow rule
type Stats struct {
	PacketCount int64
	ByteCount   int64
	FlowCount   int64
	Duration    time.Duration
	IdleTime    time.Duration
}

func (probe *ofProbe) handleFlowStats(xid uint32, rule *ofRule, actions, writeActions []goloxi.IAction, stats Stats, now, last time.Time) {
	probe.Lock()
	if requestedRule, ok := probe.requests[xid]; ok && requestedRule.Priority == rule.Priority && len(requestedRule.Filters) == len(rule.Filters) {
		monitorID := requestedRule.GetID(probe.Ctx.Graph.GetHost(), probe.bridge)
		probe.rules[monitorID] = rule.GetID(probe.Ctx.Graph.GetHost(), probe.bridge)
		delete(probe.requests, xid)
	}
	probe.Unlock()

	probe.Ctx.Graph.Lock()

	node := probe.handleFlowRule(rule, false)

	currMetric := &topology.InterfaceMetric{
		RxPackets: stats.PacketCount,
		RxBytes:   stats.ByteCount,
	}

	tr := probe.Ctx.Graph.StartMetadataTransaction(node)

	var lastUpdateMetric *topology.InterfaceMetric
	prevMetric, err := node.GetField("Metric")
	if err == nil {
		lastUpdateMetric = currMetric.Sub(prevMetric.(*topology.InterfaceMetric)).(*topology.InterfaceMetric)
	}

	// nothing changed since last update
	if lastUpdateMetric != nil && lastUpdateMetric.IsZero() {
		probe.Ctx.Graph.Unlock()
		return
	}

	tr.AddMetadata("Metric", currMetric)
	if lastUpdateMetric != nil {
		lastUpdateMetric.Start = int64(common.UnixMillis(last))
		lastUpdateMetric.Last = int64(common.UnixMillis(now))
		tr.AddMetadata("LastUpdateMetric", lastUpdateMetric)
	}
	tr.Commit()

	probe.Ctx.Graph.Unlock()
}

// NewOfProbe returns a new OpenFlow natively speaking probe
func NewOfProbe(ctx tp.Context, bridge string, address string, tlsConfig *tls.Config) BridgeOfProber {
	return &ofProbe{
		Ctx:       ctx,
		address:   address,
		bridge:    bridge,
		tlsConfig: tlsConfig,
		rules:     make(map[graph.Identifier]graph.Identifier),
		requests:  make(map[uint32]*ofRule),
	}
}
