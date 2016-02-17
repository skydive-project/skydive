package govalid

import (
	"fmt"
	"reflect"
)

// -----------------------------------------------------------------------------

// Boolean validation function.
type BooleanOpt func(bool) error

// -----------------------------------------------------------------------------

// Construct a boolean validator using the specified validation functions.
// Validates data type to be either a bool or a pointer to such.
func Boolean(opts ...BooleanOpt) Validator {
	return &boolValidator{opts}
}

// -----------------------------------------------------------------------------

// validator for boolean
type boolValidator struct {
	opts []BooleanOpt
}

// -----------------------------------------------------------------------------

// Validation function for checking boolean value.
func BoolIs(expected bool) BooleanOpt {
	return func(got bool) error {
		if got != expected {
			return fmt.Errorf("expected %t, got %t", expected, got)
		}
		return nil
	}
}

// -----------------------------------------------------------------------------

// the actual workhorse for boolean validator
func (r *boolValidator) Validate(data interface{}) (string, error) {
	var v bool

	switch tmp := data.(type) {
	case bool:
		v = tmp
	case *bool:
		if tmp == nil {
			return "Boolean", fmt.Errorf("expected (*)bool, got <nil>")
		}
		v = *tmp
	default:
		return "Boolean", fmt.Errorf("expected (*)bool, got %v", reflect.TypeOf(data))
	}

	for _, o := range r.opts {
		if err := o(v); err != nil {
			return "Boolean", err
		}
	}

	return "", nil
}

// -----------------------------------------------------------------------------
