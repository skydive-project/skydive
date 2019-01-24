package fieldexcluder

import (
	"reflect"
	"strings"
)

func getStructFields(v reflect.Value) map[string]reflect.Value {
	res := make(map[string]reflect.Value)
	for i := 0; i < v.NumField(); i++ {
		res[v.Type().Field(i).Name] = v.Field(i)
	}
	return res
}

func matchExcludedFields(field string, excludedFields []string) []string {
	res := make([]string, 0)
	for _, f := range excludedFields {
		if strings.HasPrefix(f, field) {
			if f == field {
				return nil
			}
			res = append(res, f[len(field)+1:])
		}
	}
	return res
}

func generateExcludeFieldsFunction(v reflect.Value, excludedFields []string) func(interface{}) map[string]interface{} {
	if len(excludedFields) == 0 || v.Kind().String() != "struct" {
		return nil
	}

	fieldMap := getStructFields(v)
	fieldFuncsMap := make(map[string]func(interface{}) map[string]interface{})
	for field := range fieldMap {
		excludedSubFields := matchExcludedFields(strings.ToLower(field), excludedFields)
		if excludedSubFields != nil {
			fieldFuncsMap[field] = generateExcludeFieldsFunction(fieldMap[field], excludedSubFields)
		}
	}

	return func(val interface{}) map[string]interface{} {
		v := reflect.ValueOf(val)
		res := make(map[string]interface{}, len(fieldFuncsMap))
		for field, subFunc := range fieldFuncsMap {
			fieldValue := v.FieldByName(field).Interface()
			if subFunc == nil {
				res[field] = fieldValue
			} else {
				res[field] = subFunc(fieldValue)
			}
		}
		return res
	}
}

// GenerateExcludeFieldsFunction returns a function that filters out fields determined by excludedFields
func GenerateExcludeFieldsFunction(value interface{}, excludedFields []string) func(interface{}) map[string]interface{} {
	return generateExcludeFieldsFunction(reflect.ValueOf(value), excludedFields)
}
