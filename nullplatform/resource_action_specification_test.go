package nullplatform

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
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

// A specification created with `use_default_actions` already owns a generated
// `archive` action, so declaring one is refused. The bare API message does not
// say what to do about it; the provider names the import.
func TestAnnotateExistingActionTypeError(t *testing.T) {
	spec := &ActionSpecification{Type: "archive", ServiceSpecificationId: "spec-1"}
	err := annotateExistingActionTypeError(
		errors.New(`error creating action specification, got 400: {"message":"There is already an action of type \"archive\" for service specification spec-1"}`),
		spec,
	)

	for _, want := range []string{
		"already an action of type", // the API's own message survives
		"terraform import",          // and the way out is named
		"spec-1",                    // pointed at the right specification
		"`unarchive` is never generated",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("annotated error is missing %q:\n%s", want, err.Error())
		}
	}
}

// Every other failure is passed through untouched — the annotation must not
// turn unrelated errors into archive advice.
func TestAnnotateExistingActionTypeError_LeavesOtherErrorsAlone(t *testing.T) {
	original := errors.New("error creating action specification, got 500")
	if got := annotateExistingActionTypeError(original, &ActionSpecification{Type: "custom"}); got != original {
		t.Errorf("unrelated error was rewritten: %v", got)
	}
	if got := annotateExistingActionTypeError(nil, &ActionSpecification{Type: "custom"}); got != nil {
		t.Errorf("nil error became %v", got)
	}
}

// The annotation resolves the parent from whichever specification id is set —
// a LINK-owned action must point the import at the link specification.
func TestAnnotateExistingActionTypeError_LinkParent(t *testing.T) {
	err := annotateExistingActionTypeError(
		errors.New(`got 400: {"message":"There is already an action of type \"archive\" for link specification lspec-1"}`),
		&ActionSpecification{Type: "archive", LinkSpecificationId: "lspec-1"},
	)
	if !strings.Contains(err.Error(), "lspec-1") {
		t.Errorf("annotated error should name the link specification:\n%s", err.Error())
	}
}

// The annotation is wired into the CREATE path — the duplicate refusal a real
// apply hits must come back with the import command, not bare.
func TestActionSpecificationCreate_DuplicateTypeNamesTheImport(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"There is already an action of type \"archive\" for service specification spec-1"}`))
	}))
	defer server.Close()

	d := schema.TestResourceDataRaw(t, resourceActionSpecification().Schema, map[string]any{
		"name": "Archive", "type": "archive", "service_specification_id": "spec-1",
	})
	diags := ActionSpecificationCreate(context.Background(), d, newTestClient(server))
	if !diags.HasError() {
		t.Fatal("the duplicate must be refused")
	}
	if !strings.Contains(diags[0].Summary, "terraform import") {
		t.Errorf("diagnostic %q should name the import", diags[0].Summary)
	}
}

// Update decodes parameters/results through the same optional-JSON contract as
// create: changed valid JSON travels, changed INVALID JSON is refused before
// any request leaves the provider.
func TestActionSpecificationUpdate_DecodesChangedParameters(t *testing.T) {
	var patched map[string]any
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PATCH" {
			_ = json.NewDecoder(r.Body).Decode(&patched)
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(ActionSpecification{
			Id: "as-1", Type: "custom", ServiceSpecificationId: "spec-1",
			Parameters: map[string]any{"schema": map[string]any{"type": "object"}},
			Results:    map[string]any{"schema": map[string]any{}},
		})
	}))
	defer server.Close()

	state := &terraform.InstanceState{ID: "as-1", Attributes: map[string]string{
		"id": "as-1", "name": "verb", "type": "custom",
		"service_specification_id": "spec-1",
		"parameters":               `{"schema":{"type":"object"}}`,
		"results":                  `{"schema":{}}`,
	}}
	diff, err := resourceActionSpecification().Diff(context.Background(), state, terraform.NewResourceConfigRaw(map[string]any{
		"name": "verb", "type": "custom", "service_specification_id": "spec-1",
		"parameters": `{"schema":{"type":"object","properties":{"x":{"type":"string"}}}}`,
		"results":    `{"schema":{"type":"object"}}`,
	}), nil)
	if err != nil {
		t.Fatalf("computing the diff: %v", err)
	}
	d, err := schema.InternalMap(resourceActionSpecification().Schema).Data(state, diff)
	if err != nil {
		t.Fatalf("building resource data: %v", err)
	}
	if diags := ActionSpecificationUpdate(context.Background(), d, newTestClient(server)); diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if patched["parameters"] == nil || patched["results"] == nil {
		t.Errorf("changed parameters/results must travel, got %v", patched)
	}
}

func TestActionSpecificationUpdate_RefusesInvalidJSON(t *testing.T) {
	for field, config := range map[string]map[string]any{
		"parameters": {"parameters": `{not json`, "results": `{"schema":{}}`},
		"results":    {"parameters": `{"schema":{}}`, "results": `{not json`},
	} {
		t.Run(field, func(t *testing.T) {
			state := &terraform.InstanceState{ID: "as-1", Attributes: map[string]string{
				"id": "as-1", "name": "verb", "type": "custom",
				"service_specification_id": "spec-1",
				"parameters":               `{"schema":{}}`,
				"results":                  `{"schema":{}}`,
			}}
			full := map[string]any{"name": "verb", "type": "custom", "service_specification_id": "spec-1"}
			for k, v := range config {
				full[k] = v
			}
			diff, err := resourceActionSpecification().Diff(context.Background(), state, terraform.NewResourceConfigRaw(full), nil)
			if err != nil {
				t.Fatalf("computing the diff: %v", err)
			}
			d, err := schema.InternalMap(resourceActionSpecification().Schema).Data(state, diff)
			if err != nil {
				t.Fatalf("building resource data: %v", err)
			}
			// The refusal must fire before any request leaves the provider.
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Errorf("unexpected %s %s: invalid JSON must be refused locally", r.Method, r.URL.Path)
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer server.Close()
			diags := ActionSpecificationUpdate(context.Background(), d, newTestClient(server))
			if !diags.HasError() {
				t.Fatalf("invalid %s JSON must be refused before any request", field)
			}
		})
	}
}

// The API answers `"annotations": {}` for a spec that never had any; writing
// "{}" into state against an unset attribute was a permanent phantom diff.
// Read must keep the attribute empty in that case.
func TestActionSpecificationRead_EmptyAnnotationsStayAbsent(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		empty := map[string]interface{}{}
		_ = json.NewEncoder(w).Encode(ActionSpecification{
			Id:                     "as-1",
			Name:                   "Deploy",
			Type:                   "custom",
			ServiceSpecificationId: "spec-1",
			Annotations:            &empty,
		})
	}))
	defer server.Close()

	d := schema.TestResourceDataRaw(t, resourceActionSpecification().Schema, map[string]any{
		"name": "Deploy", "type": "custom", "service_specification_id": "spec-1",
	})
	d.SetId("as-1")

	if diags := ActionSpecificationRead(context.Background(), d, newTestClient(server)); diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if got := d.Get("annotations").(string); got != "" {
		t.Errorf("annotations = %q, want empty — an empty API object must not reach state", got)
	}
}

// Populated annotations still round-trip into state.
func TestActionSpecificationRead_PopulatedAnnotationsLand(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		anns := map[string]interface{}{"np.ui/group": "ops"}
		_ = json.NewEncoder(w).Encode(ActionSpecification{
			Id: "as-1", Name: "Deploy", Type: "custom", ServiceSpecificationId: "spec-1",
			Annotations: &anns,
		})
	}))
	defer server.Close()

	d := schema.TestResourceDataRaw(t, resourceActionSpecification().Schema, map[string]any{
		"name": "Deploy", "type": "custom", "service_specification_id": "spec-1",
	})
	d.SetId("as-1")

	if diags := ActionSpecificationRead(context.Background(), d, newTestClient(server)); diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if got := d.Get("annotations").(string); !strings.Contains(got, "np.ui/group") {
		t.Errorf("annotations = %q, want the populated blob in state", got)
	}
}

// Removing the attribute from the configuration must actually clear the
// stored annotations: the PATCH body carries an explicit empty object (a nil
// pointer would be dropped by omitempty and the removal would never land).
func TestActionSpecificationUpdate_RemovedAnnotationsSendEmptyObject(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PATCH" {
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(ActionSpecification{Id: "as-1", Name: "Deploy", Type: "custom", ServiceSpecificationId: "spec-1"})
	}))
	defer server.Close()

	// Simulate "had annotations, config removed them": old state non-empty,
	// diff to empty — HasChange fires, GetOk sees nothing.
	d := schema.TestResourceDataRaw(t, resourceActionSpecification().Schema, map[string]any{
		"name": "Deploy", "type": "custom", "service_specification_id": "spec-1",
	})
	d.SetId("as-1")
	if err := d.Set("annotations", `{"np.ui/group":"ops"}`); err != nil {
		t.Fatal(err)
	}
	raw := d.State()
	raw.Attributes["annotations"] = `{"np.ui/group":"ops"}`
	diff := &terraform.InstanceDiff{Attributes: map[string]*terraform.ResourceAttrDiff{
		"annotations": {Old: `{"np.ui/group":"ops"}`, New: ""},
	}}
	dd, err := schema.InternalMap(resourceActionSpecification().Schema).Data(raw, diff)
	if err != nil {
		t.Fatal(err)
	}
	dd.SetId("as-1")

	if diags := ActionSpecificationUpdate(context.Background(), dd, newTestClient(server)); diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	anns, sent := gotBody["annotations"]
	if !sent {
		t.Fatal("PATCH body must carry annotations to clear them (omitempty would drop a nil map)")
	}
	if m, ok := anns.(map[string]any); !ok || len(m) != 0 {
		t.Errorf("annotations = %v, want an explicit empty object", anns)
	}
}

// The extended suppressor: unset config vs the API's empty object never diffs;
// real values still compare by JSON equality.
func TestSuppressEmptyOrEquivalentJSON(t *testing.T) {
	cases := []struct {
		old, new string
		want     bool
	}{
		{"", "", true},
		{"{}", "", true},
		{"", "{}", true},
		{"null", "", true},
		{`{"a":1}`, "", false},
		{"", `{"a":1}`, false},
		{`{"a":1}`, `{"a": 1}`, true},
		{`{"a":1}`, `{"a":2}`, false},
	}
	for _, c := range cases {
		if got := suppressEmptyOrEquivalentJSON("annotations", c.old, c.new, nil); got != c.want {
			t.Errorf("suppress(%q, %q) = %v, want %v", c.old, c.new, got, c.want)
		}
	}
}
