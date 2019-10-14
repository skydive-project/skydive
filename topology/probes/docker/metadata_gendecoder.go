// Code generated - DO NOT EDIT.

package docker

import (
	"github.com/skydive-project/skydive/common"
	"strings"
)

func (obj *Metadata) GetFieldBool(key string) (bool, error) {
	return false, common.ErrFieldNotFound
}

func (obj *Metadata) GetFieldInt64(key string) (int64, error) {
	return 0, common.ErrFieldNotFound
}

func (obj *Metadata) GetFieldString(key string) (string, error) {
	switch key {
	case "ContainerID":
		return string(obj.ContainerID), nil
	case "ContainerName":
		return string(obj.ContainerName), nil
	}
	return "", common.ErrFieldNotFound
}

func (obj *Metadata) GetFieldKeys() []string {
	return []string{
		"ContainerID",
		"ContainerName",
		"Labels",
		"Mounts",
	}
}

func (obj *Metadata) MatchBool(key string, predicate common.BoolPredicate) bool {
	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {
	case "Labels":
		if index != -1 && obj.Labels != nil {
			return obj.Labels.MatchBool(key[index+1:], predicate)
		}
	case "Mounts":
		if index != -1 {
			for _, obj := range obj.Mounts {
				if obj.MatchBool(key[index+1:], predicate) {
					return true
				}
			}
		}
	}
	return false
}

func (obj *Metadata) MatchInt64(key string, predicate common.Int64Predicate) bool {
	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {

	case "Labels":
		if index != -1 && obj.Labels != nil {
			return obj.Labels.MatchInt64(key[index+1:], predicate)
		}
	case "Mounts":
		if index != -1 {
			for _, obj := range obj.Mounts {
				if obj.MatchInt64(key[index+1:], predicate) {
					return true
				}
			}
		}
	}
	return false
}

func (obj *Metadata) MatchString(key string, predicate common.StringPredicate) bool {
	if b, err := obj.GetFieldString(key); err == nil {
		return predicate(b)
	}

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {

	case "Labels":
		if index != -1 && obj.Labels != nil {
			return obj.Labels.MatchString(key[index+1:], predicate)
		}
	case "Mounts":
		if index != -1 {
			for _, obj := range obj.Mounts {
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

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {
	case "Labels":
		if obj.Labels != nil {
			if index != -1 {
				return obj.Labels.GetField(key[index+1:])
			} else {
				return obj.Labels, nil
			}
		}
	case "Mounts":
		if obj.Mounts != nil {
			if index != -1 {
				var results []interface{}
				for _, obj := range obj.Mounts {
					if field, err := obj.GetField(key[index+1:]); err == nil {
						results = append(results, field)
					}
				}
				return results, nil
			} else {
				var results []interface{}
				for _, obj := range obj.Mounts {
					results = append(results, obj)
				}
				return results, nil
			}
		}

	}
	return nil, common.ErrFieldNotFound
}

func (obj *Mount) GetFieldBool(key string) (bool, error) {
	return false, common.ErrFieldNotFound
}

func (obj *Mount) GetFieldInt64(key string) (int64, error) {
	return 0, common.ErrFieldNotFound
}

func (obj *Mount) GetFieldString(key string) (string, error) {
	switch key {
	case "Source":
		return string(obj.Source), nil
	case "Destination":
		return string(obj.Destination), nil
	}
	return "", common.ErrFieldNotFound
}

func (obj *Mount) GetFieldKeys() []string {
	return []string{
		"Source",
		"Destination",
	}
}

func (obj *Mount) MatchBool(key string, predicate common.BoolPredicate) bool {
	return false
}

func (obj *Mount) MatchInt64(key string, predicate common.Int64Predicate) bool {
	return false
}

func (obj *Mount) MatchString(key string, predicate common.StringPredicate) bool {
	if b, err := obj.GetFieldString(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *Mount) GetField(key string) (interface{}, error) {
	if s, err := obj.GetFieldString(key); err == nil {
		return s, nil
	}
	return nil, common.ErrFieldNotFound
}

func init() {
	strings.Index("", ".")
}
