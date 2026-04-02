package template

import (
	"reflect"
	"testing"
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
		{"struct", struct{}{}, false},
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
