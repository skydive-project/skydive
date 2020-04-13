// Code generated - DO NOT EDIT.

package ovsdb

import (
	"github.com/skydive-project/skydive/graffiti/getter"
	"strings"
)

func (obj *OvsMetadata) GetFieldBool(key string) (bool, error) {
	return false, getter.ErrFieldNotFound
}

func (obj *OvsMetadata) GetFieldInt64(key string) (int64, error) {
	return 0, getter.ErrFieldNotFound
}

func (obj *OvsMetadata) GetFieldString(key string) (string, error) {
	switch key {
	case "DBVersion":
		return string(obj.DBVersion), nil
	case "Version":
		return string(obj.Version), nil
	case "Error":
		return string(obj.Error), nil
	}
	return "", getter.ErrFieldNotFound
}

func (obj *OvsMetadata) GetFieldKeys() []string {
	return []string{
		"OtherConfig",
		"Options",
		"Protocols",
		"DBVersion",
		"Version",
		"Error",
		"Metric",
		"LastUpdateMetric",
	}
}

func (obj *OvsMetadata) MatchBool(key string, predicate getter.BoolPredicate) bool {
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

func (obj *OvsMetadata) MatchInt64(key string, predicate getter.Int64Predicate) bool {
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

func (obj *OvsMetadata) MatchString(key string, predicate getter.StringPredicate) bool {
	if b, err := obj.GetFieldString(key); err == nil {
		return predicate(b)
	}

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {

	case "OtherConfig":
		if obj.OtherConfig != nil {
			if index == -1 {
				for _, v := range obj.OtherConfig {
					if predicate(v) {
						return true
					}
				}
			} else if v, found := obj.OtherConfig[key[index+1:]]; found {
				return predicate(v)
			}
		}
	case "Options":
		if obj.Options != nil {
			if index == -1 {
				for _, v := range obj.Options {
					if predicate(v) {
						return true
					}
				}
			} else if v, found := obj.Options[key[index+1:]]; found {
				return predicate(v)
			}
		}
	case "Protocols":
		if index == -1 {
			for _, i := range obj.Protocols {
				if predicate(i) {
					return true
				}
			}
		}
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

func (obj *OvsMetadata) GetField(key string) (interface{}, error) {
	if s, err := obj.GetFieldString(key); err == nil {
		return s, nil
	}

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {
	case "Protocols":
		if obj.Protocols != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.Protocols {
					results = append(results, obj)
				}
				return results, nil
			}
		}
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
	return nil, getter.ErrFieldNotFound
}

func init() {
	strings.Index("", ".")
}
