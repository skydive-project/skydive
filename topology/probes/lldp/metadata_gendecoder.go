// Code generated - DO NOT EDIT.

package lldp

import (
	"github.com/skydive-project/skydive/graffiti/getter"
	"strings"
)

func (obj *LinkAggregationMetadata) GetFieldBool(key string) (bool, error) {
	switch key {
	case "Enabled":
		return obj.Enabled, nil
	case "Supported":
		return obj.Supported, nil
	}

	return false, getter.ErrFieldNotFound
}

func (obj *LinkAggregationMetadata) GetFieldInt64(key string) (int64, error) {
	switch key {
	case "PortID":
		return int64(obj.PortID), nil
	}
	return 0, getter.ErrFieldNotFound
}

func (obj *LinkAggregationMetadata) GetFieldString(key string) (string, error) {
	return "", getter.ErrFieldNotFound
}

func (obj *LinkAggregationMetadata) GetFieldKeys() []string {
	return []string{
		"Enabled",
		"PortID",
		"Supported",
	}
}

func (obj *LinkAggregationMetadata) MatchBool(key string, predicate getter.BoolPredicate) bool {
	if b, err := obj.GetFieldBool(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *LinkAggregationMetadata) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	if b, err := obj.GetFieldInt64(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *LinkAggregationMetadata) MatchString(key string, predicate getter.StringPredicate) bool {
	return false
}

func (obj *LinkAggregationMetadata) GetField(key string) (interface{}, error) {
	if i, err := obj.GetFieldInt64(key); err == nil {
		return i, nil
	}

	if b, err := obj.GetFieldBool(key); err == nil {
		return b, nil
	}
	return nil, getter.ErrFieldNotFound
}

func (obj *Metadata) GetFieldBool(key string) (bool, error) {
	return false, getter.ErrFieldNotFound
}

func (obj *Metadata) GetFieldInt64(key string) (int64, error) {
	switch key {
	case "PVID":
		return int64(obj.PVID), nil
	case "VIDUsageDigest":
		return int64(obj.VIDUsageDigest), nil
	case "ManagementVID":
		return int64(obj.ManagementVID), nil
	}
	return 0, getter.ErrFieldNotFound
}

func (obj *Metadata) GetFieldString(key string) (string, error) {
	switch key {
	case "Description":
		return string(obj.Description), nil
	case "ChassisID":
		return string(obj.ChassisID), nil
	case "ChassisIDType":
		return string(obj.ChassisIDType), nil
	case "SysName":
		return string(obj.SysName), nil
	case "MgmtAddress":
		return string(obj.MgmtAddress), nil
	case "PortID":
		return string(obj.PortID), nil
	case "PortIDType":
		return string(obj.PortIDType), nil
	}
	return "", getter.ErrFieldNotFound
}

func (obj *Metadata) GetFieldKeys() []string {
	return []string{
		"Description",
		"ChassisID",
		"ChassisIDType",
		"SysName",
		"MgmtAddress",
		"PVID",
		"VIDUsageDigest",
		"ManagementVID",
		"PortID",
		"PortIDType",
		"LinkAggregation",
		"VLANNames",
		"PPVIDs",
	}
}

func (obj *Metadata) MatchBool(key string, predicate getter.BoolPredicate) bool {
	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {
	case "LinkAggregation":
		if index != -1 && obj.LinkAggregation != nil {
			return obj.LinkAggregation.MatchBool(key[index+1:], predicate)
		}
	case "VLANNames":
		if index != -1 {
			for _, obj := range obj.VLANNames {
				if obj.MatchBool(key[index+1:], predicate) {
					return true
				}
			}
		}
	case "PPVIDs":
		if index != -1 {
			for _, obj := range obj.PPVIDs {
				if obj.MatchBool(key[index+1:], predicate) {
					return true
				}
			}
		}
	}
	return false
}

func (obj *Metadata) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	if b, err := obj.GetFieldInt64(key); err == nil {
		return predicate(b)
	}

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {

	case "LinkAggregation":
		if index != -1 && obj.LinkAggregation != nil {
			return obj.LinkAggregation.MatchInt64(key[index+1:], predicate)
		}
	case "VLANNames":
		if index != -1 {
			for _, obj := range obj.VLANNames {
				if obj.MatchInt64(key[index+1:], predicate) {
					return true
				}
			}
		}
	case "PPVIDs":
		if index != -1 {
			for _, obj := range obj.PPVIDs {
				if obj.MatchInt64(key[index+1:], predicate) {
					return true
				}
			}
		}
	}
	return false
}

func (obj *Metadata) MatchString(key string, predicate getter.StringPredicate) bool {
	if b, err := obj.GetFieldString(key); err == nil {
		return predicate(b)
	}

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {

	case "LinkAggregation":
		if index != -1 && obj.LinkAggregation != nil {
			return obj.LinkAggregation.MatchString(key[index+1:], predicate)
		}
	case "VLANNames":
		if index != -1 {
			for _, obj := range obj.VLANNames {
				if obj.MatchString(key[index+1:], predicate) {
					return true
				}
			}
		}
	case "PPVIDs":
		if index != -1 {
			for _, obj := range obj.PPVIDs {
				if obj.MatchString(key[index+1:], predicate) {
					return true
				}
			}
		}
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

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {
	case "LinkAggregation":
		if obj.LinkAggregation != nil {
			if index != -1 {
				return obj.LinkAggregation.GetField(key[index+1:])
			} else {
				return obj.LinkAggregation, nil
			}
		}
	case "VLANNames":
		if obj.VLANNames != nil {
			if index != -1 {
				var results []interface{}
				for _, obj := range obj.VLANNames {
					if field, err := obj.GetField(key[index+1:]); err == nil {
						results = append(results, field)
					}
				}
				return results, nil
			} else {
				var results []interface{}
				for _, obj := range obj.VLANNames {
					results = append(results, obj)
				}
				return results, nil
			}
		}
	case "PPVIDs":
		if obj.PPVIDs != nil {
			if index != -1 {
				var results []interface{}
				for _, obj := range obj.PPVIDs {
					if field, err := obj.GetField(key[index+1:]); err == nil {
						results = append(results, field)
					}
				}
				return results, nil
			} else {
				var results []interface{}
				for _, obj := range obj.PPVIDs {
					results = append(results, obj)
				}
				return results, nil
			}
		}

	}
	return nil, getter.ErrFieldNotFound
}

func (obj *PPVIDMetadata) GetFieldBool(key string) (bool, error) {
	switch key {
	case "Enabled":
		return obj.Enabled, nil
	case "Supported":
		return obj.Supported, nil
	}

	return false, getter.ErrFieldNotFound
}

func (obj *PPVIDMetadata) GetFieldInt64(key string) (int64, error) {
	switch key {
	case "ID":
		return int64(obj.ID), nil
	}
	return 0, getter.ErrFieldNotFound
}

func (obj *PPVIDMetadata) GetFieldString(key string) (string, error) {
	return "", getter.ErrFieldNotFound
}

func (obj *PPVIDMetadata) GetFieldKeys() []string {
	return []string{
		"Enabled",
		"ID",
		"Supported",
	}
}

func (obj *PPVIDMetadata) MatchBool(key string, predicate getter.BoolPredicate) bool {
	if b, err := obj.GetFieldBool(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *PPVIDMetadata) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	if b, err := obj.GetFieldInt64(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *PPVIDMetadata) MatchString(key string, predicate getter.StringPredicate) bool {
	return false
}

func (obj *PPVIDMetadata) GetField(key string) (interface{}, error) {
	if i, err := obj.GetFieldInt64(key); err == nil {
		return i, nil
	}

	if b, err := obj.GetFieldBool(key); err == nil {
		return b, nil
	}
	return nil, getter.ErrFieldNotFound
}

func (obj *VLANNameMetadata) GetFieldBool(key string) (bool, error) {
	return false, getter.ErrFieldNotFound
}

func (obj *VLANNameMetadata) GetFieldInt64(key string) (int64, error) {
	switch key {
	case "ID":
		return int64(obj.ID), nil
	}
	return 0, getter.ErrFieldNotFound
}

func (obj *VLANNameMetadata) GetFieldString(key string) (string, error) {
	switch key {
	case "Name":
		return string(obj.Name), nil
	}
	return "", getter.ErrFieldNotFound
}

func (obj *VLANNameMetadata) GetFieldKeys() []string {
	return []string{
		"ID",
		"Name",
	}
}

func (obj *VLANNameMetadata) MatchBool(key string, predicate getter.BoolPredicate) bool {
	return false
}

func (obj *VLANNameMetadata) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	if b, err := obj.GetFieldInt64(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *VLANNameMetadata) MatchString(key string, predicate getter.StringPredicate) bool {
	if b, err := obj.GetFieldString(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *VLANNameMetadata) GetField(key string) (interface{}, error) {
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
