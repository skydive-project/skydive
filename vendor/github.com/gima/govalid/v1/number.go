package govalid

import (
	"fmt"
	"reflect"
	"strconv"
)

// -----------------------------------------------------------------------------

// Number validation function.
type NumberOpt func(float64) error

// -----------------------------------------------------------------------------

// Construct a number validator using the specified validation functions.
// Validates data type to be either any numerical type or a pointer to such
// (except a complex number).
func Number(opts ...NumberOpt) Validator {
	return &numberValidator{opts}
}

// -----------------------------------------------------------------------------

// Number validator function for checking minimum value.
func NumMin(min float64) NumberOpt {
	return func(f float64) error {
		if f < min {
			fs, mins := shortFloatStr(f, min)
			return fmt.Errorf("number %s should >= %s", fs, mins)
		}
		return nil
	}
}

// -----------------------------------------------------------------------------

// Number validator function for checking maximum value.
func NumMax(max float64) NumberOpt {
	return func(f float64) error {
		if f > max {
			fs, maxs := shortFloatStr(f, max)
			return fmt.Errorf("number %s should <= %s", fs, maxs)
		}
		return nil
	}
}

// -----------------------------------------------------------------------------

// Number validator function for checking value.
func NumIs(expected float64) NumberOpt {
	return func(f float64) error {
		if f != expected {
			expecteds, fs := shortFloatStr(expected, f)
			return fmt.Errorf("expected %s, got %s", expecteds, fs)
		}
		return nil
	}
}

// -----------------------------------------------------------------------------

// validator for a number
type numberValidator struct {
	opts []NumberOpt
}

// -----------------------------------------------------------------------------

var float64type = reflect.TypeOf(float64(0))

// the actual workhorse for number validator
func (r *numberValidator) Validate(data interface{}) (string, error) {
	value := reflect.ValueOf(data)

	switch value.Kind() {
	case reflect.Invalid:
		return "Number", fmt.Errorf("expected (*)data convertible to float64, got <nil>")

	case reflect.Ptr:
		if value.IsNil() {
			return "Number", fmt.Errorf("expected (*)data convertible to float64, got <nil> %s", value.Type())
		}
		value = value.Elem()
	}

	if value.Type().ConvertibleTo(float64type) {
		value = value.Convert(float64type)
	} else {
		return "Number", fmt.Errorf("expected (*)data convertible to float64, got %s", value.Type())
	}

	v := value.Interface().(float64)

	for _, o := range r.opts {
		if err := o(v); err != nil {
			return "Number", err
		}
	}

	return "", nil
}

// -----------------------------------------------------------------------------

func shortFloatStr(a, b float64) (string, string) {
	return strconv.FormatFloat(a, 'f', -1, 64),
		strconv.FormatFloat(b, 'f', -1, 64)
}

// -----------------------------------------------------------------------------
