// Code generated - DO NOT EDIT.

package packetinjector

import (
	"github.com/skydive-project/skydive/common"
	"strings"
)

func (obj *InjectionMetadata) GetFieldBool(key string) (bool, error) {
	return false, common.ErrFieldNotFound
}

func (obj *InjectionMetadata) GetFieldInt64(key string) (int64, error) {
	switch key {
	case "SrcPort":
		return int64(obj.SrcPort), nil
	case "DstPort":
		return int64(obj.DstPort), nil
	case "Count":
		return int64(obj.Count), nil
	case "ICMPID":
		return int64(obj.ICMPID), nil
	case "Interval":
		return int64(obj.Interval), nil
	case "IncrementPayload":
		return int64(obj.IncrementPayload), nil
	case "TTL":
		return int64(obj.TTL), nil
	case "PacketCount":
		return int64(obj.PacketCount), nil
	}
	return 0, common.ErrFieldNotFound
}

func (obj *InjectionMetadata) GetFieldString(key string) (string, error) {
	switch key {
	case "SrcIP":
		return obj.SrcIP.String(), nil
	case "SrcMAC":
		return obj.SrcMAC.String(), nil
	case "DstIP":
		return obj.DstIP.String(), nil
	case "DstMAC":
		return obj.DstMAC.String(), nil
	case "Type":
		return string(obj.Type), nil
	case "Mode":
		return string(obj.Mode), nil
	case "Payload":
		return string(obj.Payload), nil
	case "ID":
		return string(obj.ID), nil
	case "State":
		return string(obj.State), nil
	}
	return "", common.ErrFieldNotFound
}

func (obj *InjectionMetadata) GetFieldKeys() []string {
	return []string{
		"SrcIP",
		"SrcMAC",
		"SrcPort",
		"DstIP",
		"DstMAC",
		"DstPort",
		"Type",
		"Count",
		"ICMPID",
		"Interval",
		"Mode",
		"IncrementPayload",
		"Payload",
		"Pcap",
		"TTL",
		"ID",
		"State",
		"PacketCount",
	}
}

func (obj *InjectionMetadata) MatchBool(key string, predicate common.BoolPredicate) bool {
	return false
}

func (obj *InjectionMetadata) MatchInt64(key string, predicate common.Int64Predicate) bool {
	if b, err := obj.GetFieldInt64(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *InjectionMetadata) MatchString(key string, predicate common.StringPredicate) bool {
	if b, err := obj.GetFieldString(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *InjectionMetadata) GetField(key string) (interface{}, error) {
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
	case "Pcap":
		if obj.Pcap != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.Pcap {
					results = append(results, obj)
				}
				return results, nil
			}
		}

	}
	return nil, common.ErrFieldNotFound
}

func (obj *Injections) GetFieldBool(key string) (bool, error) {
	return false, common.ErrFieldNotFound
}

func (obj *Injections) GetFieldInt64(key string) (int64, error) {
	switch key {
	}
	return 0, common.ErrFieldNotFound
}

func (obj *Injections) GetFieldString(key string) (string, error) {
	switch key {
	}
	return "", common.ErrFieldNotFound
}

func (obj *Injections) GetFieldKeys() []string {
	return []string{
		"SrcIP",
		"SrcMAC",
		"SrcPort",
		"DstIP",
		"DstMAC",
		"DstPort",
		"Type",
		"Count",
		"ICMPID",
		"Interval",
		"Mode",
		"IncrementPayload",
		"Payload",
		"Pcap",
		"TTL",
		"ID",
		"State",
		"PacketCount",
	}
}

func (obj *Injections) MatchBool(key string, predicate common.BoolPredicate) bool {
	for _, obj := range *obj {
		if obj.MatchBool(key, predicate) {
			return true
		}
	}
	return false
}

func (obj *Injections) MatchInt64(key string, predicate common.Int64Predicate) bool {
	for _, obj := range *obj {
		if obj.MatchInt64(key, predicate) {
			return true
		}
	}
	return false
}

func (obj *Injections) MatchString(key string, predicate common.StringPredicate) bool {
	for _, obj := range *obj {
		if obj.MatchString(key, predicate) {
			return true
		}
	}
	return false
}

func (obj *Injections) GetField(key string) (interface{}, error) {
	var result []interface{}

	for _, o := range *obj {
		switch key {
		case "SrcIP":
			result = append(result, o.SrcIP)
		case "SrcMAC":
			result = append(result, o.SrcMAC)
		case "SrcPort":
			result = append(result, o.SrcPort)
		case "DstIP":
			result = append(result, o.DstIP)
		case "DstMAC":
			result = append(result, o.DstMAC)
		case "DstPort":
			result = append(result, o.DstPort)
		case "Type":
			result = append(result, o.Type)
		case "Count":
			result = append(result, o.Count)
		case "ICMPID":
			result = append(result, o.ICMPID)
		case "Interval":
			result = append(result, o.Interval)
		case "Mode":
			result = append(result, o.Mode)
		case "IncrementPayload":
			result = append(result, o.IncrementPayload)
		case "Payload":
			result = append(result, o.Payload)
		case "Pcap":
			result = append(result, o.Pcap)
		case "TTL":
			result = append(result, o.TTL)
		case "ID":
			result = append(result, o.ID)
		case "State":
			result = append(result, o.State)
		case "PacketCount":
			result = append(result, o.PacketCount)
		default:
			return result, common.ErrFieldNotFound
		}
	}

	return result, nil
}

func init() {
	strings.Index("", ".")
}
