// Code generated - DO NOT EDIT.

package ovnmodel

import (
	"github.com/skydive-project/skydive/graffiti/getter"
	"strings"
)

func (obj *NBGlobal) GetFieldBool(key string) (bool, error) {
	switch key {
	case "Ipsec":
		return obj.Ipsec, nil
	}

	return false, getter.ErrFieldNotFound
}

func (obj *NBGlobal) GetFieldInt64(key string) (int64, error) {
	switch key {
	case "HvCfg":
		return int64(obj.HvCfg), nil
	case "HvCfgTimestamp":
		return int64(obj.HvCfgTimestamp), nil
	case "NbCfg":
		return int64(obj.NbCfg), nil
	case "NbCfgTimestamp":
		return int64(obj.NbCfgTimestamp), nil
	case "SbCfg":
		return int64(obj.SbCfg), nil
	case "SbCfgTimestamp":
		return int64(obj.SbCfgTimestamp), nil
	}
	return 0, getter.ErrFieldNotFound
}

func (obj *NBGlobal) GetFieldString(key string) (string, error) {
	switch key {
	case "UUID":
		return string(obj.UUID), nil
	case "Name":
		return string(obj.Name), nil
	}
	return "", getter.ErrFieldNotFound
}

func (obj *NBGlobal) GetFieldKeys() []string {
	return []string{
		"UUID",
		"Connections",
		"ExternalIDs",
		"HvCfg",
		"HvCfgTimestamp",
		"Ipsec",
		"Name",
		"NbCfg",
		"NbCfgTimestamp",
		"Options",
		"SbCfg",
		"SbCfgTimestamp",
		"SSL",
		"ExternalIDsMeta",
		"OptionsMeta",
	}
}

func (obj *NBGlobal) MatchBool(key string, predicate getter.BoolPredicate) bool {
	if b, err := obj.GetFieldBool(key); err == nil {
		return predicate(b)
	}

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {
	case "ExternalIDsMeta":
		if index != -1 && obj.ExternalIDsMeta != nil {
			return obj.ExternalIDsMeta.MatchBool(key[index+1:], predicate)
		}
	case "OptionsMeta":
		if index != -1 && obj.OptionsMeta != nil {
			return obj.OptionsMeta.MatchBool(key[index+1:], predicate)
		}
	}
	return false
}

func (obj *NBGlobal) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	if b, err := obj.GetFieldInt64(key); err == nil {
		return predicate(b)
	}

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {

	case "ExternalIDsMeta":
		if index != -1 && obj.ExternalIDsMeta != nil {
			return obj.ExternalIDsMeta.MatchInt64(key[index+1:], predicate)
		}
	case "OptionsMeta":
		if index != -1 && obj.OptionsMeta != nil {
			return obj.OptionsMeta.MatchInt64(key[index+1:], predicate)
		}
	}
	return false
}

func (obj *NBGlobal) MatchString(key string, predicate getter.StringPredicate) bool {
	if b, err := obj.GetFieldString(key); err == nil {
		return predicate(b)
	}

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {

	case "Connections":
		if index == -1 {
			for _, i := range obj.Connections {
				if predicate(i) {
					return true
				}
			}
		}
	case "ExternalIDs":
		if obj.ExternalIDs != nil {
			if index == -1 {
				for _, v := range obj.ExternalIDs {
					if predicate(v) {
						return true
					}
				}
			} else if v, found := obj.ExternalIDs[key[index+1:]]; found {
				return predicate(v)
			}
		}
	case "Options":
		if obj.Options != nil {
			if index == -1 {
				for _, v := range obj.Options {
					if predicate(v) {
						return true
					}
				}
			} else if v, found := obj.Options[key[index+1:]]; found {
				return predicate(v)
			}
		}
	case "SSL":
		if index == -1 {
			for _, i := range obj.SSL {
				if predicate(i) {
					return true
				}
			}
		}
	case "ExternalIDsMeta":
		if index != -1 && obj.ExternalIDsMeta != nil {
			return obj.ExternalIDsMeta.MatchString(key[index+1:], predicate)
		}
	case "OptionsMeta":
		if index != -1 && obj.OptionsMeta != nil {
			return obj.OptionsMeta.MatchString(key[index+1:], predicate)
		}
	}
	return false
}

func (obj *NBGlobal) GetField(key string) (interface{}, error) {
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
	case "Connections":
		if obj.Connections != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.Connections {
					results = append(results, obj)
				}
				return results, nil
			}
		}
	case "SSL":
		if obj.SSL != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.SSL {
					results = append(results, obj)
				}
				return results, nil
			}
		}
	case "ExternalIDsMeta":
		if obj.ExternalIDsMeta != nil {
			if index != -1 {
				return obj.ExternalIDsMeta.GetField(key[index+1:])
			} else {
				return obj.ExternalIDsMeta, nil
			}
		}
	case "OptionsMeta":
		if obj.OptionsMeta != nil {
			if index != -1 {
				return obj.OptionsMeta.GetField(key[index+1:])
			} else {
				return obj.OptionsMeta, nil
			}
		}

	}
	return nil, getter.ErrFieldNotFound
}

func init() {
	strings.Index("", ".")
}
