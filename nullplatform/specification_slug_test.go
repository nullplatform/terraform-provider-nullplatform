package nullplatform

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// The slug a specification is created with. The API derives it from the name
// unless the request gives one, and a given one must be exactly what the
// derivation could produce; it never changes afterwards. A publisher gives one
// to keep a namespaced identifier ("np-aws-lambda") under a plain name ("AWS
// Lambda"), so the specification can live next to the copy an organization
// already runs under the derived slug.

func specificationCreateBody(t *testing.T, resource *schema.Resource, create func(context.Context, *schema.ResourceData, any) diag.Diagnostics, config map[string]any) map[string]any {
	t.Helper()
	var posted map[string]any
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			_ = json.NewDecoder(r.Body).Decode(&posted)
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "spec-1", "name": config["name"], "slug": posted["slug"]})
	}))
	defer server.Close()

	d := specificationData(t, resource, config)
	if diags := create(context.Background(), d, newTestClient(server)); diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	return posted
}

func TestSpecificationSlug_GivenSlugTravelsOnCreate(t *testing.T) {
	for name, tc := range map[string]struct {
		resource *schema.Resource
		create   func(context.Context, *schema.ResourceData, any) diag.Diagnostics
		config   map[string]any
	}{
		"service": {
			resource: resourceServiceSpecification(),
			create: func(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
				return CreateServiceSpecification(ctx, d, m)
			},
			config: map[string]any{"name": "AWS Lambda", "slug": "np-aws-lambda"},
		},
		"link": {
			resource: resourceLinkSpecification(),
			create: func(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
				return CreateLinkSpecification(ctx, d, m)
			},
			config: map[string]any{"name": "Bucket access", "slug": "np-bucket-access", "specification_id": "sspec-1"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			body := specificationCreateBody(t, tc.resource, tc.create, tc.config)
			if got := body["slug"]; got != tc.config["slug"] {
				t.Errorf("slug sent = %v, want %v (body %v)", got, tc.config["slug"], body)
			}
		})
	}
}

// Omitted, the slug stays off the request entirely, so the API derives it from
// the name as it always did: existing configurations send byte-identical bodies.
func TestSpecificationSlug_OmittedSlugIsNotSent(t *testing.T) {
	body := specificationCreateBody(t, resourceServiceSpecification(),
		func(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
			return CreateServiceSpecification(ctx, d, m)
		},
		map[string]any{"name": "AWS Lambda"})
	if _, sent := body["slug"]; sent {
		t.Errorf("a configuration without slug sent one: %v", body)
	}
}

// A slug the API would refuse is refused at plan, with the API's own rule.
func TestSpecificationSlug_NonConformingSlugFailsAtPlan(t *testing.T) {
	for _, slug := range []string{"AWS_Lambda", "aws--lambda", "-aws-lambda", "aws lambda", ""} {
		t.Run(slug, func(t *testing.T) {
			for name, tc := range map[string]struct {
				resource *schema.Resource
				config   map[string]any
			}{
				"service": {resourceServiceSpecification(), map[string]any{"name": "AWS Lambda", "slug": slug, "visible_to": []any{"organization=1"}}},
				"link":    {resourceLinkSpecification(), map[string]any{"name": "AWS Lambda", "slug": slug, "specification_id": "sspec-1", "unique": false}},
			} {
				resource, config := tc.resource, tc.config
				diags := resource.Validate(terraform.NewResourceConfigRaw(config))
				if !diags.HasError() {
					t.Errorf("%s: slug %q passed validation", name, slug)
					continue
				}
				if !strings.Contains(diags[0].Summary+diags[0].Detail, "lowercase") {
					t.Errorf("%s: the refusal does not say what a slug is: %v", name, diags)
				}
			}
		})
	}
	if diags := resourceServiceSpecification().Validate(terraform.NewResourceConfigRaw(map[string]any{"name": "AWS Lambda", "slug": "np-aws-lambda", "visible_to": []any{"organization=1"}})); diags.HasError() {
		t.Errorf("a conforming slug was refused: %v", diags)
	}
}

// The slug never changes on the platform, so changing it in the configuration
// is a new specification, planned as a replacement rather than an update the
// API would silently ignore.
func TestSpecificationSlug_ChangingTheSlugReplaces(t *testing.T) {
	state := &terraform.InstanceState{
		ID:         "spec-1",
		Attributes: map[string]string{"id": "spec-1", "name": "AWS Lambda", "slug": "aws-lambda"},
	}
	diff, err := resourceServiceSpecification().Diff(context.Background(), state,
		terraform.NewResourceConfigRaw(map[string]any{"name": "AWS Lambda", "slug": "np-aws-lambda"}), nil)
	if err != nil {
		t.Fatalf("computing the diff: %v", err)
	}
	if diff == nil || !diff.RequiresNew() {
		t.Errorf("a changed slug must plan a replacement, got %v", diff)
	}

	// Leaving the slug out of a configuration whose state has one derived is not a change.
	same, err := resourceServiceSpecification().Diff(context.Background(), state,
		terraform.NewResourceConfigRaw(map[string]any{"name": "AWS Lambda"}), nil)
	if err != nil {
		t.Fatalf("computing the diff: %v", err)
	}
	if same != nil {
		if attr, planned := same.Attributes["slug"]; planned && attr.New != "aws-lambda" {
			t.Errorf("an omitted slug planned %q -> %q", attr.Old, attr.New)
		}
	}
}
