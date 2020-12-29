// Code generated - DO NOT EDIT.

package topology

import (
	"github.com/skydive-project/skydive/graffiti/getter"
	"strings"
)

func (obj *ContainerMetadata) GetFieldBool(key string) (bool, error) {
	return false, getter.ErrFieldNotFound
}

func (obj *ContainerMetadata) GetFieldInt64(key string) (int64, error) {
	switch key {
	case "InitProcessPID":
		return int64(obj.InitProcessPID), nil
	}
	return 0, getter.ErrFieldNotFound
}

func (obj *ContainerMetadata) GetFieldString(key string) (string, error) {
	switch key {
	case "ID":
		return string(obj.ID), nil
	case "Image":
		return string(obj.Image), nil
	case "ImageID":
		return string(obj.ImageID), nil
	case "Runtime":
		return string(obj.Runtime), nil
	case "Status":
		return string(obj.Status), nil
	}
	return "", getter.ErrFieldNotFound
}

func (obj *ContainerMetadata) GetFieldKeys() []string {
	return []string{
		"ID",
		"Image",
		"ImageID",
		"Runtime",
		"Status",
		"InitProcessPID",
		"Hosts",
		"Labels",
	}
}

func (obj *ContainerMetadata) MatchBool(key string, predicate getter.BoolPredicate) bool {
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

func (obj *ContainerMetadata) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	if b, err := obj.GetFieldInt64(key); err == nil {
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
			return obj.Labels.MatchInt64(key[index+1:], predicate)
		}
	}
	return false
}

func (obj *ContainerMetadata) MatchString(key string, predicate getter.StringPredicate) bool {
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

func (obj *ContainerMetadata) GetField(key string) (interface{}, error) {
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
	case "Labels":
		if obj.Labels != nil {
			if index != -1 {
				return obj.Labels.GetField(key[index+1:])
			} else {
				return obj.Labels, nil
			}
		}

	}
	return nil, getter.ErrFieldNotFound
}

func (obj *Hosts) GetFieldBool(key string) (bool, error) {
	return false, getter.ErrFieldNotFound
}

func (obj *Hosts) GetFieldInt64(key string) (int64, error) {
	return 0, getter.ErrFieldNotFound
}

func (obj *Hosts) GetFieldString(key string) (string, error) {
	switch key {
	case "IP":
		return string(obj.IP), nil
	case "Hostname":
		return string(obj.Hostname), nil
	}
	return "", getter.ErrFieldNotFound
}

func (obj *Hosts) GetFieldKeys() []string {
	return []string{
		"IP",
		"Hostname",
		"ByIP",
	}
}

func (obj *Hosts) MatchBool(key string, predicate getter.BoolPredicate) bool {
	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {
	case "ByIP":
		if index != -1 && obj.ByIP != nil {
			return obj.ByIP.MatchBool(key[index+1:], predicate)
		}
	}
	return false
}

func (obj *Hosts) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {

	case "ByIP":
		if index != -1 && obj.ByIP != nil {
			return obj.ByIP.MatchInt64(key[index+1:], predicate)
		}
	}
	return false
}

func (obj *Hosts) MatchString(key string, predicate getter.StringPredicate) bool {
	if b, err := obj.GetFieldString(key); err == nil {
		return predicate(b)
	}

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {

	case "ByIP":
		if index != -1 && obj.ByIP != nil {
			return obj.ByIP.MatchString(key[index+1:], predicate)
		}
	}
	return false
}

func (obj *Hosts) GetField(key string) (interface{}, error) {
	if s, err := obj.GetFieldString(key); err == nil {
		return s, nil
	}

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {
	case "ByIP":
		if obj.ByIP != nil {
			if index != -1 {
				return obj.ByIP.GetField(key[index+1:])
			} else {
				return obj.ByIP, nil
			}
		}

	}
	return nil, getter.ErrFieldNotFound
}

func init() {
	strings.Index("", ".")
}
