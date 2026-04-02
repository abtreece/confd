package template

import (
	"reflect"
	"strings"
	"testing"
	"text/template"
)

func TestIsEmpty(t *testing.T) {
	tests := []struct {
		name     string
		val      interface{}
		expected bool
	}{
		{"nil", nil, true},
		{"empty string", "", true},
		{"non-empty string", "hello", false},
		{"zero int", 0, true},
		{"non-zero int", 42, false},
		{"zero int64", int64(0), true},
		{"non-zero int64", int64(1), false},
		{"zero uint", uint(0), true},
		{"non-zero uint", uint(1), false},
		{"zero float64", 0.0, true},
		{"non-zero float64", 3.14, false},
		{"false bool", false, true},
		{"true bool", true, false},
		{"empty slice", []string{}, true},
		{"non-empty slice", []string{"a"}, false},
		{"empty map", map[string]string{}, true},
		{"non-empty map", map[string]string{"k": "v"}, false},
		{"nil pointer", (*int)(nil), true},
		{"zero struct", struct{}{}, true},
		{"non-zero struct", struct{ X int }{X: 1}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isEmpty(tt.val)
			if result != tt.expected {
				t.Errorf("isEmpty(%v) = %v, want %v", tt.val, result, tt.expected)
			}
		})
	}
}

func TestDfault(t *testing.T) {
	tests := []struct {
		name       string
		defaultVal interface{}
		val        interface{}
		expected   interface{}
	}{
		{"non-empty val returns val", "default", "actual", "actual"},
		{"empty string returns default", "default", "", "default"},
		{"nil returns default", "default", nil, "default"},
		{"zero int returns default", 42, 0, 42},
		{"non-zero int returns val", 42, 7, 7},
		{"false returns default", true, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dfault(tt.defaultVal, tt.val)
			if result != tt.expected {
				t.Errorf("dfault(%v, %v) = %v, want %v", tt.defaultVal, tt.val, result, tt.expected)
			}
		})
	}
}

func TestTernary(t *testing.T) {
	tests := []struct {
		name      string
		trueVal   interface{}
		falseVal  interface{}
		condition bool
		expected  interface{}
	}{
		{"true condition", "yes", "no", true, "yes"},
		{"false condition", "yes", "no", false, "no"},
		{"true with ints", 1, 0, true, 1},
		{"false with ints", 1, 0, false, 0},
		{"true with nil falseVal", "val", nil, true, "val"},
		{"false with nil trueVal", nil, "val", false, "val"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ternary(tt.trueVal, tt.falseVal, tt.condition)
			if result != tt.expected {
				t.Errorf("ternary(%v, %v, %v) = %v, want %v", tt.trueVal, tt.falseVal, tt.condition, result, tt.expected)
			}
		})
	}
}

func TestCoalesce(t *testing.T) {
	tests := []struct {
		name     string
		vals     []interface{}
		expected interface{}
	}{
		{"first non-empty", []interface{}{"", "hello", "world"}, "hello"},
		{"all empty", []interface{}{"", nil, 0}, nil},
		{"first is non-empty", []interface{}{"first", "second"}, "first"},
		{"nil then value", []interface{}{nil, nil, "found"}, "found"},
		{"no args", []interface{}{}, nil},
		{"single non-empty", []interface{}{"only"}, "only"},
		{"single empty", []interface{}{""}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := coalesce(tt.vals...)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("coalesce(%v) = %v, want %v", tt.vals, result, tt.expected)
			}
		})
	}
}

func TestToJson(t *testing.T) {
	tests := []struct {
		name        string
		val         interface{}
		expected    string
		expectError bool
	}{
		{"string", "hello", `"hello"`, false},
		{"int", 42, "42", false},
		{"map", map[string]string{"a": "b"}, `{"a":"b"}`, false},
		{"slice", []int{1, 2, 3}, "[1,2,3]", false},
		{"nil", nil, "null", false},
		{"bool", true, "true", false},
		{"unmarshalable", make(chan int), "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := toJson(tt.val)
			if tt.expectError {
				if err == nil {
					t.Errorf("toJson(%v) expected error, got nil", tt.val)
				}
				return
			}
			if err != nil {
				t.Errorf("toJson(%v) unexpected error: %v", tt.val, err)
				return
			}
			if result != tt.expected {
				t.Errorf("toJson(%v) = %s, want %s", tt.val, result, tt.expected)
			}
		})
	}
}

func TestFromJson(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    interface{}
		expectError bool
	}{
		{"object", `{"key":"value"}`, map[string]interface{}{"key": "value"}, false},
		{"array", `[1,2,3]`, []interface{}{float64(1), float64(2), float64(3)}, false},
		{"string", `"hello"`, "hello", false},
		{"number", `42`, float64(42), false},
		{"bool", `true`, true, false},
		{"null", `null`, nil, false},
		{"invalid json", `{invalid}`, nil, true},
		{"empty string", ``, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := fromJson(tt.input)
			if tt.expectError {
				if err == nil {
					t.Errorf("fromJson(%s) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("fromJson(%s) unexpected error: %v", tt.input, err)
				return
			}
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("fromJson(%s) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestToPrettyJson(t *testing.T) {
	tests := []struct {
		name        string
		val         interface{}
		expected    string
		expectError bool
	}{
		{
			"simple map",
			map[string]string{"key": "value"},
			"{\n  \"key\": \"value\"\n}",
			false,
		},
		{
			"nil",
			nil,
			"null",
			false,
		},
		{
			"unmarshalable",
			make(chan int),
			"",
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := toPrettyJson(tt.val)
			if tt.expectError {
				if err == nil {
					t.Errorf("toPrettyJson(%v) expected error, got nil", tt.val)
				}
				return
			}
			if err != nil {
				t.Errorf("toPrettyJson(%v) unexpected error: %v", tt.val, err)
				return
			}
			if result != tt.expected {
				t.Errorf("toPrettyJson(%v) = %s, want %s", tt.val, result, tt.expected)
			}
		})
	}
}

func TestJsonRoundTrip(t *testing.T) {
	original := map[string]interface{}{
		"name":   "test",
		"count":  float64(42),
		"active": true,
	}

	jsonStr, err := toJson(original)
	if err != nil {
		t.Fatalf("toJson error: %v", err)
	}

	result, err := fromJson(jsonStr)
	if err != nil {
		t.Fatalf("fromJson error: %v", err)
	}

	if !reflect.DeepEqual(result, original) {
		t.Errorf("JSON round trip failed: got %v, want %v", result, original)
	}
}

func TestIndent(t *testing.T) {
	tests := []struct {
		name     string
		spaces   int
		input    string
		expected string
	}{
		{"simple", 4, "hello", "    hello"},
		{"multiline", 2, "line1\nline2\nline3", "  line1\n  line2\n  line3"},
		{"empty string", 4, "", ""},
		{"zero spaces", 0, "hello", "hello"},
		{"negative spaces", -1, "hello", "hello"},
		{"single newline", 2, "a\nb", "  a\n  b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := indent(tt.spaces, tt.input)
			if result != tt.expected {
				t.Errorf("indent(%d, %q) = %q, want %q", tt.spaces, tt.input, result, tt.expected)
			}
		})
	}
}

func TestNindent(t *testing.T) {
	tests := []struct {
		name     string
		spaces   int
		input    string
		expected string
	}{
		{"simple", 4, "hello", "\n    hello"},
		{"multiline", 2, "a\nb", "\n  a\n  b"},
		{"empty string", 4, "", "\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := nindent(tt.spaces, tt.input)
			if result != tt.expected {
				t.Errorf("nindent(%d, %q) = %q, want %q", tt.spaces, tt.input, result, tt.expected)
			}
		})
	}
}

func TestQuote(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple", "hello", `"hello"`},
		{"empty", "", `""`},
		{"with spaces", "hello world", `"hello world"`},
		{"with special chars", "line1\nline2", `"line1\nline2"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := quote(tt.input)
			if result != tt.expected {
				t.Errorf("quote(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSquote(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple", "hello", "'hello'"},
		{"empty", "", "''"},
		{"with spaces", "hello world", "'hello world'"},
		{"with single quote", "it's", "'it''s'"},
		{"multiple quotes", "a'b'c", "'a''b''c'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := squote(tt.input)
			if result != tt.expected {
				t.Errorf("squote(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestRegexMatch(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		input       string
		expected    bool
		expectError bool
	}{
		{"simple match", `^hello`, "hello world", true, false},
		{"no match", `^world`, "hello world", false, false},
		{"full match", `^\d+$`, "12345", true, false},
		{"empty pattern", ``, "anything", true, false},
		{"empty string", `^$`, "", true, false},
		{"invalid regex", `[invalid`, "", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := regexMatch(tt.pattern, tt.input)
			if tt.expectError {
				if err == nil {
					t.Errorf("regexMatch(%q, %q) expected error, got nil", tt.pattern, tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("regexMatch(%q, %q) unexpected error: %v", tt.pattern, tt.input, err)
				return
			}
			if result != tt.expected {
				t.Errorf("regexMatch(%q, %q) = %v, want %v", tt.pattern, tt.input, result, tt.expected)
			}
		})
	}
}

func TestRegexFind(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		input       string
		expected    string
		expectError bool
	}{
		{"find digits", `\d+`, "abc123def456", "123", false},
		{"no match", `\d+`, "abcdef", "", false},
		{"find word", `\w+`, "hello world", "hello", false},
		{"empty pattern", ``, "anything", "", false},
		{"invalid regex", `[invalid`, "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := regexFind(tt.pattern, tt.input)
			if tt.expectError {
				if err == nil {
					t.Errorf("regexFind(%q, %q) expected error, got nil", tt.pattern, tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("regexFind(%q, %q) unexpected error: %v", tt.pattern, tt.input, err)
				return
			}
			if result != tt.expected {
				t.Errorf("regexFind(%q, %q) = %q, want %q", tt.pattern, tt.input, result, tt.expected)
			}
		})
	}
}

func TestRegexReplaceAll(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		input       string
		repl        string
		expected    string
		expectError bool
	}{
		{"replace digits", `\d+`, "abc123def456", "NUM", "abcNUMdefNUM", false},
		{"no match", `\d+`, "abcdef", "NUM", "abcdef", false},
		{"replace spaces", `\s+`, "hello  world", " ", "hello world", false},
		{"empty replacement", `\d+`, "abc123", "", "abc", false},
		{"invalid regex", `[invalid`, "input", "repl", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := regexReplaceAll(tt.pattern, tt.input, tt.repl)
			if tt.expectError {
				if err == nil {
					t.Errorf("regexReplaceAll(%q, %q, %q) expected error, got nil", tt.pattern, tt.input, tt.repl)
				}
				return
			}
			if err != nil {
				t.Errorf("regexReplaceAll(%q, %q, %q) unexpected error: %v", tt.pattern, tt.input, tt.repl, err)
				return
			}
			if result != tt.expected {
				t.Errorf("regexReplaceAll(%q, %q, %q) = %q, want %q", tt.pattern, tt.input, tt.repl, result, tt.expected)
			}
		})
	}
}

func TestRepeat(t *testing.T) {
	tests := []struct {
		name     string
		count    int
		input    string
		expected string
	}{
		{"repeat 3", 3, "ab", "ababab"},
		{"repeat 0", 0, "ab", ""},
		{"repeat negative", -1, "ab", ""},
		{"repeat 1", 1, "hello", "hello"},
		{"empty string", 5, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := repeat(tt.count, tt.input)
			if result != tt.expected {
				t.Errorf("repeat(%d, %q) = %q, want %q", tt.count, tt.input, result, tt.expected)
			}
		})
	}
}

func TestNospace(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"spaces", "hello world", "helloworld"},
		{"tabs", "hello\tworld", "helloworld"},
		{"newlines", "hello\nworld", "helloworld"},
		{"mixed whitespace", " hello \t world \n ", "helloworld"},
		{"no whitespace", "helloworld", "helloworld"},
		{"empty string", "", ""},
		{"only whitespace", "   ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := nospace(tt.input)
			if result != tt.expected {
				t.Errorf("nospace(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSplitWords(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"camelCase", "helloWorld", []string{"hello", "World"}},
		{"PascalCase", "HelloWorld", []string{"Hello", "World"}},
		{"snake_case", "hello_world", []string{"hello", "world"}},
		{"kebab-case", "hello-world", []string{"hello", "world"}},
		{"consecutive upper", "HTTPServer", []string{"HTTP", "Server"}},
		{"single word", "hello", []string{"hello"}},
		{"empty", "", nil},
		{"spaces", "hello world", []string{"hello", "world"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitWords(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("splitWords(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSnakecase(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"PascalCase", "HelloWorld", "hello_world"},
		{"camelCase", "helloWorld", "hello_world"},
		{"snake_case", "hello_world", "hello_world"},
		{"kebab-case", "hello-world", "hello_world"},
		{"consecutive upper", "HTTPServer", "http_server"},
		{"single word", "hello", "hello"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := snakecase(tt.input)
			if result != tt.expected {
				t.Errorf("snakecase(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestCamelcase(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"PascalCase", "HelloWorld", "helloWorld"},
		{"camelCase", "helloWorld", "helloWorld"},
		{"snake_case", "hello_world", "helloWorld"},
		{"kebab-case", "hello-world", "helloWorld"},
		{"consecutive upper", "HTTPServer", "httpServer"},
		{"single word", "hello", "hello"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := camelcase(tt.input)
			if result != tt.expected {
				t.Errorf("camelcase(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestKebabcase(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"PascalCase", "HelloWorld", "hello-world"},
		{"camelCase", "helloWorld", "hello-world"},
		{"snake_case", "hello_world", "hello-world"},
		{"kebab-case", "hello-world", "hello-world"},
		{"consecutive upper", "HTTPServer", "http-server"},
		{"single word", "hello", "hello"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := kebabcase(tt.input)
			if result != tt.expected {
				t.Errorf("kebabcase(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestList(t *testing.T) {
	tests := []struct {
		name     string
		vals     []interface{}
		expected []interface{}
	}{
		{"multiple values", []interface{}{"a", 1, true}, []interface{}{"a", 1, true}},
		{"empty", []interface{}{}, []interface{}{}},
		{"single value", []interface{}{"only"}, []interface{}{"only"}},
		{"nil values", []interface{}{nil, nil}, []interface{}{nil, nil}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := list(tt.vals...)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("list(%v) = %v, want %v", tt.vals, result, tt.expected)
			}
		})
	}
}

func TestHasKey(t *testing.T) {
	m := map[string]interface{}{"a": 1, "b": nil}

	tests := []struct {
		name     string
		key      string
		expected bool
	}{
		{"existing key", "a", true},
		{"nil value key", "b", true},
		{"missing key", "c", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasKey(m, tt.key)
			if result != tt.expected {
				t.Errorf("hasKey(m, %q) = %v, want %v", tt.key, result, tt.expected)
			}
		})
	}
}

func TestKeys(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		expected []string
	}{
		{
			"sorted keys",
			map[string]interface{}{"c": 3, "a": 1, "b": 2},
			[]string{"a", "b", "c"},
		},
		{
			"empty map",
			map[string]interface{}{},
			[]string{},
		},
		{
			"single key",
			map[string]interface{}{"only": 1},
			[]string{"only"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := keys(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("keys(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestValues(t *testing.T) {
	t.Run("ordered by sorted keys", func(t *testing.T) {
		m := map[string]interface{}{"c": 3, "a": 1, "b": 2}
		expected := []interface{}{1, 2, 3}
		result := values(m)
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("values(%v) = %v, want %v", m, result, expected)
		}
	})

	t.Run("empty map", func(t *testing.T) {
		m := map[string]interface{}{}
		result := values(m)
		if len(result) != 0 {
			t.Errorf("values({}) = %v, want empty slice", result)
		}
	})
}

func TestAppendList(t *testing.T) {
	tests := []struct {
		name     string
		list     []interface{}
		val      interface{}
		expected []interface{}
	}{
		{"append to list", []interface{}{"a", "b"}, "c", []interface{}{"a", "b", "c"}},
		{"append to empty", []interface{}{}, "first", []interface{}{"first"}},
		{"append nil", []interface{}{"a"}, nil, []interface{}{"a", nil}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := appendList(tt.list, tt.val)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("appendList(%v, %v) = %v, want %v", tt.list, tt.val, result, tt.expected)
			}
		})
	}
}

func TestPluck(t *testing.T) {
	maps := []map[string]interface{}{
		{"name": "Alice", "age": 30},
		{"name": "Bob"},
		{"age": 25},
	}

	t.Run("pluck existing key", func(t *testing.T) {
		result := pluck("name", maps...)
		expected := []interface{}{"Alice", "Bob"}
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("pluck(\"name\", ...) = %v, want %v", result, expected)
		}
	})

	t.Run("pluck missing key", func(t *testing.T) {
		result := pluck("email", maps...)
		if result != nil {
			t.Errorf("pluck(\"email\", ...) = %v, want nil", result)
		}
	})

	t.Run("pluck no maps", func(t *testing.T) {
		result := pluck("name")
		if result != nil {
			t.Errorf("pluck(\"name\") = %v, want nil", result)
		}
	})
}

func TestSha256sum(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			"hello",
			"hello",
			"2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824",
		},
		{
			"empty string",
			"",
			"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			"hello world",
			"hello world",
			"b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sha256sum(tt.input)
			if result != tt.expected {
				t.Errorf("sha256sum(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestUtilityFuncsInTemplate(t *testing.T) {
	tmplText := `{{- $d := default "fallback" "" -}}
default: {{ $d }}
ternary: {{ ternary "yes" "no" true }}
coalesce: {{ coalesce "" nil "found" }}
toJson: {{ toJson (list 1 2 3) }}
indent: |
{{ indent 4 "line1\nline2" }}
quote: {{ quote "hello" }}
squote: {{ squote "world" }}
sha256: {{ sha256sum "test" }}
hasKey: {{ hasKey (dict "a" 1) "a" }}
regex: {{ regexMatch "^hello" "hello world" }}
snake: {{ snakecase "HelloWorld" }}
camel: {{ camelcase "hello_world" }}
kebab: {{ kebabcase "HelloWorld" }}
nospace: {{ nospace "a b c" }}
repeat: {{ repeat 3 "ab" }}
trimPrefix: {{ trimPrefix "Hello, World" "Hello, " }}
empty_true: {{ empty "" }}
empty_false: {{ empty "x" }}`

	funcMap := newFuncMap()
	tmpl, err := template.New("test").Funcs(template.FuncMap(funcMap)).Parse(tmplText)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, nil); err != nil {
		t.Fatalf("execute error: %v", err)
	}
	output := buf.String()

	checks := []string{
		"default: fallback",
		"ternary: yes",
		"coalesce: found",
		"toJson: [1,2,3]",
		"    line1",
		"    line2",
		`quote: "hello"`,
		"squote: 'world'",
		"hasKey: true",
		"regex: true",
		"snake: hello_world",
		"camel: helloWorld",
		"kebab: hello-world",
		"nospace: abc",
		"repeat: ababab",
		"trimPrefix: World",
		"empty_true: true",
		"empty_false: false",
	}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("output missing %q\nfull output:\n%s", check, output)
		}
	}
}
