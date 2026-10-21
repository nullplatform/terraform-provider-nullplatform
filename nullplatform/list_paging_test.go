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

// pagedListServer answers a listing the way the API does: `limit` defaults to
// 30 when absent, `offset` to 0, and a page never holds more than `limit`
// items. It records every listing request so tests can assert the client
// walked the pages instead of trusting the first one.
func pagedListServer(t *testing.T, path string, items []any, extra func(w http.ResponseWriter, r *http.Request) bool, requests *[]string) *httptest.Server {
	t.Helper()
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			if extra != nil && extra(w, r) {
				return
			}
			t.Errorf("unexpected request: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		*requests = append(*requests, r.URL.RawQuery)
		limit, offset := 30, 0
		if v := r.URL.Query().Get("limit"); v != "" {
			limit, _ = strconv.Atoi(v)
		}
		if v := r.URL.Query().Get("offset"); v != "" {
			offset, _ = strconv.Atoi(v)
		}
		end := offset + limit
		if end > len(items) {
			end = len(items)
		}
		page := []any{}
		if offset < len(items) {
			page = items[offset:end]
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"paging":  map[string]any{"limit": limit, "offset": offset},
			"results": page,
		})
	}))
}

// The artifact lookup must find an artifact that is not on the first page.
// An organization that owns many global images (one per scope and service
// repository) pushed the one the lookup wanted past the API's default page of
// 30, and the lookup answered "no artifact visible ... matches meta".
func TestDataSourcePlatformArtifactRead_FindsAnArtifactBeyondTheFirstPage(t *testing.T) {
	items := []any{}
	for i := 0; i < 130; i++ {
		items = append(items, PlatformArtifact{
			ResourceID:   fmt.Sprintf("art-%d", i),
			Type:         "oci_image",
			Nrn:          "organization=1",
			IdentityMeta: map[string]interface{}{"registry": "public.ecr.aws", "repository": fmt.Sprintf("nullplatform/other-%d", i)},
			VisibleTo:    []string{"organization=*"},
		})
	}
	items = append(items, PlatformArtifact{
		ResourceID:   "art-wanted",
		Type:         "oci_image",
		Nrn:          "organization=1",
		IdentityMeta: map[string]interface{}{"registry": "public.ecr.aws", "repository": "nullplatform/scopes/lambda"},
		VisibleTo:    []string{"organization=*"},
	})
	revisions := func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path != "/artifacts/art-wanted/revisions" {
			return false
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(platformArtifactRevisionListResponse{Results: []*PlatformArtifactRevision{
			{ResourceID: "art-wanted", ResourceRevisionID: "rev-1", Meta: map[string]interface{}{"registry": "public.ecr.aws", "repository": "nullplatform/scopes/lambda", "digest": "sha256:aaa"}, CreatedAt: "2026-10-16T00:00:00Z"},
		}})
		return true
	}
	var requests []string
	server := pagedListServer(t, "/artifacts", items, revisions, &requests)
	defer server.Close()

	d := schema.TestResourceDataRaw(t, dataSourcePlatformArtifact().Schema, map[string]interface{}{
		"nrn":  "organization=4",
		"type": "oci_image",
		"meta": `{"registry":"public.ecr.aws","repository":"nullplatform/scopes/lambda","digest":"sha256:aaa"}`,
	})

	diags := dataSourcePlatformArtifactRead(context.Background(), d, newTestClient(server))
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if got := d.Get("artifact_id").(string); got != "art-wanted" {
		t.Errorf("artifact_id = %q, want art-wanted", got)
	}
	if len(requests) < 2 {
		t.Errorf("the listing must be walked page by page, got %d request(s): %v", len(requests), requests)
	}
	for _, q := range requests {
		if !strings.Contains(q, "visible_to=organization=4") {
			t.Errorf("every page keeps the visible_to filter, got %q", q)
		}
	}
}

// A revision pinned by digest can be older than the newest 30: an image
// released weekly passes that in well under a year.
func TestListPlatformArtifactRevisions_WalksEveryPage(t *testing.T) {
	items := []any{}
	for i := 0; i < 75; i++ {
		items = append(items, PlatformArtifactRevision{ResourceID: "art-1", ResourceRevisionID: fmt.Sprintf("rev-%d", i)})
	}
	var requests []string
	server := pagedListServer(t, "/artifacts/art-1/revisions", items, nil, &requests)
	defer server.Close()

	revisions, err := newTestClient(server).ListPlatformArtifactRevisions("art-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(revisions) != 75 {
		t.Errorf("got %d revisions, want 75", len(revisions))
	}
}

// Resolving `version` on the package data source walks every revision: a
// package republished on every release has more than one page of them.
func TestListPackageRevisions_WalksEveryPage(t *testing.T) {
	items := []any{}
	for i := 0; i < 64; i++ {
		items = append(items, PackageRevision{ID: fmt.Sprintf("rev-%d", i), PackageID: "pkg-1", Version: fmt.Sprintf("1.0.%d", i)})
	}
	var requests []string
	server := pagedListServer(t, "/packages/pkg-1/revisions", items, nil, &requests)
	defer server.Close()

	revisions, err := newTestClient(server).ListPackageRevisions("pkg-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(revisions) != 64 {
		t.Errorf("got %d revisions, want 64", len(revisions))
	}
}

// A listing error on a later page surfaces, with the API's message, instead of
// a silently truncated result.
func TestListAllPages_ErrorOnALaterPageSurfaces(t *testing.T) {
	calls := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			items := make([]PackageRevision, listPageSize)
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(packageRevisionListResponse{Results: toPointers(items)})
			return
		}
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"Insufficient permissions to access the requested resource"}`))
	}))
	defer server.Close()

	_, err := newTestClient(server).ListPackageRevisions("pkg-1")
	if err == nil || !strings.Contains(err.Error(), "Insufficient permissions") {
		t.Fatalf("want the API's message, got %v", err)
	}
}

// A body that is not a page is an error, not an empty listing.
func TestListAllPages_UndecodablePageIsAnError(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not json`))
	}))
	defer server.Close()

	if _, err := newTestClient(server).ListPackageRevisions("pkg-1"); err == nil {
		t.Fatal("want a decode error")
	}
	if _, err := newTestClient(server).ListPlatformArtifactRevisions("art-1"); err == nil {
		t.Fatal("want a decode error")
	}
	if _, err := newTestClient(server).ListPlatformArtifacts("organization=1", "oci_image"); err == nil {
		t.Fatal("want a decode error")
	}
}

func toPointers[T any](items []T) []*T {
	out := make([]*T, len(items))
	for i := range items {
		out[i] = &items[i]
	}
	return out
}
