package nullplatform

import (
	"testing"
)

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
			name: "single string value",
			input: map[string]interface{}{
				"key1": "value1",
			},
			expected: map[string]string{
				"key1": "value1",
			},
		},
		{
			name: "multiple string values",
			input: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
				"key3": "value3",
			},
			expected: map[string]string{
				"key1": "value1",
				"key2": "value2",
				"key3": "value3",
			},
		},
		{
			name: "boolean values",
			input: map[string]interface{}{
				"enabled":  true,
				"disabled": false,
			},
			expected: map[string]string{
				"enabled":  "true",
				"disabled": "false",
			},
		},
		{
			name: "integer values from JSON (float64)",
			input: map[string]interface{}{
				"count": float64(42),
				"port":  float64(8080),
			},
			expected: map[string]string{
				"count": "42",
				"port":  "8080",
			},
		},
		{
			name: "float values",
			input: map[string]interface{}{
				"rate":       float64(3.14),
				"percentage": float64(0.5),
			},
			expected: map[string]string{
				"rate":       "3.14",
				"percentage": "0.5",
			},
		},
		{
			name: "native int values",
			input: map[string]interface{}{
				"count": int(100),
				"size":  int64(2048),
			},
			expected: map[string]string{
				"count": "100",
				"size":  "2048",
			},
		},
		{
			name: "mixed types",
			input: map[string]interface{}{
				"name":    "test-service",
				"enabled": true,
				"port":    float64(443),
				"rate":    float64(1.5),
			},
			expected: map[string]string{
				"name":    "test-service",
				"enabled": "true",
				"port":    "443",
				"rate":    "1.5",
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
					t.Errorf("expected %s=%s, got %s=%s", k, v, k, result[k])
				}
			}
		})
	}
}
