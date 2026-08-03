package nullplatform

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// The API's action type enum gained `archive` and `unarchive`; a specification
// opts into a managed archive by declaring one, so the provider must accept them.
func TestActionSpecification_TypeAcceptsEveryServerSideActionType(t *testing.T) {
	validate := resourceActionSpecification().Schema["type"].ValidateFunc

	for _, actionType := range []string{"custom", "create", "update", "delete", "archive", "unarchive", "diagnose"} {
		t.Run(actionType, func(t *testing.T) {
			_, errs := validate(actionType, "type")
			if len(errs) != 0 {
				t.Errorf("type %q must be accepted, got %v", actionType, errs)
			}
		})
	}
}

func TestActionSpecification_TypeRejectsAnythingElse(t *testing.T) {
	validate := resourceActionSpecification().Schema["type"].ValidateFunc

	for _, actionType := range []string{"unarchived", "archiving", "restore", ""} {
		t.Run(actionType, func(t *testing.T) {
			_, errs := validate(actionType, "type")
			if len(errs) == 0 {
				t.Errorf("type %q must be rejected", actionType)
			}
		})
	}
}

// An archive/unarchive action on a specification with `use_default_actions` is
// platform-generated: sending `parameters` or `results` is refused with a 400,
// so the attributes cannot stay Required. Computed lets the generated content
// land in state instead of diffing against the configuration forever.
func TestActionSpecification_ParametersAndResultsAreOptionalAndComputed(t *testing.T) {
	s := resourceActionSpecification().Schema

	for _, name := range []string{"parameters", "results"} {
		t.Run(name, func(t *testing.T) {
			if s[name].Required {
				t.Errorf("%q must not be Required: an archive opt-in cannot send it", name)
			}
			if !s[name].Optional {
				t.Errorf("%q must stay Optional so existing configurations keep setting it", name)
			}
			if !s[name].Computed {
				t.Errorf("%q must be Computed so the generated content lands without a diff", name)
			}
		})
	}
}

// The distinction that matters is on the wire: an omitted attribute must leave
// the field off the request body, not send an empty object.
func TestActionSpecificationCreate_OmittedParametersAreNotSent(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(ActionSpecification{Id: "as-1", Type: "archive"})
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(ActionSpecification{
			Id:                     "as-1",
			Name:                   "Archive",
			Type:                   "archive",
			ServiceSpecificationId: "spec-1",
			Parameters:             map[string]any{"schema": map[string]any{"type": "object"}},
			Results:                map[string]any{"schema": map[string]any{}},
		})
	}))
	defer server.Close()

	d := schema.TestResourceDataRaw(t, resourceActionSpecification().Schema, map[string]any{
		"name":                     "Archive",
		"type":                     "archive",
		"service_specification_id": "spec-1",
	})

	if diags := ActionSpecificationCreate(context.Background(), d, newTestClient(server)); diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if _, sent := gotBody["parameters"]; sent {
		t.Errorf("POST body carried `parameters` %v; an archive opt-in must not send it", gotBody["parameters"])
	}
	if _, sent := gotBody["results"]; sent {
		t.Errorf("POST body carried `results` %v; an archive opt-in must not send it", gotBody["results"])
	}
	// The read-back stores what the platform generated, so the next plan is clean.
	if d.Get("parameters").(string) == "" {
		t.Error("state must keep the platform-generated parameters")
	}
}

// A configuration that does set them keeps behaving exactly as before.
func TestActionSpecificationCreate_ConfiguredParametersAreStillSent(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(ActionSpecification{
			Id:                     "as-1",
			Name:                   "Restart",
			Type:                   "custom",
			ServiceSpecificationId: "spec-1",
			Parameters:             map[string]any{"schema": map[string]any{"type": "object"}},
			Results:                map[string]any{"schema": map[string]any{}},
		})
	}))
	defer server.Close()

	d := schema.TestResourceDataRaw(t, resourceActionSpecification().Schema, map[string]any{
		"name":                     "Restart",
		"type":                     "custom",
		"service_specification_id": "spec-1",
		"parameters":               `{"schema":{"type":"object"},"values":{}}`,
		"results":                  `{"schema":{}}`,
	})

	if diags := ActionSpecificationCreate(context.Background(), d, newTestClient(server)); diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if _, sent := gotBody["parameters"]; !sent {
		t.Error("POST body must carry the configured `parameters`")
	}
	if _, sent := gotBody["results"]; !sent {
		t.Error("POST body must carry the configured `results`")
	}
}

func TestDecodeOptionalJSONObject(t *testing.T) {
	got, err := decodeOptionalJSONObject("")
	if err != nil {
		t.Fatalf("an empty attribute is not an error, got %v", err)
	}
	if got != nil {
		t.Errorf("an empty attribute must decode to nil so `omitempty` drops it, got %v", got)
	}

	got, err = decodeOptionalJSONObject(`{"schema":{}}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := got["schema"]; !ok {
		t.Errorf("expected the decoded object to carry `schema`, got %v", got)
	}

	if _, err := decodeOptionalJSONObject("not json"); err == nil {
		t.Error("malformed JSON must still be an error")
	}
}
