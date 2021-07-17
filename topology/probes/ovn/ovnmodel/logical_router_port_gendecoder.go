// Code generated - DO NOT EDIT.

package ovnmodel

import (
	"github.com/skydive-project/skydive/graffiti/getter"
	"strings"
)

func (obj *LogicalRouterPort) GetFieldBool(key string) (bool, error) {
	return false, getter.ErrFieldNotFound
}

func (obj *LogicalRouterPort) GetFieldInt64(key string) (int64, error) {
	return 0, getter.ErrFieldNotFound
}

func (obj *LogicalRouterPort) GetFieldString(key string) (string, error) {
	switch key {
	case "UUID":
		return string(obj.UUID), nil
	case "MAC":
		return string(obj.MAC), nil
	case "Name":
		return string(obj.Name), nil
	}
	return "", getter.ErrFieldNotFound
}

func (obj *LogicalRouterPort) GetFieldKeys() []string {
	return []string{
		"UUID",
		"Enabled",
		"ExternalIDs",
		"GatewayChassis",
		"HaChassisGroup",
		"Ipv6Prefix",
		"Ipv6RaConfigs",
		"MAC",
		"Name",
		"Networks",
		"Options",
		"Peer",
		"ExternalIDsMeta",
		"Ipv6RaConfigsMeta",
		"OptionsMeta",
	}
}

func (obj *LogicalRouterPort) MatchBool(key string, predicate getter.BoolPredicate) bool {
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
	case "ExternalIDsMeta":
		if index != -1 && obj.ExternalIDsMeta != nil {
			return obj.ExternalIDsMeta.MatchBool(key[index+1:], predicate)
		}
	case "Ipv6RaConfigsMeta":
		if index != -1 && obj.Ipv6RaConfigsMeta != nil {
			return obj.Ipv6RaConfigsMeta.MatchBool(key[index+1:], predicate)
		}
	case "OptionsMeta":
		if index != -1 && obj.OptionsMeta != nil {
			return obj.OptionsMeta.MatchBool(key[index+1:], predicate)
		}
	}
	return false
}

func (obj *LogicalRouterPort) MatchInt64(key string, predicate getter.Int64Predicate) bool {
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
	case "Ipv6RaConfigsMeta":
		if index != -1 && obj.Ipv6RaConfigsMeta != nil {
			return obj.Ipv6RaConfigsMeta.MatchInt64(key[index+1:], predicate)
		}
	case "OptionsMeta":
		if index != -1 && obj.OptionsMeta != nil {
			return obj.OptionsMeta.MatchInt64(key[index+1:], predicate)
		}
	}
	return false
}

func (obj *LogicalRouterPort) MatchString(key string, predicate getter.StringPredicate) bool {
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
	case "GatewayChassis":
		if index == -1 {
			for _, i := range obj.GatewayChassis {
				if predicate(i) {
					return true
				}
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
	case "Ipv6Prefix":
		if index == -1 {
			for _, i := range obj.Ipv6Prefix {
				if predicate(i) {
					return true
				}
			}
		}
	case "Ipv6RaConfigs":
		if obj.Ipv6RaConfigs != nil {
			if index == -1 {
				for _, v := range obj.Ipv6RaConfigs {
					if predicate(v) {
						return true
					}
				}
			} else if v, found := obj.Ipv6RaConfigs[key[index+1:]]; found {
				return predicate(v)
			}
		}
	case "Networks":
		if index == -1 {
			for _, i := range obj.Networks {
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
	case "Peer":
		if index == -1 {
			for _, i := range obj.Peer {
				if predicate(i) {
					return true
				}
			}
		}
	case "ExternalIDsMeta":
		if index != -1 && obj.ExternalIDsMeta != nil {
			return obj.ExternalIDsMeta.MatchString(key[index+1:], predicate)
		}
	case "Ipv6RaConfigsMeta":
		if index != -1 && obj.Ipv6RaConfigsMeta != nil {
			return obj.Ipv6RaConfigsMeta.MatchString(key[index+1:], predicate)
		}
	case "OptionsMeta":
		if index != -1 && obj.OptionsMeta != nil {
			return obj.OptionsMeta.MatchString(key[index+1:], predicate)
		}
	}
	return false
}

func (obj *LogicalRouterPort) GetField(key string) (interface{}, error) {
	if s, err := obj.GetFieldString(key); err == nil {
		return s, nil
	}

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {
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
	case "GatewayChassis":
		if obj.GatewayChassis != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.GatewayChassis {
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
	case "Ipv6Prefix":
		if obj.Ipv6Prefix != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.Ipv6Prefix {
					results = append(results, obj)
				}
				return results, nil
			}
		}
	case "Networks":
		if obj.Networks != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.Networks {
					results = append(results, obj)
				}
				return results, nil
			}
		}
	case "Peer":
		if obj.Peer != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.Peer {
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
	case "Ipv6RaConfigsMeta":
		if obj.Ipv6RaConfigsMeta != nil {
			if index != -1 {
				return obj.Ipv6RaConfigsMeta.GetField(key[index+1:])
			} else {
				return obj.Ipv6RaConfigsMeta, nil
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
