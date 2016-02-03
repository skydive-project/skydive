package jsonv

import (
    "fmt"
)

/*
Logical AND. Ensures that both of the validators validate the data. Value in `V1` and `V2` must be present.

Note: `V1` is tried first, then `V2`.
*/
type And struct {
    V1 Validator
    V2 Validator
}

func (self *And) Validate(data *interface{}) (string, error) {

    if desc, err := self.V1.Validate(data); err != nil {
        return fmt.Sprintf("And(V1)->%s", desc), err
    }
    
    if desc, err := self.V2.Validate(data); err != nil {
        return fmt.Sprintf("And(V2)->%s", desc), err
    }
    
    return "And", nil
}
