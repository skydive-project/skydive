// Code generated - DO NOT EDIT.

package libvirt

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
	return "", getter.ErrFieldNotFound
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

func (obj *Metadata) MatchBool(key string, predicate getter.BoolPredicate) bool {
	return false
}

func (obj *Metadata) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	return false
}

func (obj *Metadata) MatchString(key string, predicate getter.StringPredicate) bool {
	if b, err := obj.GetFieldString(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *Metadata) GetField(key string) (interface{}, error) {
	if s, err := obj.GetFieldString(key); err == nil {
		return s, nil
	}
	return nil, getter.ErrFieldNotFound
}

func init() {
	strings.Index("", ".")
}
