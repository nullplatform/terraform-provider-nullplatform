package nullplatform

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// errorsOf flattens error diagnostics into one error, for message assertions.
func errorsOf(diags diag.Diagnostics) error {
	var parts []string
	for _, d := range diags {
		if d.Severity == diag.Error {
			parts = append(parts, d.Summary)
		}
	}
	return fmt.Errorf("%s", strings.Join(parts, "; "))
}

// packageListServer answers GET /packages with `results` and records the query.
func packageListServer(t *testing.T, results []*Package, revisions []*PackageRevision, gotQuery *string) *httptest.Server {
	t.Helper()
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		switch {
		case r.URL.Path == "/packages":
			*gotQuery = r.URL.RawQuery
			_ = json.NewEncoder(w).Encode(packageListResponse{Results: results})
		case strings.HasSuffix(r.URL.Path, "/revisions"):
			_ = json.NewEncoder(w).Encode(packageRevisionListResponse{Results: revisions})
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
		}
	}))
}

func readPackage(t *testing.T, server *httptest.Server, config map[string]interface{}) (*schema.ResourceData, error) {
	t.Helper()
	d := schema.TestResourceDataRaw(t, dataSourcePackage().Schema, config)
	diags := dataSourcePackageRead(context.Background(), d, newTestClient(server))
	if diags.HasError() {
		return d, errorsOf(diags)
	}
	return d, nil
}

// A package published for every organization is owned by nullplatform's own
// organization, an NRN the installing organization neither knows nor may name.
// `visible_at` finds it the way the organization sees it: by slug, among the
// packages visible at one of its own NRNs.
func TestDataSourcePackageRead_VisibleAtFindsAGlobalPackageBySlug(t *testing.T) {
	var gotQuery string
	server := packageListServer(t, []*Package{{
		ID:                "pkg-global",
		Nrn:               "organization=123", // the API projects the owner onto the caller's organization
		Slug:              "scopes-lambda-global",
		Name:              "scopes-lambda-global",
		VisibleTo:         []string{"organization=*"},
		DefaultRevisionID: "rev-10",
		DefaultVersion:    "0.10.0",
		LatestRevisionID:  "rev-10",
		LatestVersion:     "0.10.0",
	}}, nil, &gotQuery)
	defer server.Close()

	d, err := readPackage(t, server, map[string]interface{}{
		"visible_at": "organization=123:account=456",
		"slug":       "scopes-lambda-global",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotQuery, "visible_to=organization=123:account=456") || !strings.Contains(gotQuery, "slug=scopes-lambda-global") {
		t.Errorf("the lookup must list by visibility and slug, got %q", gotQuery)
	}
	if strings.Contains(gotQuery, "nrn=") {
		t.Errorf("a visibility lookup must not filter by owner, got %q", gotQuery)
	}
	if d.Id() != "pkg-global" {
		t.Errorf("id = %q, want pkg-global", d.Id())
	}
	if got := d.Get("revision_id").(string); got != "rev-10" {
		t.Errorf("revision_id = %q, want rev-10", got)
	}
}

// Two packages with the same slug can be visible at one NRN (an organization's
// own and a global one): the lookup refuses to guess.
func TestDataSourcePackageRead_VisibleAtRefusesAnAmbiguousSlug(t *testing.T) {
	var gotQuery string
	server := packageListServer(t, []*Package{{ID: "pkg-own", Slug: "lambda"}, {ID: "pkg-global", Slug: "lambda"}}, nil, &gotQuery)
	defer server.Close()

	_, err := readPackage(t, server, map[string]interface{}{"visible_at": "organization=1", "slug": "lambda"})
	if err == nil || !strings.Contains(err.Error(), "2 packages with slug lambda are visible at organization=1") {
		t.Fatalf("want an ambiguity error, got %v", err)
	}
}

func TestDataSourcePackageRead_VisibleAtNothingVisible(t *testing.T) {
	var gotQuery string
	server := packageListServer(t, nil, nil, &gotQuery)
	defer server.Close()

	_, err := readPackage(t, server, map[string]interface{}{"visible_at": "organization=1", "slug": "lambda"})
	if err == nil || !strings.Contains(err.Error(), "no package with slug lambda is visible at organization=1") {
		t.Fatalf("want a not-visible error, got %v", err)
	}
}

// The owner lookup is unchanged: same query, same result.
func TestDataSourcePackageRead_NrnLookupUnchanged(t *testing.T) {
	var gotQuery string
	server := packageListServer(t, []*Package{{ID: "pkg-1", Slug: "lambda", DefaultRevisionID: "rev-1"}},
		[]*PackageRevision{{ID: "rev-0", Version: "0.9.0"}, {ID: "rev-1", Version: "1.0.0"}}, &gotQuery)
	defer server.Close()

	d, err := readPackage(t, server, map[string]interface{}{"nrn": "organization=1", "slug": "lambda", "version": "0.9.0"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotQuery, "nrn=organization=1") || strings.Contains(gotQuery, "visible_to") {
		t.Errorf("owner lookup query changed: %q", gotQuery)
	}
	if got := d.Get("revision_id").(string); got != "rev-0" {
		t.Errorf("revision_id = %q, want rev-0", got)
	}
}

// Exactly one of nrn and visible_at.
func TestDataSourcePackage_NrnAndVisibleAtAreExclusive(t *testing.T) {
	s := dataSourcePackage().Schema
	if !s["nrn"].Optional || !s["visible_at"].Optional {
		t.Fatal("nrn and visible_at must both be optional")
	}
	if got := s["nrn"].ExactlyOneOf; len(got) != 2 {
		t.Errorf("nrn.ExactlyOneOf = %v, want [nrn visible_at]", got)
	}
}
