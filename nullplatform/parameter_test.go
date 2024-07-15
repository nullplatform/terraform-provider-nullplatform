package nullplatform

import "testing"

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

	// Test case 5: At Application level with Dimensions nor Value
	param5 := &ParameterValue{
		Nrn: "organization=1:account=2:namespace=3:application=4",
		Dimensions: map[string]string{
			"environment": "dev",
			"country":     "arg",
		},
	}

	expectedHash5 := generateParameterValueID(param5, parameterId)
	if expectedHash5 != "1d6830039cf4e3143c23e3d36dd45850f7ba5241660d2ec8c5eb77dbe7c2f15d" {
		t.Errorf("Expected hash: 1d6830039cf4e3143c23e3d36dd45850f7ba5241660d2ec8c5eb77dbe7c2f15d, got: %s", expectedHash5)
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
	if expectedHash6 != "1d6830039cf4e3143c23e3d36dd45850f7ba5241660d2ec8c5eb77dbe7c2f15d" {
		t.Errorf("Expected hash: 1d6830039cf4e3143c23e3d36dd45850f7ba5241660d2ec8c5eb77dbe7c2f15d, got: %s", expectedHash6)
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
	if expectedHash7 != "972d76cb3b1db5b9a145dea7aa72395ae6459f02e05ac18f7f6439904a93326f" {
		t.Errorf("Expected hash: 972d76cb3b1db5b9a145dea7aa72395ae6459f02e05ac18f7f6439904a93326f, got: %s", expectedHash7)
	}

}
