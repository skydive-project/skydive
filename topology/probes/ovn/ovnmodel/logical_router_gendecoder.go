// Code generated - DO NOT EDIT.

package ovnmodel

import (
	"github.com/skydive-project/skydive/graffiti/getter"
	"strings"
)

func (obj *LogicalRouter) GetFieldBool(key string) (bool, error) {
	return false, getter.ErrFieldNotFound
}

func (obj *LogicalRouter) GetFieldInt64(key string) (int64, error) {
	return 0, getter.ErrFieldNotFound
}

func (obj *LogicalRouter) GetFieldString(key string) (string, error) {
	switch key {
	case "UUID":
		return string(obj.UUID), nil
	case "Name":
		return string(obj.Name), nil
	}
	return "", getter.ErrFieldNotFound
}

func (obj *LogicalRouter) GetFieldKeys() []string {
	return []string{
		"UUID",
		"Enabled",
		"ExternalIDs",
		"LoadBalancer",
		"Name",
		"Nat",
		"Options",
		"Policies",
		"Ports",
		"StaticRoutes",
		"ExternalIDsMeta",
		"OptionsMeta",
	}
}

func (obj *LogicalRouter) MatchBool(key string, predicate getter.BoolPredicate) bool {
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
	case "OptionsMeta":
		if index != -1 && obj.OptionsMeta != nil {
			return obj.OptionsMeta.MatchBool(key[index+1:], predicate)
		}
	}
	return false
}

func (obj *LogicalRouter) MatchInt64(key string, predicate getter.Int64Predicate) bool {
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

func (obj *LogicalRouter) MatchString(key string, predicate getter.StringPredicate) bool {
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
	case "LoadBalancer":
		if index == -1 {
			for _, i := range obj.LoadBalancer {
				if predicate(i) {
					return true
				}
			}
		}
	case "Nat":
		if index == -1 {
			for _, i := range obj.Nat {
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
	case "Policies":
		if index == -1 {
			for _, i := range obj.Policies {
				if predicate(i) {
					return true
				}
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
	case "StaticRoutes":
		if index == -1 {
			for _, i := range obj.StaticRoutes {
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

func (obj *LogicalRouter) GetField(key string) (interface{}, error) {
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
	case "Nat":
		if obj.Nat != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.Nat {
					results = append(results, obj)
				}
				return results, nil
			}
		}
	case "Policies":
		if obj.Policies != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.Policies {
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
	case "StaticRoutes":
		if obj.StaticRoutes != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.StaticRoutes {
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
