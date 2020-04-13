// Code generated - DO NOT EDIT.

package ovn

import (
	"github.com/skydive-project/skydive/graffiti/getter"
	"strings"
)

func (obj *ACLMetadata) GetFieldBool(key string) (bool, error) {
	switch key {
	case "Log":
		return obj.Log, nil
	}

	return false, getter.ErrFieldNotFound
}

func (obj *ACLMetadata) GetFieldInt64(key string) (int64, error) {
	switch key {
	case "Priority":
		return int64(obj.Priority), nil
	}
	return 0, getter.ErrFieldNotFound
}

func (obj *ACLMetadata) GetFieldString(key string) (string, error) {
	switch key {
	case "Action":
		return string(obj.Action), nil
	case "Direction":
		return string(obj.Direction), nil
	case "Match":
		return string(obj.Match), nil
	}
	return "", getter.ErrFieldNotFound
}

func (obj *ACLMetadata) GetFieldKeys() []string {
	return []string{
		"Action",
		"Direction",
		"Log",
		"Match",
		"Priority",
	}
}

func (obj *ACLMetadata) MatchBool(key string, predicate getter.BoolPredicate) bool {
	if b, err := obj.GetFieldBool(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *ACLMetadata) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	if b, err := obj.GetFieldInt64(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *ACLMetadata) MatchString(key string, predicate getter.StringPredicate) bool {
	if b, err := obj.GetFieldString(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *ACLMetadata) GetField(key string) (interface{}, error) {
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

func (obj *LRPMetadata) GetFieldBool(key string) (bool, error) {
	return false, getter.ErrFieldNotFound
}

func (obj *LRPMetadata) GetFieldInt64(key string) (int64, error) {
	return 0, getter.ErrFieldNotFound
}

func (obj *LRPMetadata) GetFieldString(key string) (string, error) {
	switch key {
	case "Peer":
		return string(obj.Peer), nil
	}
	return "", getter.ErrFieldNotFound
}

func (obj *LRPMetadata) GetFieldKeys() []string {
	return []string{
		"GatewayChassis",
		"IPv6RAConfigs",
		"Networks",
		"Peer",
	}
}

func (obj *LRPMetadata) MatchBool(key string, predicate getter.BoolPredicate) bool {
	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {
	case "IPv6RAConfigs":
		if index != -1 && obj.IPv6RAConfigs != nil {
			return obj.IPv6RAConfigs.MatchBool(key[index+1:], predicate)
		}
	}
	return false
}

func (obj *LRPMetadata) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {

	case "IPv6RAConfigs":
		if index != -1 && obj.IPv6RAConfigs != nil {
			return obj.IPv6RAConfigs.MatchInt64(key[index+1:], predicate)
		}
	}
	return false
}

func (obj *LRPMetadata) MatchString(key string, predicate getter.StringPredicate) bool {
	if b, err := obj.GetFieldString(key); err == nil {
		return predicate(b)
	}

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {

	case "GatewayChassis":
		if index == -1 {
			for _, i := range obj.GatewayChassis {
				if predicate(i) {
					return true
				}
			}
		}
	case "IPv6RAConfigs":
		if index != -1 && obj.IPv6RAConfigs != nil {
			return obj.IPv6RAConfigs.MatchString(key[index+1:], predicate)
		}
	case "Networks":
		if index == -1 {
			for _, i := range obj.Networks {
				if predicate(i) {
					return true
				}
			}
		}
	}
	return false
}

func (obj *LRPMetadata) GetField(key string) (interface{}, error) {
	if s, err := obj.GetFieldString(key); err == nil {
		return s, nil
	}

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {
	case "GatewayChassis":
		if obj.GatewayChassis != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.GatewayChassis {
					results = append(results, obj)
				}
				return results, nil
			}
		}
	case "IPv6RAConfigs":
		if obj.IPv6RAConfigs != nil {
			if index != -1 {
				return obj.IPv6RAConfigs.GetField(key[index+1:])
			} else {
				return obj.IPv6RAConfigs, nil
			}
		}
	case "Networks":
		if obj.Networks != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.Networks {
					results = append(results, obj)
				}
				return results, nil
			}
		}

	}
	return nil, getter.ErrFieldNotFound
}

func (obj *LSPMetadata) GetFieldBool(key string) (bool, error) {
	return false, getter.ErrFieldNotFound
}

func (obj *LSPMetadata) GetFieldInt64(key string) (int64, error) {
	return 0, getter.ErrFieldNotFound
}

func (obj *LSPMetadata) GetFieldString(key string) (string, error) {
	switch key {
	case "DHCPv4Options":
		return string(obj.DHCPv4Options), nil
	case "DHCPv6Options":
		return string(obj.DHCPv6Options), nil
	case "Type":
		return string(obj.Type), nil
	}
	return "", getter.ErrFieldNotFound
}

func (obj *LSPMetadata) GetFieldKeys() []string {
	return []string{
		"Addresses",
		"PortSecurity",
		"DHCPv4Options",
		"DHCPv6Options",
		"Type",
	}
}

func (obj *LSPMetadata) MatchBool(key string, predicate getter.BoolPredicate) bool {
	return false
}

func (obj *LSPMetadata) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	return false
}

func (obj *LSPMetadata) MatchString(key string, predicate getter.StringPredicate) bool {
	if b, err := obj.GetFieldString(key); err == nil {
		return predicate(b)
	}

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {

	case "Addresses":
		if index == -1 {
			for _, i := range obj.Addresses {
				if predicate(i) {
					return true
				}
			}
		}
	case "PortSecurity":
		if index == -1 {
			for _, i := range obj.PortSecurity {
				if predicate(i) {
					return true
				}
			}
		}
	}
	return false
}

func (obj *LSPMetadata) GetField(key string) (interface{}, error) {
	if s, err := obj.GetFieldString(key); err == nil {
		return s, nil
	}

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {
	case "Addresses":
		if obj.Addresses != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.Addresses {
					results = append(results, obj)
				}
				return results, nil
			}
		}
	case "PortSecurity":
		if obj.PortSecurity != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.PortSecurity {
					results = append(results, obj)
				}
				return results, nil
			}
		}

	}
	return nil, getter.ErrFieldNotFound
}

func (obj *Metadata) GetFieldBool(key string) (bool, error) {
	switch key {
	case "Log":
		return obj.Log, nil
	}

	return false, getter.ErrFieldNotFound
}

func (obj *Metadata) GetFieldInt64(key string) (int64, error) {
	switch key {
	case "Priority":
		return int64(obj.Priority), nil
	}
	return 0, getter.ErrFieldNotFound
}

func (obj *Metadata) GetFieldString(key string) (string, error) {
	switch key {
	case "DHCPv4Options":
		return string(obj.DHCPv4Options), nil
	case "DHCPv6Options":
		return string(obj.DHCPv6Options), nil
	case "Type":
		return string(obj.Type), nil
	case "Peer":
		return string(obj.Peer), nil
	case "Action":
		return string(obj.Action), nil
	case "Direction":
		return string(obj.Direction), nil
	case "Match":
		return string(obj.Match), nil
	}
	return "", getter.ErrFieldNotFound
}

func (obj *Metadata) GetFieldKeys() []string {
	return []string{
		"Addresses",
		"PortSecurity",
		"DHCPv4Options",
		"DHCPv6Options",
		"Type",
		"GatewayChassis",
		"IPv6RAConfigs",
		"Networks",
		"Peer",
		"Action",
		"Direction",
		"Log",
		"Match",
		"Priority",
		"ExtID",
		"Options",
	}
}

func (obj *Metadata) MatchBool(key string, predicate getter.BoolPredicate) bool {
	if b, err := obj.GetFieldBool(key); err == nil {
		return predicate(b)
	}

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {
	case "IPv6RAConfigs":
		if index != -1 && obj.IPv6RAConfigs != nil {
			return obj.IPv6RAConfigs.MatchBool(key[index+1:], predicate)
		}
	case "ExtID":
		if index != -1 && obj.ExtID != nil {
			return obj.ExtID.MatchBool(key[index+1:], predicate)
		}
	case "Options":
		if index != -1 && obj.Options != nil {
			return obj.Options.MatchBool(key[index+1:], predicate)
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

	case "IPv6RAConfigs":
		if index != -1 && obj.IPv6RAConfigs != nil {
			return obj.IPv6RAConfigs.MatchInt64(key[index+1:], predicate)
		}
	case "ExtID":
		if index != -1 && obj.ExtID != nil {
			return obj.ExtID.MatchInt64(key[index+1:], predicate)
		}
	case "Options":
		if index != -1 && obj.Options != nil {
			return obj.Options.MatchInt64(key[index+1:], predicate)
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

	case "Addresses":
		if index == -1 {
			for _, i := range obj.Addresses {
				if predicate(i) {
					return true
				}
			}
		}
	case "PortSecurity":
		if index == -1 {
			for _, i := range obj.PortSecurity {
				if predicate(i) {
					return true
				}
			}
		}
	case "GatewayChassis":
		if index == -1 {
			for _, i := range obj.GatewayChassis {
				if predicate(i) {
					return true
				}
			}
		}
	case "IPv6RAConfigs":
		if index != -1 && obj.IPv6RAConfigs != nil {
			return obj.IPv6RAConfigs.MatchString(key[index+1:], predicate)
		}
	case "Networks":
		if index == -1 {
			for _, i := range obj.Networks {
				if predicate(i) {
					return true
				}
			}
		}
	case "ExtID":
		if index != -1 && obj.ExtID != nil {
			return obj.ExtID.MatchString(key[index+1:], predicate)
		}
	case "Options":
		if index != -1 && obj.Options != nil {
			return obj.Options.MatchString(key[index+1:], predicate)
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

	if b, err := obj.GetFieldBool(key); err == nil {
		return b, nil
	}

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {
	case "Addresses":
		if obj.Addresses != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.Addresses {
					results = append(results, obj)
				}
				return results, nil
			}
		}
	case "PortSecurity":
		if obj.PortSecurity != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.PortSecurity {
					results = append(results, obj)
				}
				return results, nil
			}
		}
	case "GatewayChassis":
		if obj.GatewayChassis != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.GatewayChassis {
					results = append(results, obj)
				}
				return results, nil
			}
		}
	case "IPv6RAConfigs":
		if obj.IPv6RAConfigs != nil {
			if index != -1 {
				return obj.IPv6RAConfigs.GetField(key[index+1:])
			} else {
				return obj.IPv6RAConfigs, nil
			}
		}
	case "Networks":
		if obj.Networks != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.Networks {
					results = append(results, obj)
				}
				return results, nil
			}
		}
	case "ExtID":
		if obj.ExtID != nil {
			if index != -1 {
				return obj.ExtID.GetField(key[index+1:])
			} else {
				return obj.ExtID, nil
			}
		}
	case "Options":
		if obj.Options != nil {
			if index != -1 {
				return obj.Options.GetField(key[index+1:])
			} else {
				return obj.Options, nil
			}
		}

	}
	return nil, getter.ErrFieldNotFound
}

func init() {
	strings.Index("", ".")
}
