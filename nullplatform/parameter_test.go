package nullplatform

import "testing"

func TestGenerateParameterValueID(t *testing.T) {
	// Test case 1: Dimensions is empty
	param1 := &ParameterValue{
		Nrn: "organization=1:account=2:namespace=3:application=4:scope=5",
	}

	expectedHash1 := generateParameterValueID(param1)
	if expectedHash1 != "045b0f80dcefa1d9cc23d79584de844611e6d99cb08674e6f1f8ebdc93bc79dd" {
		t.Errorf("Expected hash: 045b0f80dcefa1d9cc23d79584de844611e6d99cb08674e6f1f8ebdc93bc79dd, got: %s", expectedHash1)
	}

	// Test case 2: Dimensions is not empty
	param2 := &ParameterValue{
		Nrn: "organization=1:account=2:namespace=3:application=4",
		Dimensions: map[string]string{
			"environment": "dev",
			"country":     "arg",
		},
	}

	expectedHash2 := generateParameterValueID(param2)
	if expectedHash2 != "d31e22a6e36f9c5034ca3a60bd744ad78d58fc8c37d710b2e6cf4a61efabbf1d" {
		t.Errorf("Expected hash: d31e22a6e36f9c5034ca3a60bd744ad78d58fc8c37d710b2e6cf4a61efabbf1d, got: %s", expectedHash2)
	}
}
