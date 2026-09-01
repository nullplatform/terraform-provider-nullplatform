package nullplatform

import (
	"encoding/json"
	"testing"
)

// A partial update must only carry the fields that actually changed. Before the
// omitempty tags, an update that touched just the description also serialized
// `"name": ""`, which the API rejects with
// `body/name must NOT have fewer than 1 characters`, and blanked provider_id
// and provider_type on the way.
func TestScopeType_PartialUpdateOmitsUnsetFields(t *testing.T) {
	body, err := json.Marshal(&ScopeType{Description: "new description"})
	if err != nil {
		t.Fatalf("failed to marshal scope type: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("failed to unmarshal scope type: %v", err)
	}

	if want := map[string]any{"description": "new description"}; len(got) != len(want) {
		t.Fatalf("partial update body = %s, want only the changed field", body)
	}

	for _, field := range []string{"name", "type", "status", "provider_id", "provider_type"} {
		if _, present := got[field]; present {
			t.Errorf("unset field %q was serialized; it must be omitted on a partial update", field)
		}
	}
}

// Create still has to send every field, so omitempty must not drop values that
// were explicitly set.
func TestScopeType_CreateKeepsAllSetFields(t *testing.T) {
	body, err := json.Marshal(&ScopeType{
		Nrn:          "organization=1:account=2",
		Type:         "containers",
		Name:         "Containers",
		Status:       "active",
		Description:  "Docker containers on pods",
		ProviderType: "kubernetes",
		ProviderId:   "spec-1",
	})
	if err != nil {
		t.Fatalf("failed to marshal scope type: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("failed to unmarshal scope type: %v", err)
	}

	for _, field := range []string{"nrn", "type", "name", "status", "description", "provider_type", "provider_id"} {
		if _, present := got[field]; !present {
			t.Errorf("field %q was dropped from the create body: %s", field, body)
		}
	}
}
