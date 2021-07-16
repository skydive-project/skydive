// Code generated - DO NOT EDIT.

package ovnmodel

import (
	"github.com/skydive-project/skydive/graffiti/getter"
	"strings"
)

func (obj *Connection) GetFieldBool(key string) (bool, error) {
	switch key {
	case "IsConnected":
		return obj.IsConnected, nil
	}

	return false, getter.ErrFieldNotFound
}

func (obj *Connection) GetFieldInt64(key string) (int64, error) {
	return 0, getter.ErrFieldNotFound
}

func (obj *Connection) GetFieldString(key string) (string, error) {
	switch key {
	case "UUID":
		return string(obj.UUID), nil
	case "Target":
		return string(obj.Target), nil
	}
	return "", getter.ErrFieldNotFound
}

func (obj *Connection) GetFieldKeys() []string {
	return []string{
		"UUID",
		"ExternalIDs",
		"InactivityProbe",
		"IsConnected",
		"MaxBackoff",
		"OtherConfig",
		"Status",
		"Target",
		"ExternalIDsMeta",
		"OtherConfigMeta",
		"StatusMeta",
	}
}

func (obj *Connection) MatchBool(key string, predicate getter.BoolPredicate) bool {
	if b, err := obj.GetFieldBool(key); err == nil {
		return predicate(b)
	}

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {
	case "ExternalIDsMeta":
		if index != -1 && obj.ExternalIDsMeta != nil {
			return obj.ExternalIDsMeta.MatchBool(key[index+1:], predicate)
		}
	case "OtherConfigMeta":
		if index != -1 && obj.OtherConfigMeta != nil {
			return obj.OtherConfigMeta.MatchBool(key[index+1:], predicate)
		}
	case "StatusMeta":
		if index != -1 && obj.StatusMeta != nil {
			return obj.StatusMeta.MatchBool(key[index+1:], predicate)
		}
	}
	return false
}

func (obj *Connection) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {

	case "ExternalIDsMeta":
		if index != -1 && obj.ExternalIDsMeta != nil {
			return obj.ExternalIDsMeta.MatchInt64(key[index+1:], predicate)
		}
	case "OtherConfigMeta":
		if index != -1 && obj.OtherConfigMeta != nil {
			return obj.OtherConfigMeta.MatchInt64(key[index+1:], predicate)
		}
	case "StatusMeta":
		if index != -1 && obj.StatusMeta != nil {
			return obj.StatusMeta.MatchInt64(key[index+1:], predicate)
		}
	}
	return false
}

func (obj *Connection) MatchString(key string, predicate getter.StringPredicate) bool {
	if b, err := obj.GetFieldString(key); err == nil {
		return predicate(b)
	}

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {

	case "ExternalIDs":
		if obj.ExternalIDs != nil {
			if index == -1 {
				for _, v := range obj.ExternalIDs {
					if predicate(v) {
						return true
					}
				}
			} else if v, found := obj.ExternalIDs[key[index+1:]]; found {
				return predicate(v)
			}
		}
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
	case "Status":
		if obj.Status != nil {
			if index == -1 {
				for _, v := range obj.Status {
					if predicate(v) {
						return true
					}
				}
			} else if v, found := obj.Status[key[index+1:]]; found {
				return predicate(v)
			}
		}
	case "ExternalIDsMeta":
		if index != -1 && obj.ExternalIDsMeta != nil {
			return obj.ExternalIDsMeta.MatchString(key[index+1:], predicate)
		}
	case "OtherConfigMeta":
		if index != -1 && obj.OtherConfigMeta != nil {
			return obj.OtherConfigMeta.MatchString(key[index+1:], predicate)
		}
	case "StatusMeta":
		if index != -1 && obj.StatusMeta != nil {
			return obj.StatusMeta.MatchString(key[index+1:], predicate)
		}
	}
	return false
}

func (obj *Connection) GetField(key string) (interface{}, error) {
	if s, err := obj.GetFieldString(key); err == nil {
		return s, nil
	}

	if b, err := obj.GetFieldBool(key); err == nil {
		return b, nil
	}

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {
	case "InactivityProbe":
		if obj.InactivityProbe != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.InactivityProbe {
					results = append(results, obj)
				}
				return results, nil
			}
		}
	case "MaxBackoff":
		if obj.MaxBackoff != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.MaxBackoff {
					results = append(results, obj)
				}
				return results, nil
			}
		}
	case "ExternalIDsMeta":
		if obj.ExternalIDsMeta != nil {
			if index != -1 {
				return obj.ExternalIDsMeta.GetField(key[index+1:])
			} else {
				return obj.ExternalIDsMeta, nil
			}
		}
	case "OtherConfigMeta":
		if obj.OtherConfigMeta != nil {
			if index != -1 {
				return obj.OtherConfigMeta.GetField(key[index+1:])
			} else {
				return obj.OtherConfigMeta, nil
			}
		}
	case "StatusMeta":
		if obj.StatusMeta != nil {
			if index != -1 {
				return obj.StatusMeta.GetField(key[index+1:])
			} else {
				return obj.StatusMeta, nil
			}
		}

	}
	return nil, getter.ErrFieldNotFound
}

func init() {
	strings.Index("", ".")
}
