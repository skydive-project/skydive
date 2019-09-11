// Code generated - DO NOT EDIT.

package flow

import (
	"github.com/skydive-project/skydive/common"
	"strings"
)

func (obj *FlowLayer) GetFieldBool(key string) (bool, error) {
	return false, common.ErrFieldNotFound
}

func (obj *FlowLayer) GetFieldInt64(key string) (int64, error) {
	switch key {
	case "ID":
		return int64(obj.ID), nil
	}
	return 0, common.ErrFieldNotFound
}

func (obj *FlowLayer) GetFieldString(key string) (string, error) {
	switch key {
	case "Protocol":
		return obj.Protocol.String(), nil
	case "A":
		return string(obj.A), nil
	case "B":
		return string(obj.B), nil
	}
	return "", common.ErrFieldNotFound
}

func (obj *FlowLayer) GetFieldKeys() []string {
	return []string{
		"Protocol",
		"A",
		"B",
		"ID",
	}
}

func (obj *FlowLayer) MatchBool(key string, predicate common.BoolPredicate) bool {
	return false
}

func (obj *FlowLayer) MatchInt64(key string, predicate common.Int64Predicate) bool {
	if b, err := obj.GetFieldInt64(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *FlowLayer) MatchString(key string, predicate common.StringPredicate) bool {
	if b, err := obj.GetFieldString(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *FlowLayer) GetField(key string) (interface{}, error) {
	if s, err := obj.GetFieldString(key); err == nil {
		return s, nil
	}

	if i, err := obj.GetFieldInt64(key); err == nil {
		return i, nil
	}
	return nil, common.ErrFieldNotFound
}

func (obj *FlowMetric) GetFieldBool(key string) (bool, error) {
	return false, common.ErrFieldNotFound
}

func (obj *FlowMetric) GetFieldInt64(key string) (int64, error) {
	switch key {
	case "ABPackets":
		return int64(obj.ABPackets), nil
	case "ABBytes":
		return int64(obj.ABBytes), nil
	case "BAPackets":
		return int64(obj.BAPackets), nil
	case "BABytes":
		return int64(obj.BABytes), nil
	case "Start":
		return int64(obj.Start), nil
	case "Last":
		return int64(obj.Last), nil
	case "RTT":
		return int64(obj.RTT), nil
	}
	return 0, common.ErrFieldNotFound
}

func (obj *FlowMetric) GetFieldString(key string) (string, error) {
	return "", common.ErrFieldNotFound
}

func (obj *FlowMetric) GetFieldKeys() []string {
	return []string{
		"ABPackets",
		"ABBytes",
		"BAPackets",
		"BABytes",
		"Start",
		"Last",
		"RTT",
	}
}

func (obj *FlowMetric) MatchBool(key string, predicate common.BoolPredicate) bool {
	return false
}

func (obj *FlowMetric) MatchInt64(key string, predicate common.Int64Predicate) bool {
	if b, err := obj.GetFieldInt64(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *FlowMetric) MatchString(key string, predicate common.StringPredicate) bool {
	return false
}

func (obj *FlowMetric) GetField(key string) (interface{}, error) {
	if i, err := obj.GetFieldInt64(key); err == nil {
		return i, nil
	}
	return nil, common.ErrFieldNotFound
}

func (obj *ICMPLayer) GetFieldBool(key string) (bool, error) {
	return false, common.ErrFieldNotFound
}

func (obj *ICMPLayer) GetFieldInt64(key string) (int64, error) {
	switch key {
	case "Code":
		return int64(obj.Code), nil
	case "ID":
		return int64(obj.ID), nil
	}
	return 0, common.ErrFieldNotFound
}

func (obj *ICMPLayer) GetFieldString(key string) (string, error) {
	switch key {
	case "Type":
		return obj.Type.String(), nil
	}
	return "", common.ErrFieldNotFound
}

func (obj *ICMPLayer) GetFieldKeys() []string {
	return []string{
		"Type",
		"Code",
		"ID",
	}
}

func (obj *ICMPLayer) MatchBool(key string, predicate common.BoolPredicate) bool {
	return false
}

func (obj *ICMPLayer) MatchInt64(key string, predicate common.Int64Predicate) bool {
	if b, err := obj.GetFieldInt64(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *ICMPLayer) MatchString(key string, predicate common.StringPredicate) bool {
	if b, err := obj.GetFieldString(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *ICMPLayer) GetField(key string) (interface{}, error) {
	if s, err := obj.GetFieldString(key); err == nil {
		return s, nil
	}

	if i, err := obj.GetFieldInt64(key); err == nil {
		return i, nil
	}
	return nil, common.ErrFieldNotFound
}

func (obj *IPMetric) GetFieldBool(key string) (bool, error) {
	return false, common.ErrFieldNotFound
}

func (obj *IPMetric) GetFieldInt64(key string) (int64, error) {
	switch key {
	case "Fragments":
		return int64(obj.Fragments), nil
	case "FragmentErrors":
		return int64(obj.FragmentErrors), nil
	}
	return 0, common.ErrFieldNotFound
}

func (obj *IPMetric) GetFieldString(key string) (string, error) {
	return "", common.ErrFieldNotFound
}

func (obj *IPMetric) GetFieldKeys() []string {
	return []string{
		"Fragments",
		"FragmentErrors",
	}
}

func (obj *IPMetric) MatchBool(key string, predicate common.BoolPredicate) bool {
	return false
}

func (obj *IPMetric) MatchInt64(key string, predicate common.Int64Predicate) bool {
	if b, err := obj.GetFieldInt64(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *IPMetric) MatchString(key string, predicate common.StringPredicate) bool {
	return false
}

func (obj *IPMetric) GetField(key string) (interface{}, error) {
	if i, err := obj.GetFieldInt64(key); err == nil {
		return i, nil
	}
	return nil, common.ErrFieldNotFound
}

func (obj *TCPMetric) GetFieldBool(key string) (bool, error) {
	return false, common.ErrFieldNotFound
}

func (obj *TCPMetric) GetFieldInt64(key string) (int64, error) {
	switch key {
	case "ABSynStart":
		return int64(obj.ABSynStart), nil
	case "BASynStart":
		return int64(obj.BASynStart), nil
	case "ABSynTTL":
		return int64(obj.ABSynTTL), nil
	case "BASynTTL":
		return int64(obj.BASynTTL), nil
	case "ABFinStart":
		return int64(obj.ABFinStart), nil
	case "BAFinStart":
		return int64(obj.BAFinStart), nil
	case "ABRstStart":
		return int64(obj.ABRstStart), nil
	case "BARstStart":
		return int64(obj.BARstStart), nil
	case "ABSegmentOutOfOrder":
		return int64(obj.ABSegmentOutOfOrder), nil
	case "ABSegmentSkipped":
		return int64(obj.ABSegmentSkipped), nil
	case "ABSegmentSkippedBytes":
		return int64(obj.ABSegmentSkippedBytes), nil
	case "ABPackets":
		return int64(obj.ABPackets), nil
	case "ABBytes":
		return int64(obj.ABBytes), nil
	case "ABSawStart":
		return int64(obj.ABSawStart), nil
	case "ABSawEnd":
		return int64(obj.ABSawEnd), nil
	case "BASegmentOutOfOrder":
		return int64(obj.BASegmentOutOfOrder), nil
	case "BASegmentSkipped":
		return int64(obj.BASegmentSkipped), nil
	case "BASegmentSkippedBytes":
		return int64(obj.BASegmentSkippedBytes), nil
	case "BAPackets":
		return int64(obj.BAPackets), nil
	case "BABytes":
		return int64(obj.BABytes), nil
	case "BASawStart":
		return int64(obj.BASawStart), nil
	case "BASawEnd":
		return int64(obj.BASawEnd), nil
	}
	return 0, common.ErrFieldNotFound
}

func (obj *TCPMetric) GetFieldString(key string) (string, error) {
	return "", common.ErrFieldNotFound
}

func (obj *TCPMetric) GetFieldKeys() []string {
	return []string{
		"ABSynStart",
		"BASynStart",
		"ABSynTTL",
		"BASynTTL",
		"ABFinStart",
		"BAFinStart",
		"ABRstStart",
		"BARstStart",
		"ABSegmentOutOfOrder",
		"ABSegmentSkipped",
		"ABSegmentSkippedBytes",
		"ABPackets",
		"ABBytes",
		"ABSawStart",
		"ABSawEnd",
		"BASegmentOutOfOrder",
		"BASegmentSkipped",
		"BASegmentSkippedBytes",
		"BAPackets",
		"BABytes",
		"BASawStart",
		"BASawEnd",
	}
}

func (obj *TCPMetric) MatchBool(key string, predicate common.BoolPredicate) bool {
	return false
}

func (obj *TCPMetric) MatchInt64(key string, predicate common.Int64Predicate) bool {
	if b, err := obj.GetFieldInt64(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *TCPMetric) MatchString(key string, predicate common.StringPredicate) bool {
	return false
}

func (obj *TCPMetric) GetField(key string) (interface{}, error) {
	if i, err := obj.GetFieldInt64(key); err == nil {
		return i, nil
	}
	return nil, common.ErrFieldNotFound
}

func (obj *TransportLayer) GetFieldBool(key string) (bool, error) {
	return false, common.ErrFieldNotFound
}

func (obj *TransportLayer) GetFieldInt64(key string) (int64, error) {
	switch key {
	case "A":
		return int64(obj.A), nil
	case "B":
		return int64(obj.B), nil
	case "ID":
		return int64(obj.ID), nil
	}
	return 0, common.ErrFieldNotFound
}

func (obj *TransportLayer) GetFieldString(key string) (string, error) {
	switch key {
	case "Protocol":
		return obj.Protocol.String(), nil
	}
	return "", common.ErrFieldNotFound
}

func (obj *TransportLayer) GetFieldKeys() []string {
	return []string{
		"Protocol",
		"A",
		"B",
		"ID",
	}
}

func (obj *TransportLayer) MatchBool(key string, predicate common.BoolPredicate) bool {
	return false
}

func (obj *TransportLayer) MatchInt64(key string, predicate common.Int64Predicate) bool {
	if b, err := obj.GetFieldInt64(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *TransportLayer) MatchString(key string, predicate common.StringPredicate) bool {
	if b, err := obj.GetFieldString(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *TransportLayer) GetField(key string) (interface{}, error) {
	if s, err := obj.GetFieldString(key); err == nil {
		return s, nil
	}

	if i, err := obj.GetFieldInt64(key); err == nil {
		return i, nil
	}
	return nil, common.ErrFieldNotFound
}

func init() {
	strings.Index("", ".")
}
