package jsonv

import (
    "fmt"
    "reflect"
)

/*
Ensures that the data is boolean.
*/
type Boolean struct {}

func (self *Boolean) Validate(data *interface{}) (string, error) {
    switch (*data).(type) {
    case bool:
    case *bool:
    default:
        return "Boolean", fmt.Errorf("expected bool, was %v", reflect.TypeOf(*data))
    }
    
    return "Boolean", nil
}
