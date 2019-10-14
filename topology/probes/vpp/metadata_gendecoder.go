// Code generated - DO NOT EDIT.

package vpp

import (
	"github.com/skydive-project/skydive/common"
	"strings"
)

func (obj *Metadata) GetFieldBool(key string) (bool, error) {
	switch key {
	case "Master":
		return obj.Master, nil
	case "LinkUpDown":
		return obj.LinkUpDown, nil
	}

	return false, common.ErrFieldNotFound
}

func (obj *Metadata) GetFieldInt64(key string) (int64, error) {
	switch key {
	case "ID":
		return int64(obj.ID), nil
	case "SocketID":
		return int64(obj.SocketID), nil
	case "RingSize":
		return int64(obj.RingSize), nil
	case "BufferSize":
		return int64(obj.BufferSize), nil
	}
	return 0, common.ErrFieldNotFound
}

func (obj *Metadata) GetFieldString(key string) (string, error) {
	switch key {
	case "SocketFilename":
		return string(obj.SocketFilename), nil
	case "Mode":
		return string(obj.Mode), nil
	}
	return "", common.ErrFieldNotFound
}

func (obj *Metadata) GetFieldKeys() []string {
	return []string{
		"SocketFilename",
		"ID",
		"SocketID",
		"Master",
		"Mode",
		"RingSize",
		"BufferSize",
		"LinkUpDown",
	}
}

func (obj *Metadata) MatchBool(key string, predicate common.BoolPredicate) bool {
	if b, err := obj.GetFieldBool(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *Metadata) MatchInt64(key string, predicate common.Int64Predicate) bool {
	if b, err := obj.GetFieldInt64(key); err == nil {
		return predicate(b)
	}
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

	if i, err := obj.GetFieldInt64(key); err == nil {
		return i, nil
	}

	if b, err := obj.GetFieldBool(key); err == nil {
		return b, nil
	}
	return nil, common.ErrFieldNotFound
}

func init() {
	strings.Index("", ".")
}
