// Code generated - DO NOT EDIT.

package topology

import (
	"github.com/skydive-project/skydive/common"
	"strings"
)

func (obj *Route) GetFieldBool(key string) (bool, error) {
	return false, common.ErrFieldNotFound
}

func (obj *Route) GetFieldInt64(key string) (int64, error) {
	switch key {
	case "Protocol":
		return int64(obj.Protocol), nil
	}
	return 0, common.ErrFieldNotFound
}

func (obj *Route) GetFieldString(key string) (string, error) {
	switch key {
	case "Prefix":
		return obj.Prefix.String(), nil
	}
	return "", common.ErrFieldNotFound
}

func (obj *Route) GetFieldKeys() []string {
	return []string{
		"Protocol",
		"Prefix",
		"NextHops",
	}
}

func (obj *Route) MatchBool(key string, predicate common.BoolPredicate) bool {
	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {
	case "NextHops":
		if index != -1 {
			for _, obj := range obj.NextHops {
				if obj.MatchBool(key[index+1:], predicate) {
					return true
				}
			}
		}
	}
	return false
}

func (obj *Route) MatchInt64(key string, predicate common.Int64Predicate) bool {
	if b, err := obj.GetFieldInt64(key); err == nil {
		return predicate(b)
	}

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {

	case "NextHops":
		if index != -1 {
			for _, obj := range obj.NextHops {
				if obj.MatchInt64(key[index+1:], predicate) {
					return true
				}
			}
		}
	}
	return false
}

func (obj *Route) MatchString(key string, predicate common.StringPredicate) bool {
	if b, err := obj.GetFieldString(key); err == nil {
		return predicate(b)
	}

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {

	case "NextHops":
		if index != -1 {
			for _, obj := range obj.NextHops {
				if obj.MatchString(key[index+1:], predicate) {
					return true
				}
			}
		}
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

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {
	case "NextHops":
		if obj.NextHops != nil {
			if index != -1 {
				var results []interface{}
				for _, obj := range obj.NextHops {
					if field, err := obj.GetField(key[index+1:]); err == nil {
						results = append(results, field)
					}
				}
				return results, nil
			} else {
				var results []interface{}
				for _, obj := range obj.NextHops {
					results = append(results, obj)
				}
				return results, nil
			}
		}

	}
	return nil, common.ErrFieldNotFound
}

func (obj *RoutingTable) GetFieldBool(key string) (bool, error) {
	return false, common.ErrFieldNotFound
}

func (obj *RoutingTable) GetFieldInt64(key string) (int64, error) {
	switch key {
	case "ID":
		return int64(obj.ID), nil
	}
	return 0, common.ErrFieldNotFound
}

func (obj *RoutingTable) GetFieldString(key string) (string, error) {
	switch key {
	case "Src":
		return obj.Src.String(), nil
	}
	return "", common.ErrFieldNotFound
}

func (obj *RoutingTable) GetFieldKeys() []string {
	return []string{
		"ID",
		"Src",
		"Routes",
	}
}

func (obj *RoutingTable) MatchBool(key string, predicate common.BoolPredicate) bool {
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

func (obj *RoutingTable) MatchInt64(key string, predicate common.Int64Predicate) bool {
	if b, err := obj.GetFieldInt64(key); err == nil {
		return predicate(b)
	}

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

func (obj *RoutingTable) MatchString(key string, predicate common.StringPredicate) bool {
	if b, err := obj.GetFieldString(key); err == nil {
		return predicate(b)
	}

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {

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
	return nil, common.ErrFieldNotFound
}

func (obj *RoutingTables) GetFieldBool(key string) (bool, error) {
	return false, common.ErrFieldNotFound
}

func (obj *RoutingTables) GetFieldInt64(key string) (int64, error) {
	switch key {
	}
	return 0, common.ErrFieldNotFound
}

func (obj *RoutingTables) GetFieldString(key string) (string, error) {
	switch key {
	}
	return "", common.ErrFieldNotFound
}

func (obj *RoutingTables) GetFieldKeys() []string {
	return []string{
		"ID",
		"Src",
		"Routes",
	}
}

func (obj *RoutingTables) MatchBool(key string, predicate common.BoolPredicate) bool {
	for _, obj := range *obj {
		if obj.MatchBool(key, predicate) {
			return true
		}
	}
	return false
}

func (obj *RoutingTables) MatchInt64(key string, predicate common.Int64Predicate) bool {
	for _, obj := range *obj {
		if obj.MatchInt64(key, predicate) {
			return true
		}
	}
	return false
}

func (obj *RoutingTables) MatchString(key string, predicate common.StringPredicate) bool {
	for _, obj := range *obj {
		if obj.MatchString(key, predicate) {
			return true
		}
	}
	return false
}

func (obj *RoutingTables) GetField(key string) (interface{}, error) {
	var result []interface{}

	for _, o := range *obj {
		switch key {
		case "ID":
			result = append(result, o.ID)
		case "Src":
			result = append(result, o.Src)
		case "Routes":
			result = append(result, o.Routes)
		default:
			return result, common.ErrFieldNotFound
		}
	}

	return result, nil
}

func init() {
	strings.Index("", ".")
}
