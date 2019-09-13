// Code generated - DO NOT EDIT.

package runc

import (
	"github.com/skydive-project/skydive/common"
	"strings"
)

func (obj *CreateConfig) GetFieldBool(key string) (bool, error) {
	return false, common.ErrFieldNotFound
}

func (obj *CreateConfig) GetFieldInt64(key string) (int64, error) {
	return 0, common.ErrFieldNotFound
}

func (obj *CreateConfig) GetFieldString(key string) (string, error) {
	switch key {
	case "Image":
		return string(obj.Image), nil
	case "ImageID":
		return string(obj.ImageID), nil
	}
	return "", common.ErrFieldNotFound
}

func (obj *CreateConfig) GetFieldKeys() []string {
	return []string{
		"Image",
		"ImageID",
		"Labels",
	}
}

func (obj *CreateConfig) MatchBool(key string, predicate common.BoolPredicate) bool {
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

func (obj *CreateConfig) MatchInt64(key string, predicate common.Int64Predicate) bool {
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

func (obj *CreateConfig) MatchString(key string, predicate common.StringPredicate) bool {
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

func (obj *CreateConfig) GetField(key string) (interface{}, error) {
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

func (obj *Hosts) GetFieldBool(key string) (bool, error) {
	return false, common.ErrFieldNotFound
}

func (obj *Hosts) GetFieldInt64(key string) (int64, error) {
	return 0, common.ErrFieldNotFound
}

func (obj *Hosts) GetFieldString(key string) (string, error) {
	switch key {
	case "IP":
		return string(obj.IP), nil
	case "Hostname":
		return string(obj.Hostname), nil
	}
	return "", common.ErrFieldNotFound
}

func (obj *Hosts) GetFieldKeys() []string {
	return []string{
		"IP",
		"Hostname",
		"ByIP",
	}
}

func (obj *Hosts) MatchBool(key string, predicate common.BoolPredicate) bool {
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

func (obj *Hosts) MatchInt64(key string, predicate common.Int64Predicate) bool {
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

func (obj *Hosts) MatchString(key string, predicate common.StringPredicate) bool {
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
	return nil, common.ErrFieldNotFound
}

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
	case "Status":
		return string(obj.Status), nil
	}
	return "", common.ErrFieldNotFound
}

func (obj *Metadata) GetFieldKeys() []string {
	return []string{
		"ContainerID",
		"Status",
		"Labels",
		"CreateConfig",
		"Hosts",
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
	case "CreateConfig":
		if index != -1 && obj.CreateConfig != nil {
			return obj.CreateConfig.MatchBool(key[index+1:], predicate)
		}
	case "Hosts":
		if index != -1 && obj.Hosts != nil {
			return obj.Hosts.MatchBool(key[index+1:], predicate)
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
	case "CreateConfig":
		if index != -1 && obj.CreateConfig != nil {
			return obj.CreateConfig.MatchInt64(key[index+1:], predicate)
		}
	case "Hosts":
		if index != -1 && obj.Hosts != nil {
			return obj.Hosts.MatchInt64(key[index+1:], predicate)
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
	case "CreateConfig":
		if index != -1 && obj.CreateConfig != nil {
			return obj.CreateConfig.MatchString(key[index+1:], predicate)
		}
	case "Hosts":
		if index != -1 && obj.Hosts != nil {
			return obj.Hosts.MatchString(key[index+1:], predicate)
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
	case "CreateConfig":
		if obj.CreateConfig != nil {
			if index != -1 {
				return obj.CreateConfig.GetField(key[index+1:])
			} else {
				return obj.CreateConfig, nil
			}
		}
	case "Hosts":
		if obj.Hosts != nil {
			if index != -1 {
				return obj.Hosts.GetField(key[index+1:])
			} else {
				return obj.Hosts, nil
			}
		}

	}
	return nil, common.ErrFieldNotFound
}

func init() {
	strings.Index("", ".")
}
