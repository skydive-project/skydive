package jsonv

import (
    "fmt"
    "reflect"
)

/*
Ensures the data is string, with optional content and length checks.

Note: Value 0 in either `MinLen` or `MaxLen` is ignored.
*/
type String struct {
    Value string
    MinLen int
    MaxLen int
}

func (self *String) Validate(data *interface{}) (string, error) {
    var validate *string
    
    switch tmp := (*data).(type) {
    case string:
        validate = &tmp
    case *string:
        validate = tmp
    default:
        return "String", fmt.Errorf("expected string, was %v", reflect.TypeOf(*data))
    }
    
    if len(self.Value) != 0 {
        if *validate != self.Value { return "String", fmt.Errorf("expected %s, got %s", self.Value, *validate) }
    }
    if self.MinLen != 0 {
        if len(*validate) < self.MinLen { return "String", fmt.Errorf("length should >=%d, was %d", self.MinLen, len(*validate)) }
    }
    if self.MaxLen != 0 {
        if len(*validate) > self.MaxLen { return "String", fmt.Errorf("length should <=%d, was %d", self.MaxLen, len(*validate)) }
    }
    
    return "String", nil
}
