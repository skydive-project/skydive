package jsonv

import (
    "fmt"
)

/*
Run the data through a function, which decides whether the data is valid or not. Value in `ValidatorFunc` must be present.

Note: See `Validator` interface for `ValidatorFunc`'s description.
*/
type Function struct {
    ValidatorFunc func(*interface{}) (string, error)
}

func (self *Function) Validate(data *interface{}) (string, error) {
    desc, err := self.ValidatorFunc(data)
    return fmt.Sprintf("Function(%s)", desc), err
}
