package jsonv

import (
    "fmt"
    "reflect"
)

/*
Ensures that the data is (json) array, with optional length checks and validation of elements. Value in `Each` must be present

Note: Value 0 in either `AtLeast` or `AtMost` is ignored.
*/
type Array struct {
    AtLeast int
    AtMost int
    Each Validator
}

func (self *Array) Validate(data *interface{}) (string, error) {
    var v *[]interface{}
    
    switch tmp := (*data).(type) {
    case []interface{}:
        v = &tmp
    case *[]interface{}:
        v = tmp
    default:
        return "Array", fmt.Errorf("expected []interface{}, was %v", reflect.TypeOf(*data))
    }
    
    if self.AtLeast != 0 {
        if len(*v) < self.AtLeast { return "Array", fmt.Errorf("length should >=%d, was %d", self.AtLeast, len(*v)) }
    }
    
    if self.AtMost != 0 {
        if len(*v) > self.AtMost { return "Array", fmt.Errorf("length should <=%d, was %d", self.AtMost, len(*v)) }
    }
    
    // cannot range iterate, since wish is to avoid copying data
    if (self.Each != nil) { for i := 0; i < len(*v); i++ {
        if desc, err := self.Each.Validate(&(*v)[i]); err != nil {
            return fmt.Sprintf("Array[%d]->%s", i, desc), err
        }
    }}
    
    return "Array", nil
}
