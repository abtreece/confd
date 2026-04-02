# Utility Template Functions Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add 28 utility template functions to confd with zero external dependencies.

**Architecture:** New file `pkg/template/utility_funcs.go` exports `utilityFuncMap()` which is loaded as the base layer in `newFuncMap()`. Confd's existing functions overlay on top, preserving all existing behavior. Tests in `pkg/template/utility_funcs_test.go`.

**Tech Stack:** Go standard library only (`encoding/json`, `regexp`, `crypto/sha256`, `strings`, `reflect`, `fmt`, `sort`, `unicode`)

**Spec:** `docs/specs/2026-04-01-utility-template-functions-design.md`

---

## File Structure

| File | Responsibility |
|------|----------------|
| `pkg/template/utility_funcs.go` | **New** — 28 function implementations + `utilityFuncMap()` |
| `pkg/template/utility_funcs_test.go` | **New** — table-driven tests for all 28 functions |
| `pkg/template/template_funcs.go` | **Modify** — change `newFuncMap()` to use `utilityFuncMap()` as base layer |
| `docs/templates.md` | **Modify** — add documentation for new functions |
| `CHANGELOG` | **Modify** — add entry |

---

## Task 1: Scaffold utility_funcs.go with empty FuncMap and wire into newFuncMap

**Files:**
- Create: `pkg/template/utility_funcs.go`
- Modify: `pkg/template/template_funcs.go:20-58`
- Test: existing tests must still pass

- [ ] **Step 1: Create utility_funcs.go with empty utilityFuncMap**

```go
package template

// utilityFuncMap returns utility template functions.
// These are loaded as the base layer in newFuncMap().
// Any confd function with the same name will override these.
func utilityFuncMap() map[string]interface{} {
  m := make(map[string]interface{})
  return m
}
```

- [ ] **Step 2: Modify newFuncMap() to use utilityFuncMap as base**

In `pkg/template/template_funcs.go`, change:

```go
func newFuncMap() map[string]interface{} {
  m := make(map[string]interface{})
```

To:

```go
func newFuncMap() map[string]interface{} {
  // Start with utility functions as base layer.
  // Confd's existing functions are assigned below and override
  // any utility function with the same name.
  m := utilityFuncMap()
```

- [ ] **Step 3: Run existing tests to verify no regression**

Run: `go test ./pkg/template/ -v -count=1 2>&1 | tail -5`
Expected: `ok  github.com/abtreece/confd/pkg/template`

- [ ] **Step 4: Commit**

```
git add pkg/template/utility_funcs.go pkg/template/template_funcs.go
git commit -m "refactor: wire utilityFuncMap as base layer in newFuncMap"
```

---

## Task 2: Implement defaults/conditionals — empty, default, ternary, coalesce

**Files:**
- Modify: `pkg/template/utility_funcs.go`
- Create: `pkg/template/utility_funcs_test.go`

- [ ] **Step 1: Write tests for empty()**

```go
package template

import (
  "reflect"
  "testing"
)

func TestEmpty(t *testing.T) {
  tests := []struct {
    name     string
    input    interface{}
    expected bool
  }{
    {"nil", nil, true},
    {"empty string", "", true},
    {"non-empty string", "hello", false},
    {"zero int", 0, true},
    {"non-zero int", 42, false},
    {"zero float", 0.0, true},
    {"non-zero float", 3.14, false},
    {"false bool", false, true},
    {"true bool", true, false},
    {"empty slice", []string{}, true},
    {"non-empty slice", []string{"a"}, false},
    {"nil slice", []string(nil), true},
    {"empty map", map[string]string{}, true},
    {"non-empty map", map[string]string{"a": "b"}, false},
    {"nil map", map[string]string(nil), true},
  }

  for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
      result := isEmpty(tt.input)
      if result != tt.expected {
        t.Errorf("isEmpty(%v) = %v, want %v", tt.input, result, tt.expected)
      }
    })
  }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/template/ -run TestEmpty -v -count=1`
Expected: FAIL — `isEmpty` not defined

- [ ] **Step 3: Implement isEmpty and register empty in utilityFuncMap**

Add to `utility_funcs.go`:

```go
import "reflect"

// isEmpty returns true if the given value is the zero value for its type,
// nil, or an empty string/slice/map.
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
```

Register in `utilityFuncMap()`:

```go
m["empty"] = isEmpty
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/template/ -run TestEmpty -v -count=1`
Expected: PASS

- [ ] **Step 5: Write tests for default, ternary, coalesce**

Add to `utility_funcs_test.go`:

```go
func TestDefault(t *testing.T) {
  tests := []struct {
    name       string
    defaultVal interface{}
    val        interface{}
    expected   interface{}
  }{
    {"non-empty value", "fallback", "actual", "actual"},
    {"empty string", "fallback", "", "fallback"},
    {"nil value", "fallback", nil, "fallback"},
    {"zero int", 99, 0, 99},
    {"non-zero int", 99, 42, 42},
    {"empty slice", []string{"default"}, []string{}, []string{"default"}},
  }

  for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
      result := dfault(tt.defaultVal, tt.val)
      if !reflect.DeepEqual(result, tt.expected) {
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
    {"first non-empty", []interface{}{"", nil, "hello", "world"}, "hello"},
    {"all empty", []interface{}{"", nil, 0}, nil},
    {"first is non-empty", []interface{}{"first", "second"}, "first"},
    {"no args", []interface{}{}, nil},
  }

  for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
      result := coalesce(tt.vals...)
      if result != tt.expected {
        t.Errorf("coalesce(%v) = %v, want %v", tt.vals, result, tt.expected)
      }
    })
  }
}
```

- [ ] **Step 6: Implement default, ternary, coalesce and register**

```go
func dfault(defaultVal, val interface{}) interface{} {
  if isEmpty(val) {
    return defaultVal
  }
  return val
}

func ternary(trueVal, falseVal interface{}, condition bool) interface{} {
  if condition {
    return trueVal
  }
  return falseVal
}

func coalesce(vals ...interface{}) interface{} {
  for _, v := range vals {
    if !isEmpty(v) {
      return v
    }
  }
  return nil
}
```

Register:

```go
m["default"] = dfault
m["ternary"] = ternary
m["coalesce"] = coalesce
```

- [ ] **Step 7: Run all tests**

Run: `go test ./pkg/template/ -run "TestEmpty|TestDefault|TestTernary|TestCoalesce" -v -count=1`
Expected: PASS

- [ ] **Step 8: Commit**

```
git add pkg/template/utility_funcs.go pkg/template/utility_funcs_test.go
git commit -m "feat: add default, ternary, coalesce, empty template functions"
```

---

## Task 3: Implement JSON functions — toJson, fromJson, toPrettyJson

**Files:**
- Modify: `pkg/template/utility_funcs.go`
- Modify: `pkg/template/utility_funcs_test.go`

- [ ] **Step 1: Write tests**

```go
func TestToJson(t *testing.T) {
  tests := []struct {
    name        string
    input       interface{}
    expected    string
    expectError bool
  }{
    {"map", map[string]string{"a": "b"}, `{"a":"b"}`, false},
    {"string", "hello", `"hello"`, false},
    {"int", 42, "42", false},
    {"nil", nil, "null", false},
    {"slice", []int{1, 2, 3}, "[1,2,3]", false},
  }

  for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
      result, err := toJson(tt.input)
      if tt.expectError {
        if err == nil {
          t.Error("expected error, got nil")
        }
        return
      }
      if err != nil {
        t.Errorf("unexpected error: %v", err)
        return
      }
      if result != tt.expected {
        t.Errorf("toJson(%v) = %s, want %s", tt.input, result, tt.expected)
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
    {"object", `{"a":"b"}`, map[string]interface{}{"a": "b"}, false},
    {"array", `[1,2]`, []interface{}{float64(1), float64(2)}, false},
    {"string", `"hello"`, "hello", false},
    {"number", `42`, float64(42), false},
    {"invalid", `{bad}`, nil, true},
  }

  for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
      result, err := fromJson(tt.input)
      if tt.expectError {
        if err == nil {
          t.Error("expected error, got nil")
        }
        return
      }
      if err != nil {
        t.Errorf("unexpected error: %v", err)
        return
      }
      if !reflect.DeepEqual(result, tt.expected) {
        t.Errorf("fromJson(%s) = %v, want %v", tt.input, result, tt.expected)
      }
    })
  }
}

func TestToPrettyJson(t *testing.T) {
  result, err := toPrettyJson(map[string]string{"a": "b"})
  if err != nil {
    t.Fatalf("unexpected error: %v", err)
  }
  expected := "{\n  \"a\": \"b\"\n}"
  if result != expected {
    t.Errorf("toPrettyJson() = %q, want %q", result, expected)
  }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/template/ -run "TestToJson|TestFromJson|TestToPrettyJson" -v -count=1`
Expected: FAIL

- [ ] **Step 3: Implement and register**

```go
import "encoding/json"

func toJson(val interface{}) (string, error) {
  b, err := json.Marshal(val)
  if err != nil {
    return "", err
  }
  return string(b), nil
}

func fromJson(s string) (interface{}, error) {
  var result interface{}
  err := json.Unmarshal([]byte(s), &result)
  return result, err
}

func toPrettyJson(val interface{}) (string, error) {
  b, err := json.MarshalIndent(val, "", "  ")
  if err != nil {
    return "", err
  }
  return string(b), nil
}
```

Register: `m["toJson"]`, `m["fromJson"]`, `m["toPrettyJson"]`

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/template/ -run "TestToJson|TestFromJson|TestToPrettyJson" -v -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```
git add pkg/template/utility_funcs.go pkg/template/utility_funcs_test.go
git commit -m "feat: add toJson, fromJson, toPrettyJson template functions"
```

---

## Task 4: Implement formatting functions — indent, nindent, quote, squote

**Files:**
- Modify: `pkg/template/utility_funcs.go`
- Modify: `pkg/template/utility_funcs_test.go`

- [ ] **Step 1: Write tests**

```go
func TestIndent(t *testing.T) {
  tests := []struct {
    name     string
    spaces   int
    input    string
    expected string
  }{
    {"simple", 4, "line1\nline2", "    line1\n    line2"},
    {"zero spaces", 0, "line1\nline2", "line1\nline2"},
    {"empty string", 4, "", ""},
    {"single line", 2, "hello", "  hello"},
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
  result := nindent(2, "line1\nline2")
  expected := "\n  line1\n  line2"
  if result != expected {
    t.Errorf("nindent(2, ...) = %q, want %q", result, expected)
  }
}

func TestQuote(t *testing.T) {
  if quote("hello") != `"hello"` {
    t.Error("quote failed")
  }
  if squote("hello") != "'hello'" {
    t.Error("squote failed")
  }
}
```

- [ ] **Step 2: Run tests to verify they fail**

- [ ] **Step 3: Implement and register**

```go
import "strings"

func indent(spaces int, s string) string {
  if s == "" {
    return s
  }
  pad := strings.Repeat(" ", spaces)
  return pad + strings.Replace(s, "\n", "\n"+pad, -1)
}

func nindent(spaces int, s string) string {
  return "\n" + indent(spaces, s)
}

func quote(s string) string {
  return `"` + s + `"`
}

func squote(s string) string {
  return "'" + s + "'"
}
```

Register: `m["indent"]`, `m["nindent"]`, `m["quote"]`, `m["squote"]`

- [ ] **Step 4: Run tests to verify they pass**

- [ ] **Step 5: Commit**

```
git commit -m "feat: add indent, nindent, quote, squote template functions"
```

---

## Task 5: Implement regex functions — regexMatch, regexFind, regexReplaceAll

**Files:**
- Modify: `pkg/template/utility_funcs.go`
- Modify: `pkg/template/utility_funcs_test.go`

- [ ] **Step 1: Write tests**

```go
func TestRegexMatch(t *testing.T) {
  tests := []struct {
    name        string
    pattern     string
    input       string
    expected    bool
    expectError bool
  }{
    {"match", `^hello`, "hello world", true, false},
    {"no match", `^world`, "hello world", false, false},
    {"invalid pattern", `[invalid`, "test", false, true},
  }

  for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
      result, err := regexMatch(tt.pattern, tt.input)
      if tt.expectError {
        if err == nil {
          t.Error("expected error, got nil")
        }
        return
      }
      if err != nil {
        t.Errorf("unexpected error: %v", err)
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
    {"found", `\d+`, "abc123def", "123", false},
    {"not found", `\d+`, "abcdef", "", false},
    {"invalid pattern", `[invalid`, "test", "", true},
  }

  for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
      result, err := regexFind(tt.pattern, tt.input)
      if tt.expectError {
        if err == nil {
          t.Error("expected error, got nil")
        }
        return
      }
      if err != nil {
        t.Errorf("unexpected error: %v", err)
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
    {"invalid pattern", `[invalid`, "test", "x", "", true},
  }

  for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
      result, err := regexReplaceAll(tt.pattern, tt.input, tt.repl)
      if tt.expectError {
        if err == nil {
          t.Error("expected error, got nil")
        }
        return
      }
      if err != nil {
        t.Errorf("unexpected error: %v", err)
        return
      }
      if result != tt.expected {
        t.Errorf("regexReplaceAll(%q, %q, %q) = %q, want %q", tt.pattern, tt.input, tt.repl, result, tt.expected)
      }
    })
  }
}
```

- [ ] **Step 2: Run tests to verify they fail**

- [ ] **Step 3: Implement and register**

```go
import "regexp"

func regexMatch(pattern, s string) (bool, error) {
  return regexp.MatchString(pattern, s)
}

func regexFind(pattern, s string) (string, error) {
  re, err := regexp.Compile(pattern)
  if err != nil {
    return "", err
  }
  return re.FindString(s), nil
}

func regexReplaceAll(pattern, s, repl string) (string, error) {
  re, err := regexp.Compile(pattern)
  if err != nil {
    return "", err
  }
  return re.ReplaceAllString(s, repl), nil
}
```

Register: `m["regexMatch"]`, `m["regexFind"]`, `m["regexReplaceAll"]`

- [ ] **Step 4: Run tests to verify they pass**

- [ ] **Step 5: Commit**

```
git commit -m "feat: add regexMatch, regexFind, regexReplaceAll template functions"
```

---

## Task 6: Implement string functions — trimPrefix, repeat, nospace, snakecase, camelcase, kebabcase

**Files:**
- Modify: `pkg/template/utility_funcs.go`
- Modify: `pkg/template/utility_funcs_test.go`

- [ ] **Step 1: Write tests**

```go
func TestRepeat(t *testing.T) {
  tests := []struct {
    name     string
    count    int
    input    string
    expected string
  }{
    {"repeat 3", 3, "ab", "ababab"},
    {"repeat 0", 0, "ab", ""},
    {"repeat 1", 1, "ab", "ab"},
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
    input    string
    expected string
  }{
    {"hello world", "helloworld"},
    {"  spaces  ", "spaces"},
    {"no\ttabs\nnewlines", "notabsnewlines"},
    {"", ""},
  }

  for _, tt := range tests {
    t.Run(tt.input, func(t *testing.T) {
      result := nospace(tt.input)
      if result != tt.expected {
        t.Errorf("nospace(%q) = %q, want %q", tt.input, result, tt.expected)
      }
    })
  }
}

func TestCaseConversions(t *testing.T) {
  tests := []struct {
    input    string
    snake    string
    camel    string
    kebab    string
  }{
    {"HelloWorld", "hello_world", "helloWorld", "hello-world"},
    {"helloWorld", "hello_world", "helloWorld", "hello-world"},
    {"hello_world", "hello_world", "helloWorld", "hello-world"},
    {"hello-world", "hello_world", "helloWorld", "hello-world"},
    {"HTTPServer", "http_server", "httpServer", "http-server"},
    {"hello", "hello", "hello", "hello"},
    {"", "", "", ""},
  }

  for _, tt := range tests {
    t.Run(tt.input, func(t *testing.T) {
      if result := snakecase(tt.input); result != tt.snake {
        t.Errorf("snakecase(%q) = %q, want %q", tt.input, result, tt.snake)
      }
      if result := camelcase(tt.input); result != tt.camel {
        t.Errorf("camelcase(%q) = %q, want %q", tt.input, result, tt.camel)
      }
      if result := kebabcase(tt.input); result != tt.kebab {
        t.Errorf("kebabcase(%q) = %q, want %q", tt.input, result, tt.kebab)
      }
    })
  }
}
```

- [ ] **Step 2: Run tests to verify they fail**

- [ ] **Step 3: Implement and register**

`trimPrefix` is registered as `strings.TrimPrefix` directly (same pattern as existing `trimSuffix`).

`repeat` wraps `strings.Repeat`.

`nospace` uses `strings.Map` to remove all unicode whitespace.

For case conversion, implement a shared `splitWords(s string) []string` helper that splits on:
- Uppercase boundaries (`fooBar` -> `foo`, `Bar`)
- Underscores and hyphens
- Spaces
- Consecutive uppercase runs (`HTTPServer` -> `HTTP`, `Server`)

Then `snakecase` = `strings.ToLower(strings.Join(words, "_"))`, `camelcase` = first word lower + rest title-cased, `kebabcase` = `strings.ToLower(strings.Join(words, "-"))`.

Register: `m["trimPrefix"]`, `m["repeat"]`, `m["nospace"]`, `m["snakecase"]`, `m["camelcase"]`, `m["kebabcase"]`

- [ ] **Step 4: Run tests to verify they pass**

- [ ] **Step 5: Run all tests**

Run: `go test ./pkg/template/ -v -count=1 2>&1 | tail -5`
Expected: `ok  github.com/abtreece/confd/pkg/template`

- [ ] **Step 6: Commit**

```
git commit -m "feat: add trimPrefix, repeat, nospace, snakecase, camelcase, kebabcase template functions"
```

---

## Task 7: Implement collection functions — dict, list, hasKey, keys, values, append, pluck

**Files:**
- Modify: `pkg/template/utility_funcs.go`
- Modify: `pkg/template/utility_funcs_test.go`

- [ ] **Step 1: Write tests**

```go
func TestDict(t *testing.T) {
  // dict is an alias for CreateMap — test that it's registered and works
  m := utilityFuncMap()
  dictFn, ok := m["dict"].(func(...interface{}) (map[string]interface{}, error))
  if !ok {
    t.Fatal("dict not registered or wrong type")
  }
  result, err := dictFn("a", 1, "b", 2)
  if err != nil {
    t.Fatalf("unexpected error: %v", err)
  }
  if result["a"] != 1 || result["b"] != 2 {
    t.Errorf("dict result = %v", result)
  }
  // Odd args
  _, err = dictFn("a")
  if err == nil {
    t.Error("expected error for odd args")
  }
}

func TestList(t *testing.T) {
  result := list(1, "two", 3.0)
  if len(result) != 3 || result[0] != 1 || result[1] != "two" {
    t.Errorf("list(1, two, 3.0) = %v", result)
  }
  // Empty
  result = list()
  if len(result) != 0 {
    t.Errorf("list() = %v, want empty", result)
  }
}

func TestHasKey(t *testing.T) {
  m := map[string]interface{}{"a": 1, "b": nil}
  if !hasKey(m, "a") {
    t.Error("hasKey should find 'a'")
  }
  if !hasKey(m, "b") {
    t.Error("hasKey should find 'b' even with nil value")
  }
  if hasKey(m, "c") {
    t.Error("hasKey should not find 'c'")
  }
}

func TestKeys(t *testing.T) {
  m := map[string]interface{}{"c": 3, "a": 1, "b": 2}
  result := keys(m)
  expected := []string{"a", "b", "c"}
  if !reflect.DeepEqual(result, expected) {
    t.Errorf("keys() = %v, want %v (sorted)", result, expected)
  }
}

func TestValues(t *testing.T) {
  m := map[string]interface{}{"b": 2, "a": 1}
  result := values(m)
  // Sorted by key: a=1, b=2
  if len(result) != 2 || result[0] != 1 || result[1] != 2 {
    t.Errorf("values() = %v, want [1 2] (sorted by key)", result)
  }
}

func TestAppendList(t *testing.T) {
  result := appendList([]interface{}{1, 2}, 3)
  if len(result) != 3 || result[2] != 3 {
    t.Errorf("appendList() = %v", result)
  }
  // Append to nil
  result = appendList(nil, "first")
  if len(result) != 1 {
    t.Errorf("appendList(nil, first) = %v", result)
  }
}

func TestPluck(t *testing.T) {
  maps := []map[string]interface{}{
    {"name": "a", "port": 80},
    {"name": "b", "port": 443},
    {"name": "c"},
  }
  result := pluck("port", maps...)
  if len(result) != 2 || result[0] != 80 || result[1] != 443 {
    t.Errorf("pluck(port) = %v, want [80 443]", result)
  }
}
```

- [ ] **Step 2: Run tests to verify they fail**

- [ ] **Step 3: Implement and register**

```go
func list(vals ...interface{}) []interface{} {
  return vals
}

func hasKey(m map[string]interface{}, key string) bool {
  _, ok := m[key]
  return ok
}

func keys(m map[string]interface{}) []string {
  k := make([]string, 0, len(m))
  for key := range m {
    k = append(k, key)
  }
  sort.Strings(k)
  return k
}

func values(m map[string]interface{}) []interface{} {
  sorted := keys(m)
  v := make([]interface{}, len(sorted))
  for i, key := range sorted {
    v[i] = m[key]
  }
  return v
}

func appendList(l []interface{}, val interface{}) []interface{} {
  return append(l, val)
}

func pluck(key string, maps ...map[string]interface{}) []interface{} {
  var result []interface{}
  for _, m := range maps {
    if val, ok := m[key]; ok {
      result = append(result, val)
    }
  }
  return result
}
```

Register `dict` as `CreateMap` (reuse existing implementation), plus `list`, `hasKey`, `keys`, `values`, `append` (-> `appendList`), `pluck`.

- [ ] **Step 4: Run tests to verify they pass**

- [ ] **Step 5: Commit**

```
git commit -m "feat: add dict, list, hasKey, keys, values, append, pluck template functions"
```

---

## Task 8: Implement hashing — sha256sum

**Files:**
- Modify: `pkg/template/utility_funcs.go`
- Modify: `pkg/template/utility_funcs_test.go`

- [ ] **Step 1: Write test**

```go
func TestSha256sum(t *testing.T) {
  // Known SHA-256 of "hello"
  result := sha256sum("hello")
  expected := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
  if result != expected {
    t.Errorf("sha256sum(hello) = %s, want %s", result, expected)
  }
  // Empty string
  result = sha256sum("")
  expected = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
  if result != expected {
    t.Errorf("sha256sum('') = %s, want %s", result, expected)
  }
}
```

- [ ] **Step 2: Run test to verify it fails**

- [ ] **Step 3: Implement and register**

```go
import (
  "crypto/sha256"
  "fmt"
)

func sha256sum(s string) string {
  hash := sha256.Sum256([]byte(s))
  return fmt.Sprintf("%x", hash)
}
```

Register: `m["sha256sum"]`

- [ ] **Step 4: Run tests to verify they pass**

- [ ] **Step 5: Run full test suite**

Run: `go test ./pkg/... -count=1 2>&1 | tail -25`
Expected: All packages pass

- [ ] **Step 6: Commit**

```
git commit -m "feat: add sha256sum template function"
```

---

## Task 9: Integration test — verify all functions work in templates

**Files:**
- Modify: `pkg/template/utility_funcs_test.go`

- [ ] **Step 1: Write integration test**

Write a test that creates a Go template using a mix of new and existing functions, renders it, and verifies the output. This confirms the FuncMap merging works end-to-end.

```go
func TestUtilityFuncsInTemplate(t *testing.T) {
  tmplText := `{{- $d := default "fallback" "" -}}
default: {{ $d }}
ternary: {{ ternary "yes" "no" true }}
coalesce: {{ coalesce "" nil "found" }}
toJson: {{ toJson (list 1 2 3) }}
indent: |
{{ indent 4 "line1\nline2" }}
quote: {{ quote "hello" }}
sha256: {{ sha256sum "test" }}
hasKey: {{ hasKey (dict "a" 1) "a" }}
regex: {{ regexMatch "^hello" "hello world" }}`

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
    `quote: "hello"`,
    "hasKey: true",
    "regex: true",
  }
  for _, check := range checks {
    if !strings.Contains(output, check) {
      t.Errorf("output missing %q\nfull output:\n%s", check, output)
    }
  }
}
```

- [ ] **Step 2: Run test**

Run: `go test ./pkg/template/ -run TestUtilityFuncsInTemplate -v -count=1`
Expected: PASS

- [ ] **Step 3: Commit**

```
git commit -m "test: add integration test for utility template functions"
```

---

## Task 10: Update documentation

**Files:**
- Modify: `docs/templates.md`
- Modify: `CHANGELOG`

- [ ] **Step 1: Add new function documentation to docs/templates.md**

Add a new section `### Utility Functions` before the `## Example Usage` section (line 571). Document each function with a brief description and template example, following the existing documentation style. Group by category (Defaults & Conditionals, JSON, Formatting, Regex, Strings, Collections, Hashing).

- [ ] **Step 2: Add CHANGELOG entry**

Add to the top of `CHANGELOG`, above the v0.40.0 entry:

```
### Unreleased

* feat: Add 28 utility template functions (#272)
  - Defaults & conditionals: `default`, `ternary`, `coalesce`, `empty`
  - JSON: `toJson`, `fromJson`, `toPrettyJson`
  - Formatting: `indent`, `nindent`, `quote`, `squote`
  - Regex: `regexMatch`, `regexFind`, `regexReplaceAll`
  - Strings: `trimPrefix`, `repeat`, `nospace`, `snakecase`, `camelcase`, `kebabcase`
  - Collections: `dict`, `list`, `hasKey`, `keys`, `values`, `append`, `pluck`
  - Hashing: `sha256sum`
```

- [ ] **Step 3: Commit**

```
git add docs/templates.md CHANGELOG
git commit -m "docs: document utility template functions (#272)"
```

---

## Task 11: Final verification and PR

- [ ] **Step 1: Run full test suite with race detector**

Run: `go test -race ./pkg/... -count=1`
Expected: All pass

- [ ] **Step 2: Run go vet**

Run: `go vet ./...`
Expected: Clean

- [ ] **Step 3: Build binary**

Run: `make build && ./bin/confd --version`
Expected: Builds successfully

- [ ] **Step 4: Push branch and create PR**

```
git push -u origin feat/utility-template-functions
gh pr create --repo abtreece/confd --title "feat: add utility template functions (#272)" --body "..."
```
