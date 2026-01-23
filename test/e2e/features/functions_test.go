//go:build e2e

package features

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/abtreece/confd/test/e2e/operations"
)

// TestFunctions_StringManipulation verifies string manipulation functions work correctly.
// Tests: split, join, toUpper, toLower, replace, trimSuffix, contains
func TestFunctions_StringManipulation(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	destPath := env.DestPath("string-funcs.txt")

	// Write template using various string functions
	env.WriteTemplate("string-funcs.tmpl", `{{ $parts := split (getv "/data/csv") "," }}
split_count: {{ len $parts }}
first_part: {{ index $parts 0 }}
joined: {{ join $parts "-" }}
upper: {{ toUpper (getv "/data/text") }}
lower: {{ toLower (getv "/data/text") }}
replaced: {{ replace (getv "/data/text") "World" "Universe" -1 }}
trimmed: {{ trimSuffix (getv "/data/suffix") ".txt" }}
contains_yes: {{ contains (getv "/data/text") "World" }}
contains_no: {{ contains (getv "/data/text") "Mars" }}
`)

	// Write config
	env.WriteConfig("string-funcs.toml", fmt.Sprintf(`[template]
src = "string-funcs.tmpl"
dest = "%s"
keys = ["/data/csv", "/data/text", "/data/suffix"]
`, destPath))

	// Run confd
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	confd.SetEnv("DATA_CSV", "apple,banana,cherry")
	confd.SetEnv("DATA_TEXT", "Hello World")
	confd.SetEnv("DATA_SUFFIX", "document.txt")
	err := confd.Start(ctx, "env", "--onetime", "--confdir", env.ConfDir, "--log-level", "error")
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}

	exitCode, err := confd.Wait()
	if err != nil {
		t.Fatalf("Error waiting for confd: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	// Verify output
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	output := string(content)

	// Verify each function result
	checks := []struct {
		name     string
		expected string
	}{
		{"split_count", "split_count: 3"},
		{"first_part", "first_part: apple"},
		{"joined", "joined: apple-banana-cherry"},
		{"upper", "upper: HELLO WORLD"},
		{"lower", "lower: hello world"},
		{"replaced", "replaced: Hello Universe"},
		{"trimmed", "trimmed: document"},
		{"contains_yes", "contains_yes: true"},
		{"contains_no", "contains_no: false"},
	}

	for _, check := range checks {
		if !strings.Contains(output, check.expected) {
			t.Errorf("%s: expected %q in output, got:\n%s", check.name, check.expected, output)
		}
	}
}

// TestFunctions_MathOperations verifies math functions work correctly.
// Tests: add, sub, mul, div, mod
func TestFunctions_MathOperations(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	destPath := env.DestPath("math-funcs.txt")

	// Write template using math functions
	// Note: We need to convert string values to int using atoi
	env.WriteTemplate("math-funcs.tmpl", `{{ $a := atoi (getv "/math/a") }}{{ $b := atoi (getv "/math/b") }}
add: {{ add $a $b }}
sub: {{ sub $a $b }}
mul: {{ mul $a $b }}
div: {{ div $a $b }}
mod: {{ mod $a $b }}
seq: {{ range $i := seq 1 3 }}{{ $i }} {{ end }}
`)

	// Write config
	env.WriteConfig("math-funcs.toml", fmt.Sprintf(`[template]
src = "math-funcs.tmpl"
dest = "%s"
keys = ["/math/a", "/math/b"]
`, destPath))

	// Run confd
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	confd.SetEnv("MATH_A", "20")
	confd.SetEnv("MATH_B", "7")
	err := confd.Start(ctx, "env", "--onetime", "--confdir", env.ConfDir, "--log-level", "error")
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}

	exitCode, err := confd.Wait()
	if err != nil {
		t.Fatalf("Error waiting for confd: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	// Verify output
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	output := string(content)

	// Verify each function result: 20 + 7 = 27, 20 - 7 = 13, 20 * 7 = 140, 20 / 7 = 2, 20 % 7 = 6
	checks := []struct {
		name     string
		expected string
	}{
		{"add", "add: 27"},
		{"sub", "sub: 13"},
		{"mul", "mul: 140"},
		{"div", "div: 2"},
		{"mod", "mod: 6"},
		{"seq", "seq: 1 2 3"},
	}

	for _, check := range checks {
		if !strings.Contains(output, check.expected) {
			t.Errorf("%s: expected %q in output, got:\n%s", check.name, check.expected, output)
		}
	}
}

// TestFunctions_Encoding verifies encoding functions work correctly.
// Tests: base64Encode, base64Decode
func TestFunctions_Encoding(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	destPath := env.DestPath("encoding-funcs.txt")

	// Write template using encoding functions
	env.WriteTemplate("encoding-funcs.tmpl", `original: {{ getv "/data/plain" }}
encoded: {{ base64Encode (getv "/data/plain") }}
decoded: {{ base64Decode (getv "/data/encoded") }}
roundtrip: {{ base64Decode (base64Encode (getv "/data/plain")) }}
`)

	// Write config
	env.WriteConfig("encoding-funcs.toml", fmt.Sprintf(`[template]
src = "encoding-funcs.tmpl"
dest = "%s"
keys = ["/data/plain", "/data/encoded"]
`, destPath))

	// Run confd
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	confd.SetEnv("DATA_PLAIN", "Hello World")
	confd.SetEnv("DATA_ENCODED", "SGVsbG8gV29ybGQ=") // "Hello World" in base64
	err := confd.Start(ctx, "env", "--onetime", "--confdir", env.ConfDir, "--log-level", "error")
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}

	exitCode, err := confd.Wait()
	if err != nil {
		t.Fatalf("Error waiting for confd: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	// Verify output
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	output := string(content)

	// Verify each function result
	checks := []struct {
		name     string
		expected string
	}{
		{"original", "original: Hello World"},
		{"encoded", "encoded: SGVsbG8gV29ybGQ="},
		{"decoded", "decoded: Hello World"},
		{"roundtrip", "roundtrip: Hello World"},
	}

	for _, check := range checks {
		if !strings.Contains(output, check.expected) {
			t.Errorf("%s: expected %q in output, got:\n%s", check.name, check.expected, output)
		}
	}
}

// TestFunctions_JSON verifies JSON parsing functions work correctly.
// Tests: json, jsonArray
func TestFunctions_JSON(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	destPath := env.DestPath("json-funcs.txt")

	// Write template using JSON functions
	env.WriteTemplate("json-funcs.tmpl", `{{ $obj := json (getv "/data/object") }}
name: {{ index $obj "name" }}
count: {{ index $obj "count" }}
{{ $arr := jsonArray (getv "/data/array") }}
array_len: {{ len $arr }}
first_item: {{ index $arr 0 }}
last_item: {{ index $arr 2 }}
`)

	// Write config
	env.WriteConfig("json-funcs.toml", fmt.Sprintf(`[template]
src = "json-funcs.tmpl"
dest = "%s"
keys = ["/data/object", "/data/array"]
`, destPath))

	// Run confd
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	confd.SetEnv("DATA_OBJECT", `{"name":"test-service","count":"42"}`)
	confd.SetEnv("DATA_ARRAY", `["alpha","beta","gamma"]`)
	err := confd.Start(ctx, "env", "--onetime", "--confdir", env.ConfDir, "--log-level", "error")
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}

	exitCode, err := confd.Wait()
	if err != nil {
		t.Fatalf("Error waiting for confd: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	// Verify output
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	output := string(content)

	// Verify each function result
	checks := []struct {
		name     string
		expected string
	}{
		{"name", "name: test-service"},
		{"count", "count: 42"},
		{"array_len", "array_len: 3"},
		{"first_item", "first_item: alpha"},
		{"last_item", "last_item: gamma"},
	}

	for _, check := range checks {
		if !strings.Contains(output, check.expected) {
			t.Errorf("%s: expected %q in output, got:\n%s", check.name, check.expected, output)
		}
	}
}

// TestFunctions_NetworkLookup verifies network lookup functions work correctly.
// Tests: lookupIP (using localhost which should always resolve)
// Note: This test may be skipped in some CI environments without proper DNS.
func TestFunctions_NetworkLookup(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	destPath := env.DestPath("network-funcs.txt")

	// Write template using network lookup functions
	// Use "localhost" which should always resolve
	env.WriteTemplate("network-funcs.tmpl", `hostname_input: {{ getv "/data/hostname" }}
{{ $ips := lookupIP (getv "/data/hostname") }}
ip_count: {{ len $ips }}
{{ if gt (len $ips) 0 }}has_ip: true{{ else }}has_ip: false{{ end }}
`)

	// Write config
	env.WriteConfig("network-funcs.toml", fmt.Sprintf(`[template]
src = "network-funcs.tmpl"
dest = "%s"
keys = ["/data/hostname"]
`, destPath))

	// Run confd
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	confd.SetEnv("DATA_HOSTNAME", "localhost")
	err := confd.Start(ctx, "env", "--onetime", "--confdir", env.ConfDir, "--log-level", "error")
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}

	exitCode, err := confd.Wait()
	if err != nil {
		t.Fatalf("Error waiting for confd: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	// Verify output
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	output := string(content)

	// Verify hostname was processed
	if !strings.Contains(output, "hostname_input: localhost") {
		t.Errorf("Expected hostname_input in output, got:\n%s", output)
	}

	// localhost should resolve to at least one IP
	if !strings.Contains(output, "has_ip: true") {
		t.Errorf("Expected lookupIP to find IPs for localhost, got:\n%s", output)
	}
}

// TestFunctions_Composition verifies that functions can be composed/nested correctly.
// Tests nested function calls and pipeline operations.
func TestFunctions_Composition(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	destPath := env.DestPath("composition-funcs.txt")

	// Write template with nested/composed function calls
	env.WriteTemplate("composition-funcs.tmpl", `{{ $input := getv "/data/input" }}
original: {{ $input }}
upper_b64: {{ base64Encode (toUpper $input) }}
split_join_upper: {{ toUpper (join (split $input ",") "-") }}
{{ $nums := split (getv "/data/numbers") "," }}
{{ $first := atoi (index $nums 0) }}{{ $second := atoi (index $nums 1) }}
sum_doubled: {{ mul (add $first $second) 2 }}
nested_replace: {{ toLower (replace (toUpper $input) "," "_" -1) }}
`)

	// Write config
	env.WriteConfig("composition-funcs.toml", fmt.Sprintf(`[template]
src = "composition-funcs.tmpl"
dest = "%s"
keys = ["/data/input", "/data/numbers"]
`, destPath))

	// Run confd
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	confd.SetEnv("DATA_INPUT", "foo,bar,baz")
	confd.SetEnv("DATA_NUMBERS", "5,10")
	err := confd.Start(ctx, "env", "--onetime", "--confdir", env.ConfDir, "--log-level", "error")
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}

	exitCode, err := confd.Wait()
	if err != nil {
		t.Fatalf("Error waiting for confd: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	// Verify output
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	output := string(content)

	// Verify composed function results
	checks := []struct {
		name     string
		expected string
	}{
		{"original", "original: foo,bar,baz"},
		{"upper_b64", "upper_b64: Rk9PLEJBUixCQVo="}, // base64("FOO,BAR,BAZ")
		{"split_join_upper", "split_join_upper: FOO-BAR-BAZ"},
		{"sum_doubled", "sum_doubled: 30"}, // (5 + 10) * 2 = 30
		{"nested_replace", "nested_replace: foo_bar_baz"},
	}

	for _, check := range checks {
		if !strings.Contains(output, check.expected) {
			t.Errorf("%s: expected %q in output, got:\n%s", check.name, check.expected, output)
		}
	}
}

// TestFunctions_PathOperations verifies path manipulation functions work correctly.
// Tests: base (basename), dir (directory)
func TestFunctions_PathOperations(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	destPath := env.DestPath("path-funcs.txt")

	// Write template using path functions
	env.WriteTemplate("path-funcs.tmpl", `# Path operations on: {{ getv "/data/path" }}
base_result: {{ base (getv "/data/path") }}
dir_result: {{ dir (getv "/data/path") }}
# Nested path
nested_base: {{ base (getv "/data/nested") }}
nested_dir: {{ dir (getv "/data/nested") }}
# Pipeline style
piped_base: {{ "/database/host" | base }}
piped_dir: {{ "/database/host" | dir }}
`)

	// Write config
	env.WriteConfig("path-funcs.toml", fmt.Sprintf(`[template]
src = "path-funcs.tmpl"
dest = "%s"
keys = ["/data/path", "/data/nested"]
`, destPath))

	// Run confd
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	confd.SetEnv("DATA_PATH", "/etc/confd/config.toml")
	confd.SetEnv("DATA_NESTED", "/a/b/c/d/file.txt")
	err := confd.Start(ctx, "env", "--onetime", "--confdir", env.ConfDir, "--log-level", "error")
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}

	exitCode, err := confd.Wait()
	if err != nil {
		t.Fatalf("Error waiting for confd: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	// Verify output
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	output := string(content)

	checks := []struct {
		name     string
		expected string
	}{
		{"base_result", "base_result: config.toml"},
		{"dir_result", "dir_result: /etc/confd"},
		{"nested_base", "nested_base: file.txt"},
		{"nested_dir", "nested_dir: /a/b/c/d"},
		{"piped_base", "piped_base: host"},
		{"piped_dir", "piped_dir: /database"},
	}

	for _, check := range checks {
		if !strings.Contains(output, check.expected) {
			t.Errorf("%s: expected %q in output, got:\n%s", check.name, check.expected, output)
		}
	}
}

// TestFunctions_DataUtilities verifies data utility functions work correctly.
// Tests: getenv, map, parseBool
func TestFunctions_DataUtilities(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	destPath := env.DestPath("data-utils.txt")

	// Write template using data utility functions
	env.WriteTemplate("data-utils.tmpl", `# Data utility functions
# getenv - read from environment
hostname: {{ getenv "HOSTNAME" }}
custom_env: {{ getenv "MY_CUSTOM_VAR" }}
# map - create key-value map
{{- $mymap := map "key1" "value1" "key2" "value2" "key3" "value3" }}
map_key1: {{ index $mymap "key1" }}
map_key2: {{ index $mymap "key2" }}
map_key3: {{ index $mymap "key3" }}
# parseBool - parse boolean strings
bool_true: {{ if parseBool "true" }}yes{{ else }}no{{ end }}
bool_false: {{ if parseBool "false" }}yes{{ else }}no{{ end }}
bool_one: {{ if parseBool "1" }}yes{{ else }}no{{ end }}
bool_zero: {{ if parseBool "0" }}yes{{ else }}no{{ end }}
bool_TRUE: {{ if parseBool "TRUE" }}yes{{ else }}no{{ end }}
`)

	// Write config
	env.WriteConfig("data-utils.toml", fmt.Sprintf(`[template]
src = "data-utils.tmpl"
dest = "%s"
keys = ["/data/dummy"]
`, destPath))

	// Run confd
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	confd.SetEnv("HOSTNAME", "test-host.local")
	confd.SetEnv("MY_CUSTOM_VAR", "custom-value-123")
	confd.SetEnv("DATA_DUMMY", "dummy")
	err := confd.Start(ctx, "env", "--onetime", "--confdir", env.ConfDir, "--log-level", "error")
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}

	exitCode, err := confd.Wait()
	if err != nil {
		t.Fatalf("Error waiting for confd: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	// Verify output
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	output := string(content)

	checks := []struct {
		name     string
		expected string
	}{
		{"hostname", "hostname: test-host.local"},
		{"custom_env", "custom_env: custom-value-123"},
		{"map_key1", "map_key1: value1"},
		{"map_key2", "map_key2: value2"},
		{"map_key3", "map_key3: value3"},
		{"bool_true", "bool_true: yes"},
		{"bool_false", "bool_false: no"},
		{"bool_one", "bool_one: yes"},
		{"bool_zero", "bool_zero: no"},
		{"bool_TRUE", "bool_TRUE: yes"},
	}

	for _, check := range checks {
		if !strings.Contains(output, check.expected) {
			t.Errorf("%s: expected %q in output, got:\n%s", check.name, check.expected, output)
		}
	}
}

// TestFunctions_SortingAndReverse verifies sorting and reverse functions work correctly.
// Tests: reverse
func TestFunctions_SortingAndReverse(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	destPath := env.DestPath("sorting-funcs.txt")

	// Write template using sorting/reverse functions
	env.WriteTemplate("sorting-funcs.tmpl", `# Sorting and reverse functions
{{- $items := split (getv "/data/items") ":" }}
original: {{ join $items ":" }}
{{- $reversed := reverse $items }}
reversed: {{ join $reversed ":" }}
# Reverse a different list
{{- $parts := split (getv "/data/parts") "," }}
parts_original: {{ join $parts "," }}
{{- $parts_rev := reverse $parts }}
parts_reversed: {{ join $parts_rev "," }}
# Access reversed items individually
first_reversed: {{ index $reversed 0 }}
last_reversed: {{ index $reversed 2 }}
`)

	// Write config
	env.WriteConfig("sorting-funcs.toml", fmt.Sprintf(`[template]
src = "sorting-funcs.tmpl"
dest = "%s"
keys = ["/data/items", "/data/parts"]
`, destPath))

	// Run confd
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	confd.SetEnv("DATA_ITEMS", "alpha:beta:gamma")
	confd.SetEnv("DATA_PARTS", "one,two,three,four")
	err := confd.Start(ctx, "env", "--onetime", "--confdir", env.ConfDir, "--log-level", "error")
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}

	exitCode, err := confd.Wait()
	if err != nil {
		t.Fatalf("Error waiting for confd: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	// Verify output
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	output := string(content)

	checks := []struct {
		name     string
		expected string
	}{
		{"original", "original: alpha:beta:gamma"},
		{"reversed", "reversed: gamma:beta:alpha"},
		{"parts_original", "parts_original: one,two,three,four"},
		{"parts_reversed", "parts_reversed: four,three,two,one"},
		{"first_reversed", "first_reversed: gamma"},
		{"last_reversed", "last_reversed: alpha"},
	}

	for _, check := range checks {
		if !strings.Contains(output, check.expected) {
			t.Errorf("%s: expected %q in output, got:\n%s", check.name, check.expected, output)
		}
	}
}

// TestFunctions_ExistsAndConditional verifies the exists function works correctly.
// Tests: exists (checking if keys exist in backend)
func TestFunctions_ExistsAndConditional(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	destPath := env.DestPath("exists-funcs.txt")

	// Write template using exists function
	env.WriteTemplate("exists-funcs.tmpl", `# Exists function tests
# Check existing key
exists_key: {{ if exists "/data/present" }}found{{ else }}missing{{ end }}
# Check non-existent key
exists_fake: {{ if exists "/nonexistent/key" }}found{{ else }}missing{{ end }}
# Use exists with getv safely
safe_value: {{ if exists "/data/present" }}{{ getv "/data/present" }}{{ else }}default{{ end }}
safe_missing: {{ if exists "/data/missing" }}{{ getv "/data/missing" }}{{ else }}default{{ end }}
# Nested exists checks
{{- if exists "/data/present" }}
conditional_block: present key exists
{{- else }}
conditional_block: present key missing
{{- end }}
`)

	// Write config
	env.WriteConfig("exists-funcs.toml", fmt.Sprintf(`[template]
src = "exists-funcs.tmpl"
dest = "%s"
keys = ["/data/present"]
`, destPath))

	// Run confd
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	confd.SetEnv("DATA_PRESENT", "actual-value")
	// Note: DATA_MISSING is intentionally not set
	err := confd.Start(ctx, "env", "--onetime", "--confdir", env.ConfDir, "--log-level", "error")
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}

	exitCode, err := confd.Wait()
	if err != nil {
		t.Fatalf("Error waiting for confd: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	// Verify output
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	output := string(content)

	checks := []struct {
		name     string
		expected string
	}{
		{"exists_key", "exists_key: found"},
		{"exists_fake", "exists_fake: missing"},
		{"safe_value", "safe_value: actual-value"},
		{"safe_missing", "safe_missing: default"},
		{"conditional_block", "conditional_block: present key exists"},
	}

	for _, check := range checks {
		if !strings.Contains(output, check.expected) {
			t.Errorf("%s: expected %q in output, got:\n%s", check.name, check.expected, output)
		}
	}
}

// TestFunctions_GetsAndRange verifies the gets function works correctly with range iteration.
// Tests: gets (get multiple keys with prefix), range iteration over key-value pairs
func TestFunctions_GetsAndRange(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	destPath := env.DestPath("gets-funcs.txt")

	// Write template using gets function with range
	env.WriteTemplate("gets-funcs.tmpl", `# Gets and range iteration
all_services:
{{- range gets "/services/*" }}
  - {{ .Key }}: {{ .Value }}
{{- end }}
# Count services
{{- $services := gets "/services/*" }}
service_count: {{ len $services }}
# Access specific service by iteration
first_service_key: {{ (index $services 0).Key }}
first_service_value: {{ (index $services 0).Value }}
`)

	// Write config - note we need to specify the prefix keys
	env.WriteConfig("gets-funcs.toml", fmt.Sprintf(`[template]
src = "gets-funcs.tmpl"
dest = "%s"
keys = ["/services/web", "/services/api", "/services/db"]
`, destPath))

	// Run confd
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	confd.SetEnv("SERVICES_WEB", "10.0.1.1:80")
	confd.SetEnv("SERVICES_API", "10.0.1.2:8080")
	confd.SetEnv("SERVICES_DB", "10.0.1.3:5432")
	err := confd.Start(ctx, "env", "--onetime", "--confdir", env.ConfDir, "--log-level", "error")
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}

	exitCode, err := confd.Wait()
	if err != nil {
		t.Fatalf("Error waiting for confd: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	// Verify output
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	output := string(content)

	// Verify that gets/range produced correct output
	// Note: Order may vary based on backend, so check each key-value pair separately
	checks := []struct {
		name     string
		expected string
	}{
		{"web_service", "/services/web: 10.0.1.1:80"},
		{"api_service", "/services/api: 10.0.1.2:8080"},
		{"db_service", "/services/db: 10.0.1.3:5432"},
		{"service_count", "service_count: 3"},
	}

	for _, check := range checks {
		if !strings.Contains(output, check.expected) {
			t.Errorf("%s: expected %q in output, got:\n%s", check.name, check.expected, output)
		}
	}

	// Verify the first service key exists (order may vary)
	if !strings.Contains(output, "first_service_key: /services/") {
		t.Errorf("Expected first_service_key to contain '/services/' prefix, got:\n%s", output)
	}
}
