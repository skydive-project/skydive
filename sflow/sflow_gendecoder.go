// Code generated - DO NOT EDIT.

package sflow

import (
	"github.com/skydive-project/skydive/common"
	"strings"
)

func (obj *EthMetric) GetFieldBool(key string) (bool, error) {
	return false, common.ErrFieldNotFound
}

func (obj *EthMetric) GetFieldInt64(key string) (int64, error) {
	switch key {
	case "EthAlignmentErrors":
		return int64(obj.EthAlignmentErrors), nil
	case "EthFCSErrors":
		return int64(obj.EthFCSErrors), nil
	case "EthSingleCollisionFrames":
		return int64(obj.EthSingleCollisionFrames), nil
	case "EthMultipleCollisionFrames":
		return int64(obj.EthMultipleCollisionFrames), nil
	case "EthSQETestErrors":
		return int64(obj.EthSQETestErrors), nil
	case "EthDeferredTransmissions":
		return int64(obj.EthDeferredTransmissions), nil
	case "EthLateCollisions":
		return int64(obj.EthLateCollisions), nil
	case "EthExcessiveCollisions":
		return int64(obj.EthExcessiveCollisions), nil
	case "EthInternalMacReceiveErrors":
		return int64(obj.EthInternalMacReceiveErrors), nil
	case "EthInternalMacTransmitErrors":
		return int64(obj.EthInternalMacTransmitErrors), nil
	case "EthCarrierSenseErrors":
		return int64(obj.EthCarrierSenseErrors), nil
	case "EthFrameTooLongs":
		return int64(obj.EthFrameTooLongs), nil
	case "EthSymbolErrors":
		return int64(obj.EthSymbolErrors), nil
	}
	return 0, common.ErrFieldNotFound
}

func (obj *EthMetric) GetFieldString(key string) (string, error) {
	return "", common.ErrFieldNotFound
}

func (obj *EthMetric) GetFieldKeys() []string {
	return []string{
		"EthAlignmentErrors",
		"EthFCSErrors",
		"EthSingleCollisionFrames",
		"EthMultipleCollisionFrames",
		"EthSQETestErrors",
		"EthDeferredTransmissions",
		"EthLateCollisions",
		"EthExcessiveCollisions",
		"EthInternalMacReceiveErrors",
		"EthInternalMacTransmitErrors",
		"EthCarrierSenseErrors",
		"EthFrameTooLongs",
		"EthSymbolErrors",
	}
}

func (obj *EthMetric) MatchBool(key string, predicate common.BoolPredicate) bool {
	return false
}

func (obj *EthMetric) MatchInt64(key string, predicate common.Int64Predicate) bool {
	if b, err := obj.GetFieldInt64(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *EthMetric) MatchString(key string, predicate common.StringPredicate) bool {
	return false
}

func (obj *EthMetric) GetField(key string) (interface{}, error) {
	if i, err := obj.GetFieldInt64(key); err == nil {
		return i, nil
	}
	return nil, common.ErrFieldNotFound
}

func (obj *IfMetric) GetFieldBool(key string) (bool, error) {
	return false, common.ErrFieldNotFound
}

func (obj *IfMetric) GetFieldInt64(key string) (int64, error) {
	switch key {
	case "IfInOctets":
		return int64(obj.IfInOctets), nil
	case "IfInUcastPkts":
		return int64(obj.IfInUcastPkts), nil
	case "IfInMulticastPkts":
		return int64(obj.IfInMulticastPkts), nil
	case "IfInBroadcastPkts":
		return int64(obj.IfInBroadcastPkts), nil
	case "IfInDiscards":
		return int64(obj.IfInDiscards), nil
	case "IfInErrors":
		return int64(obj.IfInErrors), nil
	case "IfInUnknownProtos":
		return int64(obj.IfInUnknownProtos), nil
	case "IfOutOctets":
		return int64(obj.IfOutOctets), nil
	case "IfOutUcastPkts":
		return int64(obj.IfOutUcastPkts), nil
	case "IfOutMulticastPkts":
		return int64(obj.IfOutMulticastPkts), nil
	case "IfOutBroadcastPkts":
		return int64(obj.IfOutBroadcastPkts), nil
	case "IfOutDiscards":
		return int64(obj.IfOutDiscards), nil
	case "IfOutErrors":
		return int64(obj.IfOutErrors), nil
	}
	return 0, common.ErrFieldNotFound
}

func (obj *IfMetric) GetFieldString(key string) (string, error) {
	return "", common.ErrFieldNotFound
}

func (obj *IfMetric) GetFieldKeys() []string {
	return []string{
		"IfInOctets",
		"IfInUcastPkts",
		"IfInMulticastPkts",
		"IfInBroadcastPkts",
		"IfInDiscards",
		"IfInErrors",
		"IfInUnknownProtos",
		"IfOutOctets",
		"IfOutUcastPkts",
		"IfOutMulticastPkts",
		"IfOutBroadcastPkts",
		"IfOutDiscards",
		"IfOutErrors",
	}
}

func (obj *IfMetric) MatchBool(key string, predicate common.BoolPredicate) bool {
	return false
}

func (obj *IfMetric) MatchInt64(key string, predicate common.Int64Predicate) bool {
	if b, err := obj.GetFieldInt64(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *IfMetric) MatchString(key string, predicate common.StringPredicate) bool {
	return false
}

func (obj *IfMetric) GetField(key string) (interface{}, error) {
	if i, err := obj.GetFieldInt64(key); err == nil {
		return i, nil
	}
	return nil, common.ErrFieldNotFound
}

func (obj *OvsMetric) GetFieldBool(key string) (bool, error) {
	return false, common.ErrFieldNotFound
}

func (obj *OvsMetric) GetFieldInt64(key string) (int64, error) {
	switch key {
	case "OvsDpNHit":
		return int64(obj.OvsDpNHit), nil
	case "OvsDpNMissed":
		return int64(obj.OvsDpNMissed), nil
	case "OvsDpNLost":
		return int64(obj.OvsDpNLost), nil
	case "OvsDpNMaskHit":
		return int64(obj.OvsDpNMaskHit), nil
	case "OvsDpNFlows":
		return int64(obj.OvsDpNFlows), nil
	case "OvsDpNMasks":
		return int64(obj.OvsDpNMasks), nil
	case "OvsAppFdOpen":
		return int64(obj.OvsAppFdOpen), nil
	case "OvsAppFdMax":
		return int64(obj.OvsAppFdMax), nil
	case "OvsAppConnOpen":
		return int64(obj.OvsAppConnOpen), nil
	case "OvsAppConnMax":
		return int64(obj.OvsAppConnMax), nil
	case "OvsAppMemUsed":
		return int64(obj.OvsAppMemUsed), nil
	case "OvsAppMemMax":
		return int64(obj.OvsAppMemMax), nil
	}
	return 0, common.ErrFieldNotFound
}

func (obj *OvsMetric) GetFieldString(key string) (string, error) {
	return "", common.ErrFieldNotFound
}

func (obj *OvsMetric) GetFieldKeys() []string {
	return []string{
		"OvsDpNHit",
		"OvsDpNMissed",
		"OvsDpNLost",
		"OvsDpNMaskHit",
		"OvsDpNFlows",
		"OvsDpNMasks",
		"OvsAppFdOpen",
		"OvsAppFdMax",
		"OvsAppConnOpen",
		"OvsAppConnMax",
		"OvsAppMemUsed",
		"OvsAppMemMax",
	}
}

func (obj *OvsMetric) MatchBool(key string, predicate common.BoolPredicate) bool {
	return false
}

func (obj *OvsMetric) MatchInt64(key string, predicate common.Int64Predicate) bool {
	if b, err := obj.GetFieldInt64(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *OvsMetric) MatchString(key string, predicate common.StringPredicate) bool {
	return false
}

func (obj *OvsMetric) GetField(key string) (interface{}, error) {
	if i, err := obj.GetFieldInt64(key); err == nil {
		return i, nil
	}
	return nil, common.ErrFieldNotFound
}

func (obj *SFMetric) GetFieldBool(key string) (bool, error) {
	return false, common.ErrFieldNotFound
}

func (obj *SFMetric) GetFieldInt64(key string) (int64, error) {
	switch key {
	case "IfInOctets":
		return int64(obj.IfInOctets), nil
	case "IfInUcastPkts":
		return int64(obj.IfInUcastPkts), nil
	case "IfInMulticastPkts":
		return int64(obj.IfInMulticastPkts), nil
	case "IfInBroadcastPkts":
		return int64(obj.IfInBroadcastPkts), nil
	case "IfInDiscards":
		return int64(obj.IfInDiscards), nil
	case "IfInErrors":
		return int64(obj.IfInErrors), nil
	case "IfInUnknownProtos":
		return int64(obj.IfInUnknownProtos), nil
	case "IfOutOctets":
		return int64(obj.IfOutOctets), nil
	case "IfOutUcastPkts":
		return int64(obj.IfOutUcastPkts), nil
	case "IfOutMulticastPkts":
		return int64(obj.IfOutMulticastPkts), nil
	case "IfOutBroadcastPkts":
		return int64(obj.IfOutBroadcastPkts), nil
	case "IfOutDiscards":
		return int64(obj.IfOutDiscards), nil
	case "IfOutErrors":
		return int64(obj.IfOutErrors), nil
	case "OvsDpNHit":
		return int64(obj.OvsDpNHit), nil
	case "OvsDpNMissed":
		return int64(obj.OvsDpNMissed), nil
	case "OvsDpNLost":
		return int64(obj.OvsDpNLost), nil
	case "OvsDpNMaskHit":
		return int64(obj.OvsDpNMaskHit), nil
	case "OvsDpNFlows":
		return int64(obj.OvsDpNFlows), nil
	case "OvsDpNMasks":
		return int64(obj.OvsDpNMasks), nil
	case "OvsAppFdOpen":
		return int64(obj.OvsAppFdOpen), nil
	case "OvsAppFdMax":
		return int64(obj.OvsAppFdMax), nil
	case "OvsAppConnOpen":
		return int64(obj.OvsAppConnOpen), nil
	case "OvsAppConnMax":
		return int64(obj.OvsAppConnMax), nil
	case "OvsAppMemUsed":
		return int64(obj.OvsAppMemUsed), nil
	case "OvsAppMemMax":
		return int64(obj.OvsAppMemMax), nil
	case "VlanOctets":
		return int64(obj.VlanOctets), nil
	case "VlanUcastPkts":
		return int64(obj.VlanUcastPkts), nil
	case "VlanMulticastPkts":
		return int64(obj.VlanMulticastPkts), nil
	case "VlanBroadcastPkts":
		return int64(obj.VlanBroadcastPkts), nil
	case "VlanDiscards":
		return int64(obj.VlanDiscards), nil
	case "EthAlignmentErrors":
		return int64(obj.EthAlignmentErrors), nil
	case "EthFCSErrors":
		return int64(obj.EthFCSErrors), nil
	case "EthSingleCollisionFrames":
		return int64(obj.EthSingleCollisionFrames), nil
	case "EthMultipleCollisionFrames":
		return int64(obj.EthMultipleCollisionFrames), nil
	case "EthSQETestErrors":
		return int64(obj.EthSQETestErrors), nil
	case "EthDeferredTransmissions":
		return int64(obj.EthDeferredTransmissions), nil
	case "EthLateCollisions":
		return int64(obj.EthLateCollisions), nil
	case "EthExcessiveCollisions":
		return int64(obj.EthExcessiveCollisions), nil
	case "EthInternalMacReceiveErrors":
		return int64(obj.EthInternalMacReceiveErrors), nil
	case "EthInternalMacTransmitErrors":
		return int64(obj.EthInternalMacTransmitErrors), nil
	case "EthCarrierSenseErrors":
		return int64(obj.EthCarrierSenseErrors), nil
	case "EthFrameTooLongs":
		return int64(obj.EthFrameTooLongs), nil
	case "EthSymbolErrors":
		return int64(obj.EthSymbolErrors), nil
	case "Start":
		return int64(obj.Start), nil
	case "Last":
		return int64(obj.Last), nil
	}
	return 0, common.ErrFieldNotFound
}

func (obj *SFMetric) GetFieldString(key string) (string, error) {
	return "", common.ErrFieldNotFound
}

func (obj *SFMetric) GetFieldKeys() []string {
	return []string{
		"IfInOctets",
		"IfInUcastPkts",
		"IfInMulticastPkts",
		"IfInBroadcastPkts",
		"IfInDiscards",
		"IfInErrors",
		"IfInUnknownProtos",
		"IfOutOctets",
		"IfOutUcastPkts",
		"IfOutMulticastPkts",
		"IfOutBroadcastPkts",
		"IfOutDiscards",
		"IfOutErrors",
		"OvsDpNHit",
		"OvsDpNMissed",
		"OvsDpNLost",
		"OvsDpNMaskHit",
		"OvsDpNFlows",
		"OvsDpNMasks",
		"OvsAppFdOpen",
		"OvsAppFdMax",
		"OvsAppConnOpen",
		"OvsAppConnMax",
		"OvsAppMemUsed",
		"OvsAppMemMax",
		"VlanOctets",
		"VlanUcastPkts",
		"VlanMulticastPkts",
		"VlanBroadcastPkts",
		"VlanDiscards",
		"EthAlignmentErrors",
		"EthFCSErrors",
		"EthSingleCollisionFrames",
		"EthMultipleCollisionFrames",
		"EthSQETestErrors",
		"EthDeferredTransmissions",
		"EthLateCollisions",
		"EthExcessiveCollisions",
		"EthInternalMacReceiveErrors",
		"EthInternalMacTransmitErrors",
		"EthCarrierSenseErrors",
		"EthFrameTooLongs",
		"EthSymbolErrors",
		"Start",
		"Last",
	}
}

func (obj *SFMetric) MatchBool(key string, predicate common.BoolPredicate) bool {
	return false
}

func (obj *SFMetric) MatchInt64(key string, predicate common.Int64Predicate) bool {
	if b, err := obj.GetFieldInt64(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *SFMetric) MatchString(key string, predicate common.StringPredicate) bool {
	return false
}

func (obj *SFMetric) GetField(key string) (interface{}, error) {
	if i, err := obj.GetFieldInt64(key); err == nil {
		return i, nil
	}
	return nil, common.ErrFieldNotFound
}

func (obj *SFlow) GetFieldBool(key string) (bool, error) {
	return false, common.ErrFieldNotFound
}

func (obj *SFlow) GetFieldInt64(key string) (int64, error) {
	return 0, common.ErrFieldNotFound
}

func (obj *SFlow) GetFieldString(key string) (string, error) {
	return "", common.ErrFieldNotFound
}

func (obj *SFlow) GetFieldKeys() []string {
	return []string{
		"Metric",
		"LastUpdateMetric",
	}
}

func (obj *SFlow) MatchBool(key string, predicate common.BoolPredicate) bool {
	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {
	case "Metric":
		if index != -1 && obj.Metric != nil {
			return obj.Metric.MatchBool(key[index+1:], predicate)
		}
	case "LastUpdateMetric":
		if index != -1 && obj.LastUpdateMetric != nil {
			return obj.LastUpdateMetric.MatchBool(key[index+1:], predicate)
		}
	}
	return false
}

func (obj *SFlow) MatchInt64(key string, predicate common.Int64Predicate) bool {
	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {

	case "Metric":
		if index != -1 && obj.Metric != nil {
			return obj.Metric.MatchInt64(key[index+1:], predicate)
		}
	case "LastUpdateMetric":
		if index != -1 && obj.LastUpdateMetric != nil {
			return obj.LastUpdateMetric.MatchInt64(key[index+1:], predicate)
		}
	}
	return false
}

func (obj *SFlow) MatchString(key string, predicate common.StringPredicate) bool {
	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {

	case "Metric":
		if index != -1 && obj.Metric != nil {
			return obj.Metric.MatchString(key[index+1:], predicate)
		}
	case "LastUpdateMetric":
		if index != -1 && obj.LastUpdateMetric != nil {
			return obj.LastUpdateMetric.MatchString(key[index+1:], predicate)
		}
	}
	return false
}

func (obj *SFlow) GetField(key string) (interface{}, error) {
	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {
	case "Metric":
		if obj.Metric != nil {
			if index != -1 {
				return obj.Metric.GetField(key[index+1:])
			} else {
				return obj.Metric, nil
			}
		}
	case "LastUpdateMetric":
		if obj.LastUpdateMetric != nil {
			if index != -1 {
				return obj.LastUpdateMetric.GetField(key[index+1:])
			} else {
				return obj.LastUpdateMetric, nil
			}
		}

	}
	return nil, common.ErrFieldNotFound
}

func (obj *VlanMetric) GetFieldBool(key string) (bool, error) {
	return false, common.ErrFieldNotFound
}

func (obj *VlanMetric) GetFieldInt64(key string) (int64, error) {
	switch key {
	case "VlanOctets":
		return int64(obj.VlanOctets), nil
	case "VlanUcastPkts":
		return int64(obj.VlanUcastPkts), nil
	case "VlanMulticastPkts":
		return int64(obj.VlanMulticastPkts), nil
	case "VlanBroadcastPkts":
		return int64(obj.VlanBroadcastPkts), nil
	case "VlanDiscards":
		return int64(obj.VlanDiscards), nil
	}
	return 0, common.ErrFieldNotFound
}

func (obj *VlanMetric) GetFieldString(key string) (string, error) {
	return "", common.ErrFieldNotFound
}

func (obj *VlanMetric) GetFieldKeys() []string {
	return []string{
		"VlanOctets",
		"VlanUcastPkts",
		"VlanMulticastPkts",
		"VlanBroadcastPkts",
		"VlanDiscards",
	}
}

func (obj *VlanMetric) MatchBool(key string, predicate common.BoolPredicate) bool {
	return false
}

func (obj *VlanMetric) MatchInt64(key string, predicate common.Int64Predicate) bool {
	if b, err := obj.GetFieldInt64(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *VlanMetric) MatchString(key string, predicate common.StringPredicate) bool {
	return false
}

func (obj *VlanMetric) GetField(key string) (interface{}, error) {
	if i, err := obj.GetFieldInt64(key); err == nil {
		return i, nil
	}
	return nil, common.ErrFieldNotFound
}

func init() {
	strings.Index("", ".")
}
