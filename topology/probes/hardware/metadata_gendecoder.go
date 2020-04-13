// Code generated - DO NOT EDIT.

package hardware

import (
	"github.com/skydive-project/skydive/graffiti/getter"
	"strings"
)

func (obj *CPUInfo) GetFieldBool(key string) (bool, error) {
	return false, getter.ErrFieldNotFound
}

func (obj *CPUInfo) GetFieldInt64(key string) (int64, error) {
	switch key {
	case "CPU":
		return int64(obj.CPU), nil
	case "Stepping":
		return int64(obj.Stepping), nil
	case "Cores":
		return int64(obj.Cores), nil
	case "Mhz":
		return int64(obj.Mhz), nil
	case "CacheSize":
		return int64(obj.CacheSize), nil
	}
	return 0, getter.ErrFieldNotFound
}

func (obj *CPUInfo) GetFieldString(key string) (string, error) {
	switch key {
	case "VendorID":
		return string(obj.VendorID), nil
	case "Family":
		return string(obj.Family), nil
	case "Model":
		return string(obj.Model), nil
	case "PhysicalID":
		return string(obj.PhysicalID), nil
	case "CoreID":
		return string(obj.CoreID), nil
	case "ModelName":
		return string(obj.ModelName), nil
	case "Microcode":
		return string(obj.Microcode), nil
	}
	return "", getter.ErrFieldNotFound
}

func (obj *CPUInfo) GetFieldKeys() []string {
	return []string{
		"CPU",
		"VendorID",
		"Family",
		"Model",
		"Stepping",
		"PhysicalID",
		"CoreID",
		"Cores",
		"ModelName",
		"Mhz",
		"CacheSize",
		"Microcode",
	}
}

func (obj *CPUInfo) MatchBool(key string, predicate getter.BoolPredicate) bool {
	return false
}

func (obj *CPUInfo) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	if b, err := obj.GetFieldInt64(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *CPUInfo) MatchString(key string, predicate getter.StringPredicate) bool {
	if b, err := obj.GetFieldString(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *CPUInfo) GetField(key string) (interface{}, error) {
	if s, err := obj.GetFieldString(key); err == nil {
		return s, nil
	}

	if i, err := obj.GetFieldInt64(key); err == nil {
		return i, nil
	}
	return nil, getter.ErrFieldNotFound
}

func (obj *CPUInfos) GetFieldBool(key string) (bool, error) {
	return false, getter.ErrFieldNotFound
}

func (obj *CPUInfos) GetFieldInt64(key string) (int64, error) {
	switch key {
	}
	return 0, getter.ErrFieldNotFound
}

func (obj *CPUInfos) GetFieldString(key string) (string, error) {
	switch key {
	}
	return "", getter.ErrFieldNotFound
}

func (obj *CPUInfos) GetFieldKeys() []string {
	return []string{
		"CPU",
		"VendorID",
		"Family",
		"Model",
		"Stepping",
		"PhysicalID",
		"CoreID",
		"Cores",
		"ModelName",
		"Mhz",
		"CacheSize",
		"Microcode",
	}
}

func (obj *CPUInfos) MatchBool(key string, predicate getter.BoolPredicate) bool {
	for _, obj := range *obj {
		if obj.MatchBool(key, predicate) {
			return true
		}
	}
	return false
}

func (obj *CPUInfos) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	for _, obj := range *obj {
		if obj.MatchInt64(key, predicate) {
			return true
		}
	}
	return false
}

func (obj *CPUInfos) MatchString(key string, predicate getter.StringPredicate) bool {
	for _, obj := range *obj {
		if obj.MatchString(key, predicate) {
			return true
		}
	}
	return false
}

func (obj *CPUInfos) GetField(key string) (interface{}, error) {
	var result []interface{}

	for _, o := range *obj {
		switch key {
		case "CPU":
			result = append(result, o.CPU)
		case "VendorID":
			result = append(result, o.VendorID)
		case "Family":
			result = append(result, o.Family)
		case "Model":
			result = append(result, o.Model)
		case "Stepping":
			result = append(result, o.Stepping)
		case "PhysicalID":
			result = append(result, o.PhysicalID)
		case "CoreID":
			result = append(result, o.CoreID)
		case "Cores":
			result = append(result, o.Cores)
		case "ModelName":
			result = append(result, o.ModelName)
		case "Mhz":
			result = append(result, o.Mhz)
		case "CacheSize":
			result = append(result, o.CacheSize)
		case "Microcode":
			result = append(result, o.Microcode)
		default:
			return result, getter.ErrFieldNotFound
		}
	}

	return result, nil
}

func init() {
	strings.Index("", ".")
}
