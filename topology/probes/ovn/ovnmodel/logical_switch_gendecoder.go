// Code generated - DO NOT EDIT.

package ovnmodel

import (
	"github.com/skydive-project/skydive/graffiti/getter"
	"strings"
)

func (obj *LogicalSwitch) GetFieldBool(key string) (bool, error) {
	return false, getter.ErrFieldNotFound
}

func (obj *LogicalSwitch) GetFieldInt64(key string) (int64, error) {
	return 0, getter.ErrFieldNotFound
}

func (obj *LogicalSwitch) GetFieldString(key string) (string, error) {
	switch key {
	case "UUID":
		return string(obj.UUID), nil
	case "Name":
		return string(obj.Name), nil
	}
	return "", getter.ErrFieldNotFound
}

func (obj *LogicalSwitch) GetFieldKeys() []string {
	return []string{
		"UUID",
		"ACLs",
		"DNSRecords",
		"ExternalIDs",
		"ForwardingGroups",
		"LoadBalancer",
		"Name",
		"OtherConfig",
		"Ports",
		"QOSRules",
		"ExternalIDsMeta",
		"OtherConfigMeta",
	}
}

func (obj *LogicalSwitch) MatchBool(key string, predicate getter.BoolPredicate) bool {
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
	}
	return false
}

func (obj *LogicalSwitch) MatchInt64(key string, predicate getter.Int64Predicate) bool {
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
	}
	return false
}

func (obj *LogicalSwitch) MatchString(key string, predicate getter.StringPredicate) bool {
	if b, err := obj.GetFieldString(key); err == nil {
		return predicate(b)
	}

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {

	case "ACLs":
		if index == -1 {
			for _, i := range obj.ACLs {
				if predicate(i) {
					return true
				}
			}
		}
	case "DNSRecords":
		if index == -1 {
			for _, i := range obj.DNSRecords {
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
	case "ForwardingGroups":
		if index == -1 {
			for _, i := range obj.ForwardingGroups {
				if predicate(i) {
					return true
				}
			}
		}
	case "LoadBalancer":
		if index == -1 {
			for _, i := range obj.LoadBalancer {
				if predicate(i) {
					return true
				}
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
	case "Ports":
		if index == -1 {
			for _, i := range obj.Ports {
				if predicate(i) {
					return true
				}
			}
		}
	case "QOSRules":
		if index == -1 {
			for _, i := range obj.QOSRules {
				if predicate(i) {
					return true
				}
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
	}
	return false
}

func (obj *LogicalSwitch) GetField(key string) (interface{}, error) {
	if s, err := obj.GetFieldString(key); err == nil {
		return s, nil
	}

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {
	case "ACLs":
		if obj.ACLs != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.ACLs {
					results = append(results, obj)
				}
				return results, nil
			}
		}
	case "DNSRecords":
		if obj.DNSRecords != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.DNSRecords {
					results = append(results, obj)
				}
				return results, nil
			}
		}
	case "ForwardingGroups":
		if obj.ForwardingGroups != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.ForwardingGroups {
					results = append(results, obj)
				}
				return results, nil
			}
		}
	case "LoadBalancer":
		if obj.LoadBalancer != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.LoadBalancer {
					results = append(results, obj)
				}
				return results, nil
			}
		}
	case "Ports":
		if obj.Ports != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.Ports {
					results = append(results, obj)
				}
				return results, nil
			}
		}
	case "QOSRules":
		if obj.QOSRules != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.QOSRules {
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

	}
	return nil, getter.ErrFieldNotFound
}

func init() {
	strings.Index("", ".")
}
