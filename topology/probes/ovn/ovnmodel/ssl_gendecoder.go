// Code generated - DO NOT EDIT.

package ovnmodel

import (
	"github.com/skydive-project/skydive/graffiti/getter"
	"strings"
)

func (obj *SSL) GetFieldBool(key string) (bool, error) {
	switch key {
	case "BootstrapCaCert":
		return obj.BootstrapCaCert, nil
	}

	return false, getter.ErrFieldNotFound
}

func (obj *SSL) GetFieldInt64(key string) (int64, error) {
	return 0, getter.ErrFieldNotFound
}

func (obj *SSL) GetFieldString(key string) (string, error) {
	switch key {
	case "UUID":
		return string(obj.UUID), nil
	case "CaCert":
		return string(obj.CaCert), nil
	case "Certificate":
		return string(obj.Certificate), nil
	case "PrivateKey":
		return string(obj.PrivateKey), nil
	case "SSLCiphers":
		return string(obj.SSLCiphers), nil
	case "SSLProtocols":
		return string(obj.SSLProtocols), nil
	}
	return "", getter.ErrFieldNotFound
}

func (obj *SSL) GetFieldKeys() []string {
	return []string{
		"UUID",
		"BootstrapCaCert",
		"CaCert",
		"Certificate",
		"ExternalIDs",
		"PrivateKey",
		"SSLCiphers",
		"SSLProtocols",
		"ExternalIDsMeta",
	}
}

func (obj *SSL) MatchBool(key string, predicate getter.BoolPredicate) bool {
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
	}
	return false
}

func (obj *SSL) MatchInt64(key string, predicate getter.Int64Predicate) bool {
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
	}
	return false
}

func (obj *SSL) MatchString(key string, predicate getter.StringPredicate) bool {
	if b, err := obj.GetFieldString(key); err == nil {
		return predicate(b)
	}

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {

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
	case "ExternalIDsMeta":
		if index != -1 && obj.ExternalIDsMeta != nil {
			return obj.ExternalIDsMeta.MatchString(key[index+1:], predicate)
		}
	}
	return false
}

func (obj *SSL) GetField(key string) (interface{}, error) {
	if s, err := obj.GetFieldString(key); err == nil {
		return s, nil
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
	case "ExternalIDsMeta":
		if obj.ExternalIDsMeta != nil {
			if index != -1 {
				return obj.ExternalIDsMeta.GetField(key[index+1:])
			} else {
				return obj.ExternalIDsMeta, nil
			}
		}

	}
	return nil, getter.ErrFieldNotFound
}

func init() {
	strings.Index("", ".")
}
