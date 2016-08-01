package govalid

import (
	"fmt"
	"reflect"
)

// -----------------------------------------------------------------------------

// Construct an optional validator using the specified validator.
// Validation succeeds when given nil or a pointer to nil.
//
// Note: json.Unmarshal() converts JSON null to Go's nil.
func Optional(validator Validator) Validator {
	return &optionalValidator{validator}
}

// -----------------------------------------------------------------------------

// validator for optional
type optionalValidator struct {
	validator Validator
}

// -----------------------------------------------------------------------------

// the actual workhorse for optional validator
func (r *optionalValidator) Validate(data interface{}) (string, error) {

	if data == nil {
		return "", nil
	}

	typ := reflect.TypeOf(data)
	if typ.Kind() == reflect.Ptr && reflect.ValueOf(data).IsNil() {
		return "", nil
	}

	if path, err := r.validator.Validate(data); err != nil {
		return fmt.Sprintf("Optional->%s", path), err
	}

	return "", nil
}

// -----------------------------------------------------------------------------
