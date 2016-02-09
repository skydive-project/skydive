package jsonv

import (
    "fmt"
)

/*
Validates the data with the specified validator, if the data is present. Validation succeeds if it is not. Value in `V` must be present.

Note: `json.Unmarshal` will convert JSON `null` to Go's `nil`, and thus if this validator is given a pointer to `nil`, it is treated as not present, and thus validates.
*/
type Optional struct {
    V Validator
}

func (self *Optional) Validate(data *interface{}) (string, error) {
    if (*data == nil) {
        return "Optional(Not present)", nil
    } else {
        desc, err := self.V.Validate(data)
        return fmt.Sprintf("Optional->%s", desc), err
    }
}
