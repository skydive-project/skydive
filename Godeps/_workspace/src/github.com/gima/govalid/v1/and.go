package govalid

import (
	"fmt"
)

// -----------------------------------------------------------------------------

// Construct a logical-and validator using the specified validators.
// Given no validators, this validator passes always.
func And(validators ...Validator) Validator {
	return &andValidator{validators}
}

// -----------------------------------------------------------------------------

// validator for logical-and
type andValidator struct {
	validators []Validator
}

// -----------------------------------------------------------------------------

// the actual workhorse for logical-and validator
func (r *andValidator) Validate(data interface{}) (string, error) {

	for i, v := range r.validators {
		if path, err := v.Validate(data); err != nil {
			return fmt.Sprintf("And(idx: %d)->%s", i+1, path), err
		}
	}

	return "", nil
}

// -----------------------------------------------------------------------------
