// Code generated - DO NOT EDIT.

package ovnmodel

import (
	"github.com/skydive-project/skydive/graffiti/getter"
	"strings"
)

func (obj *LogicalSwitchPort) GetFieldBool(key string) (bool, error) {
	return false, getter.ErrFieldNotFound
}

func (obj *LogicalSwitchPort) GetFieldInt64(key string) (int64, error) {
	return 0, getter.ErrFieldNotFound
}

func (obj *LogicalSwitchPort) GetFieldString(key string) (string, error) {
	switch key {
	case "UUID":
		return string(obj.UUID), nil
	case "Name":
		return string(obj.Name), nil
	case "Type":
		return string(obj.Type), nil
	}
	return "", getter.ErrFieldNotFound
}

func (obj *LogicalSwitchPort) GetFieldKeys() []string {
	return []string{
		"UUID",
		"Addresses",
		"Dhcpv4Options",
		"Dhcpv6Options",
		"DynamicAddresses",
		"Enabled",
		"ExternalIDs",
		"HaChassisGroup",
		"Name",
		"Options",
		"ParentName",
		"PortSecurity",
		"Tag",
		"TagRequest",
		"Type",
		"Up",
		"ExternalIDsMeta",
		"OptionsMeta",
	}
}

func (obj *LogicalSwitchPort) MatchBool(key string, predicate getter.BoolPredicate) bool {
	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {
	case "Enabled":
		if index == -1 {
			for _, i := range obj.Enabled {
				if predicate(i) {
					return true
				}
			}
		}
	case "Up":
		if index == -1 {
			for _, i := range obj.Up {
				if predicate(i) {
					return true
				}
			}
		}
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

func (obj *LogicalSwitchPort) MatchInt64(key string, predicate getter.Int64Predicate) bool {
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

func (obj *LogicalSwitchPort) MatchString(key string, predicate getter.StringPredicate) bool {
	if b, err := obj.GetFieldString(key); err == nil {
		return predicate(b)
	}

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {

	case "Addresses":
		if index == -1 {
			for _, i := range obj.Addresses {
				if predicate(i) {
					return true
				}
			}
		}
	case "Dhcpv4Options":
		if index == -1 {
			for _, i := range obj.Dhcpv4Options {
				if predicate(i) {
					return true
				}
			}
		}
	case "Dhcpv6Options":
		if index == -1 {
			for _, i := range obj.Dhcpv6Options {
				if predicate(i) {
					return true
				}
			}
		}
	case "DynamicAddresses":
		if index == -1 {
			for _, i := range obj.DynamicAddresses {
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
	case "HaChassisGroup":
		if index == -1 {
			for _, i := range obj.HaChassisGroup {
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
	case "ParentName":
		if index == -1 {
			for _, i := range obj.ParentName {
				if predicate(i) {
					return true
				}
			}
		}
	case "PortSecurity":
		if index == -1 {
			for _, i := range obj.PortSecurity {
				if predicate(i) {
					return true
				}
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

func (obj *LogicalSwitchPort) GetField(key string) (interface{}, error) {
	if s, err := obj.GetFieldString(key); err == nil {
		return s, nil
	}

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {
	case "Addresses":
		if obj.Addresses != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.Addresses {
					results = append(results, obj)
				}
				return results, nil
			}
		}
	case "Dhcpv4Options":
		if obj.Dhcpv4Options != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.Dhcpv4Options {
					results = append(results, obj)
				}
				return results, nil
			}
		}
	case "Dhcpv6Options":
		if obj.Dhcpv6Options != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.Dhcpv6Options {
					results = append(results, obj)
				}
				return results, nil
			}
		}
	case "DynamicAddresses":
		if obj.DynamicAddresses != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.DynamicAddresses {
					results = append(results, obj)
				}
				return results, nil
			}
		}
	case "Enabled":
		if obj.Enabled != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.Enabled {
					results = append(results, obj)
				}
				return results, nil
			}
		}
	case "HaChassisGroup":
		if obj.HaChassisGroup != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.HaChassisGroup {
					results = append(results, obj)
				}
				return results, nil
			}
		}
	case "ParentName":
		if obj.ParentName != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.ParentName {
					results = append(results, obj)
				}
				return results, nil
			}
		}
	case "PortSecurity":
		if obj.PortSecurity != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.PortSecurity {
					results = append(results, obj)
				}
				return results, nil
			}
		}
	case "Tag":
		if obj.Tag != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.Tag {
					results = append(results, obj)
				}
				return results, nil
			}
		}
	case "TagRequest":
		if obj.TagRequest != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.TagRequest {
					results = append(results, obj)
				}
				return results, nil
			}
		}
	case "Up":
		if obj.Up != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.Up {
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
