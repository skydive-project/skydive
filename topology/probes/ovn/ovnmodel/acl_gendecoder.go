// Code generated - DO NOT EDIT.

package ovnmodel

import (
	"github.com/skydive-project/skydive/graffiti/getter"
	"strings"
)

func (obj *ACL) GetFieldBool(key string) (bool, error) {
	switch key {
	case "Log":
		return obj.Log, nil
	}

	return false, getter.ErrFieldNotFound
}

func (obj *ACL) GetFieldInt64(key string) (int64, error) {
	switch key {
	case "Priority":
		return int64(obj.Priority), nil
	}
	return 0, getter.ErrFieldNotFound
}

func (obj *ACL) GetFieldString(key string) (string, error) {
	switch key {
	case "UUID":
		return string(obj.UUID), nil
	case "Action":
		return string(obj.Action), nil
	case "Direction":
		return string(obj.Direction), nil
	case "Match":
		return string(obj.Match), nil
	}
	return "", getter.ErrFieldNotFound
}

func (obj *ACL) GetFieldKeys() []string {
	return []string{
		"UUID",
		"Action",
		"Direction",
		"ExternalIDs",
		"Log",
		"Match",
		"Meter",
		"Name",
		"Priority",
		"Severity",
		"ExternalIDsMeta",
	}
}

func (obj *ACL) MatchBool(key string, predicate getter.BoolPredicate) bool {
	if b, err := obj.GetFieldBool(key); err == nil {
		return predicate(b)
	}

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
	}
	return false
}

func (obj *ACL) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	if b, err := obj.GetFieldInt64(key); err == nil {
		return predicate(b)
	}

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
	}
	return false
}

func (obj *ACL) MatchString(key string, predicate getter.StringPredicate) bool {
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
	case "Meter":
		if index == -1 {
			for _, i := range obj.Meter {
				if predicate(i) {
					return true
				}
			}
		}
	case "Name":
		if index == -1 {
			for _, i := range obj.Name {
				if predicate(i) {
					return true
				}
			}
		}
	case "Severity":
		if index == -1 {
			for _, i := range obj.Severity {
				if predicate(i) {
					return true
				}
			}
		}
	case "ExternalIDsMeta":
		if index != -1 && obj.ExternalIDsMeta != nil {
			return obj.ExternalIDsMeta.MatchString(key[index+1:], predicate)
		}
	}
	return false
}

func (obj *ACL) GetField(key string) (interface{}, error) {
	if s, err := obj.GetFieldString(key); err == nil {
		return s, nil
	}

	if i, err := obj.GetFieldInt64(key); err == nil {
		return i, nil
	}

	if b, err := obj.GetFieldBool(key); err == nil {
		return b, nil
	}

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {
	case "Meter":
		if obj.Meter != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.Meter {
					results = append(results, obj)
				}
				return results, nil
			}
		}
	case "Name":
		if obj.Name != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.Name {
					results = append(results, obj)
				}
				return results, nil
			}
		}
	case "Severity":
		if obj.Severity != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.Severity {
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

	}
	return nil, getter.ErrFieldNotFound
}

func init() {
	strings.Index("", ".")
}
