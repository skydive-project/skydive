package govalid

import (
	"fmt"
)

// -----------------------------------------------------------------------------

// Construct a logical-or validator using the specified validators.
// Given no validators, this validator passes always.
func Or(validators ...Validator) Validator {
	return &orValidator{validators}
}

// -----------------------------------------------------------------------------

// validator for logical-or
type orValidator struct {
	validators []Validator
}

// -----------------------------------------------------------------------------

// the actual workhorse for logical-or validator
func (r *orValidator) Validate(data interface{}) (string, error) {

	numvalidators := len(r.validators)

	for i, v := range r.validators {
		path, err := v.Validate(data)
		if err == nil {
			return "", nil
		}

		if i == numvalidators-1 {
			return fmt.Sprintf("Or(%d)->%s", i+1, path), err
		}
	}

	// never should reach this
	return "", nil
}

// -----------------------------------------------------------------------------
