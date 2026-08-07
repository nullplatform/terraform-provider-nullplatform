package nullplatform

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// rawConfigFor builds the cty configuration object Terraform core would send:
// every attribute of the resource present, null unless the test set it. The SDK
// exposes no setter for it, so tests attach it to the diff by hand.
func rawConfigFor(t *testing.T, resource *schema.Resource, config map[string]any) cty.Value {
	t.Helper()
	implied := schema.InternalMap(resource.Schema).CoreConfigSchema().ImpliedType()
	values := map[string]cty.Value{}
	for name, attrType := range implied.AttributeTypes() {
		raw, set := config[name]
		if !set {
			values[name] = cty.NullVal(attrType)
			continue
		}
		switch typed := raw.(type) {
		case bool:
			values[name] = cty.BoolVal(typed)
		case string:
			values[name] = cty.StringVal(typed)
		default:
			values[name] = cty.NullVal(attrType)
		}
	}
	return cty.ObjectVal(values)
}

func specificationData(t *testing.T, resource *schema.Resource, config map[string]any) *schema.ResourceData {
	t.Helper()
	diff, err := resource.Diff(context.Background(), nil, terraform.NewResourceConfigRaw(config), nil)
	if err != nil {
		t.Fatalf("computing the diff: %v", err)
	}
	if diff == nil {
		diff = &terraform.InstanceDiff{}
	}
	diff.RawConfig = rawConfigFor(t, resource, config)

	d, err := schema.InternalMap(resource.Schema).Data(nil, diff)
	if err != nil {
		t.Fatalf("building resource data: %v", err)
	}
	return d
}

// The regression that motivated this change: the provider declared
// `Default: false` while the API defaults the flag to `true`, so a configuration
// that never mentioned the attribute planned `true -> false` on every plan — and
// the apply could not settle it, because `omitempty` dropped the false from the
// request body.
func TestSpecificationFlags_OmittedFlagsPlanClean(t *testing.T) {
	for name, resource := range map[string]*schema.Resource{
		"service": resourceServiceSpecification(),
		"link":    resourceLinkSpecification(),
	} {
		t.Run(name, func(t *testing.T) {
			state := &terraform.InstanceState{
				ID: "spec-1",
				Attributes: map[string]string{
					"id":                  "spec-1",
					"name":                "redis",
					"use_default_actions": "true",
					"use_default_naming":  "true",
					"use_managed_actions": "false",
				},
			}

			diff, err := resource.Diff(context.Background(), state,
				terraform.NewResourceConfigRaw(map[string]any{"name": "redis"}), nil)
			if err != nil {
				t.Fatalf("computing the diff: %v", err)
			}
			if diff == nil || diff.Empty() {
				return
			}
			for _, key := range []string{"use_default_actions", "use_default_naming", "use_managed_actions"} {
				if attr, planned := diff.Attributes[key]; planned {
					t.Errorf("planned %s %q -> %q for a configuration that never mentions it", key, attr.Old, attr.New)
				}
			}
		})
	}
}

// The other half: an explicit value has to reach the wire. `false` was the one
// that could not, which is what made `use_default_actions = false` impossible to
// express through Terraform.
func TestSpecificationFlags_ExplicitValuesAreTransmitted(t *testing.T) {
	for _, tc := range []struct {
		name   string
		config map[string]any
		want   map[string]any
	}{
		{
			name:   "explicit false is sent, not dropped by omitempty",
			config: map[string]any{"name": "redis", "use_default_actions": false},
			want:   map[string]any{"use_default_actions": false},
		},
		{
			name:   "managed opt-in is sent",
			config: map[string]any{"name": "redis", "use_default_actions": true, "use_managed_actions": true},
			want:   map[string]any{"use_default_actions": true, "use_managed_actions": true},
		},
		{
			name:   "naming opt-out is sent",
			config: map[string]any{"name": "redis", "use_default_naming": false},
			want:   map[string]any{"use_default_naming": false},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := createdSpecificationBody(t, tc.config)
			for key, want := range tc.want {
				got, present := body[key]
				if !present {
					t.Errorf("%s never reached the request body: %v", key, body)
					continue
				}
				if got != want {
					t.Errorf("%s = %v, want %v", key, got, want)
				}
			}
		})
	}
}

// A flag the configuration does not mention stays off the request entirely, so
// the API applies its own default instead of the provider guessing at it.
func TestSpecificationFlags_UnsetFlagsAreNotSent(t *testing.T) {
	body := createdSpecificationBody(t, map[string]any{"name": "redis"})

	for _, key := range []string{"use_default_actions", "use_default_naming", "use_managed_actions"} {
		if _, present := body[key]; present {
			t.Errorf("%s was sent although the configuration never set it: %v", key, body)
		}
	}
}

func createdSpecificationBody(t *testing.T, config map[string]any) map[string]any {
	t.Helper()
	var body map[string]any
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "spec-1", "name": "redis"})
	}))
	defer server.Close()

	resource := resourceServiceSpecification()
	d := specificationData(t, resource, config)

	if diags := CreateServiceSpecification(context.Background(), d, newTestClient(server)); diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	return body
}

// The API rejects managed CRUD on a specification that authors its own actions.
// Mirroring it turns a mid-apply 400 into a plan-time error — and, like the
// API's own guard, it fires only when BOTH flags are explicitly configured: the
// two defaults (managed false, defaults true) cannot combine into a violation,
// so an absent flag must not be assumed either way.
func TestManagedRequiresDefaultActions(t *testing.T) {
	for _, tc := range []struct {
		name      string
		managed   *bool
		defaults  *bool
		wantError bool
	}{
		{name: "managed on, defaults off", managed: boolPtr(true), defaults: boolPtr(false), wantError: true},
		{name: "managed on, defaults on", managed: boolPtr(true), defaults: boolPtr(true)},
		{name: "managed on, defaults unset", managed: boolPtr(true), defaults: nil},
		{name: "managed unset, defaults off", managed: nil, defaults: boolPtr(false)},
		{name: "managed off, defaults off", managed: boolPtr(false), defaults: boolPtr(false)},
		{name: "both unset", managed: nil, defaults: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := managedRequiresDefaultActions(tc.managed, tc.defaults)
			if tc.wantError && err == nil {
				t.Error("expected the combination to be refused")
			}
			if !tc.wantError && err != nil {
				t.Errorf("unexpected refusal: %v", err)
			}
		})
	}
}

// Both specification resources wire the guard into their plan.
func TestSpecificationResources_WireTheManagedGuard(t *testing.T) {
	for name, resource := range map[string]*schema.Resource{
		"service": resourceServiceSpecification(),
		"link":    resourceLinkSpecification(),
	} {
		if resource.CustomizeDiff == nil {
			t.Errorf("%s specification does not validate the flag combination at plan time", name)
		}
	}
}

func TestOptionalBoolFromRaw_DistinguishesUnsetFromFalse(t *testing.T) {
	object := func(value cty.Value) cty.Value {
		return cty.ObjectVal(map[string]cty.Value{"use_default_actions": value})
	}

	for _, tc := range []struct {
		name string
		raw  cty.Value
		want *bool
	}{
		{name: "explicit false", raw: object(cty.False), want: boolPtr(false)},
		{name: "explicit true", raw: object(cty.True), want: boolPtr(true)},
		{name: "null attribute", raw: object(cty.NullVal(cty.Bool)), want: nil},
		{name: "unknown attribute", raw: object(cty.UnknownVal(cty.Bool)), want: nil},
		{name: "null object (import, no config)", raw: cty.NullVal(cty.EmptyObject), want: nil},
		{name: "attribute absent from the object", raw: cty.EmptyObjectVal, want: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := optionalBoolFromRaw(tc.raw, "use_default_actions")
			switch {
			case tc.want == nil && got != nil:
				t.Errorf("got %v, want nil (unset)", *got)
			case tc.want != nil && got == nil:
				t.Errorf("got nil, want %v", *tc.want)
			case tc.want != nil && *got != *tc.want:
				t.Errorf("got %v, want %v", *got, *tc.want)
			}
		})
	}
}

func boolPtr(b bool) *bool { return &b }

// A specification response without `selectors` used to panic both resource
// reads — a plugin crash, not an error. The data source already guarded it.
func TestSpecificationRead_SurvivesAResponseWithoutSelectors(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "spec-1", "name": "redis"})
	}))
	defer server.Close()

	serviceData := schema.TestResourceDataRaw(t, resourceServiceSpecification().Schema, map[string]any{"name": "redis"})
	serviceData.SetId("spec-1")
	if diags := ReadServiceSpecification(context.Background(), serviceData, newTestClient(server)); diags.HasError() {
		t.Errorf("service specification read: %v", diags)
	}

	linkData := schema.TestResourceDataRaw(t, resourceLinkSpecification().Schema, map[string]any{"name": "redis"})
	linkData.SetId("spec-1")
	if diags := ReadLinkSpecification(context.Background(), linkData, newTestClient(server)); diags.HasError() {
		t.Errorf("link specification read: %v", diags)
	}
}

// A full-bodied read: all three flags and the selectors block land in state.
// The no-selectors test above proves the guard; this one proves the values —
// including the data source, whose read is its own code path.
func TestSpecificationRead_RecordsFlagsAndSelectors(t *testing.T) {
	managed := true
	defaults := true
	naming := false
	body := map[string]any{
		"id": "spec-1", "name": "redis",
		"use_default_actions": defaults, "use_default_naming": naming, "use_managed_actions": managed,
		"selectors": map[string]any{"category": "Databases", "imported": false, "provider": "AWS", "sub_category": "Key Value"},
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(body)
	}))
	defer server.Close()

	serviceData := schema.TestResourceDataRaw(t, resourceServiceSpecification().Schema, map[string]any{"name": "redis"})
	serviceData.SetId("spec-1")
	if diags := ReadServiceSpecification(context.Background(), serviceData, newTestClient(server)); diags.HasError() {
		t.Fatalf("service specification read: %v", diags)
	}
	if !serviceData.Get("use_managed_actions").(bool) || serviceData.Get("use_default_naming").(bool) {
		t.Errorf("flags did not land: managed=%v naming=%v",
			serviceData.Get("use_managed_actions"), serviceData.Get("use_default_naming"))
	}
	selectors := serviceData.Get("selectors").([]interface{})
	if len(selectors) != 1 || selectors[0].(map[string]interface{})["category"] != "Databases" {
		t.Errorf("selectors did not land: %v", selectors)
	}

	linkData := schema.TestResourceDataRaw(t, resourceLinkSpecification().Schema, map[string]any{"name": "redis"})
	linkData.SetId("spec-1")
	if diags := ReadLinkSpecification(context.Background(), linkData, newTestClient(server)); diags.HasError() {
		t.Fatalf("link specification read: %v", diags)
	}
	if !linkData.Get("use_managed_actions").(bool) {
		t.Error("link specification did not record use_managed_actions")
	}
	linkSelectors := linkData.Get("selectors").([]interface{})
	if len(linkSelectors) != 1 {
		t.Errorf("link selectors did not land: %v", linkSelectors)
	}

	dsData := schema.TestResourceDataRaw(t, dataSourceServiceSpecification().Schema, map[string]any{"id": "spec-1"})
	if diags := dataSourceServiceSpecificationRead(context.Background(), dsData, newTestClient(server)); diags.HasError() {
		t.Fatalf("data source read: %v", diags)
	}
	if !dsData.Get("use_managed_actions").(bool) {
		t.Error("data source did not record use_managed_actions")
	}
}

// Updates carry every configured flag — including the managed one — through
// the same always-send rule #146 established, on BOTH specification resources.
func TestSpecificationUpdate_ConfiguredFlagsTravel(t *testing.T) {
	for name, tc := range map[string]struct {
		resource *schema.Resource
		update   func(context.Context, *schema.ResourceData, any) diag.Diagnostics
	}{
		"service": {resourceServiceSpecification(), UpdateServiceSpecification},
		"link":    {resourceLinkSpecification(), UpdateLinkSpecification},
	} {
		t.Run(name, func(t *testing.T) {
			var patched map[string]any
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "PATCH" {
					_ = json.NewDecoder(r.Body).Decode(&patched)
				}
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]any{"id": "spec-1", "name": "redis"})
			}))
			defer server.Close()

			config := map[string]any{"name": "redis", "use_default_actions": true, "use_managed_actions": true}
			if name == "link" {
				config["specification_id"] = "svc-spec-1"
			}
			d := specificationData(t, tc.resource, config)
			d.SetId("spec-1")

			if diags := tc.update(context.Background(), d, newTestClient(server)); diags.HasError() {
				t.Fatalf("unexpected error: %v", diags)
			}
			if patched["use_managed_actions"] != true {
				t.Errorf("use_managed_actions did not travel on update: %v", patched)
			}
		})
	}
}

// The link specification CREATE sends the managed flag too — its payload is
// built by different code than the service one.
func TestLinkSpecificationCreate_ManagedFlagTravels(t *testing.T) {
	var posted map[string]any
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			_ = json.NewDecoder(r.Body).Decode(&posted)
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "lspec-1", "name": "redis"})
	}))
	defer server.Close()

	d := specificationData(t, resourceLinkSpecification(), map[string]any{
		"name": "redis", "specification_id": "svc-spec-1",
		"use_default_actions": true, "use_managed_actions": true,
	})
	if diags := CreateLinkSpecification(context.Background(), d, newTestClient(server)); diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if posted["use_managed_actions"] != true {
		t.Errorf("use_managed_actions did not travel on create: %v", posted)
	}
}
