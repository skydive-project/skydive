// Code generated - DO NOT EDIT.

package topology

import (
	"github.com/skydive-project/skydive/common"
	"strings"
)

func (obj *Neighbor) GetFieldBool(key string) (bool, error) {
	return false, common.ErrFieldNotFound
}

func (obj *Neighbor) GetFieldInt64(key string) (int64, error) {
	switch key {
	case "Vlan":
		return int64(obj.Vlan), nil
	case "VNI":
		return int64(obj.VNI), nil
	case "IfIndex":
		return int64(obj.IfIndex), nil
	}
	return 0, common.ErrFieldNotFound
}

func (obj *Neighbor) GetFieldString(key string) (string, error) {
	switch key {
	case "MAC":
		return string(obj.MAC), nil
	case "IP":
		return obj.IP.String(), nil
	}
	return "", common.ErrFieldNotFound
}

func (obj *Neighbor) GetFieldKeys() []string {
	return []string{
		"Flags",
		"MAC",
		"IP",
		"State",
		"Vlan",
		"VNI",
		"IfIndex",
	}
}

func (obj *Neighbor) MatchBool(key string, predicate common.BoolPredicate) bool {
	return false
}

func (obj *Neighbor) MatchInt64(key string, predicate common.Int64Predicate) bool {
	if b, err := obj.GetFieldInt64(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *Neighbor) MatchString(key string, predicate common.StringPredicate) bool {
	if b, err := obj.GetFieldString(key); err == nil {
		return predicate(b)
	}

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {

	case "Flags":
		if index == -1 {
			for _, i := range obj.Flags {
				if predicate(i) {
					return true
				}
			}
		}
	case "State":
		if index == -1 {
			for _, i := range obj.State {
				if predicate(i) {
					return true
				}
			}
		}
	}
	return false
}

func (obj *Neighbor) GetField(key string) (interface{}, error) {
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
	case "Flags":
		if obj.Flags != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.Flags {
					results = append(results, obj)
				}
				return results, nil
			}
		}
	case "State":
		if obj.State != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.State {
					results = append(results, obj)
				}
				return results, nil
			}
		}

	}
	return nil, common.ErrFieldNotFound
}

func (obj *Neighbors) GetFieldBool(key string) (bool, error) {
	return false, common.ErrFieldNotFound
}

func (obj *Neighbors) GetFieldInt64(key string) (int64, error) {
	switch key {
	}
	return 0, common.ErrFieldNotFound
}

func (obj *Neighbors) GetFieldString(key string) (string, error) {
	switch key {
	}
	return "", common.ErrFieldNotFound
}

func (obj *Neighbors) GetFieldKeys() []string {
	return []string{
		"Flags",
		"MAC",
		"IP",
		"State",
		"Vlan",
		"VNI",
		"IfIndex",
	}
}

func (obj *Neighbors) MatchBool(key string, predicate common.BoolPredicate) bool {
	for _, obj := range *obj {
		if obj.MatchBool(key, predicate) {
			return true
		}
	}
	return false
}

func (obj *Neighbors) MatchInt64(key string, predicate common.Int64Predicate) bool {
	for _, obj := range *obj {
		if obj.MatchInt64(key, predicate) {
			return true
		}
	}
	return false
}

func (obj *Neighbors) MatchString(key string, predicate common.StringPredicate) bool {
	for _, obj := range *obj {
		if obj.MatchString(key, predicate) {
			return true
		}
	}
	return false
}

func (obj *Neighbors) GetField(key string) (interface{}, error) {
	var result []interface{}

	for _, o := range *obj {
		switch key {
		case "Flags":
			result = append(result, o.Flags)
		case "MAC":
			result = append(result, o.MAC)
		case "IP":
			result = append(result, o.IP)
		case "State":
			result = append(result, o.State)
		case "Vlan":
			result = append(result, o.Vlan)
		case "VNI":
			result = append(result, o.VNI)
		case "IfIndex":
			result = append(result, o.IfIndex)
		default:
			return result, common.ErrFieldNotFound
		}
	}

	return result, nil
}

func init() {
	strings.Index("", ".")
}
