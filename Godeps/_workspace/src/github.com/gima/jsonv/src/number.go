package jsonv

import (
    "fmt"
    "reflect"
)

/*
Ensures that the data is (json) number, with optional length checks.

Note: Value 0 in either `Min` or `Max` is ignored.
*/
type Number struct {
    Min float64
    Max float64
}

func (self *Number) Validate(data *interface{}) (string, error) {
    var validate *float64
    
    switch tmp := (*data).(type) {
    case float64:
        validate = &tmp
    case *float64:
        validate = tmp
    default:
        return "Number", fmt.Errorf("expected float64, was %v", reflect.TypeOf(*data))
    }
    
    if self.Min != 0 {
        if *validate < self.Min { return "Number", fmt.Errorf("should >=%g, was %g", self.Min, *validate) }
    }
    if self.Max != 0 {
        if *validate > self.Max { return "Number", fmt.Errorf("should <=%g, was %g", self.Max, *validate) }
    }
    
    return "Number", nil
}
