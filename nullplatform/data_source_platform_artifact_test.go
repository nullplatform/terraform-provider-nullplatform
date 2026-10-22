package nullplatform

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// artifactServer serves the two endpoints the data source hits: the visibility-
// scoped artifact listing and one artifact's revision listing. Both page as
// the real API does — by offset, 30 per page unless a limit is sent, the
// paging echoed back and no total — so a set past one page is only whole to a
// client that walks every page. It records the listing query string so tests
// can assert the visible_to filter is used (and, with paging, the last page
// read).
func artifactServer(t *testing.T, artifacts []*PlatformArtifact, revisions map[string][]*PlatformArtifactRevision, gotListQuery *string) *httptest.Server {
	t.Helper()
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if r.URL.Path == "/artifacts" {
			*gotListQuery = r.URL.RawQuery
			page, paging := artifactPage(r, artifacts)
			_ = json.NewEncoder(w).Encode(platformArtifactListResponse{Results: page, Paging: paging})
			return
		}
		if strings.HasSuffix(r.URL.Path, "/revisions") {
			id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/artifacts/"), "/revisions")
			page, paging := artifactPage(r, revisions[id])
			_ = json.NewEncoder(w).Encode(platformArtifactRevisionListResponse{Results: page, Paging: paging})
			return
		}
		t.Errorf("unexpected request: %s", r.URL.Path)
	}))
}

// artifactPage is the API's paging: `limit` items (30 when not sent) from
// `offset`, both echoed back in `paging`.
func artifactPage[T any](r *http.Request, all []T) ([]T, *Paging) {
	limit, offset := 30, 0
	if n, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && n > 0 {
		limit = n
	}
	if n, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && n > 0 {
		offset = n
	}
	start := min(offset, len(all))
	return all[start:min(start+limit, len(all))], &Paging{Offset: offset, Limit: limit}
}

// setPlatformArtifactPageSize shrinks the page the client reads in, so a
// handful of items spans several pages.
func setPlatformArtifactPageSize(t *testing.T, n int) {
	t.Helper()
	was := platformArtifactPageSize
	platformArtifactPageSize = n
	t.Cleanup(func() { platformArtifactPageSize = was })
}

// The lookup must query by visibility, not ownership: a global artifact
// (visible_to organization=*) owned by another organization is otherwise
// unreachable — the bug this filter change fixes.
func TestDataSourcePlatformArtifactRead_GlobalArtifactViaVisibleTo(t *testing.T) {
	var gotQuery string
	server := artifactServer(t,
		[]*PlatformArtifact{{
			ResourceID:       "art-1",
			Type:             "oci_image",
			Nrn:              "organization=1", // owned by ANOTHER org
			IdentityMeta:     map[string]interface{}{"registry": "public.ecr.aws", "repository": "nullplatform/scopes/lambda"},
			VisibleTo:        []string{"organization=*"},
			LatestRevisionID: "rev-2",
		}},
		map[string][]*PlatformArtifactRevision{"art-1": {
			{ResourceID: "art-1", ResourceRevisionID: "rev-2", Meta: map[string]interface{}{"registry": "public.ecr.aws", "repository": "nullplatform/scopes/lambda", "digest": "sha256:bbb"}, CreatedAt: "2026-09-02T00:00:00Z"},
			{ResourceID: "art-1", ResourceRevisionID: "rev-1", Meta: map[string]interface{}{"registry": "public.ecr.aws", "repository": "nullplatform/scopes/lambda", "digest": "sha256:aaa"}, CreatedAt: "2026-09-01T00:00:00Z"},
		}},
		&gotQuery,
	)
	defer server.Close()

	d := schema.TestResourceDataRaw(t, dataSourcePlatformArtifact().Schema, map[string]interface{}{
		"nrn":  "organization=4",
		"type": "oci_image",
		"meta": `{"registry":"public.ecr.aws","repository":"nullplatform/scopes/lambda"}`,
	})

	diags := dataSourcePlatformArtifactRead(context.Background(), d, newTestClient(server))
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if !strings.Contains(gotQuery, "visible_to=organization=4") {
		t.Errorf("listing must filter by visible_to, got query %q", gotQuery)
	}
	if strings.Contains(gotQuery, "nrn=") {
		t.Errorf("listing must not filter by owner nrn, got query %q", gotQuery)
	}
	if got := d.Get("revision_id").(string); got != "rev-2" {
		t.Errorf("revision_id = %q, want rev-2 (latest)", got)
	}
	if got := d.Get("digest").(string); got != "sha256:bbb" {
		t.Errorf("digest = %q, want sha256:bbb", got)
	}
}

// A tag selects the NEWEST revision registered with it and resolves its
// digest — the moved-tag case: v1 was re-registered against a new digest, so
// the newer revision must win even though an older one carries the same tag.
func TestDataSourcePlatformArtifactRead_TagResolvesNewestRevision(t *testing.T) {
	var gotQuery string
	server := artifactServer(t,
		[]*PlatformArtifact{{
			ResourceID:   "art-1",
			Type:         "oci_image",
			Nrn:          "organization=4",
			IdentityMeta: map[string]interface{}{"registry": "r.io", "repository": "org/app"},
		}},
		map[string][]*PlatformArtifactRevision{"art-1": {
			// Deliberately oldest-first: the data source must sort, not trust API order.
			{ResourceRevisionID: "rev-old", Meta: map[string]interface{}{"registry": "r.io", "repository": "org/app", "digest": "sha256:old", "tag": "v1"}, CreatedAt: "2026-09-01T00:00:00Z"},
			{ResourceRevisionID: "rev-other", Meta: map[string]interface{}{"registry": "r.io", "repository": "org/app", "digest": "sha256:zzz", "tag": "v2"}, CreatedAt: "2026-09-02T00:00:00Z"},
			{ResourceRevisionID: "rev-new", Meta: map[string]interface{}{"registry": "r.io", "repository": "org/app", "digest": "sha256:new", "tag": "v1"}, CreatedAt: "2026-09-03T00:00:00Z"},
		}},
		&gotQuery,
	)
	defer server.Close()

	d := schema.TestResourceDataRaw(t, dataSourcePlatformArtifact().Schema, map[string]interface{}{
		"nrn":  "organization=4",
		"type": "oci_image",
		"meta": `{"registry":"r.io","repository":"org/app","tag":"v1"}`,
	})

	diags := dataSourcePlatformArtifactRead(context.Background(), d, newTestClient(server))
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if got := d.Get("revision_id").(string); got != "rev-new" {
		t.Errorf("revision_id = %q, want rev-new (newest with tag v1)", got)
	}
	if got := d.Get("digest").(string); got != "sha256:new" {
		t.Errorf("digest = %q, want sha256:new (resolved for the caller)", got)
	}
}

// When a visibility-scoped listing surfaces both an owned artifact and a
// global one with the same identity, the owned one wins — configs written
// before globals existed keep resolving unchanged.
func TestDataSourcePlatformArtifactRead_OwnedBeatsGlobalOnAmbiguity(t *testing.T) {
	var gotQuery string
	server := artifactServer(t,
		[]*PlatformArtifact{
			{
				ResourceID:   "art-global",
				Type:         "oci_image",
				Nrn:          "organization=1",
				IdentityMeta: map[string]interface{}{"registry": "r.io", "repository": "org/app"},
				VisibleTo:    []string{"organization=*"},
			},
			{
				ResourceID:   "art-owned",
				Type:         "oci_image",
				Nrn:          "organization=4",
				IdentityMeta: map[string]interface{}{"registry": "r.io", "repository": "org/app"},
			},
		},
		map[string][]*PlatformArtifactRevision{"art-owned": {
			{ResourceRevisionID: "rev-owned", Meta: map[string]interface{}{"registry": "r.io", "repository": "org/app", "digest": "sha256:mine"}, CreatedAt: "2026-09-01T00:00:00Z"},
		}},
		&gotQuery,
	)
	defer server.Close()

	d := schema.TestResourceDataRaw(t, dataSourcePlatformArtifact().Schema, map[string]interface{}{
		"nrn":  "organization=4",
		"type": "oci_image",
		"meta": `{"registry":"r.io","repository":"org/app"}`,
	})

	diags := dataSourcePlatformArtifactRead(context.Background(), d, newTestClient(server))
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if got := d.Get("artifact_id").(string); got != "art-owned" {
		t.Errorf("artifact_id = %q, want art-owned (owner match wins over global)", got)
	}
}

// An owned artifact registered by digest before the upstream one existed must
// not shadow the global one forever: when the owned artifact has no revision
// matching the requested fields (here a tag only the global carries), the
// lookup falls back to the next candidate instead of failing. Artifacts are
// immutable and cannot be deleted, so without this a lookup by tag would be
// impossible from any nrn that ever registered its own copy.
func TestDataSourcePlatformArtifactRead_FallsBackToGlobalWhenOwnedHasNoMatchingRevision(t *testing.T) {
	var gotQuery string
	server := artifactServer(t,
		[]*PlatformArtifact{
			{
				ResourceID:   "art-owned",
				Type:         "oci_image",
				Nrn:          "organization=4",
				IdentityMeta: map[string]interface{}{"registry": "r.io", "repository": "org/app"},
			},
			{
				ResourceID:   "art-global",
				Type:         "oci_image",
				Nrn:          "organization=1",
				IdentityMeta: map[string]interface{}{"registry": "r.io", "repository": "org/app"},
				VisibleTo:    []string{"organization=*"},
			},
		},
		map[string][]*PlatformArtifactRevision{
			"art-owned": {
				{ResourceRevisionID: "rev-owned", Meta: map[string]interface{}{"registry": "r.io", "repository": "org/app", "digest": "sha256:mine"}, CreatedAt: "2026-09-01T00:00:00Z"},
			},
			"art-global": {
				{ResourceRevisionID: "rev-global-tagged", Meta: map[string]interface{}{"registry": "r.io", "repository": "org/app", "tag": "v1.0.0", "digest": "sha256:theirs"}, CreatedAt: "2026-09-02T00:00:00Z"},
			},
		},
		&gotQuery,
	)
	defer server.Close()

	d := schema.TestResourceDataRaw(t, dataSourcePlatformArtifact().Schema, map[string]interface{}{
		"nrn":  "organization=4",
		"type": "oci_image",
		"meta": `{"registry":"r.io","repository":"org/app","tag":"v1.0.0"}`,
	})

	diags := dataSourcePlatformArtifactRead(context.Background(), d, newTestClient(server))
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if got := d.Get("artifact_id").(string); got != "art-global" {
		t.Errorf("artifact_id = %q, want art-global (owned has no revision with the tag)", got)
	}
	if got := d.Get("revision_id").(string); got != "rev-global-tagged" {
		t.Errorf("revision_id = %q, want rev-global-tagged", got)
	}
	if got := d.Get("digest").(string); got != "sha256:theirs" {
		t.Errorf("digest = %q, want sha256:theirs", got)
	}
}

// When no candidate — owned or global — holds a revision matching the requested
// fields, the error names every artifact that was examined so the reader can
// tell which registrations were considered and what they lack.
func TestDataSourcePlatformArtifactRead_NoCandidateRevisionMatchesErrors(t *testing.T) {
	var gotQuery string
	server := artifactServer(t,
		[]*PlatformArtifact{
			{
				ResourceID:   "art-owned",
				Type:         "oci_image",
				Nrn:          "organization=4",
				IdentityMeta: map[string]interface{}{"registry": "r.io", "repository": "org/app"},
			},
			{
				ResourceID:   "art-global",
				Type:         "oci_image",
				Nrn:          "organization=1",
				IdentityMeta: map[string]interface{}{"registry": "r.io", "repository": "org/app"},
				VisibleTo:    []string{"organization=*"},
			},
		},
		map[string][]*PlatformArtifactRevision{
			"art-owned":  {{ResourceRevisionID: "rev-owned", Meta: map[string]interface{}{"registry": "r.io", "repository": "org/app", "digest": "sha256:mine"}, CreatedAt: "2026-09-01T00:00:00Z"}},
			"art-global": {{ResourceRevisionID: "rev-global", Meta: map[string]interface{}{"registry": "r.io", "repository": "org/app", "tag": "v1.0.0", "digest": "sha256:theirs"}, CreatedAt: "2026-09-02T00:00:00Z"}},
		},
		&gotQuery,
	)
	defer server.Close()

	d := schema.TestResourceDataRaw(t, dataSourcePlatformArtifact().Schema, map[string]interface{}{
		"nrn":  "organization=4",
		"type": "oci_image",
		"meta": `{"registry":"r.io","repository":"org/app","tag":"v9.9.9"}`,
	})

	diags := dataSourcePlatformArtifactRead(context.Background(), d, newTestClient(server))
	if !diags.HasError() {
		t.Fatal("expected an error when no candidate has a revision matching the tag")
	}
	msg := diags[0].Summary
	for _, want := range []string{"art-owned", "art-global", "v9.9.9"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q should mention %q", msg, want)
		}
	}
}

// A failure listing a candidate's revisions surfaces as the read's error
// instead of being skipped as "no match".
func TestDataSourcePlatformArtifactRead_RevisionListingErrorPropagates(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/artifacts" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(platformArtifactListResponse{Results: []*PlatformArtifact{{
				ResourceID:   "art-owned",
				Type:         "oci_image",
				Nrn:          "organization=4",
				IdentityMeta: map[string]interface{}{"registry": "r.io", "repository": "org/app"},
			}}})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	d := schema.TestResourceDataRaw(t, dataSourcePlatformArtifact().Schema, map[string]interface{}{
		"nrn":  "organization=4",
		"type": "oci_image",
		"meta": `{"registry":"r.io","repository":"org/app"}`,
	})

	diags := dataSourcePlatformArtifactRead(context.Background(), d, newTestClient(server))
	if !diags.HasError() {
		t.Fatal("expected the revision listing failure to be returned as an error")
	}
}

// No visible artifact matching the identity meta is an explicit error, not an
// empty result.
func TestDataSourcePlatformArtifactRead_NoMatchErrors(t *testing.T) {
	var gotQuery string
	server := artifactServer(t, []*PlatformArtifact{}, nil, &gotQuery)
	defer server.Close()

	d := schema.TestResourceDataRaw(t, dataSourcePlatformArtifact().Schema, map[string]interface{}{
		"nrn":  "organization=4",
		"type": "oci_image",
		"meta": `{"registry":"r.io","repository":"org/nope"}`,
	})

	diags := dataSourcePlatformArtifactRead(context.Background(), d, newTestClient(server))
	if !diags.HasError() {
		t.Fatal("expected an error for a meta matching no visible artifact")
	}
	if !strings.Contains(diags[0].Summary, "no oci_image artifact visible at organization=4") {
		t.Errorf("unexpected error: %s", diags[0].Summary)
	}
}

// Two DISTINCT owned artifacts matching the identity meta stay ambiguous —
// the owner-preference tiebreak only resolves owned-vs-shared, never owned-vs-owned.
func TestDataSourcePlatformArtifactRead_AmbiguousOwnedMatchesError(t *testing.T) {
	var gotQuery string
	server := artifactServer(t,
		[]*PlatformArtifact{
			{ResourceID: "art-a", Type: "oci_image", Nrn: "organization=4", IdentityMeta: map[string]interface{}{"registry": "r.io"}},
			{ResourceID: "art-b", Type: "oci_image", Nrn: "organization=4", IdentityMeta: map[string]interface{}{"registry": "r.io"}},
		},
		nil, &gotQuery,
	)
	defer server.Close()

	d := schema.TestResourceDataRaw(t, dataSourcePlatformArtifact().Schema, map[string]interface{}{
		"nrn":  "organization=4",
		"type": "oci_image",
		"meta": `{"registry":"r.io","repository":"org/app"}`,
	})

	diags := dataSourcePlatformArtifactRead(context.Background(), d, newTestClient(server))
	if !diags.HasError() {
		t.Fatal("expected an ambiguity error for two owned identity matches")
	}
	if !strings.Contains(diags[0].Summary, "matches 2 oci_image artifacts") {
		t.Errorf("unexpected error: %s", diags[0].Summary)
	}
}

// Retro-compatibility: a digest in the meta pins its exact revision — even an
// OLD one — regardless of the newest-first ordering that tag lookups rely on.
func TestDataSourcePlatformArtifactRead_DigestPinsOldRevision(t *testing.T) {
	var gotQuery string
	server := artifactServer(t,
		[]*PlatformArtifact{{
			ResourceID:   "art-1",
			Type:         "oci_image",
			Nrn:          "organization=4",
			IdentityMeta: map[string]interface{}{"registry": "r.io", "repository": "org/app"},
		}},
		map[string][]*PlatformArtifactRevision{"art-1": {
			{ResourceRevisionID: "rev-new", Meta: map[string]interface{}{"registry": "r.io", "repository": "org/app", "digest": "sha256:new"}, CreatedAt: "2026-09-03T00:00:00Z"},
			{ResourceRevisionID: "rev-old", Meta: map[string]interface{}{"registry": "r.io", "repository": "org/app", "digest": "sha256:old"}, CreatedAt: "2026-09-01T00:00:00Z"},
		}},
		&gotQuery,
	)
	defer server.Close()

	d := schema.TestResourceDataRaw(t, dataSourcePlatformArtifact().Schema, map[string]interface{}{
		"nrn":  "organization=4",
		"type": "oci_image",
		"meta": `{"registry":"r.io","repository":"org/app","digest":"sha256:old"}`,
	})

	diags := dataSourcePlatformArtifactRead(context.Background(), d, newTestClient(server))
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if got := d.Get("revision_id").(string); got != "rev-old" {
		t.Errorf("revision_id = %q, want rev-old (digest pins exactly, newer revisions don't shadow it)", got)
	}
	if got := d.Get("digest").(string); got != "sha256:old" {
		t.Errorf("digest = %q, want sha256:old", got)
	}
}

// The listing pages at 30 by default and answers no total. When the globals
// crossed 30 (2026-10-06) the older production ones — scopes/lambda,
// scopes/containers, services/s3 — fell to page two, where no provider
// version looked, and every lookup of them failed as "not visible". The
// wanted artifact is the LAST of 31: absent from the first page, present only
// to a client that walks them all.
func TestDataSourcePlatformArtifactRead_ArtifactBeyondFirstPage(t *testing.T) {
	setPlatformArtifactPageSize(t, 10)
	var artifacts []*PlatformArtifact
	for i := 0; i < 30; i++ {
		artifacts = append(artifacts, &PlatformArtifact{
			ResourceID:   fmt.Sprintf("art-beta-%d", i),
			Type:         "oci_image",
			Nrn:          "organization=1",
			IdentityMeta: map[string]interface{}{"registry": "public.ecr.aws", "repository": fmt.Sprintf("nullplatform/beta/%d", i)},
			VisibleTo:    []string{"organization=*"},
		})
	}
	artifacts = append(artifacts, &PlatformArtifact{
		ResourceID:       "art-lambda",
		Type:             "oci_image",
		Nrn:              "organization=1",
		IdentityMeta:     map[string]interface{}{"registry": "public.ecr.aws", "repository": "nullplatform/scopes/lambda"},
		VisibleTo:        []string{"organization=*"},
		LatestRevisionID: "rev-1",
	})
	var gotQuery string
	server := artifactServer(t, artifacts,
		map[string][]*PlatformArtifactRevision{"art-lambda": {
			{ResourceID: "art-lambda", ResourceRevisionID: "rev-1", Meta: map[string]interface{}{"registry": "public.ecr.aws", "repository": "nullplatform/scopes/lambda", "tag": "v0.8.3", "digest": "sha256:aaa"}, CreatedAt: "2026-09-01T00:00:00Z"},
		}},
		&gotQuery,
	)
	defer server.Close()

	d := schema.TestResourceDataRaw(t, dataSourcePlatformArtifact().Schema, map[string]interface{}{
		"nrn":  "organization=4",
		"type": "oci_image",
		"meta": `{"registry":"public.ecr.aws","repository":"nullplatform/scopes/lambda","tag":"v0.8.3"}`,
	})

	diags := dataSourcePlatformArtifactRead(context.Background(), d, newTestClient(server))
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if got := d.Get("artifact_id").(string); got != "art-lambda" {
		t.Errorf("artifact_id = %q, want art-lambda (31st visible artifact)", got)
	}
	if got := d.Get("digest").(string); got != "sha256:aaa" {
		t.Errorf("digest = %q, want sha256:aaa", got)
	}
	if !strings.Contains(gotQuery, "offset=30") {
		t.Errorf("listing must be read to its last page, final query %q", gotQuery)
	}
}

// A revision listing pages the same way, newest first, so an artifact with
// more than a page of revisions loses its OLDEST ones to a client that reads
// one page — a tag only an early revision carries becomes unresolvable (the
// lambda global already holds 21). The wanted tag is on the 31st revision.
func TestDataSourcePlatformArtifactRead_RevisionBeyondFirstPage(t *testing.T) {
	setPlatformArtifactPageSize(t, 10)
	var revisions []*PlatformArtifactRevision
	for i := 30; i >= 0; i-- { // newest first, as the API answers
		revisions = append(revisions, &PlatformArtifactRevision{
			ResourceID:         "art-1",
			ResourceRevisionID: fmt.Sprintf("rev-%d", i),
			Meta:               map[string]interface{}{"registry": "r.io", "repository": "org/app", "tag": fmt.Sprintf("v0.%d.0", i), "digest": fmt.Sprintf("sha256:%d", i)},
			CreatedAt:          fmt.Sprintf("2026-08-%02dT00:00:00Z", i+1),
		})
	}
	var gotQuery string
	server := artifactServer(t,
		[]*PlatformArtifact{{
			ResourceID:   "art-1",
			Type:         "oci_image",
			Nrn:          "organization=4",
			IdentityMeta: map[string]interface{}{"registry": "r.io", "repository": "org/app"},
		}},
		map[string][]*PlatformArtifactRevision{"art-1": revisions},
		&gotQuery,
	)
	defer server.Close()

	d := schema.TestResourceDataRaw(t, dataSourcePlatformArtifact().Schema, map[string]interface{}{
		"nrn":  "organization=4",
		"type": "oci_image",
		"meta": `{"registry":"r.io","repository":"org/app","tag":"v0.0.0"}`,
	})

	diags := dataSourcePlatformArtifactRead(context.Background(), d, newTestClient(server))
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if got := d.Get("revision_id").(string); got != "rev-0" {
		t.Errorf("revision_id = %q, want rev-0 (31st, oldest revision)", got)
	}
	if got := d.Get("digest").(string); got != "sha256:0" {
		t.Errorf("digest = %q, want sha256:0", got)
	}
}

// A listing page that does not decode is the read's error, named as such —
// not an empty page silently taken as the end of the listing.
func TestDataSourcePlatformArtifactRead_UndecodableListingPageErrors(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"results": "not a list"}`))
	}))
	defer server.Close()

	d := schema.TestResourceDataRaw(t, dataSourcePlatformArtifact().Schema, map[string]interface{}{
		"nrn":  "organization=4",
		"type": "oci_image",
		"meta": `{"registry":"r.io","repository":"org/app"}`,
	})

	diags := dataSourcePlatformArtifactRead(context.Background(), d, newTestClient(server))
	if !diags.HasError() {
		t.Fatal("expected an error for an undecodable listing page")
	}
	if got := diags[0].Summary; !strings.Contains(got, "error decoding artifacts list") {
		t.Errorf("error = %q, want it to name the listing decode failure", got)
	}
}
