// Code generated - DO NOT EDIT.

package topology

import (
	"github.com/skydive-project/skydive/graffiti/getter"
	"strings"
)

func (obj *NextHop) GetFieldBool(key string) (bool, error) {
	return false, getter.ErrFieldNotFound
}

func (obj *NextHop) GetFieldInt64(key string) (int64, error) {
	switch key {
	case "Priority":
		return int64(obj.Priority), nil
	case "IfIndex":
		return int64(obj.IfIndex), nil
	}
	return 0, getter.ErrFieldNotFound
}

func (obj *NextHop) GetFieldString(key string) (string, error) {
	switch key {
	case "IP":
		return obj.IP.String(), nil
	case "MAC":
		return string(obj.MAC), nil
	}
	return "", getter.ErrFieldNotFound
}

func (obj *NextHop) GetFieldKeys() []string {
	return []string{
		"Priority",
		"IP",
		"MAC",
		"IfIndex",
	}
}

func (obj *NextHop) MatchBool(key string, predicate getter.BoolPredicate) bool {
	return false
}

func (obj *NextHop) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	if b, err := obj.GetFieldInt64(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *NextHop) MatchString(key string, predicate getter.StringPredicate) bool {
	if b, err := obj.GetFieldString(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *NextHop) GetField(key string) (interface{}, error) {
	if s, err := obj.GetFieldString(key); err == nil {
		return s, nil
	}

	if i, err := obj.GetFieldInt64(key); err == nil {
		return i, nil
	}
	return nil, getter.ErrFieldNotFound
}

func init() {
	strings.Index("", ".")
}
