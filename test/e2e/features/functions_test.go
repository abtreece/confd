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
