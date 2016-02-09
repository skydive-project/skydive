package jsonv

/*
Validates data. Parameter is expected to be a non-`nil` pointer to the data-to-be-validated. Failure to satisfy this will cause a panic as a result of pointer indirection.

Return (string): Description or path of the validator that failed. The bundled validators that use other validators as part of their functionality, construct a path by appending the validator's description to their own (separating them by string "->"), and returning it. This helps in interpreting a failed validation by showing which validator failed.

Return (error): Reason for the failure, if the validation didin't succeed. `nil` otherwise.

Note: `Validate` requires `*interface{}` parameter type, because `json.Unmarshal` requires that too. That, and validation is more efficient when passing pointers around, rather than copying data.
*/
type Validator interface {
    Validate(*interface{}) (string, error)
}
