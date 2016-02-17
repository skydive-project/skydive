package govalid

// -----------------------------------------------------------------------------

// Arbitrary validation function.
type ValidatorFunc func(data interface{}) (path string, err error)

// -----------------------------------------------------------------------------

// Construct a validator using a validation function.
func Function(validatorfunc ValidatorFunc) Validator {
	return &functionValidator{validatorfunc}
}

// -----------------------------------------------------------------------------

// validator for using an arbitrary function
type functionValidator struct {
	validatorfunc ValidatorFunc
}

// -----------------------------------------------------------------------------

// the actual workhorse for arbitrary function validator
func (r *functionValidator) Validate(data interface{}) (string, error) {
	return r.validatorfunc(data)
}

// -----------------------------------------------------------------------------
