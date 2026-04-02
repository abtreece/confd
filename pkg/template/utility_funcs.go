package template

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"unicode"
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
	// Regex
	m["regexMatch"] = regexMatch
	m["regexFind"] = regexFind
	m["regexReplaceAll"] = regexReplaceAll
	// Strings
	m["trimPrefix"] = strings.TrimPrefix
	m["repeat"] = repeat
	m["nospace"] = nospace
	m["snakecase"] = snakecase
	m["camelcase"] = camelcase
	m["kebabcase"] = kebabcase
	// Collections
	m["dict"] = CreateMap
	m["list"] = list
	m["hasKey"] = hasKey
	m["keys"] = keys
	m["values"] = values
	m["append"] = appendList
	m["pluck"] = pluck
	// Hashing
	m["sha256sum"] = sha256sum
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

// regexMatch returns true if s matches the given regular expression pattern.
func regexMatch(pattern, s string) (bool, error) {
	return regexp.MatchString(pattern, s)
}

// regexFind returns the first match of pattern in s, or empty string if no match.
func regexFind(pattern, s string) (string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", err
	}
	return re.FindString(s), nil
}

// regexReplaceAll replaces all matches of pattern in s with repl.
func regexReplaceAll(pattern, s, repl string) (string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", err
	}
	return re.ReplaceAllString(s, repl), nil
}

// repeat returns s repeated count times. Args are (count, s) for pipeline use.
func repeat(count int, s string) string {
	return strings.Repeat(s, count)
}

// nospace removes all whitespace from s.
func nospace(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
}

// splitWords splits a string into words on uppercase boundaries, underscores,
// hyphens, and spaces.
func splitWords(s string) []string {
	if s == "" {
		return nil
	}
	runes := []rune(s)
	var words []string
	var current []rune

	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r == '_' || r == '-' || r == ' ' {
			if len(current) > 0 {
				words = append(words, string(current))
				current = nil
			}
			continue
		}
		if unicode.IsUpper(r) {
			// Check if this starts a new word
			if len(current) > 0 {
				// If next char is lowercase, split before this uppercase
				// (handles "HTTPServer" -> "HTTP", "Server")
				if i+1 < len(runes) && unicode.IsLower(runes[i+1]) {
					words = append(words, string(current))
					current = nil
				} else if !unicode.IsUpper(runes[i-1]) {
					// Previous was lowercase, start new word
					words = append(words, string(current))
					current = nil
				}
			}
		}
		current = append(current, r)
	}
	if len(current) > 0 {
		words = append(words, string(current))
	}
	return words
}

// snakecase converts a string to snake_case.
func snakecase(s string) string {
	words := splitWords(s)
	if len(words) == 0 {
		return ""
	}
	for i, w := range words {
		words[i] = strings.ToLower(w)
	}
	return strings.Join(words, "_")
}

// camelcase converts a string to camelCase.
func camelcase(s string) string {
	words := splitWords(s)
	if len(words) == 0 {
		return ""
	}
	for i, w := range words {
		if i == 0 {
			words[i] = strings.ToLower(w)
		} else {
			words[i] = strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
		}
	}
	return strings.Join(words, "")
}

// kebabcase converts a string to kebab-case.
func kebabcase(s string) string {
	words := splitWords(s)
	if len(words) == 0 {
		return ""
	}
	for i, w := range words {
		words[i] = strings.ToLower(w)
	}
	return strings.Join(words, "-")
}

// list creates a slice of interface{} from the given arguments.
func list(vals ...interface{}) []interface{} {
	return vals
}

// hasKey returns true if the map contains the given key.
func hasKey(m map[string]interface{}, key string) bool {
	_, ok := m[key]
	return ok
}

// keys returns the sorted keys of a map.
func keys(m map[string]interface{}) []string {
	k := make([]string, 0, len(m))
	for key := range m {
		k = append(k, key)
	}
	sort.Strings(k)
	return k
}

// values returns the values of a map, ordered by sorted keys.
func values(m map[string]interface{}) []interface{} {
	k := keys(m)
	vals := make([]interface{}, len(k))
	for i, key := range k {
		vals[i] = m[key]
	}
	return vals
}

// appendList appends a value to a slice. Registered as "append" in the FuncMap.
func appendList(l []interface{}, val interface{}) []interface{} {
	return append(l, val)
}

// pluck extracts a key from each map and returns the collected values.
func pluck(key string, maps ...map[string]interface{}) []interface{} {
	var result []interface{}
	for _, m := range maps {
		if v, ok := m[key]; ok {
			result = append(result, v)
		}
	}
	return result
}

// sha256sum returns the SHA-256 hex digest of s.
func sha256sum(s string) string {
	hash := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", hash)
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
