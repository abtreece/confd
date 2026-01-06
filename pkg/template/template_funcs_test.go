package template

import (
	"reflect"
	"testing"

	"github.com/kelseyhightower/memkv"
)

func TestSeq(t *testing.T) {
	tests := []struct {
		name     string
		first    int
		last     int
		expected []int
	}{
		{
			name:     "ascending sequence",
			first:    1,
			last:     5,
			expected: []int{1, 2, 3, 4, 5},
		},
		{
			name:     "single element",
			first:    3,
			last:     3,
			expected: []int{3},
		},
		{
			name:     "negative to positive",
			first:    -2,
			last:     2,
			expected: []int{-2, -1, 0, 1, 2},
		},
		{
			name:     "empty when first > last",
			first:    5,
			last:     1,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Seq(tt.first, tt.last)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Seq(%d, %d) = %v, want %v", tt.first, tt.last, result, tt.expected)
			}
		})
	}
}

func TestSortByLength(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "sort strings by length",
			input:    []string{"aaa", "a", "aa"},
			expected: []string{"a", "aa", "aaa"},
		},
		{
			name:     "already sorted",
			input:    []string{"x", "xx", "xxx"},
			expected: []string{"x", "xx", "xxx"},
		},
		{
			name:     "empty slice",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "single element",
			input:    []string{"hello"},
			expected: []string{"hello"},
		},
		{
			name:     "same length elements",
			input:    []string{"abc", "def", "ghi"},
			expected: []string{"abc", "def", "ghi"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy to avoid modifying test data
			input := make([]string, len(tt.input))
			copy(input, tt.input)
			result := SortByLength(input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("SortByLength(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSortKVByLength(t *testing.T) {
	tests := []struct {
		name     string
		input    []memkv.KVPair
		expected []memkv.KVPair
	}{
		{
			name: "sort KV pairs by key length",
			input: []memkv.KVPair{
				{Key: "/long/key", Value: "v1"},
				{Key: "/k", Value: "v2"},
				{Key: "/med", Value: "v3"},
			},
			expected: []memkv.KVPair{
				{Key: "/k", Value: "v2"},
				{Key: "/med", Value: "v3"},
				{Key: "/long/key", Value: "v1"},
			},
		},
		{
			name:     "empty slice",
			input:    []memkv.KVPair{},
			expected: []memkv.KVPair{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := make([]memkv.KVPair, len(tt.input))
			copy(input, tt.input)
			result := SortKVByLength(input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("SortKVByLength() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestReverse(t *testing.T) {
	t.Run("reverse string slice", func(t *testing.T) {
		input := []string{"a", "b", "c", "d"}
		expected := []string{"d", "c", "b", "a"}
		result := Reverse(input).([]string)
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("Reverse(%v) = %v, want %v", input, result, expected)
		}
	})

	t.Run("reverse KVPair slice", func(t *testing.T) {
		input := []memkv.KVPair{
			{Key: "/a", Value: "1"},
			{Key: "/b", Value: "2"},
			{Key: "/c", Value: "3"},
		}
		expected := []memkv.KVPair{
			{Key: "/c", Value: "3"},
			{Key: "/b", Value: "2"},
			{Key: "/a", Value: "1"},
		}
		result := Reverse(input).([]memkv.KVPair)
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("Reverse() = %v, want %v", result, expected)
		}
	})

	t.Run("reverse empty slice", func(t *testing.T) {
		input := []string{}
		result := Reverse(input).([]string)
		if len(result) != 0 {
			t.Errorf("Reverse([]) should return empty slice, got %v", result)
		}
	})

	t.Run("reverse single element", func(t *testing.T) {
		input := []string{"only"}
		expected := []string{"only"}
		result := Reverse(input).([]string)
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("Reverse(%v) = %v, want %v", input, result, expected)
		}
	})
}

func TestGetenv(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue []string
		envValue     string
		setEnv       bool
		expected     string
	}{
		{
			name:         "env var exists",
			key:          "TEST_GETENV_EXISTS",
			defaultValue: []string{"default"},
			envValue:     "actual_value",
			setEnv:       true,
			expected:     "actual_value",
		},
		{
			name:         "env var not set with default",
			key:          "TEST_GETENV_MISSING",
			defaultValue: []string{"default_value"},
			envValue:     "",
			setEnv:       false,
			expected:     "default_value",
		},
		{
			name:         "env var not set without default",
			key:          "TEST_GETENV_NO_DEFAULT",
			defaultValue: []string{},
			envValue:     "",
			setEnv:       false,
			expected:     "",
		},
		{
			name:         "env var empty with default",
			key:          "TEST_GETENV_EMPTY",
			defaultValue: []string{"fallback"},
			envValue:     "",
			setEnv:       true,
			expected:     "fallback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setEnv {
				t.Setenv(tt.key, tt.envValue)
			}
			result := Getenv(tt.key, tt.defaultValue...)
			if result != tt.expected {
				t.Errorf("Getenv(%s, %v) = %s, want %s", tt.key, tt.defaultValue, result, tt.expected)
			}
		})
	}
}

func TestCreateMap(t *testing.T) {
	tests := []struct {
		name        string
		values      []interface{}
		expected    map[string]interface{}
		expectError bool
	}{
		{
			name:     "valid map creation",
			values:   []interface{}{"key1", "value1", "key2", 42},
			expected: map[string]interface{}{"key1": "value1", "key2": 42},
		},
		{
			name:     "empty map",
			values:   []interface{}{},
			expected: map[string]interface{}{},
		},
		{
			name:        "odd number of arguments",
			values:      []interface{}{"key1", "value1", "key2"},
			expectError: true,
		},
		{
			name:        "non-string key",
			values:      []interface{}{123, "value1"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := CreateMap(tt.values...)
			if tt.expectError {
				if err == nil {
					t.Errorf("CreateMap(%v) expected error, got nil", tt.values)
				}
				return
			}
			if err != nil {
				t.Errorf("CreateMap(%v) unexpected error: %v", tt.values, err)
				return
			}
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("CreateMap(%v) = %v, want %v", tt.values, result, tt.expected)
			}
		})
	}
}

func TestUnmarshalJsonObject(t *testing.T) {
	tests := []struct {
		name        string
		data        string
		expected    map[string]interface{}
		expectError bool
	}{
		{
			name:     "valid JSON object",
			data:     `{"key": "value", "number": 42}`,
			expected: map[string]interface{}{"key": "value", "number": float64(42)},
		},
		{
			name:     "empty object",
			data:     `{}`,
			expected: map[string]interface{}{},
		},
		{
			name:     "nested object",
			data:     `{"outer": {"inner": "value"}}`,
			expected: map[string]interface{}{"outer": map[string]interface{}{"inner": "value"}},
		},
		{
			name:        "invalid JSON",
			data:        `{invalid}`,
			expectError: true,
		},
		{
			name:        "JSON array instead of object",
			data:        `[1, 2, 3]`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := UnmarshalJsonObject(tt.data)
			if tt.expectError {
				if err == nil {
					t.Errorf("UnmarshalJsonObject(%s) expected error, got nil", tt.data)
				}
				return
			}
			if err != nil {
				t.Errorf("UnmarshalJsonObject(%s) unexpected error: %v", tt.data, err)
				return
			}
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("UnmarshalJsonObject(%s) = %v, want %v", tt.data, result, tt.expected)
			}
		})
	}
}

func TestUnmarshalJsonArray(t *testing.T) {
	tests := []struct {
		name        string
		data        string
		expected    []interface{}
		expectError bool
	}{
		{
			name:     "valid JSON array",
			data:     `[1, 2, 3]`,
			expected: []interface{}{float64(1), float64(2), float64(3)},
		},
		{
			name:     "empty array",
			data:     `[]`,
			expected: []interface{}{},
		},
		{
			name:     "mixed types array",
			data:     `["string", 42, true, null]`,
			expected: []interface{}{"string", float64(42), true, nil},
		},
		{
			name:        "invalid JSON",
			data:        `[invalid]`,
			expectError: true,
		},
		{
			name:        "JSON object instead of array",
			data:        `{"key": "value"}`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := UnmarshalJsonArray(tt.data)
			if tt.expectError {
				if err == nil {
					t.Errorf("UnmarshalJsonArray(%s) expected error, got nil", tt.data)
				}
				return
			}
			if err != nil {
				t.Errorf("UnmarshalJsonArray(%s) unexpected error: %v", tt.data, err)
				return
			}
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("UnmarshalJsonArray(%s) = %v, want %v", tt.data, result, tt.expected)
			}
		})
	}
}

func TestBase64Encode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "encode simple string",
			input:    "hello",
			expected: "aGVsbG8=",
		},
		{
			name:     "encode empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "encode with special characters",
			input:    "hello world!",
			expected: "aGVsbG8gd29ybGQh",
		},
		{
			name:     "encode unicode",
			input:    "hello 世界",
			expected: "aGVsbG8g5LiW55WM",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Base64Encode(tt.input)
			if result != tt.expected {
				t.Errorf("Base64Encode(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestBase64Decode(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    string
		expectError bool
	}{
		{
			name:     "decode simple string",
			input:    "aGVsbG8=",
			expected: "hello",
		},
		{
			name:     "decode empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "decode with special characters",
			input:    "aGVsbG8gd29ybGQh",
			expected: "hello world!",
		},
		{
			name:        "invalid base64",
			input:       "not-valid-base64!!!",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Base64Decode(tt.input)
			if tt.expectError {
				if err == nil {
					t.Errorf("Base64Decode(%s) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("Base64Decode(%s) unexpected error: %v", tt.input, err)
				return
			}
			if result != tt.expected {
				t.Errorf("Base64Decode(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestBase64RoundTrip(t *testing.T) {
	testStrings := []string{
		"hello world",
		"",
		"special chars: !@#$%^&*()",
		"unicode: 日本語",
		"newlines\nand\ttabs",
	}

	for _, s := range testStrings {
		t.Run(s, func(t *testing.T) {
			encoded := Base64Encode(s)
			decoded, err := Base64Decode(encoded)
			if err != nil {
				t.Errorf("Base64Decode(Base64Encode(%q)) error: %v", s, err)
				return
			}
			if decoded != s {
				t.Errorf("Base64 round trip failed: got %q, want %q", decoded, s)
			}
		})
	}
}
