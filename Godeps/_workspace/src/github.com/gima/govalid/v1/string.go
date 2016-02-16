package govalid

import (
	"fmt"
	"reflect"
	"regexp"
)

// -----------------------------------------------------------------------------

// String validator function.
type StringOpt func(*string) error

// -----------------------------------------------------------------------------

// Construct a string validator using the specified validator functions.
// Validates data type to be either a string or a pointer to such.
func String(opts ...StringOpt) Validator {
	return &stringValidator{opts}
}

// -----------------------------------------------------------------------------

// String validator function for checking minimum string length.
func StrMin(minlen int) StringOpt {
	return func(s *string) error {
		if len(*s) < minlen {
			return fmt.Errorf("length should >=%d, was %d", minlen, len(*s))
		}
		return nil
	}
}

// -----------------------------------------------------------------------------

// String validator function for checking maximum string length.
func StrMax(maxlen int) StringOpt {
	return func(s *string) error {
		if len(*s) > maxlen {
			return fmt.Errorf("length should <=%d, was %d", maxlen, len(*s))
		}
		return nil
	}
}

// -----------------------------------------------------------------------------

// String validator function for checking string equality.
func StrIs(expected string) StringOpt {
	return func(s *string) error {
		if *s != expected {
			return fmt.Errorf("expected %s, got %s", expected, *s)
		}
		return nil
	}
}

// -----------------------------------------------------------------------------

// String validator function for checking regular expression match of the string.
// If regular expression has error, the error is returned when validation is performed.
func StrRegExp(expr string) StringOpt {
	re, err := regexp.Compile(expr)
	return func(s *string) error {
		if err != nil {
			return fmt.Errorf("regexp compile error: %s", err)
		}

		if !re.MatchString(*s) {
			return fmt.Errorf("regexp doesn't match")
		}
		return nil
	}
}

// -----------------------------------------------------------------------------

// validator for a string
type stringValidator struct {
	opts []StringOpt
}

// -----------------------------------------------------------------------------

// the actual workhorse for string validator
func (r *stringValidator) Validate(data interface{}) (string, error) {
	var v *string

	switch tmp := data.(type) {
	case string:
		v = &tmp
	case *string:
		if tmp == nil {
			return "String", fmt.Errorf("expected (*)string, got <nil>")
		}
		v = tmp
	default:
		return "String", fmt.Errorf("expected (*)string, was %v", reflect.TypeOf(data))
	}

	for _, o := range r.opts {
		if err := o(v); err != nil {
			return "String", err
		}
	}

	return "", nil
}

// -----------------------------------------------------------------------------
