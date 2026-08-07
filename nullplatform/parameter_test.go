package nullplatform

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
)

func TestGenerateParameterValueID(t *testing.T) {
	parameterId := 1
	// Test case 1: At Scope level without Dimensions nor Value
	param1 := &ParameterValue{
		Nrn: "organization=1:account=2:namespace=3:application=4:scope=5",
	}

	expectedHash1 := generateParameterValueID(param1, parameterId)
	if expectedHash1 != "6523e8f336a33e0da14184b31454be7df1224f64466ee090e0e28f0c41c4a261" {
		t.Errorf("Expected hash: 6523e8f336a33e0da14184b31454be7df1224f64466ee090e0e28f0c41c4a261, got: %s", expectedHash1)
	}

	// Test case 2: At Scope level with empty Value, and without Dimensions
	param2 := &ParameterValue{
		Nrn:   "organization=1:account=2:namespace=3:application=4:scope=5",
		Value: "",
	}

	expectedHash2 := generateParameterValueID(param2, parameterId)
	if expectedHash2 != "6523e8f336a33e0da14184b31454be7df1224f64466ee090e0e28f0c41c4a261" {
		t.Errorf("Expected hash: 6523e8f336a33e0da14184b31454be7df1224f64466ee090e0e28f0c41c4a261, got: %s", expectedHash2)
	}

	// Test case 3: At Scope level with Value, and without Dimensions
	param3 := &ParameterValue{
		Nrn:   "organization=1:account=2:namespace=3:application=4:scope=5",
		Value: "_VALUE_",
	}

	expectedHash3 := generateParameterValueID(param3, parameterId)
	if expectedHash3 != "6523e8f336a33e0da14184b31454be7df1224f64466ee090e0e28f0c41c4a261" {
		t.Errorf("Expected hash: 6523e8f336a33e0da14184b31454be7df1224f64466ee090e0e28f0c41c4a261, got: %s", expectedHash3)
	}

	// Test case 4: At Scope level with empty Dimensions
	param4 := &ParameterValue{
		Nrn:        "organization=1:account=2:namespace=3:application=4:scope=5",
		Value:      "_VALUE_",
		Dimensions: map[string]string{},
	}

	expectedHash4 := generateParameterValueID(param4, parameterId)
	if expectedHash4 != "6523e8f336a33e0da14184b31454be7df1224f64466ee090e0e28f0c41c4a261" {
		t.Errorf("Expected hash: 6523e8f336a33e0da14184b31454be7df1224f64466ee090e0e28f0c41c4a261, got: %s", expectedHash4)
	}

	// The multi-dimension expectations below are the SORTED-order hashes
	// (country before environment). The previous constants encoded whichever
	// random map order the author's run produced, so this test failed whenever
	// the runtime picked the other order — the same nondeterminism that made a
	// two-dimension value miss its own read lookup.

	// Test case 5: At Application level with Dimensions nor Value
	param5 := &ParameterValue{
		Nrn: "organization=1:account=2:namespace=3:application=4",
		Dimensions: map[string]string{
			"environment": "dev",
			"country":     "arg",
		},
	}

	expectedHash5 := generateParameterValueID(param5, parameterId)
	if expectedHash5 != "617957040f4f6a292ef3f01a7db627363d91a661cb1b03473d2ddbdc99f376e9" {
		t.Errorf("Expected hash: 617957040f4f6a292ef3f01a7db627363d91a661cb1b03473d2ddbdc99f376e9, got: %s", expectedHash5)
	}

	// Test case 6: At Application level with Value, and Dimensions
	param6 := &ParameterValue{
		Nrn:   "organization=1:account=2:namespace=3:application=4",
		Value: "_VALUE_",
		Dimensions: map[string]string{
			"environment": "dev",
			"country":     "arg",
		},
	}

	expectedHash6 := generateParameterValueID(param6, parameterId)
	if expectedHash6 != "617957040f4f6a292ef3f01a7db627363d91a661cb1b03473d2ddbdc99f376e9" {
		t.Errorf("Expected hash: 617957040f4f6a292ef3f01a7db627363d91a661cb1b03473d2ddbdc99f376e9, got: %s", expectedHash6)
	}

	// Test case 7: At Scope level with Value, and Dimensions. This case shoud not exists but it can be handled
	param7 := &ParameterValue{
		Nrn:   "organization=1:account=2:namespace=3:application=4:scope=5",
		Value: "_VALUE_",
		Dimensions: map[string]string{
			"environment": "dev",
			"country":     "arg",
		},
	}

	expectedHash7 := generateParameterValueID(param7, parameterId)
	if expectedHash7 != "fd120e5ce41a86b23b3bcca38378ea81cfd03388ba4ca602afe5c0cf39d458b5" {
		t.Errorf("Expected hash: fd120e5ce41a86b23b3bcca38378ea81cfd03388ba4ca602afe5c0cf39d458b5, got: %s", expectedHash7)
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"plain error", errors.New("boom"), false},
		{"408 timeout", &HTTPStatusError{Status: http.StatusRequestTimeout, Message: "x"}, true},
		{"409 conflict", &HTTPStatusError{Status: http.StatusConflict, Message: "x"}, true},
		{"429 too many", &HTTPStatusError{Status: http.StatusTooManyRequests, Message: "x"}, true},
		{"502 bad gateway", &HTTPStatusError{Status: http.StatusBadGateway, Message: "x"}, true},
		{"503 unavailable", &HTTPStatusError{Status: http.StatusServiceUnavailable, Message: "x"}, true},
		{"504 gateway timeout", &HTTPStatusError{Status: http.StatusGatewayTimeout, Message: "x"}, true},
		{"400 already exists", &HTTPStatusError{Status: http.StatusBadRequest, Message: "The parameter already exists"}, true},
		{"400 other", &HTTPStatusError{Status: http.StatusBadRequest, Message: "invalid input"}, false},
		{"404 not found", &HTTPStatusError{Status: http.StatusNotFound, Message: "x"}, false},
		{"500 internal", &HTTPStatusError{Status: http.StatusInternalServerError, Message: "x"}, false},
		{"wrapped 503", fmt.Errorf("context: %w", &HTTPStatusError{Status: http.StatusServiceUnavailable, Message: "x"}), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRetryableError(tt.err); got != tt.want {
				t.Errorf("isRetryableError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// The id is recomputed on every read and compared against the one in state, so
// it must be a pure function of its inputs. It was not: dimensions were
// concatenated in Go's randomized map-iteration order, so a value with two or
// more dimensions failed its own lookup on most reads. One hundred rounds make
// the old behavior fail this test virtually every run.
func TestGenerateParameterValueID_DeterministicAcrossDimensionOrder(t *testing.T) {
	value := &ParameterValue{
		Nrn: "organization=1:account=2:namespace=3:application=4",
		Dimensions: map[string]string{
			"environment": "production",
			"country":     "argentina",
			"region":      "coastal",
		},
	}

	first := generateParameterValueID(value, 7)
	for i := 0; i < 100; i++ {
		if got := generateParameterValueID(value, 7); got != first {
			t.Fatalf("round %d: id %s differs from %s — the hash depends on map iteration order", i, got, first)
		}
	}
}
