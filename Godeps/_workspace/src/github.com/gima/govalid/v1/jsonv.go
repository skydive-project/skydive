package govalid

// Validator validates any data type that the implementing validator supports.
type Validator interface {
	// Validate the given data.
	// In the case of a failed validation, path indicates which validator failed.
	Validate(interface{}) (path string, err error)
}
