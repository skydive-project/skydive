// Code generated - DO NOT EDIT.

package netlink

import (
	"github.com/skydive-project/skydive/graffiti/getter"
	"strings"
)

func (obj *VF) GetFieldBool(key string) (bool, error) {
	switch key {
	case "Spoofchk":
		return obj.Spoofchk, nil
	}

	return false, getter.ErrFieldNotFound
}

func (obj *VF) GetFieldInt64(key string) (int64, error) {
	switch key {
	case "ID":
		return int64(obj.ID), nil
	case "LinkState":
		return int64(obj.LinkState), nil
	case "Qos":
		return int64(obj.Qos), nil
	case "TxRate":
		return int64(obj.TxRate), nil
	case "Vlan":
		return int64(obj.Vlan), nil
	}
	return 0, getter.ErrFieldNotFound
}

func (obj *VF) GetFieldString(key string) (string, error) {
	switch key {
	case "MAC":
		return string(obj.MAC), nil
	}
	return "", getter.ErrFieldNotFound
}

func (obj *VF) GetFieldKeys() []string {
	return []string{
		"ID",
		"LinkState",
		"MAC",
		"Qos",
		"Spoofchk",
		"TxRate",
		"Vlan",
	}
}

func (obj *VF) MatchBool(key string, predicate getter.BoolPredicate) bool {
	if b, err := obj.GetFieldBool(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *VF) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	if b, err := obj.GetFieldInt64(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *VF) MatchString(key string, predicate getter.StringPredicate) bool {
	if b, err := obj.GetFieldString(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *VF) GetField(key string) (interface{}, error) {
	if s, err := obj.GetFieldString(key); err == nil {
		return s, nil
	}

	if i, err := obj.GetFieldInt64(key); err == nil {
		return i, nil
	}

	if b, err := obj.GetFieldBool(key); err == nil {
		return b, nil
	}
	return nil, getter.ErrFieldNotFound
}

func (obj *VFS) GetFieldBool(key string) (bool, error) {
	switch key {
	}

	return false, getter.ErrFieldNotFound
}

func (obj *VFS) GetFieldInt64(key string) (int64, error) {
	switch key {
	}
	return 0, getter.ErrFieldNotFound
}

func (obj *VFS) GetFieldString(key string) (string, error) {
	switch key {
	}
	return "", getter.ErrFieldNotFound
}

func (obj *VFS) GetFieldKeys() []string {
	return []string{
		"ID",
		"LinkState",
		"MAC",
		"Qos",
		"Spoofchk",
		"TxRate",
		"Vlan",
	}
}

func (obj *VFS) MatchBool(key string, predicate getter.BoolPredicate) bool {
	for _, obj := range *obj {
		if obj.MatchBool(key, predicate) {
			return true
		}
	}
	return false
}

func (obj *VFS) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	for _, obj := range *obj {
		if obj.MatchInt64(key, predicate) {
			return true
		}
	}
	return false
}

func (obj *VFS) MatchString(key string, predicate getter.StringPredicate) bool {
	for _, obj := range *obj {
		if obj.MatchString(key, predicate) {
			return true
		}
	}
	return false
}

func (obj *VFS) GetField(key string) (interface{}, error) {
	var result []interface{}

	for _, o := range *obj {
		switch key {
		case "ID":
			result = append(result, o.ID)
		case "LinkState":
			result = append(result, o.LinkState)
		case "MAC":
			result = append(result, o.MAC)
		case "Qos":
			result = append(result, o.Qos)
		case "Spoofchk":
			result = append(result, o.Spoofchk)
		case "TxRate":
			result = append(result, o.TxRate)
		case "Vlan":
			result = append(result, o.Vlan)
		default:
			return result, getter.ErrFieldNotFound
		}
	}

	return result, nil
}

func init() {
	strings.Index("", ".")
}
