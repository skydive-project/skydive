package libovsdb

import (
	"encoding/json"
	"errors"
	"reflect"
)

// OvsSet is an OVSDB style set
// RFC 7047 has a wierd (but understandable) notation for set as described as :
// Either an <atom>, representing a set with exactly one element, or
// a 2-element JSON array that represents a database set value.  The
// first element of the array must be the string "set", and the
// second element must be an array of zero or more <atom>s giving the
// values in the set.  All of the <atom>s must have the same type.
type OvsSet struct {
	GoSet []interface{}
}

// NewOvsSet creates a new OVSDB style set from a Go slice
func NewOvsSet(goSlice interface{}) (*OvsSet, error) {
	v := reflect.ValueOf(goSlice)
	if v.Kind() != reflect.Slice {
		return nil, errors.New("OvsSet supports only Go Slice types")
	}

	var ovsSet []interface{}
	for i := 0; i < v.Len(); i++ {
		ovsSet = append(ovsSet, v.Index(i).Interface())
	}
	return &OvsSet{ovsSet}, nil
}

// MarshalJSON wil marshal an OVSDB style set in to a JSON byte array
func (o OvsSet) MarshalJSON() ([]byte, error) {
	var oSet []interface{}
	oSet = append(oSet, "set")
	oSet = append(oSet, o.GoSet)
	return json.Marshal(oSet)
}

// UnmarshalJSON will unmarshal a JSON byte array to an OVSDB style set
func (o *OvsSet) UnmarshalJSON(b []byte) (err error) {
	var oSet []interface{}
	if err = json.Unmarshal(b, &oSet); err == nil && len(oSet) > 1 {
		innerSet := oSet[1].([]interface{})
		for _, val := range innerSet {
			goVal, err := ovsSliceToGoNotation(val)
			if err == nil {
				o.GoSet = append(o.GoSet, goVal)
			}
		}
	}
	return err
}
