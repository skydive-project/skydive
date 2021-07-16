// Code generated - DO NOT EDIT.

package ovnmodel

import (
	"github.com/skydive-project/skydive/graffiti/getter"
	"strings"
)

func (obj *LoadBalancer) GetFieldBool(key string) (bool, error) {
	return false, getter.ErrFieldNotFound
}

func (obj *LoadBalancer) GetFieldInt64(key string) (int64, error) {
	return 0, getter.ErrFieldNotFound
}

func (obj *LoadBalancer) GetFieldString(key string) (string, error) {
	switch key {
	case "UUID":
		return string(obj.UUID), nil
	case "Name":
		return string(obj.Name), nil
	}
	return "", getter.ErrFieldNotFound
}

func (obj *LoadBalancer) GetFieldKeys() []string {
	return []string{
		"UUID",
		"ExternalIDs",
		"HealthCheck",
		"IPPortMappings",
		"Name",
		"Protocol",
		"SelectionFields",
		"Vips",
		"ExternalIDsMeta",
		"IPPortMappingsMeta",
		"VipsMeta",
	}
}

func (obj *LoadBalancer) MatchBool(key string, predicate getter.BoolPredicate) bool {
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
	case "IPPortMappingsMeta":
		if index != -1 && obj.IPPortMappingsMeta != nil {
			return obj.IPPortMappingsMeta.MatchBool(key[index+1:], predicate)
		}
	case "VipsMeta":
		if index != -1 && obj.VipsMeta != nil {
			return obj.VipsMeta.MatchBool(key[index+1:], predicate)
		}
	}
	return false
}

func (obj *LoadBalancer) MatchInt64(key string, predicate getter.Int64Predicate) bool {
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
	case "IPPortMappingsMeta":
		if index != -1 && obj.IPPortMappingsMeta != nil {
			return obj.IPPortMappingsMeta.MatchInt64(key[index+1:], predicate)
		}
	case "VipsMeta":
		if index != -1 && obj.VipsMeta != nil {
			return obj.VipsMeta.MatchInt64(key[index+1:], predicate)
		}
	}
	return false
}

func (obj *LoadBalancer) MatchString(key string, predicate getter.StringPredicate) bool {
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
	case "HealthCheck":
		if index == -1 {
			for _, i := range obj.HealthCheck {
				if predicate(i) {
					return true
				}
			}
		}
	case "IPPortMappings":
		if obj.IPPortMappings != nil {
			if index == -1 {
				for _, v := range obj.IPPortMappings {
					if predicate(v) {
						return true
					}
				}
			} else if v, found := obj.IPPortMappings[key[index+1:]]; found {
				return predicate(v)
			}
		}
	case "Protocol":
		if index == -1 {
			for _, i := range obj.Protocol {
				if predicate(i) {
					return true
				}
			}
		}
	case "SelectionFields":
		if index == -1 {
			for _, i := range obj.SelectionFields {
				if predicate(i) {
					return true
				}
			}
		}
	case "Vips":
		if obj.Vips != nil {
			if index == -1 {
				for _, v := range obj.Vips {
					if predicate(v) {
						return true
					}
				}
			} else if v, found := obj.Vips[key[index+1:]]; found {
				return predicate(v)
			}
		}
	case "ExternalIDsMeta":
		if index != -1 && obj.ExternalIDsMeta != nil {
			return obj.ExternalIDsMeta.MatchString(key[index+1:], predicate)
		}
	case "IPPortMappingsMeta":
		if index != -1 && obj.IPPortMappingsMeta != nil {
			return obj.IPPortMappingsMeta.MatchString(key[index+1:], predicate)
		}
	case "VipsMeta":
		if index != -1 && obj.VipsMeta != nil {
			return obj.VipsMeta.MatchString(key[index+1:], predicate)
		}
	}
	return false
}

func (obj *LoadBalancer) GetField(key string) (interface{}, error) {
	if s, err := obj.GetFieldString(key); err == nil {
		return s, nil
	}

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {
	case "HealthCheck":
		if obj.HealthCheck != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.HealthCheck {
					results = append(results, obj)
				}
				return results, nil
			}
		}
	case "Protocol":
		if obj.Protocol != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.Protocol {
					results = append(results, obj)
				}
				return results, nil
			}
		}
	case "SelectionFields":
		if obj.SelectionFields != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.SelectionFields {
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
	case "IPPortMappingsMeta":
		if obj.IPPortMappingsMeta != nil {
			if index != -1 {
				return obj.IPPortMappingsMeta.GetField(key[index+1:])
			} else {
				return obj.IPPortMappingsMeta, nil
			}
		}
	case "VipsMeta":
		if obj.VipsMeta != nil {
			if index != -1 {
				return obj.VipsMeta.GetField(key[index+1:])
			} else {
				return obj.VipsMeta, nil
			}
		}

	}
	return nil, getter.ErrFieldNotFound
}

func init() {
	strings.Index("", ".")
}
