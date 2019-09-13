// Code generated - DO NOT EDIT.

package libvirt

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
	case "MAC":
		return string(obj.MAC), nil
	case "Domain":
		return string(obj.Domain), nil
	case "BusType":
		return string(obj.BusType), nil
	case "BusInfo":
		return string(obj.BusInfo), nil
	case "Alias":
		return string(obj.Alias), nil
	}
	return "", common.ErrFieldNotFound
}

func (obj *Metadata) GetFieldKeys() []string {
	return []string{
		"MAC",
		"Domain",
		"BusType",
		"BusInfo",
		"Alias",
	}
}

func (obj *Metadata) MatchBool(key string, predicate common.BoolPredicate) bool {
	return false
}

func (obj *Metadata) MatchInt64(key string, predicate common.Int64Predicate) bool {
	return false
}

func (obj *Metadata) MatchString(key string, predicate common.StringPredicate) bool {
	if b, err := obj.GetFieldString(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *Metadata) GetField(key string) (interface{}, error) {
	if s, err := obj.GetFieldString(key); err == nil {
		return s, nil
	}
	return nil, common.ErrFieldNotFound
}

func init() {
	strings.Index("", ".")
}
