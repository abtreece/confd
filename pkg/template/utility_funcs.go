package template

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

func utilityFuncMap() map[string]interface{} {
	m := make(map[string]interface{})
	// Defaults & Conditionals
	m["default"] = dfault
	m["ternary"] = ternary
	m["coalesce"] = coalesce
	m["empty"] = isEmpty
	// JSON
	m["toJson"] = toJson
	m["fromJson"] = fromJson
	m["toPrettyJson"] = toPrettyJson
	// Formatting
	m["indent"] = indent
	m["nindent"] = nindent
	m["quote"] = quote
	m["squote"] = squote
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

// toJson marshals val to a JSON string.
func toJson(val interface{}) (string, error) {
	b, err := json.Marshal(val)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// fromJson unmarshals a JSON string into an interface{}.
func fromJson(s string) (interface{}, error) {
	var result interface{}
	err := json.Unmarshal([]byte(s), &result)
	return result, err
}

// toPrettyJson marshals val to a pretty-printed JSON string with 2-space indent.
func toPrettyJson(val interface{}) (string, error) {
	b, err := json.MarshalIndent(val, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// indent prepends each line of s with the given number of spaces.
func indent(spaces int, s string) string {
	if s == "" {
		return ""
	}
	pad := strings.Repeat(" ", spaces)
	return pad + strings.ReplaceAll(s, "\n", "\n"+pad)
}

// nindent is like indent but prepends a newline before the indented string.
func nindent(spaces int, s string) string {
	return "\n" + indent(spaces, s)
}

// quote wraps s in double quotes.
func quote(s string) string {
	return fmt.Sprintf("%q", s)
}

// squote wraps s in single quotes.
func squote(s string) string {
	return "'" + s + "'"
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
