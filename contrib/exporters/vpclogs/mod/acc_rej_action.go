/*
 * Copyright (C) 2019 IBM, Inc.
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

/*
 * This code is to support the "action" feature of aws in the vpc flow logs.
 * For each flow, specify whether it was Accepted or Rejected.
 * The assumption is that we have 2 points where the flows are captured: one on each side of a firewall.
 * If a flow appears on only one side of the firewall, we declare it as Rejected.
 * If the flow appears on both sides of the firewall, we declare it as Accepted.
 * When a flow is blocked mid-stream, we need to accurately report the number of bytes and packets that were Accepted.
 *
 * The algorith to implement the feature is as follows.
 * For each accepted flow, we should see the flow logs at both points of capture.
 * For each rejected flow, we should see the flow logs at only one of the points of capture.
 * For each round of reported flows, match up the pairs of accepted flows using their common Tracking ID.
 * Measurements on the 2 interfaces may happen independently of one another.
 * Since a flow may change from Accepted to Rejected mid-stream, we should be careful to report as Accepted 
 * only those bytes and packets that have already been confirmed at both capture points.
 * We therefore report the lesser of the numbers (bytes, packets, time) recorded in the 2 measurements of the same flow.
 */

package mod

import (
	"fmt"
	"time"

	"github.com/spf13/viper"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/contrib/exporters/core"
)

type flowInfoStatus int32

const (
	flowInfoNoState     flowInfoStatus = 0
	flowInfoOneEntry    flowInfoStatus = 1 // startup state
	flowInfoTwoEntries  flowInfoStatus = 2 // startup state
	flowInfoOldFlow     flowInfoStatus = 3 // standard state
	flowInfoOneEntryDup flowInfoStatus = 4 // error state
	flowInfoThreeFlows  flowInfoStatus = 5 // error state
	flowInfoTimedOut    flowInfoStatus = 6 // error state
	flowInfoRejected    flowInfoStatus = 7 // error state
)

// time for cleanup of old connections
const timeoutPeriodCleanup = 20

type singleFlowInfo struct {
	UUID string
	flow *VpclogsFlow // most recent Flow seen for this UUID
	saw  bool         // true if this UUID was seen in most recent interation
}

type combinedFlowInfo struct {
	status         flowInfoStatus
	prevStatus     flowInfoStatus
	metricReported *flow.FlowMetric
	flow1          singleFlowInfo
	flow2          singleFlowInfo
}

type Action struct {
	// table of saved flows
	state             map[string]*combinedFlowInfo
	timeOfLastCleanup int64
	tid1              string
	tid2              string
}

// NewAction - creates data structures needed for vpc Accept/Reject based on measurements flows at 2 points
func NewAction(cfg *viper.Viper) (interface{}, error) {
	state := make(map[string]*combinedFlowInfo)
	tid1 := cfg.GetString(core.CfgRoot + "action.tid1")
	if tid1 == "" {
		logging.GetLogger().Errorf("tid1 not defined")
		return nil, fmt.Errorf("Failed to detect tid1")
	}
	tid2 := cfg.GetString(core.CfgRoot + "action.tid2")
	if tid2 == "" {
		logging.GetLogger().Errorf("tid2 not defined")
		return nil, fmt.Errorf("Failed to detect tid2")
	}
	logging.GetLogger().Infof("tid1 = %s", tid1)
	logging.GetLogger().Infof("tid2 = %s", tid2)
	now := time.Now()
	secs := now.Unix()
	accRej := &Action{
		state:             state,
		timeOfLastCleanup: secs,
		tid1:              tid1,
		tid2:              tid2,
	}
	return accRej, nil
}

// init - initialize internal data structure for this round
func (t *Action) init() {
	// reset the entries in the table for this round
	for _, ars := range t.state {
		ars.flow1.saw = false
		ars.flow2.saw = false
		ars.prevStatus = ars.status
		if ars.status == flowInfoTwoEntries {
			ars.status = flowInfoOldFlow
		}
	}
}

// updateInfo - go over all the flows and fill in our internal data structure with relevant info
func (t *Action) updateInfo(fl interface{}) {
	fl1 := fl.([]interface{})
	for _, i := range fl1 {
		f := i.(*VpclogsFlow)
		// only consider flows that match the specified TIDs
		if f.TID != t.tid1  && f.TID != t.tid2 {
			logging.GetLogger().Errorf("TIDs don't match: tid1 = %s, tid2 = %s, flowInfo = %s", t.tid1, t.tid2, f)
			return
		}
		TrackingID := f.TrackingID
		flowInfState, ok := t.state[TrackingID]
		newFlowInfo := singleFlowInfo {
			flow: f,
			UUID: f.UUID,
			saw:  true,
		}
		if !ok {
			// this flow is not yet in our table
			t.state[TrackingID] = &combinedFlowInfo{
				status: flowInfoOneEntry,
				flow1:  newFlowInfo,
			}
			continue
		} else if flowInfState.status == flowInfoOneEntry {
			if flowInfState.flow1.UUID != f.UUID {
				flowInfState.flow2 = newFlowInfo
				flowInfState.status = flowInfoTwoEntries
			} else {
				// got duplicate from same flow; need to indicate reject
				// or accumulate the stats until next time
				flowInfState.status = flowInfoOneEntryDup
				if f.Last > flowInfState.flow1.flow.Last {
					flowInfState.flow1.flow = f
				}
				flowInfState.flow1.saw = true
			}
		} else { // normal case; we have info from 2 flows
			if flowInfState.flow1.UUID == f.UUID {
				if f.Last > flowInfState.flow1.flow.Last {
					flowInfState.flow1.flow = f
				}
				flowInfState.flow1.saw = true
			} else if flowInfState.flow2.UUID == f.UUID {
				if f.Last > flowInfState.flow2.flow.Last {
					flowInfState.flow2.flow = f
				}
				flowInfState.flow2.saw = true
			} else {
				// this is a third flow with the same TrackingID;
				logging.GetLogger().Errorf("third flow with same tracking id; f = %s, flow1 = %s, flow2 = %s", f, flowInfState.flow1, flowInfState.flow2)
				flowInfState.status = flowInfoThreeFlows
			}
		}
	}
}

func prepareCombinedMetric(M1, M2 *flow.FlowMetric) *flow.FlowMetric {
	// take byte and packet count as minimum of the 2 reports
	return &flow.FlowMetric{
		ABPackets: common.MinInt64(M1.ABPackets, M2.ABPackets),
		BAPackets: common.MinInt64(M1.BAPackets, M2.BAPackets),
		ABBytes:   common.MinInt64(M1.ABBytes, M2.ABBytes),
		BABytes:   common.MinInt64(M1.BABytes, M2.BABytes),
		Last:      common.MinInt64(M1.Last, M2.Last),
		Start:     M1.Start,
	}
}

// combineFlows - helper function to combine 2 seen flows into one, based on common minimums
func (t *Action) combineFlows(f *combinedFlowInfo) *VpclogsFlow {
	if f.status == flowInfoThreeFlows { // error status; return no combined flow
		return nil
	}
	logging.GetLogger().Debugf("combineFlows: flow1 = %s", f.flow1.flow)
	logging.GetLogger().Debugf("combineFlows: flow2 = %s", f.flow2.flow)
	// perform shallow copy of flow and then update some fields
	newF := *f.flow1.flow
	// verify we received flows from each of the 2 specified interfaces
	tidOK := true
	if f.flow1.flow.TID != t.tid1 {
		if f.flow1.flow.TID != t.tid2 || f.flow2.flow.TID != t.tid1 {
			tidOK = false;
		}
	} else if f.flow2.flow.TID != t.tid2 {
		tidOK = false
	}
	if !tidOK {
		logging.GetLogger().Errorf("TIDs don't match: tid1 = %s, tid2 = %s, flowInfo = %s", t.tid1, t.tid2, f)
		newF.SetAction("REJECT")
		f.status = flowInfoRejected
		return &newF
	}
	newMetricReported := prepareCombinedMetric(f.flow1.flow.Metric, f.flow2.flow.Metric)

	// if we are at the end of the flow, set Last time to latest of the times measured
	if f.flow1.flow.FinishType == "ENDED" && f.flow2.flow.FinishType == "ENDED" {
		newMetricReported.Last = common.MaxInt64(f.flow1.flow.Metric.Last, f.flow2.flow.Metric.Last)
	}

	var newLastUpdateMetric *flow.FlowMetric = new(flow.FlowMetric)
	// compute LastUpdateMetric based on previous metricReported and current newMetricReported
	if f.status == flowInfoOldFlow {
		newLastUpdateMetric.ABPackets = newMetricReported.ABPackets - f.metricReported.ABPackets
		newLastUpdateMetric.ABBytes = newMetricReported.ABBytes - f.metricReported.ABBytes
		newLastUpdateMetric.BAPackets = newMetricReported.BAPackets - f.metricReported.BAPackets
		newLastUpdateMetric.BABytes = newMetricReported.BABytes - f.metricReported.BABytes
		newLastUpdateMetric.Start = f.metricReported.Last
	} else {
		// this is the first report for the flow
		newLastUpdateMetric = prepareCombinedMetric(f.flow1.flow.LastUpdateMetric, f.flow2.flow.LastUpdateMetric)
		newLastUpdateMetric.Start = newMetricReported.Start
	}
	newLastUpdateMetric.Last = newMetricReported.Last

	newF.Metric = newMetricReported
	newF.LastUpdateMetric = newLastUpdateMetric

	// What should we do with UpdateCount? In the meantime, we have the value from flow1.flow
	newF.SetAction("ACCEPT")
	f.metricReported = newMetricReported
	logging.GetLogger().Debugf("combineFlows: %s\n\n", newF)
	return &newF
}

// specialHandling - called when we did not see 2 copies of a flow during this iteration; figure out if we need to Reject
func (t *Action) specialHandling(f *combinedFlowInfo) *VpclogsFlow {
	// almost all exception cases are Rejected. Do special handling only for cases that are not Rejected
	if f.status == flowInfoOneEntry {
		if f.prevStatus == flowInfoNoState { // give it another iteration
			return nil
		}
	} else if f.status == flowInfoOldFlow {
		if !f.flow1.saw && !f.flow2.saw { // this might be OK
			// if the flow is indicated as ENDED, then mark it for deletion
			if f.flow1.flow.FinishType == "ENDED" || f.flow2.flow.FinishType == "ENDED" { // flow was finished
				f.status = flowInfoTimedOut
			}
			return nil
		} else if f.flow1.flow.Metric.Last > f.metricReported.Last && f.flow2.flow.Metric.Last > f.metricReported.Last {
			// only one flow was received this iteration; report left-over bytes from previous report
			return t.combineFlows(f)
		}
	}
	// all other cases are marked as Rejected
	var f2 *VpclogsFlow
	f2 = f.flow1.flow
	f2.SetAction("REJECT")
	f.status = flowInfoRejected
	logging.GetLogger().Debugf("rejectFlow: %s\n\n", f2)
	return f2
}

// cleanupOldEntries - delete old entries from table; allocate a new table and preserve only the live connections
func (t *Action) cleanupOldEntries() {
	// allocate a new status buffer
	stateNew := make(map[string]*combinedFlowInfo)
	logging.GetLogger().Debugf("Action cleanupOldEntries, len of old state table: %d", len(t.state))
	for _, f := range t.state {
		if f.status == flowInfoRejected {
			continue
		}
		if f.status == flowInfoThreeFlows { // let this flow be re-initialized on the next iteration
			continue
		}
		if f.flow1.flow.FinishType == "ENDED" && f.flow2.flow.FinishType == "ENDED" { // flow was finished
			continue
		}
		if f.status == flowInfoOneEntry && f.prevStatus == flowInfoOneEntry {
			continue
		}
		if f.status == flowInfoTimedOut {
			continue
		}
		TrackingID := f.flow1.flow.TrackingID
		stateNew[TrackingID], _ = t.state[TrackingID]
	}
	t.state = stateNew
	logging.GetLogger().Debugf("Action cleanupOldEntries, len of new state table: %d", len(t.state))
	now := time.Now()
	t.timeOfLastCleanup = now.Unix()
}

// Action - combines flows from 2 measurement points to determine whether a flow was Accepted or Rejected
func (t *Action) Action(fl interface{}) interface{} {
	t.init()
	t.updateInfo(fl)

	// go over the table and prepare the items that need to be reported
	// flows that were seen twice are Accepted; flows seen once need special handling for accounting purposes
	var newFl []interface{}
	var f2 *VpclogsFlow
	for _, f := range t.state {
		if f.flow1.saw && f.flow2.saw {
			f2 = t.combineFlows(f)
		} else {
			f2 = t.specialHandling(f)
		}
		// if flow is valid but did not appear in most recent round, report nothing
		if f2 != nil {
			newFl = append(newFl, f2)
		}
	}
	// delete old Rejected entries every so often
	now := time.Now()
	secs := now.Unix()
	secs2 := t.timeOfLastCleanup + timeoutPeriodCleanup
	if secs > secs2 {
		t.cleanupOldEntries()
	}
	return newFl
}
