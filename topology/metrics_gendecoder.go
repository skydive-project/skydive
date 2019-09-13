// Code generated - DO NOT EDIT.

package topology

import (
	"github.com/skydive-project/skydive/common"
	"strings"
)

func (obj *InterfaceMetric) GetFieldBool(key string) (bool, error) {
	return false, common.ErrFieldNotFound
}

func (obj *InterfaceMetric) GetFieldInt64(key string) (int64, error) {
	switch key {
	case "Collisions":
		return int64(obj.Collisions), nil
	case "Multicast":
		return int64(obj.Multicast), nil
	case "RxBytes":
		return int64(obj.RxBytes), nil
	case "RxCompressed":
		return int64(obj.RxCompressed), nil
	case "RxCrcErrors":
		return int64(obj.RxCrcErrors), nil
	case "RxDropped":
		return int64(obj.RxDropped), nil
	case "RxErrors":
		return int64(obj.RxErrors), nil
	case "RxFifoErrors":
		return int64(obj.RxFifoErrors), nil
	case "RxFrameErrors":
		return int64(obj.RxFrameErrors), nil
	case "RxLengthErrors":
		return int64(obj.RxLengthErrors), nil
	case "RxMissedErrors":
		return int64(obj.RxMissedErrors), nil
	case "RxOverErrors":
		return int64(obj.RxOverErrors), nil
	case "RxPackets":
		return int64(obj.RxPackets), nil
	case "TxAbortedErrors":
		return int64(obj.TxAbortedErrors), nil
	case "TxBytes":
		return int64(obj.TxBytes), nil
	case "TxCarrierErrors":
		return int64(obj.TxCarrierErrors), nil
	case "TxCompressed":
		return int64(obj.TxCompressed), nil
	case "TxDropped":
		return int64(obj.TxDropped), nil
	case "TxErrors":
		return int64(obj.TxErrors), nil
	case "TxFifoErrors":
		return int64(obj.TxFifoErrors), nil
	case "TxHeartbeatErrors":
		return int64(obj.TxHeartbeatErrors), nil
	case "TxPackets":
		return int64(obj.TxPackets), nil
	case "TxWindowErrors":
		return int64(obj.TxWindowErrors), nil
	case "Start":
		return int64(obj.Start), nil
	case "Last":
		return int64(obj.Last), nil
	}
	return 0, common.ErrFieldNotFound
}

func (obj *InterfaceMetric) GetFieldString(key string) (string, error) {
	return "", common.ErrFieldNotFound
}

func (obj *InterfaceMetric) GetFieldKeys() []string {
	return []string{
		"Collisions",
		"Multicast",
		"RxBytes",
		"RxCompressed",
		"RxCrcErrors",
		"RxDropped",
		"RxErrors",
		"RxFifoErrors",
		"RxFrameErrors",
		"RxLengthErrors",
		"RxMissedErrors",
		"RxOverErrors",
		"RxPackets",
		"TxAbortedErrors",
		"TxBytes",
		"TxCarrierErrors",
		"TxCompressed",
		"TxDropped",
		"TxErrors",
		"TxFifoErrors",
		"TxHeartbeatErrors",
		"TxPackets",
		"TxWindowErrors",
		"Start",
		"Last",
	}
}

func (obj *InterfaceMetric) MatchBool(key string, predicate common.BoolPredicate) bool {
	return false
}

func (obj *InterfaceMetric) MatchInt64(key string, predicate common.Int64Predicate) bool {
	if b, err := obj.GetFieldInt64(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *InterfaceMetric) MatchString(key string, predicate common.StringPredicate) bool {
	return false
}

func (obj *InterfaceMetric) GetField(key string) (interface{}, error) {
	if i, err := obj.GetFieldInt64(key); err == nil {
		return i, nil
	}
	return nil, common.ErrFieldNotFound
}

func init() {
	strings.Index("", ".")
}
