package nullplatform

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// scopeTypeServer serves a scope type and captures the body of the write
// request, so a test can assert on the exact payload the provider put on the
// wire rather than on the struct tags that shaped it.
func scopeTypeServer(t *testing.T, writeMethod string, captured *map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == writeMethod {
			if err := json.NewDecoder(r.Body).Decode(captured); err != nil {
				t.Errorf("decoding the %s body: %v", writeMethod, err)
			}
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(ScopeType{
			Id: 1, Nrn: "organization=1:account=2", Type: "custom",
			Name: "Containers", Status: "active", Description: "new description",
			ProviderType: "service", ProviderId: "spec-1",
		})
	}))
}

// A partial update must carry only the fields that changed. Before ScopeType's
// fields were tagged omitempty, an update that touched just the description
// also serialized `"name": ""`, which the API rejects with
// `body/name must NOT have fewer than 1 characters`, and blanked provider_id
// and provider_type on the way.
func TestScopeTypeUpdate_SendsOnlyChangedFields(t *testing.T) {
	var patched map[string]any
	server := scopeTypeServer(t, http.MethodPut, &patched)
	defer server.Close()

	state := &terraform.InstanceState{ID: "1", Attributes: map[string]string{
		"id": "1", "nrn": "organization=1:account=2", "type": "custom",
		"name": "Containers", "status": "active", "description": "old description",
		"provider_type": "service", "provider_id": "spec-1",
	}}
	diff, err := resourceScopeType().Diff(context.Background(), state, terraform.NewResourceConfigRaw(map[string]any{
		"nrn": "organization=1:account=2", "type": "custom",
		"name": "Containers", "description": "new description",
		"provider_type": "service", "provider_id": "spec-1",
	}), nil)
	if err != nil {
		t.Fatalf("computing the diff: %v", err)
	}
	d, err := schema.InternalMap(resourceScopeType().Schema).Data(state, diff)
	if err != nil {
		t.Fatalf("building resource data: %v", err)
	}

	if err := ScopeTypeUpdate(d, newTestClient(server)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := map[string]any{"description": "new description"}
	if !reflect.DeepEqual(patched, want) {
		t.Errorf("partial update body = %v, want %v", patched, want)
	}
}

// Create still has to send every field, so omitempty must not drop values the
// operator set.
func TestScopeTypeCreate_SendsEveryField(t *testing.T) {
	var created map[string]any
	server := scopeTypeServer(t, http.MethodPost, &created)
	defer server.Close()

	d := schema.TestResourceDataRaw(t, resourceScopeType().Schema, map[string]any{
		"nrn": "organization=1:account=2", "type": "custom",
		"name": "Containers", "description": "Docker containers on pods",
		"provider_type": "service", "provider_id": "spec-1",
	})

	if err := ScopeTypeCreate(d, newTestClient(server)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := map[string]any{
		"nrn": "organization=1:account=2", "type": "custom",
		"name": "Containers", "status": "active",
		"description":   "Docker containers on pods",
		"provider_type": "service", "provider_id": "spec-1",
	}
	if !reflect.DeepEqual(created, want) {
		t.Errorf("create body = %v, want %v", created, want)
	}
}
