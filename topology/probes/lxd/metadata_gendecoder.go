// Code generated - DO NOT EDIT.

package lxd

import (
	"github.com/skydive-project/skydive/graffiti/getter"
	"strings"
)

func (obj *Metadata) GetFieldBool(key string) (bool, error) {
	return false, getter.ErrFieldNotFound
}

func (obj *Metadata) GetFieldInt64(key string) (int64, error) {
	return 0, getter.ErrFieldNotFound
}

func (obj *Metadata) GetFieldString(key string) (string, error) {
	switch key {
	case "Architecture":
		return string(obj.Architecture), nil
	case "CreatedAt":
		return string(obj.CreatedAt), nil
	case "Description":
		return string(obj.Description), nil
	case "Ephemeral":
		return string(obj.Ephemeral), nil
	case "Restore":
		return string(obj.Restore), nil
	case "Status":
		return string(obj.Status), nil
	case "Stateful":
		return string(obj.Stateful), nil
	}
	return "", getter.ErrFieldNotFound
}

func (obj *Metadata) GetFieldKeys() []string {
	return []string{
		"Architecture",
		"Config",
		"CreatedAt",
		"Description",
		"Devices",
		"Ephemeral",
		"Profiles",
		"Restore",
		"Status",
		"Stateful",
	}
}

func (obj *Metadata) MatchBool(key string, predicate getter.BoolPredicate) bool {
	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {
	case "Config":
		if index != -1 && obj.Config != nil {
			return obj.Config.MatchBool(key[index+1:], predicate)
		}
	case "Devices":
		if index != -1 && obj.Devices != nil {
			return obj.Devices.MatchBool(key[index+1:], predicate)
		}
	}
	return false
}

func (obj *Metadata) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {

	case "Config":
		if index != -1 && obj.Config != nil {
			return obj.Config.MatchInt64(key[index+1:], predicate)
		}
	case "Devices":
		if index != -1 && obj.Devices != nil {
			return obj.Devices.MatchInt64(key[index+1:], predicate)
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

	case "Config":
		if index != -1 && obj.Config != nil {
			return obj.Config.MatchString(key[index+1:], predicate)
		}
	case "Devices":
		if index != -1 && obj.Devices != nil {
			return obj.Devices.MatchString(key[index+1:], predicate)
		}
	case "Profiles":
		if index == -1 {
			for _, i := range obj.Profiles {
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

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {
	case "Config":
		if obj.Config != nil {
			if index != -1 {
				return obj.Config.GetField(key[index+1:])
			} else {
				return obj.Config, nil
			}
		}
	case "Devices":
		if obj.Devices != nil {
			if index != -1 {
				return obj.Devices.GetField(key[index+1:])
			} else {
				return obj.Devices, nil
			}
		}
	case "Profiles":
		if obj.Profiles != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.Profiles {
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
