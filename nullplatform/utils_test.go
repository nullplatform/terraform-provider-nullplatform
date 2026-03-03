package nullplatform

import (
	"testing"
)

func TestValueToString(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{"string", "hello", "hello"},
		{"empty string", "", ""},
		{"bool true", true, "true"},
		{"bool false", false, "false"},
		{"float64 integer", float64(42), "42"},
		{"float64 decimal", float64(3.14), "3.14"},
		{"float64 half", float64(0.5), "0.5"},
		{"int", int(100), "100"},
		{"int64", int64(2048), "2048"},
		{"unknown type (slice)", []int{1, 2}, "[1 2]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := valueToString(tt.input)
			if result != tt.expected {
				t.Errorf("valueToString(%v) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMapOfInterfacesToMapOfStrings(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		expected map[string]string
	}{
		{
			name:     "empty map",
			input:    map[string]interface{}{},
			expected: map[string]string{},
		},
		{
			name:     "nil map",
			input:    nil,
			expected: map[string]string{},
		},
		{
			name: "mixed types",
			input: map[string]interface{}{
				"name":    "test-service",
				"enabled": true,
				"port":    float64(443),
				"rate":    float64(1.5),
				"count":   int(100),
			},
			expected: map[string]string{
				"name":    "test-service",
				"enabled": "true",
				"port":    "443",
				"rate":    "1.5",
				"count":   "100",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapOfInterfacesToMapOfStrings(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("expected length %d, got %d", len(tt.expected), len(result))
			}
			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("key %q: expected %q, got %q", k, v, result[k])
				}
			}
		})
	}
}
