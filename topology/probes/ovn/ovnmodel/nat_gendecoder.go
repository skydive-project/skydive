// Code generated - DO NOT EDIT.

package ovnmodel

import (
	"github.com/skydive-project/skydive/graffiti/getter"
	"strings"
)

func (obj *NAT) GetFieldBool(key string) (bool, error) {
	return false, getter.ErrFieldNotFound
}

func (obj *NAT) GetFieldInt64(key string) (int64, error) {
	return 0, getter.ErrFieldNotFound
}

func (obj *NAT) GetFieldString(key string) (string, error) {
	switch key {
	case "UUID":
		return string(obj.UUID), nil
	case "ExternalIP":
		return string(obj.ExternalIP), nil
	case "ExternalPortRange":
		return string(obj.ExternalPortRange), nil
	case "LogicalIP":
		return string(obj.LogicalIP), nil
	case "Type":
		return string(obj.Type), nil
	}
	return "", getter.ErrFieldNotFound
}

func (obj *NAT) GetFieldKeys() []string {
	return []string{
		"UUID",
		"AllowedExtIPs",
		"ExemptedExtIPs",
		"ExternalIDs",
		"ExternalIP",
		"ExternalMAC",
		"ExternalPortRange",
		"LogicalIP",
		"LogicalPort",
		"Options",
		"Type",
		"ExternalIDsMeta",
		"OptionsMeta",
	}
}

func (obj *NAT) MatchBool(key string, predicate getter.BoolPredicate) bool {
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
	case "OptionsMeta":
		if index != -1 && obj.OptionsMeta != nil {
			return obj.OptionsMeta.MatchBool(key[index+1:], predicate)
		}
	}
	return false
}

func (obj *NAT) MatchInt64(key string, predicate getter.Int64Predicate) bool {
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
	case "OptionsMeta":
		if index != -1 && obj.OptionsMeta != nil {
			return obj.OptionsMeta.MatchInt64(key[index+1:], predicate)
		}
	}
	return false
}

func (obj *NAT) MatchString(key string, predicate getter.StringPredicate) bool {
	if b, err := obj.GetFieldString(key); err == nil {
		return predicate(b)
	}

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {

	case "AllowedExtIPs":
		if index == -1 {
			for _, i := range obj.AllowedExtIPs {
				if predicate(i) {
					return true
				}
			}
		}
	case "ExemptedExtIPs":
		if index == -1 {
			for _, i := range obj.ExemptedExtIPs {
				if predicate(i) {
					return true
				}
			}
		}
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
	case "ExternalMAC":
		if index == -1 {
			for _, i := range obj.ExternalMAC {
				if predicate(i) {
					return true
				}
			}
		}
	case "LogicalPort":
		if index == -1 {
			for _, i := range obj.LogicalPort {
				if predicate(i) {
					return true
				}
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
	case "ExternalIDsMeta":
		if index != -1 && obj.ExternalIDsMeta != nil {
			return obj.ExternalIDsMeta.MatchString(key[index+1:], predicate)
		}
	case "OptionsMeta":
		if index != -1 && obj.OptionsMeta != nil {
			return obj.OptionsMeta.MatchString(key[index+1:], predicate)
		}
	}
	return false
}

func (obj *NAT) GetField(key string) (interface{}, error) {
	if s, err := obj.GetFieldString(key); err == nil {
		return s, nil
	}

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {
	case "AllowedExtIPs":
		if obj.AllowedExtIPs != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.AllowedExtIPs {
					results = append(results, obj)
				}
				return results, nil
			}
		}
	case "ExemptedExtIPs":
		if obj.ExemptedExtIPs != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.ExemptedExtIPs {
					results = append(results, obj)
				}
				return results, nil
			}
		}
	case "ExternalMAC":
		if obj.ExternalMAC != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.ExternalMAC {
					results = append(results, obj)
				}
				return results, nil
			}
		}
	case "LogicalPort":
		if obj.LogicalPort != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.LogicalPort {
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
	case "OptionsMeta":
		if obj.OptionsMeta != nil {
			if index != -1 {
				return obj.OptionsMeta.GetField(key[index+1:])
			} else {
				return obj.OptionsMeta, nil
			}
		}

	}
	return nil, getter.ErrFieldNotFound
}

func init() {
	strings.Index("", ".")
}
