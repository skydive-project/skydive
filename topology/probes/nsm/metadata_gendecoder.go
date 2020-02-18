// Code generated - DO NOT EDIT.

package nsm

import (
	"github.com/skydive-project/skydive/graffiti/getter"
	"strings"
)

func (obj *BaseConnectionMetadata) GetFieldBool(key string) (bool, error) {
	return false, getter.ErrFieldNotFound
}

func (obj *BaseConnectionMetadata) GetFieldInt64(key string) (int64, error) {
	return 0, getter.ErrFieldNotFound
}

func (obj *BaseConnectionMetadata) GetFieldString(key string) (string, error) {
	switch key {
	case "MechanismType":
		return string(obj.MechanismType), nil
	}
	return "", getter.ErrFieldNotFound
}

func (obj *BaseConnectionMetadata) GetFieldKeys() []string {
	return []string{
		"MechanismType",
		"MechanismParameters",
		"Labels",
	}
}

func (obj *BaseConnectionMetadata) MatchBool(key string, predicate getter.BoolPredicate) bool {
	return false
}

func (obj *BaseConnectionMetadata) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	return false
}

func (obj *BaseConnectionMetadata) MatchString(key string, predicate getter.StringPredicate) bool {
	if b, err := obj.GetFieldString(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *BaseConnectionMetadata) GetField(key string) (interface{}, error) {
	if s, err := obj.GetFieldString(key); err == nil {
		return s, nil
	}
	return nil, getter.ErrFieldNotFound
}

func (obj *BaseNSMMetadata) GetFieldBool(key string) (bool, error) {
	return false, getter.ErrFieldNotFound
}

func (obj *BaseNSMMetadata) GetFieldInt64(key string) (int64, error) {
	return 0, getter.ErrFieldNotFound
}

func (obj *BaseNSMMetadata) GetFieldString(key string) (string, error) {
	switch key {
	case "NetworkService":
		return string(obj.NetworkService), nil
	case "Payload":
		return string(obj.Payload), nil
	}
	return "", getter.ErrFieldNotFound
}

func (obj *BaseNSMMetadata) GetFieldKeys() []string {
	return []string{
		"NetworkService",
		"Payload",
		"Source",
		"Destination",
	}
}

func (obj *BaseNSMMetadata) MatchBool(key string, predicate getter.BoolPredicate) bool {
	return false
}

func (obj *BaseNSMMetadata) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	return false
}

func (obj *BaseNSMMetadata) MatchString(key string, predicate getter.StringPredicate) bool {
	if b, err := obj.GetFieldString(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *BaseNSMMetadata) GetField(key string) (interface{}, error) {
	if s, err := obj.GetFieldString(key); err == nil {
		return s, nil
	}
	return nil, getter.ErrFieldNotFound
}

func (obj *EdgeMetadata) GetFieldBool(key string) (bool, error) {
	return false, getter.ErrFieldNotFound
}

func (obj *EdgeMetadata) GetFieldInt64(key string) (int64, error) {
	return 0, getter.ErrFieldNotFound
}

func (obj *EdgeMetadata) GetFieldString(key string) (string, error) {
	switch key {
	case "NetworkService":
		return string(obj.NetworkService), nil
	case "Payload":
		return string(obj.Payload), nil
	case "CrossConnectID":
		return string(obj.CrossConnectID), nil
	case "SourceCrossConnectID":
		return string(obj.SourceCrossConnectID), nil
	case "DestinationCrossConnectID":
		return string(obj.DestinationCrossConnectID), nil
	}
	return "", getter.ErrFieldNotFound
}

func (obj *EdgeMetadata) GetFieldKeys() []string {
	return []string{
		"NetworkService",
		"Payload",
		"Source",
		"Destination",
		"CrossConnectID",
		"SourceCrossConnectID",
		"DestinationCrossConnectID",
		"Via",
	}
}

func (obj *EdgeMetadata) MatchBool(key string, predicate getter.BoolPredicate) bool {
	return false
}

func (obj *EdgeMetadata) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	return false
}

func (obj *EdgeMetadata) MatchString(key string, predicate getter.StringPredicate) bool {
	if b, err := obj.GetFieldString(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *EdgeMetadata) GetField(key string) (interface{}, error) {
	if s, err := obj.GetFieldString(key); err == nil {
		return s, nil
	}
	return nil, getter.ErrFieldNotFound
}

func (obj *LocalConnectionMetadata) GetFieldBool(key string) (bool, error) {
	return false, getter.ErrFieldNotFound
}

func (obj *LocalConnectionMetadata) GetFieldInt64(key string) (int64, error) {
	return 0, getter.ErrFieldNotFound
}

func (obj *LocalConnectionMetadata) GetFieldString(key string) (string, error) {
	switch key {
	case "MechanismType":
		return string(obj.MechanismType), nil
	case "IP":
		return string(obj.IP), nil
	}
	return "", getter.ErrFieldNotFound
}

func (obj *LocalConnectionMetadata) GetFieldKeys() []string {
	return []string{
		"MechanismType",
		"MechanismParameters",
		"Labels",
		"IP",
	}
}

func (obj *LocalConnectionMetadata) MatchBool(key string, predicate getter.BoolPredicate) bool {
	return false
}

func (obj *LocalConnectionMetadata) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	return false
}

func (obj *LocalConnectionMetadata) MatchString(key string, predicate getter.StringPredicate) bool {
	if b, err := obj.GetFieldString(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *LocalConnectionMetadata) GetField(key string) (interface{}, error) {
	if s, err := obj.GetFieldString(key); err == nil {
		return s, nil
	}
	return nil, getter.ErrFieldNotFound
}

func (obj *LocalNSMMetadata) GetFieldBool(key string) (bool, error) {
	return false, getter.ErrFieldNotFound
}

func (obj *LocalNSMMetadata) GetFieldInt64(key string) (int64, error) {
	return 0, getter.ErrFieldNotFound
}

func (obj *LocalNSMMetadata) GetFieldString(key string) (string, error) {
	switch key {
	case "CrossConnectID":
		return string(obj.CrossConnectID), nil
	}
	return "", getter.ErrFieldNotFound
}

func (obj *LocalNSMMetadata) GetFieldKeys() []string {
	return []string{
		"CrossConnectID",
	}
}

func (obj *LocalNSMMetadata) MatchBool(key string, predicate getter.BoolPredicate) bool {
	return false
}

func (obj *LocalNSMMetadata) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	return false
}

func (obj *LocalNSMMetadata) MatchString(key string, predicate getter.StringPredicate) bool {
	if b, err := obj.GetFieldString(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *LocalNSMMetadata) GetField(key string) (interface{}, error) {
	if s, err := obj.GetFieldString(key); err == nil {
		return s, nil
	}
	return nil, getter.ErrFieldNotFound
}

func (obj *RemoteConnectionMetadata) GetFieldBool(key string) (bool, error) {
	return false, getter.ErrFieldNotFound
}

func (obj *RemoteConnectionMetadata) GetFieldInt64(key string) (int64, error) {
	return 0, getter.ErrFieldNotFound
}

func (obj *RemoteConnectionMetadata) GetFieldString(key string) (string, error) {
	switch key {
	case "MechanismType":
		return string(obj.MechanismType), nil
	case "SourceNSM":
		return string(obj.SourceNSM), nil
	case "DestinationNSM":
		return string(obj.DestinationNSM), nil
	case "NetworkServiceEndpoint":
		return string(obj.NetworkServiceEndpoint), nil
	}
	return "", getter.ErrFieldNotFound
}

func (obj *RemoteConnectionMetadata) GetFieldKeys() []string {
	return []string{
		"MechanismType",
		"MechanismParameters",
		"Labels",
		"SourceNSM",
		"DestinationNSM",
		"NetworkServiceEndpoint",
	}
}

func (obj *RemoteConnectionMetadata) MatchBool(key string, predicate getter.BoolPredicate) bool {
	return false
}

func (obj *RemoteConnectionMetadata) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	return false
}

func (obj *RemoteConnectionMetadata) MatchString(key string, predicate getter.StringPredicate) bool {
	if b, err := obj.GetFieldString(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *RemoteConnectionMetadata) GetField(key string) (interface{}, error) {
	if s, err := obj.GetFieldString(key); err == nil {
		return s, nil
	}
	return nil, getter.ErrFieldNotFound
}

func (obj *RemoteNSMMetadata) GetFieldBool(key string) (bool, error) {
	return false, getter.ErrFieldNotFound
}

func (obj *RemoteNSMMetadata) GetFieldInt64(key string) (int64, error) {
	return 0, getter.ErrFieldNotFound
}

func (obj *RemoteNSMMetadata) GetFieldString(key string) (string, error) {
	switch key {
	case "SourceCrossConnectID":
		return string(obj.SourceCrossConnectID), nil
	case "DestinationCrossConnectID":
		return string(obj.DestinationCrossConnectID), nil
	}
	return "", getter.ErrFieldNotFound
}

func (obj *RemoteNSMMetadata) GetFieldKeys() []string {
	return []string{
		"SourceCrossConnectID",
		"DestinationCrossConnectID",
		"Via",
	}
}

func (obj *RemoteNSMMetadata) MatchBool(key string, predicate getter.BoolPredicate) bool {
	return false
}

func (obj *RemoteNSMMetadata) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	return false
}

func (obj *RemoteNSMMetadata) MatchString(key string, predicate getter.StringPredicate) bool {
	if b, err := obj.GetFieldString(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *RemoteNSMMetadata) GetField(key string) (interface{}, error) {
	if s, err := obj.GetFieldString(key); err == nil {
		return s, nil
	}
	return nil, getter.ErrFieldNotFound
}

func init() {
	strings.Index("", ".")
}
