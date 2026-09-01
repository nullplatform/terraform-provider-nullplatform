package nullplatform

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
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
			Name: "Containers", Status: "active", Description: "old description",
			ProviderType: "service", ProviderId: "spec-1",
		})
	}))
}

func scopeTypeState() map[string]string {
	return map[string]string{
		"id": "1", "nrn": "organization=1:account=2", "type": "custom",
		"name": "Containers", "status": "active", "description": "old description",
		"provider_type": "service", "provider_id": "spec-1",
	}
}

func scopeTypeConfig(name, description string) map[string]any {
	return map[string]any{
		"nrn": "organization=1:account=2", "type": "custom",
		"name": name, "description": description,
		"provider_type": "service", "provider_id": "spec-1",
	}
}

// updateResourceData builds the ResourceData Terraform itself would hand to
// Update, so d.HasChange reports a real state-versus-config diff instead of a
// flag the test set by hand.
func updateResourceData(t *testing.T, config map[string]any) *schema.ResourceData {
	t.Helper()
	state := &terraform.InstanceState{ID: "1", Attributes: scopeTypeState()}
	diff, err := resourceScopeType().Diff(context.Background(), state, terraform.NewResourceConfigRaw(config), nil)
	if err != nil {
		t.Fatalf("computing the diff: %v", err)
	}
	d, err := schema.InternalMap(resourceScopeType().Schema).Data(state, diff)
	if err != nil {
		t.Fatalf("building resource data: %v", err)
	}
	return d
}

// A partial update must carry only the fields that changed. Before ScopeType's
// fields were tagged omitempty, every update serialized the fields Update never
// assigns — type, status, provider_id and provider_type — as empty strings, and
// one that left the name alone sent `"name": ""`, which the API rejects with
// `body/name must NOT have fewer than 1 characters`.
func TestScopeTypeUpdate_SendsOnlyChangedFields(t *testing.T) {
	for _, tc := range []struct {
		name   string
		config map[string]any
		want   map[string]any
	}{
		{
			name:   "description only",
			config: scopeTypeConfig("Containers", "new description"),
			want:   map[string]any{"description": "new description"},
		},
		{
			// name is the field the API refused, and the description-only case
			// never reaches its branch in ScopeTypeUpdate.
			name:   "name only",
			config: scopeTypeConfig("Pods", "old description"),
			want:   map[string]any{"name": "Pods"},
		},
		{
			name:   "both",
			config: scopeTypeConfig("Pods", "new description"),
			want:   map[string]any{"name": "Pods", "description": "new description"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var patched map[string]any
			server := scopeTypeServer(t, http.MethodPut, &patched)
			defer server.Close()

			if err := ScopeTypeUpdate(updateResourceData(t, tc.config), newTestClient(server)); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(patched, tc.want) {
				t.Errorf("partial update body = %v, want %v", patched, tc.want)
			}
		})
	}
}

// The API's refusal is contract: the provider has to surface the response body
// verbatim, because the message is the only thing naming the guard that
// refused.
func TestScopeTypeUpdate_SurfacesAPIRefusal(t *testing.T) {
	const refusal = `{"message":"body/name must NOT have fewer than 1 characters"}`
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, refusal)
	}))
	defer server.Close()

	err := ScopeTypeUpdate(updateResourceData(t, scopeTypeConfig("Pods", "old description")), newTestClient(server))
	if err == nil {
		t.Fatal("an API refusal must fail the update")
	}
	if !strings.Contains(err.Error(), refusal) {
		t.Errorf("error = %q, want it to carry the API response %q", err, refusal)
	}
}

// omitempty cannot tell "unchanged" from "deliberately emptied", so an update
// clearing name or description would be dropped from the body: the API keeps
// the old value, Read writes it back, and the plan never converges. Rejecting
// empty at plan time is what keeps that state unreachable.
func TestScopeType_RejectsEmptyRequiredStrings(t *testing.T) {
	for _, field := range []string{"name", "description"} {
		t.Run(field, func(t *testing.T) {
			validate := resourceScopeType().Schema[field].ValidateFunc
			if validate == nil {
				t.Fatalf("%q must validate, or omitempty can swallow a deliberate clear", field)
			}
			if _, errs := validate("", field); len(errs) == 0 {
				t.Errorf("%q must reject the empty string", field)
			}
		})
	}
}

// Create still has to send every field, so omitempty must not drop values the
// operator set.
func TestScopeTypeCreate_SendsEveryField(t *testing.T) {
	var created map[string]any
	server := scopeTypeServer(t, http.MethodPost, &created)
	defer server.Close()

	d := schema.TestResourceDataRaw(t, resourceScopeType().Schema,
		scopeTypeConfig("Containers", "Docker containers on pods"))

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
