// Code generated - DO NOT EDIT.

package blockdev

import (
	"github.com/skydive-project/skydive/graffiti/getter"
	"strings"
)

func (obj *BlockMetric) GetFieldBool(key string) (bool, error) {
	return false, getter.ErrFieldNotFound
}

func (obj *BlockMetric) GetFieldInt64(key string) (int64, error) {
	switch key {
	case "ReadsPerSec":
		return int64(obj.ReadsPerSec), nil
	case "WritesPerSec":
		return int64(obj.WritesPerSec), nil
	case "ReadsKBPerSec":
		return int64(obj.ReadsKBPerSec), nil
	case "WritesKBPerSec":
		return int64(obj.WritesKBPerSec), nil
	case "ReadsMergedPerSec":
		return int64(obj.ReadsMergedPerSec), nil
	case "WritesMergedPerSec":
		return int64(obj.WritesMergedPerSec), nil
	case "ReadsMerged":
		return int64(obj.ReadsMerged), nil
	case "WritesMerged":
		return int64(obj.WritesMerged), nil
	case "ReadServiceTime":
		return int64(obj.ReadServiceTime), nil
	case "WriteServiceTime":
		return int64(obj.WriteServiceTime), nil
	case "AverageQueueSize":
		return int64(obj.AverageQueueSize), nil
	case "AverageReadSize":
		return int64(obj.AverageReadSize), nil
	case "AverageWriteSize":
		return int64(obj.AverageWriteSize), nil
	case "ServiceTime":
		return int64(obj.ServiceTime), nil
	case "Utilization":
		return int64(obj.Utilization), nil
	case "Start":
		return int64(obj.Start), nil
	case "Last":
		return int64(obj.Last), nil
	}
	return 0, getter.ErrFieldNotFound
}

func (obj *BlockMetric) GetFieldString(key string) (string, error) {
	return "", getter.ErrFieldNotFound
}

func (obj *BlockMetric) GetFieldKeys() []string {
	return []string{
		"ReadsPerSec",
		"WritesPerSec",
		"ReadsKBPerSec",
		"WritesKBPerSec",
		"ReadsMergedPerSec",
		"WritesMergedPerSec",
		"ReadsMerged",
		"WritesMerged",
		"ReadServiceTime",
		"WriteServiceTime",
		"AverageQueueSize",
		"AverageReadSize",
		"AverageWriteSize",
		"ServiceTime",
		"Utilization",
		"Start",
		"Last",
	}
}

func (obj *BlockMetric) MatchBool(key string, predicate getter.BoolPredicate) bool {
	return false
}

func (obj *BlockMetric) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	if b, err := obj.GetFieldInt64(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *BlockMetric) MatchString(key string, predicate getter.StringPredicate) bool {
	return false
}

func (obj *BlockMetric) GetField(key string) (interface{}, error) {
	if i, err := obj.GetFieldInt64(key); err == nil {
		return i, nil
	}
	return nil, getter.ErrFieldNotFound
}

func init() {
	strings.Index("", ".")
}
