// Code generated - DO NOT EDIT.

package probes

import (
	"github.com/skydive-project/skydive/common"
	"strings"
)

func (obj *CaptureMetadata) GetFieldBool(key string) (bool, error) {
	return false, common.ErrFieldNotFound
}

func (obj *CaptureMetadata) GetFieldInt64(key string) (int64, error) {
	switch key {
	case "PacketsReceived":
		return int64(obj.PacketsReceived), nil
	case "PacketsDropped":
		return int64(obj.PacketsDropped), nil
	case "PacketsIfDropped":
		return int64(obj.PacketsIfDropped), nil
	}
	return 0, common.ErrFieldNotFound
}

func (obj *CaptureMetadata) GetFieldString(key string) (string, error) {
	switch key {
	case "ID":
		return string(obj.ID), nil
	case "State":
		return string(obj.State), nil
	case "Name":
		return string(obj.Name), nil
	case "Description":
		return string(obj.Description), nil
	case "BPFFilter":
		return string(obj.BPFFilter), nil
	case "Type":
		return string(obj.Type), nil
	case "PCAPSocket":
		return string(obj.PCAPSocket), nil
	case "MirrorOf":
		return string(obj.MirrorOf), nil
	case "SFlowSocket":
		return string(obj.SFlowSocket), nil
	case "Error":
		return string(obj.Error), nil
	}
	return "", common.ErrFieldNotFound
}

func (obj *CaptureMetadata) GetFieldKeys() []string {
	return []string{
		"PacketsReceived",
		"PacketsDropped",
		"PacketsIfDropped",
		"ID",
		"State",
		"Name",
		"Description",
		"BPFFilter",
		"Type",
		"PCAPSocket",
		"MirrorOf",
		"SFlowSocket",
		"Error",
	}
}

func (obj *CaptureMetadata) MatchBool(key string, predicate common.BoolPredicate) bool {
	return false
}

func (obj *CaptureMetadata) MatchInt64(key string, predicate common.Int64Predicate) bool {
	if b, err := obj.GetFieldInt64(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *CaptureMetadata) MatchString(key string, predicate common.StringPredicate) bool {
	if b, err := obj.GetFieldString(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *CaptureMetadata) GetField(key string) (interface{}, error) {
	if s, err := obj.GetFieldString(key); err == nil {
		return s, nil
	}

	if i, err := obj.GetFieldInt64(key); err == nil {
		return i, nil
	}
	return nil, common.ErrFieldNotFound
}

func (obj *CaptureStats) GetFieldBool(key string) (bool, error) {
	return false, common.ErrFieldNotFound
}

func (obj *CaptureStats) GetFieldInt64(key string) (int64, error) {
	switch key {
	case "PacketsReceived":
		return int64(obj.PacketsReceived), nil
	case "PacketsDropped":
		return int64(obj.PacketsDropped), nil
	case "PacketsIfDropped":
		return int64(obj.PacketsIfDropped), nil
	}
	return 0, common.ErrFieldNotFound
}

func (obj *CaptureStats) GetFieldString(key string) (string, error) {
	return "", common.ErrFieldNotFound
}

func (obj *CaptureStats) GetFieldKeys() []string {
	return []string{
		"PacketsReceived",
		"PacketsDropped",
		"PacketsIfDropped",
	}
}

func (obj *CaptureStats) MatchBool(key string, predicate common.BoolPredicate) bool {
	return false
}

func (obj *CaptureStats) MatchInt64(key string, predicate common.Int64Predicate) bool {
	if b, err := obj.GetFieldInt64(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *CaptureStats) MatchString(key string, predicate common.StringPredicate) bool {
	return false
}

func (obj *CaptureStats) GetField(key string) (interface{}, error) {
	if i, err := obj.GetFieldInt64(key); err == nil {
		return i, nil
	}
	return nil, common.ErrFieldNotFound
}

func (obj *Captures) GetFieldBool(key string) (bool, error) {
	return false, common.ErrFieldNotFound
}

func (obj *Captures) GetFieldInt64(key string) (int64, error) {
	switch key {
	}
	return 0, common.ErrFieldNotFound
}

func (obj *Captures) GetFieldString(key string) (string, error) {
	switch key {
	}
	return "", common.ErrFieldNotFound
}

func (obj *Captures) GetFieldKeys() []string {
	return []string{
		"PacketsReceived",
		"PacketsDropped",
		"PacketsIfDropped",
		"ID",
		"State",
		"Name",
		"Description",
		"BPFFilter",
		"Type",
		"PCAPSocket",
		"MirrorOf",
		"SFlowSocket",
		"Error",
	}
}

func (obj *Captures) MatchBool(key string, predicate common.BoolPredicate) bool {
	for _, obj := range *obj {
		if obj.MatchBool(key, predicate) {
			return true
		}
	}
	return false
}

func (obj *Captures) MatchInt64(key string, predicate common.Int64Predicate) bool {
	for _, obj := range *obj {
		if obj.MatchInt64(key, predicate) {
			return true
		}
	}
	return false
}

func (obj *Captures) MatchString(key string, predicate common.StringPredicate) bool {
	for _, obj := range *obj {
		if obj.MatchString(key, predicate) {
			return true
		}
	}
	return false
}

func (obj *Captures) GetField(key string) (interface{}, error) {
	var result []interface{}

	for _, o := range *obj {
		switch key {
		case "PacketsReceived":
			result = append(result, o.PacketsReceived)
		case "PacketsDropped":
			result = append(result, o.PacketsDropped)
		case "PacketsIfDropped":
			result = append(result, o.PacketsIfDropped)
		case "ID":
			result = append(result, o.ID)
		case "State":
			result = append(result, o.State)
		case "Name":
			result = append(result, o.Name)
		case "Description":
			result = append(result, o.Description)
		case "BPFFilter":
			result = append(result, o.BPFFilter)
		case "Type":
			result = append(result, o.Type)
		case "PCAPSocket":
			result = append(result, o.PCAPSocket)
		case "MirrorOf":
			result = append(result, o.MirrorOf)
		case "SFlowSocket":
			result = append(result, o.SFlowSocket)
		case "Error":
			result = append(result, o.Error)
		default:
			return result, common.ErrFieldNotFound
		}
	}

	return result, nil
}

func init() {
	strings.Index("", ".")
}
