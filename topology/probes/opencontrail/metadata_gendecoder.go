// Code generated - DO NOT EDIT.

package opencontrail

import (
	"github.com/skydive-project/skydive/graffiti/getter"
	"strings"
)

func (obj *Metadata) GetFieldBool(key string) (bool, error) {
	return false, getter.ErrFieldNotFound
}

func (obj *Metadata) GetFieldInt64(key string) (int64, error) {
	switch key {
	case "VRFID":
		return int64(obj.VRFID), nil
	}
	return 0, getter.ErrFieldNotFound
}

func (obj *Metadata) GetFieldString(key string) (string, error) {
	switch key {
	case "UUID":
		return string(obj.UUID), nil
	case "MAC":
		return string(obj.MAC), nil
	case "VRF":
		return string(obj.VRF), nil
	case "LocalIP":
		return string(obj.LocalIP), nil
	}
	return "", getter.ErrFieldNotFound
}

func (obj *Metadata) GetFieldKeys() []string {
	return []string{
		"UUID",
		"MAC",
		"VRF",
		"VRFID",
		"LocalIP",
		"RoutingTable",
	}
}

func (obj *Metadata) MatchBool(key string, predicate getter.BoolPredicate) bool {
	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {
	case "RoutingTable":
		if index != -1 {
			for _, obj := range obj.RoutingTable {
				if obj.MatchBool(key[index+1:], predicate) {
					return true
				}
			}
		}
	}
	return false
}

func (obj *Metadata) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	if b, err := obj.GetFieldInt64(key); err == nil {
		return predicate(b)
	}

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {

	case "RoutingTable":
		if index != -1 {
			for _, obj := range obj.RoutingTable {
				if obj.MatchInt64(key[index+1:], predicate) {
					return true
				}
			}
		}
	}
	return false
}

func (obj *Metadata) MatchString(key string, predicate getter.StringPredicate) bool {
	if b, err := obj.GetFieldString(key); err == nil {
		return predicate(b)
	}

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {

	case "RoutingTable":
		if index != -1 {
			for _, obj := range obj.RoutingTable {
				if obj.MatchString(key[index+1:], predicate) {
					return true
				}
			}
		}
	}
	return false
}

func (obj *Metadata) GetField(key string) (interface{}, error) {
	if s, err := obj.GetFieldString(key); err == nil {
		return s, nil
	}

	if i, err := obj.GetFieldInt64(key); err == nil {
		return i, nil
	}

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {
	case "RoutingTable":
		if obj.RoutingTable != nil {
			if index != -1 {
				var results []interface{}
				for _, obj := range obj.RoutingTable {
					if field, err := obj.GetField(key[index+1:]); err == nil {
						results = append(results, field)
					}
				}
				return results, nil
			} else {
				var results []interface{}
				for _, obj := range obj.RoutingTable {
					results = append(results, obj)
				}
				return results, nil
			}
		}

	}
	return nil, getter.ErrFieldNotFound
}

func (obj *Route) GetFieldBool(key string) (bool, error) {
	return false, getter.ErrFieldNotFound
}

func (obj *Route) GetFieldInt64(key string) (int64, error) {
	switch key {
	case "NhID":
		return int64(obj.NhID), nil
	case "Protocol":
		return int64(obj.Protocol), nil
	}
	return 0, getter.ErrFieldNotFound
}

func (obj *Route) GetFieldString(key string) (string, error) {
	switch key {
	case "Family":
		return string(obj.Family), nil
	case "Prefix":
		return string(obj.Prefix), nil
	}
	return "", getter.ErrFieldNotFound
}

func (obj *Route) GetFieldKeys() []string {
	return []string{
		"Family",
		"Prefix",
		"NhID",
		"Protocol",
	}
}

func (obj *Route) MatchBool(key string, predicate getter.BoolPredicate) bool {
	return false
}

func (obj *Route) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	if b, err := obj.GetFieldInt64(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *Route) MatchString(key string, predicate getter.StringPredicate) bool {
	if b, err := obj.GetFieldString(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *Route) GetField(key string) (interface{}, error) {
	if s, err := obj.GetFieldString(key); err == nil {
		return s, nil
	}

	if i, err := obj.GetFieldInt64(key); err == nil {
		return i, nil
	}
	return nil, getter.ErrFieldNotFound
}

func (obj *RoutingTable) GetFieldBool(key string) (bool, error) {
	return false, getter.ErrFieldNotFound
}

func (obj *RoutingTable) GetFieldInt64(key string) (int64, error) {
	return 0, getter.ErrFieldNotFound
}

func (obj *RoutingTable) GetFieldString(key string) (string, error) {
	return "", getter.ErrFieldNotFound
}

func (obj *RoutingTable) GetFieldKeys() []string {
	return []string{
		"InterfacesUUID",
		"Routes",
	}
}

func (obj *RoutingTable) MatchBool(key string, predicate getter.BoolPredicate) bool {
	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {
	case "Routes":
		if index != -1 {
			for _, obj := range obj.Routes {
				if obj.MatchBool(key[index+1:], predicate) {
					return true
				}
			}
		}
	}
	return false
}

func (obj *RoutingTable) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {

	case "Routes":
		if index != -1 {
			for _, obj := range obj.Routes {
				if obj.MatchInt64(key[index+1:], predicate) {
					return true
				}
			}
		}
	}
	return false
}

func (obj *RoutingTable) MatchString(key string, predicate getter.StringPredicate) bool {
	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {

	case "InterfacesUUID":
		if index == -1 {
			for _, i := range obj.InterfacesUUID {
				if predicate(i) {
					return true
				}
			}
		}
	case "Routes":
		if index != -1 {
			for _, obj := range obj.Routes {
				if obj.MatchString(key[index+1:], predicate) {
					return true
				}
			}
		}
	}
	return false
}

func (obj *RoutingTable) GetField(key string) (interface{}, error) {
	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {
	case "InterfacesUUID":
		if obj.InterfacesUUID != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.InterfacesUUID {
					results = append(results, obj)
				}
				return results, nil
			}
		}
	case "Routes":
		if obj.Routes != nil {
			if index != -1 {
				var results []interface{}
				for _, obj := range obj.Routes {
					if field, err := obj.GetField(key[index+1:]); err == nil {
						results = append(results, field)
					}
				}
				return results, nil
			} else {
				var results []interface{}
				for _, obj := range obj.Routes {
					results = append(results, obj)
				}
				return results, nil
			}
		}

	}
	return nil, getter.ErrFieldNotFound
}

func init() {
	strings.Index("", ".")
}
