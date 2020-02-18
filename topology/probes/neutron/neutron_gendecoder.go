// Code generated - DO NOT EDIT.

package neutron

import (
	"github.com/skydive-project/skydive/graffiti/getter"
	"strings"
)

func (obj *Metadata) GetFieldBool(key string) (bool, error) {
	return false, getter.ErrFieldNotFound
}

func (obj *Metadata) GetFieldInt64(key string) (int64, error) {
	switch key {
	case "VNI":
		return int64(obj.VNI), nil
	}
	return 0, getter.ErrFieldNotFound
}

func (obj *Metadata) GetFieldString(key string) (string, error) {
	switch key {
	case "PortID":
		return string(obj.PortID), nil
	case "TenantID":
		return string(obj.TenantID), nil
	case "NetworkID":
		return string(obj.NetworkID), nil
	case "NetworkName":
		return string(obj.NetworkName), nil
	}
	return "", getter.ErrFieldNotFound
}

func (obj *Metadata) GetFieldKeys() []string {
	return []string{
		"PortID",
		"TenantID",
		"NetworkID",
		"NetworkName",
		"IPV4",
		"IPV6",
		"VNI",
	}
}

func (obj *Metadata) MatchBool(key string, predicate getter.BoolPredicate) bool {
	return false
}

func (obj *Metadata) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	if b, err := obj.GetFieldInt64(key); err == nil {
		return predicate(b)
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

	case "IPV4":
		if index == -1 {
			for _, i := range obj.IPV4 {
				if predicate(i) {
					return true
				}
			}
		}
	case "IPV6":
		if index == -1 {
			for _, i := range obj.IPV6 {
				if predicate(i) {
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
	case "IPV4":
		if obj.IPV4 != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.IPV4 {
					results = append(results, obj)
				}
				return results, nil
			}
		}
	case "IPV6":
		if obj.IPV6 != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.IPV6 {
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
