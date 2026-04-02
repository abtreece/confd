package template

import (
	"reflect"
)

func utilityFuncMap() map[string]interface{} {
	m := make(map[string]interface{})
	// Defaults & Conditionals
	m["default"] = dfault
	m["ternary"] = ternary
	m["coalesce"] = coalesce
	m["empty"] = isEmpty
	return m
}

// isEmpty returns true if the given value is the zero value for its type,
// nil, or an empty collection/string.
func isEmpty(val interface{}) bool {
	if val == nil {
		return true
	}
	v := reflect.ValueOf(val)
	switch v.Kind() {
	case reflect.String, reflect.Array, reflect.Slice, reflect.Map:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Ptr, reflect.Interface:
		return v.IsNil()
	default:
		return false
	}
}

// dfault returns val if it is non-empty, otherwise defaultVal.
func dfault(defaultVal, val interface{}) interface{} {
	if isEmpty(val) {
		return defaultVal
	}
	return val
}

// ternary returns trueVal if condition is true, otherwise falseVal.
// condition is the last argument for pipeline use.
func ternary(trueVal, falseVal interface{}, condition bool) interface{} {
	if condition {
		return trueVal
	}
	return falseVal
}

// coalesce returns the first non-empty value, or nil if all are empty.
func coalesce(vals ...interface{}) interface{} {
	for _, v := range vals {
		if !isEmpty(v) {
			return v
		}
	}
	return nil
}
