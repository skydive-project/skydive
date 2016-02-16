package govalid

import (
	"fmt"
	"github.com/gima/govalid/v1/internal"
	"reflect"
)

// -----------------------------------------------------------------------------

// Object validation function. Parameter is reflection to a map.
type ObjectOpt func(m *reflect.Value) (path string, err error)

// -----------------------------------------------------------------------------

// Construct an object validator using the specified validation functions.
// Validates data type to be either a map or a pointer to such
//
// Currently this validator supports only map data type.
func Object(opts ...ObjectOpt) Validator {
	return &objectValidator{opts}
}

// -----------------------------------------------------------------------------

// Array validator function for validating every key (of key->value pair) with the specified validator.
func ObjKeys(v Validator) ObjectOpt {
	return func(m *reflect.Value) (path string, _ error) {
		for _, key := range m.MapKeys() {
			k := key.Interface()
			if path, err := v.Validate(k); err != nil {
				return fmt.Sprintf("Key(%v)->%s", k, path), err
			}
		}
		return "", nil
	}
}

// -----------------------------------------------------------------------------

// Array validator function for validating every value (of key->value pair) with the specified validator.
func ObjValues(v Validator) ObjectOpt {
	return func(m *reflect.Value) (path string, _ error) {
		for _, key := range m.MapKeys() {
			value := m.MapIndex(key).Interface()
			if path, err := v.Validate(value); err != nil {
				return fmt.Sprintf("Key[%v].Value->%s", key.Interface(), path), err
			}
		}
		return "", nil
	}
}

// -----------------------------------------------------------------------------

// Array validator function for validating a specific key's value (of key->value pair) with the specified validator.
// If the map under validation doesn't have such key, nil is passed to the Validator (hint: Optional Validator).
//
// keys keys keys
func ObjKV(key interface{}, v Validator) ObjectOpt {
	return func(m *reflect.Value) (path string, _ error) {
		var refkey reflect.Value

		if key == nil {
			refkey = reflect.Zero(m.Type().Key())
		} else {
			refkey = reflect.ValueOf(key)
		}

		var value interface{}

		refval := m.MapIndex(refkey)
		if !refval.IsValid() {
			value = nil
		} else {
			value = refval.Interface()
		}

		if path, err := v.Validate(value); err != nil {
			return fmt.Sprintf("Key[%v].Value->%s", key, path), err
		}

		return "", nil
	}
}

// -----------------------------------------------------------------------------

// validator for an object
type objectValidator struct {
	opts []ObjectOpt
}

// -----------------------------------------------------------------------------

// the actual workhorse for object validator
func (r *objectValidator) Validate(data interface{}) (string, error) {

	value, err := internal.ReflectOrIndirect(data)
	if err != nil {
		return "Object", fmt.Errorf("expected (*)map, got %s", err)
	}

	if value.Kind() != reflect.Map {
		return "Object", fmt.Errorf("expected (*)map, got %s", value.Type())
	}

	for _, o := range r.opts {
		if path, err := o(value); err != nil {
			return "Object->" + path, err
		}
	}

	return "", nil
}
