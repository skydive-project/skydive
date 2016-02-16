// This package is internal, do not use externally.
package internal

import (
	"errors"
	"fmt"
	"reflect"
)

// Get reflection to i. If i is a pointer, indirect it.
// Fails if i is nil or i is a pointer to nil.
func ReflectOrIndirect(i interface{}) (*reflect.Value, error) {
	value := reflect.ValueOf(i)
	switch value.Kind() {

	case reflect.Invalid:
		// ValueOf(nil) returns the zero Value
		// , its Kind method returns Invalid
		return nil, errors.New("<nil>")

	case reflect.Ptr:
		if value.IsNil() {
			// ensure not nil pointer
			return nil, fmt.Errorf("<nil> %s", value.Type())
		}

		// reflection to indirection of i
		value = value.Elem()
		return &value, nil

	default:
		// reflection to i
		return &value, nil
	}
}

// Get reflection to pointer i. If i is not a pointer, fabricate one.
// Fails if i is nil or i is a pointer to nil.
func ReflectPtrOrFabricate(i interface{}) (*reflect.Value, error) {
	value := reflect.ValueOf(i)
	switch value.Kind() {
	case reflect.Invalid:
		// ValueOf(nil) returns the zero Value
		// , its Kind method returns Invalid
		return nil, errors.New("<nil>")

	case reflect.Ptr:
		if value.IsNil() {
			// ensure not nil pointer
			return nil, fmt.Errorf("nil pointer %s", value.Type())
		}

		// reflection to i
		return &value, nil

	default:
		// reflection to address of a copy of i
		ptr := reflect.New(value.Type())
		ptr.Elem().Set(value)
		return &ptr, nil
	}
}
