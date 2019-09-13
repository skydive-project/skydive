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

	}
	return nil, common.ErrFieldNotFound
}

func init() {
	strings.Index("", ".")
}
