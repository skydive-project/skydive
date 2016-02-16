package govalid

import (
	"fmt"
	"reflect"
)

// -----------------------------------------------------------------------------

// Array validation function. Parameter is reflection to array or slice.
type ArrayOpt func(slice *reflect.Value) (path string, err error)

// -----------------------------------------------------------------------------

// Construct an array validator using the specified validation functions.
// Validates data type to be either an array or a slice, or a pointer to such.
func Array(opts ...ArrayOpt) Validator {
	return &arrayValidator{opts}
}

// -----------------------------------------------------------------------------

// Array validator function for checking minimum number of entries.
func ArrMin(min int) ArrayOpt {
	return func(slice *reflect.Value) (path string, err error) {
		if slice.Len() < min {
			return "", fmt.Errorf("number of entries %d should >= %d", slice.Len(), min)
		}
		return "", nil
	}
}

// -----------------------------------------------------------------------------

// Array validator function for checking maximum number of entries.
func ArrMax(max int) ArrayOpt {
	return func(slice *reflect.Value) (path string, err error) {
		if slice.Len() > max {
			return "", fmt.Errorf("number of entries %d should <= %d", slice.Len(), max)
		}
		return "", nil
	}
}

// -----------------------------------------------------------------------------

// Array validator function for checking each entry with the specified validator.
func ArrEach(validator Validator) ArrayOpt {
	return func(slice *reflect.Value) (path string, err error) {
		for i := 0; i < slice.Len(); i++ {
			path, err := validator.Validate(slice.Index(i).Interface())
			if err != nil {
				return path, fmt.Errorf("idx %d: %s", i, err.Error())
			}
		}
		return "", nil
	}
}

// -----------------------------------------------------------------------------

// validator for array
type arrayValidator struct {
	opts []ArrayOpt
}

// -----------------------------------------------------------------------------

// the actual workhorse for array validator
func (r *arrayValidator) Validate(data interface{}) (string, error) {
	value := reflect.ValueOf(data)
	switch value.Kind() {
	case reflect.Invalid:
		return "Array", fmt.Errorf("expected (*)array/slice, got <nil>")

	case reflect.Ptr:
		if value.IsNil() {
			return "Array", fmt.Errorf("expected (*)array/slice, got <nil> %s", value.Kind())
		}
		value = value.Elem()

	case reflect.Array:
		// pass through

	case reflect.Slice:
		// pass through

	default:
		return "Array", fmt.Errorf("expected (*)array/slice, got %s", value.Kind())
	}

	for _, o := range r.opts {
		if path, err := o(&value); err != nil {
			if path == "" {
				return "Array", err
			}
			return "Array->" + path, err
		}
	}

	return "", nil
}

// -----------------------------------------------------------------------------
