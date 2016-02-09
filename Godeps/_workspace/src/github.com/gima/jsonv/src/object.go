package jsonv

import (
    "fmt"
    "reflect"
)

/*
Ensures that the data is (json) object, with optional "per property" or "all properties" validator. Either `Each` or `Properties` must be set.
*/
type Object struct {
    Each ObjectEach
    Properties []ObjectItem
}

/*
Specifies separate validator for "key" and "data", that are used to validate all the properties of (json) object. Value in `KeyValidator` and `DataValidator` must be present.
*/
type ObjectEach struct {
    KeyValidator Validator
    DataValidator Validator
}

/*
Specifies a validator for "data", that is used to validate data of a named (json) object's property. Value in `Key` and `DataValidator` must be present.
*/
type ObjectItem struct {
    Key string
    DataValidator Validator
}

func (self *Object) Validate(data *interface{}) (string, error) {
    var validate *map[string]interface{}
    
    switch tmp := (*data).(type) {
    case map[string]interface{}:
        validate = &tmp
    case *map[string]interface{}:
        validate = tmp
    default:
        return "Object", fmt.Errorf("expected map[string]interface{}, was %v", reflect.TypeOf(*data))
    }
    
    if self.Each != (ObjectEach{}) {
        // can do loop without copying?
        for key, val := range *validate {
            tmpKey := interface{}(&key)
            if desc, err := self.Each.KeyValidator.Validate(&tmpKey); err != nil {
                return fmt.Sprintf(`Object(Each, key("%s"))->%s`, key, desc), err
            }
            tmpVal := interface{}(val)
            if desc, err := self.Each.DataValidator.Validate(&tmpVal); err != nil {
                return fmt.Sprintf(`Object(Each, key("%s").Value->%s`, key, desc), err
            }
        }
        return "Object(Each)", nil
    }
    
    if self.Properties != nil {
        // can do loop without copying?
        for _, proofProperty := range self.Properties {
            val := interface{}((*validate)[proofProperty.Key])
            if desc, err := proofProperty.DataValidator.Validate(&val); err != nil {
                return fmt.Sprintf(`Object(Items, key("%s").Value)->%s`, proofProperty.Key, desc), err
            }
        }
        return "Object(Items)", nil
    }
    
    return "Object", nil
}
