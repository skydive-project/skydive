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
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/cnf/structhash"
	"github.com/davecgh/go-spew/spew"
	goloxi "github.com/skydive-project/goloxi"
	"github.com/skydive-project/goloxi/of10"
	"github.com/skydive-project/goloxi/of14"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/openflow"
	"github.com/skydive-project/skydive/ovs/monitor"
	"github.com/skydive-project/skydive/topology"
)

type ofProbe struct {
	sync.Mutex
	bridge           string
	address          string
	g                *graph.Graph
	node             *graph.Node
	client           *openflow.Client
	monitor          *monitor.Monitor
	ctx              context.Context
	cancel           context.CancelFunc
	lastUpdateMetric time.Time
	debouncer        *common.Debouncer
	rules            map[graph.Identifier]graph.Identifier
	requests         map[uint32]*ofRule
}

type ofFilter struct {
	Type  string
	Value interface{}
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

type ofRule struct {
	monitorID    graph.Identifier
	Cookie       int64
	Table        int64
	Priority     int64
	IdleTimeout  int64
	HardTimeout  int64
	Importance   int64
	Flags        of14.FlowModFlags
	Filters      []*ofFilter
	Actions      []*ofAction
	WriteActions []*ofAction
}

type ofBucket struct {
	Len        int64
	Weight     int64
	WatchPort  of14.Port
	WatchGroup of14.Group
	Actions    []*ofAction
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

	filters := make([]interface{}, len(r.Filters))
	for i, filter := range r.Filters {
		var value interface{}
		switch t := filter.Value.(type) {
		case net.HardwareAddr:
			value = t.String()
		case net.IP:
			value = t.String()
		case uint64:
			value = int64(t)
		case uint32:
			value = int64(t)
		case uint16:
			value = int64(t)
		case uint8:
			value = int64(t)
		default:
			if s, ok := t.(fmt.Stringer); ok {
				value = s.String()
			} else {
				value = t
			}
		}
		filters[i] = map[string]interface{}{
			"Type":  filter.Type,
			"Value": value,
		}
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

func newOfRule(cookie uint64, table uint8, priority, idleTimeout, hardTimeout, importance uint16, flags of14.FlowModFlags, match *of14.Match, actions []goloxi.IAction, writeActions []goloxi.IAction) (*ofRule, error) {
	rule := &ofRule{
		Cookie:       int64(cookie),
		Table:        int64(table),
		Priority:     int64(priority),
		IdleTimeout:  int64(idleTimeout),
		HardTimeout:  int64(hardTimeout),
		Importance:   int64(importance),
		Flags:        flags,
		Filters:      make([]*ofFilter, len(match.OxmList)),
		Actions:      make([]*ofAction, len(actions)),
		WriteActions: make([]*ofAction, len(writeActions)),
	}

	for i, entry := range match.OxmList {
		oxm, ok := entry.(of14.IOxm)
		if !ok {
			return nil, fmt.Errorf("Invalid match entry %+v", oxm)
		}

		rule.Filters[i] = &ofFilter{
			Type:  oxm.GetOXMName(),
			Value: oxm.GetOXMValue(),
		}
	}

	for i, action := range actions {
		rule.Actions[i] = newOfAction(action)
	}

	for i, action := range writeActions {
		rule.WriteActions[i] = newOfAction(action)
	}

	if len(actions) == 0 {
		rule.Actions = append(rule.Actions, &ofAction{Type: "drop"})
	}

	return rule, nil
}

type ofGroup struct {
	GroupType of14.GroupType
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

func newOfGroup(groupType of14.GroupType, id int64, buckets []*of14.Bucket) *ofGroup {
	ofGroup := &ofGroup{
		GroupType: groupType,
		ID:        id,
	}
	for _, bucket := range buckets {
		ofBucket := &ofBucket{
			WatchGroup: of14.Group(bucket.WatchGroup),
			WatchPort:  bucket.WatchPort,
			Weight:     int64(bucket.Weight),
			Actions:    make([]*ofAction, len(bucket.Actions)),
		}
		for i, action := range bucket.Actions {
			ofBucket.Actions[i] = newOfAction(action)
		}
		ofGroup.Buckets = append(ofGroup.Buckets, ofBucket)
	}
	return ofGroup
}

func (probe *ofProbe) sendRoleRequest() error {
	request := of14.NewRoleRequest()
	request.Role = of14.OFPCRRoleSlave
	request.GenerationId = 0x8000000000000002
	return probe.client.SendMessage(request)
}

func (probe *ofProbe) requestGroupForward() error {
	request := of14.NewAsyncSet()
	prop := of14.NewAsyncConfigPropRequestforwardSlave()
	prop.Mask = 0x1
	prop.Length = 8
	request.Properties = append(request.Properties, prop)
	return probe.client.SendMessage(request)
}

// sendFlowStatsRequest creates a flow stats request, similar to ovs-ofctl dump-flows
func (probe *ofProbe) newFlowStatsRequest(match *of14.Match) goloxi.Message {
	request := of14.NewFlowStatsRequest()
	request.TableId = of14.OFPTTAll
	request.OutPort = of14.OFPPAny
	request.OutGroup = of14.OFPGAny
	if match == nil {
		match = of14.NewMatchV3()
		match.Length = 4
		match.Type = of14.OFPMTOXM
	}
	request.Match = *match
	return request
}

// SendGroupStatsDescRequest sends a group stats request, similar to ovs-ofctl dump-groups
func (probe *ofProbe) sendGroupStatsDescRequest() error {
	return probe.client.SendMessage(of14.NewGroupDescStatsRequest())
}

func (probe *ofProbe) Monitor(ctx context.Context) (err error) {
	probe.monitor, err = monitor.NewMonitor(probe.address)
	if err != nil {
		return err
	}

	probe.client, err = openflow.NewClient(probe.address, openflow.OpenFlow14)
	if err != nil {
		return err
	}

	probe.lastUpdateMetric = time.Now().UTC()
	probe.monitor.RegisterListener(probe)
	probe.client.RegisterListener(probe)

	if err = common.Retry(func() error { return probe.client.Start(ctx) }, 5, time.Second); err != nil {
		return err
	}

	if err = common.Retry(func() error { return probe.monitor.Start(ctx) }, 5, time.Second); err != nil {
		return err
	}

	go func() {
		probe.queryStats(ctx)
	}()

	return nil
}

func (probe *ofProbe) queryStats(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	timer := time.NewTicker(time.Second * 10)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			probe.client.SendMessage(probe.newFlowStatsRequest(nil))
			probe.sendGroupStatsDescRequest()
		case <-ctx.Done():
			return
		}
	}
}

func (probe *ofProbe) MonitorGroup() error {
	if err := probe.sendRoleRequest(); err != nil {
		return err
	}

	if err := probe.client.SendMessage(of14.NewBarrierRequest()); err != nil {
		return err
	}

	if err := probe.requestGroupForward(); err != nil {
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
	n := probe.g.GetNode(id)
	if delete {
		if n != nil {
			if err := probe.g.DelNode(n); err != nil {
				logging.GetLogger().Error(err)
			}
		}
	} else {
		if n != nil {
			tr := probe.g.StartMetadataTransaction(n)
			for k, v := range m {
				tr.AddMetadata(k, v)
			}
			tr.Commit()
		} else {
			m["Metric"] = &topology.InterfaceMetric{}
			newNode, err := probe.g.NewNode(id, m)
			if err != nil {
				logging.GetLogger().Error(err)
				return nil
			}
			n = newNode
			topology.AddOwnershipLink(probe.g, probe.node, n, nil)
		}
	}
	return n
}

func (probe *ofProbe) handleGroup(group *ofGroup, delete bool) *graph.Node {
	id := graph.GenID(fmt.Sprintf("%d", group.ID), probe.bridge)
	return probe.syncNode(id, group.GetMetadata(), delete)
}

func (probe *ofProbe) handleFlowRule(rule *ofRule, delete bool) *graph.Node {
	id := rule.GetID(probe.g.GetHost(), probe.bridge)
	return probe.syncNode(id, rule.GetMetadata(), delete)
}

func (probe *ofProbe) handleRequestForward(request of14.IHeader) {
	probe.g.Lock()
	defer probe.g.Unlock()

	switch group := request.(type) {
	case *of14.GroupAdd:
		probe.handleGroup(newOfGroup(group.GroupType, int64(group.GroupId), group.Buckets), false)
	case *of14.GroupModify:
		probe.handleGroup(newOfGroup(group.GroupType, int64(group.GroupId), group.Buckets), false)
	case *of14.GroupDelete:
		if group.GroupId == of14.OFPGAll {
			for _, children := range probe.g.LookupChildren(probe.node, graph.Metadata{"Type": "ofgroup"}, nil) {
				probe.g.DelNode(children)
			}
		} else {
			probe.handleGroup(newOfGroup(group.GroupType, int64(group.GroupId), group.Buckets), true)
		}
	}
}

func (probe *ofProbe) handleFlowStats(xid uint32, flowStats []*of14.FlowStatsEntry, now, last time.Time) {
	for _, stats := range flowStats {
		var actions, writeActions []goloxi.IAction
		for _, instruction := range stats.Instructions {
			if instruction, ok := instruction.(*of14.InstructionApplyActions); ok {
				actions = append(actions, instruction.Actions...)
			}
			if instruction, ok := instruction.(*of14.InstructionWriteActions); ok {
				writeActions = append(writeActions, instruction.Actions...)
			}
		}

		rule, err := newOfRule(stats.Cookie, stats.TableId, stats.Priority, stats.IdleTimeout, stats.HardTimeout, stats.Importance, stats.Flags, &stats.Match, actions, writeActions)
		if err != nil {
			logging.GetLogger().Error(err)
			continue
		}

		probe.Lock()
		if requestedRule, ok := probe.requests[xid]; ok && requestedRule.Priority == rule.Priority && len(requestedRule.Filters) == len(rule.Filters) {
			monitorID := requestedRule.GetID(probe.g.GetHost(), probe.bridge)
			probe.rules[monitorID] = rule.GetID(probe.g.GetHost(), probe.bridge)
			delete(probe.requests, xid)
		}
		probe.Unlock()

		probe.g.Lock()

		node := probe.handleFlowRule(rule, false)

		currMetric := &topology.InterfaceMetric{
			RxPackets: int64(stats.PacketCount),
			RxBytes:   int64(stats.ByteCount),
		}

		tr := probe.g.StartMetadataTransaction(node)

		var lastUpdateMetric *topology.InterfaceMetric
		prevMetric, err := node.GetField("Metric")
		if err == nil {
			lastUpdateMetric = currMetric.Sub(prevMetric.(*topology.InterfaceMetric)).(*topology.InterfaceMetric)
		}

		// nothing changed since last update
		if lastUpdateMetric != nil && lastUpdateMetric.IsZero() {
			probe.g.Unlock()
			continue
		}

		tr.AddMetadata("Metric", currMetric)
		if lastUpdateMetric != nil {
			lastUpdateMetric.Start = int64(common.UnixMillis(last))
			lastUpdateMetric.Last = int64(common.UnixMillis(now))
			tr.AddMetadata("LastUpdateMetric", lastUpdateMetric)
		}
		tr.Commit()

		probe.g.Unlock()
	}
}

func (probe *ofProbe) handleFlowMonitorReply(reply *of10.NiciraFlowMonitorReply) {
	logging.GetLogger().Debugf("Handling flow monitor %s", spew.Sdump(reply))
	nxm2oxm := func(nxm of14.INiciraMatch, matchLen uint16) *of14.MatchV3 {
		oxm := of14.NewMatchV3()
		for _, e := range nxm.GetNxmEntries() {
			oxm.OxmList = append(oxm.OxmList, e)
		}
		oxm.Length = matchLen + 4
		oxm.Type = of14.OFPMTOXM
		return oxm
	}

	for _, update := range reply.Updates {
		switch u := update.(type) {
		case *of10.NiciraFlowUpdateFullAdd:
			rule, err := newOfRule(u.Cookie, u.TableId, u.Priority, u.IdleTimeout, u.HardTimeout, 0, 0, nxm2oxm(&u.Match, u.MatchLen), u.Actions, nil)
			if err != nil {
				logging.GetLogger().Errorf("Failed to parse update: %s", err)
				continue
			}
			msg := probe.newFlowStatsRequest(nxm2oxm(&u.Match, u.MatchLen))
			probe.client.PrepareMessage(msg)
			probe.Lock()
			probe.requests[msg.GetXid()] = rule
			probe.Unlock()
			probe.client.SendMessage(msg)
		case *of10.NiciraFlowUpdateFullDeleted:
			monitorRule, err := newOfRule(u.Cookie, u.TableId, u.Priority, u.IdleTimeout, u.HardTimeout, 0, 0, nxm2oxm(&u.Match, u.MatchLen), u.Actions, nil)
			if err != nil {
				logging.GetLogger().Errorf("Failed to parse update: %s", err)
				continue
			}
			monitorID := monitorRule.GetID(probe.g.GetHost(), probe.bridge)
			probe.Lock()
			ruleID := probe.rules[monitorID]
			delete(probe.rules, monitorID)
			probe.Unlock()

			probe.g.Lock()
			if n := probe.g.GetNode(ruleID); n != nil {
				probe.g.DelNode(n)
			}
			probe.g.Unlock()
		}
	}
}

func (probe *ofProbe) OnMessage(msg goloxi.Message) {
	switch t := msg.(type) {
	case *of10.NiciraFlowMonitorReply: // Received on connection and on events
		probe.handleFlowMonitorReply(t)
	case *of14.FlowStatsReply: // Received with ticker and in response to requests
		now := time.Now().UTC()
		probe.handleFlowStats(t.GetXid(), t.Entries, now, probe.lastUpdateMetric)
		probe.lastUpdateMetric = now
	case *of14.Requestforward:
		probe.handleRequestForward(t.Request)
	case *of14.GroupDescStatsReply: // Received on initial sync
		probe.g.Lock()
		defer probe.g.Unlock()

		for _, group := range t.Entries {
			probe.handleGroup(newOfGroup(group.GroupType, int64(group.GroupId), group.Buckets), false)
		}
	}
}

// NewOfProbe returns a new OpenFlow natively speaking probe
func NewOfProbe(bridge string, address string, g *graph.Graph, node *graph.Node) BridgeOfProber {
	return &ofProbe{
		address:  address,
		g:        g,
		node:     node,
		bridge:   bridge,
		rules:    make(map[graph.Identifier]graph.Identifier),
		requests: make(map[uint32]*ofRule),
	}
}
