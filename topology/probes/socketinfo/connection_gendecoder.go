// Code generated - DO NOT EDIT.

package socketinfo

import (
	"github.com/skydive-project/skydive/graffiti/getter"
	"strings"
)

func (obj *ConnectionInfo) GetFieldBool(key string) (bool, error) {
	return false, getter.ErrFieldNotFound
}

func (obj *ConnectionInfo) GetFieldInt64(key string) (int64, error) {
	switch key {
	case "Pid":
		return int64(obj.Pid), nil
	case "LocalPort":
		return int64(obj.LocalPort), nil
	case "RemotePort":
		return int64(obj.RemotePort), nil
	}
	return 0, getter.ErrFieldNotFound
}

func (obj *ConnectionInfo) GetFieldString(key string) (string, error) {
	switch key {
	case "Process":
		return string(obj.Process), nil
	case "Name":
		return string(obj.Name), nil
	case "LocalAddress":
		return string(obj.LocalAddress), nil
	case "RemoteAddress":
		return string(obj.RemoteAddress), nil
	case "Protocol":
		return obj.Protocol.String(), nil
	case "State":
		return string(obj.State), nil
	}
	return "", getter.ErrFieldNotFound
}

func (obj *ConnectionInfo) GetFieldKeys() []string {
	return []string{
		"Process",
		"Pid",
		"Name",
		"LocalAddress",
		"LocalPort",
		"RemoteAddress",
		"RemotePort",
		"Protocol",
		"State",
	}
}

func (obj *ConnectionInfo) MatchBool(key string, predicate getter.BoolPredicate) bool {
	return false
}

func (obj *ConnectionInfo) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	if b, err := obj.GetFieldInt64(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *ConnectionInfo) MatchString(key string, predicate getter.StringPredicate) bool {
	if b, err := obj.GetFieldString(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *ConnectionInfo) GetField(key string) (interface{}, error) {
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
