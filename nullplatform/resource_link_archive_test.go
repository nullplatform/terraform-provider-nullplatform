package nullplatform

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func linkState(status, archivedAt string) *terraform.InstanceState {
	return &terraform.InstanceState{
		ID: "lnk-1",
		Attributes: map[string]string{
			"id":                       "lnk-1",
			"name":                     "link-redis",
			"slug":                     "link-redis",
			"service_id":               "svc-1",
			"specification_id":         "spec-1",
			"desired_specification_id": "spec-1",
			"entity_nrn":               "organization=1:account=2",
			"status":                   status,
			"archived_at":              archivedAt,
			"archive_on_destroy":       "false",
			"linkable_to.#":            "0",
		},
	}
}

func linkDiff(t *testing.T, state *terraform.InstanceState, config map[string]any) *terraform.InstanceDiff {
	t.Helper()
	diff, err := resourceLink().Diff(
		context.Background(),
		state,
		terraform.NewResourceConfigRaw(config),
		nil,
	)
	if err != nil {
		t.Fatalf("computing the diff: %v", err)
	}
	return diff
}

func linkConfig(extra map[string]any) map[string]any {
	config := map[string]any{
		"name":             "link-redis",
		"service_id":       "svc-1",
		"specification_id": "spec-1",
		"entity_nrn":       "organization=1:account=2",
	}
	for k, v := range extra {
		config[k] = v
	}
	return config
}

// `status` was plain Optional, so an omitted configuration value diffed against
// the `active` the API assigns and every single plan showed a change that
// PATCHed nothing. Computed makes the omitted attribute track the platform.
func TestLinkResource_OmittedStatusDoesNotPhantomDiff(t *testing.T) {
	diff := linkDiff(t, linkState("active", ""), linkConfig(nil))

	if diff != nil && !diff.Empty() {
		t.Fatalf("a configuration without `status` must plan clean, got %s", diff.GoString())
	}
}

// The same schema change is what stops an unrelated apply from restoring a link
// archived out of band.
func TestLinkResource_ArchivedOutOfBandNeverPlansAStatusChange(t *testing.T) {
	diff := linkDiff(t, linkState("archived", "2026-08-01T10:00:00.000Z"), linkConfig(nil))

	if diff == nil {
		return
	}
	if attr, planned := diff.Attributes["status"]; planned {
		t.Fatalf("planned a status change %q -> %q on an archived link the configuration says nothing about", attr.Old, attr.New)
	}
}

func TestLinkResource_ExplicitActiveOnArchivedLinkIsADiff(t *testing.T) {
	diff := linkDiff(t, linkState("archived", "2026-08-01T10:00:00.000Z"), linkConfig(map[string]any{"status": "active"}))

	if diff == nil || diff.Empty() {
		t.Fatal("an explicit `status = \"active\"` on an archived link must show a diff")
	}
	attr, ok := diff.Attributes["status"]
	if !ok {
		t.Fatalf("expected a `status` attribute diff, got %s", diff.GoString())
	}
	if attr.Old != "archived" || attr.New != "active" {
		t.Errorf("got status diff %q -> %q, want archived -> active", attr.Old, attr.New)
	}
}

// Both computed attributes have to land a concrete value at create time.
func TestLinkCreate_RecordsTheStatusTheAPIAssigned(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(Link{Id: "lnk-1", Status: "active"})
	}))
	defer server.Close()

	d := schema.TestResourceDataRaw(t, resourceLink().Schema, linkConfig(nil))

	if err := LinkCreate(d, newTestClient(server)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := d.Get("status").(string); got != "active" {
		t.Errorf("state kept status %q after create, want active", got)
	}
}

func TestLinkRead_RecordsArchivedAt(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(Link{
			Id:         "lnk-1",
			Name:       "link-redis",
			Status:     "archived",
			ArchivedAt: "2026-08-02T09:00:00.000Z",
		})
	}))
	defer server.Close()

	d := schema.TestResourceDataRaw(t, resourceLink().Schema, linkConfig(nil))
	d.SetId("lnk-1")

	if err := LinkRead(d, newTestClient(server)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := d.Get("status").(string); got != "archived" {
		t.Errorf("got status %q, want archived", got)
	}
	if got := d.Get("archived_at").(string); got != "2026-08-02T09:00:00.000Z" {
		t.Errorf("got archived_at %q, want the timestamp the API reported", got)
	}
}

func linkDataWithDiff(t *testing.T, state *terraform.InstanceState, config map[string]any) *schema.ResourceData {
	t.Helper()
	diff := linkDiff(t, state, config)
	d, err := schema.InternalMap(resourceLink().Schema).Data(state, diff)
	if err != nil {
		t.Fatalf("building resource data: %v", err)
	}
	return d
}

// A managed link archive returns from the PATCH while the transition is still
// running. Without a waiter the apply finished on `archiving`, and the next plan
// re-PATCHed `archived` — which the API refuses, because only an active, failed
// or cancelled link archives.
func TestLinkUpdate_ArchiveWaitsForTheTransition(t *testing.T) {
	shortenPolling(t)

	var patched Link
	var gets int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PATCH" {
			_ = json.NewDecoder(r.Body).Decode(&patched)
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(Link{Id: "lnk-1", Status: "active"})
			return
		}
		status, archivedAt := "archiving", ""
		if atomic.AddInt32(&gets, 1) >= 2 {
			status, archivedAt = "archived", "2026-08-02T09:00:00.000Z"
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(Link{Id: "lnk-1", Status: status, ArchivedAt: archivedAt})
	}))
	defer server.Close()

	d := linkDataWithDiff(t, linkState("active", ""), linkConfig(map[string]any{"status": "archived"}))

	if diags := LinkUpdateContext(context.Background(), d, newTestClient(server)); diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if patched.Status != "archived" {
		t.Errorf("PATCH sent status %q, want archived", patched.Status)
	}
	if got := d.Get("status").(string); got != "archived" {
		t.Errorf("state kept status %q, want the landed archived", got)
	}
	if got := d.Get("archived_at").(string); got != "2026-08-02T09:00:00.000Z" {
		t.Errorf("state kept archived_at %q, want the timestamp the transition produced", got)
	}
}

// A restore transits `updating`; there is no `unarchiving` status.
func TestLinkUpdate_RestoreTransitsUpdating(t *testing.T) {
	shortenPolling(t)

	var gets int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PATCH" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(Link{Id: "lnk-1", Status: "updating"})
			return
		}
		status := "updating"
		if atomic.AddInt32(&gets, 1) >= 2 {
			status = "active"
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(Link{Id: "lnk-1", Status: status})
	}))
	defer server.Close()

	d := linkDataWithDiff(t, linkState("archived", "2026-08-01T10:00:00.000Z"), linkConfig(map[string]any{"status": "active"}))

	if diags := LinkUpdateContext(context.Background(), d, newTestClient(server)); diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if got := d.Get("status").(string); got != "active" {
		t.Errorf("state kept status %q, want active", got)
	}
}

// The reason archive_on_destroy exists on links at all: a service cannot be
// archived while any of its links is not archived, and Terraform destroys the
// links first.
func TestLinkDelete_ArchiveOnDestroyArchivesInsteadOfDeleting(t *testing.T) {
	shortenPolling(t)

	var deleted bool
	var patchedStatus string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "DELETE":
			deleted = true
			w.WriteHeader(http.StatusOK)
		case "PATCH":
			var body Link
			_ = json.NewDecoder(r.Body).Decode(&body)
			patchedStatus = body.Status
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(Link{Id: "lnk-1", Status: "archiving"})
		default:
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(Link{Id: "lnk-1", Status: statusAfterArchivePatch(patchedStatus)})
		}
	}))
	defer server.Close()

	state := linkState("active", "")
	state.Attributes["archive_on_destroy"] = "true"
	d, err := schema.InternalMap(resourceLink().Schema).Data(state, nil)
	if err != nil {
		t.Fatalf("building resource data: %v", err)
	}

	if diags := LinkDeleteContext(context.Background(), d, newTestClient(server)); diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if deleted {
		t.Error("archive_on_destroy must not issue a DELETE")
	}
	if patchedStatus != "archived" {
		t.Errorf("PATCH sent status %q, want archived", patchedStatus)
	}
	if d.Id() != "" {
		t.Error("the resource must leave state once the archive lands")
	}
}

// statusAfterArchivePatch reports `archived` once the archive PATCH has been
// issued, so the waiter's first GET terminates.
func statusAfterArchivePatch(patched string) string {
	if patched == "archived" {
		return "archived"
	}
	return "active"
}

// Re-archiving an already-archived link is refused by the API, so destroy must
// recognise the state and just drop the resource.
func TestLinkDelete_ArchiveOnDestroyIsANoOpOnAnArchivedLink(t *testing.T) {
	shortenPolling(t)

	var mutated bool
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			mutated = true
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(Link{Id: "lnk-1", Status: "archived", ArchivedAt: "2026-08-01T10:00:00.000Z"})
	}))
	defer server.Close()

	state := linkState("archived", "2026-08-01T10:00:00.000Z")
	state.Attributes["archive_on_destroy"] = "true"
	d, err := schema.InternalMap(resourceLink().Schema).Data(state, nil)
	if err != nil {
		t.Fatalf("building resource data: %v", err)
	}

	if diags := LinkDeleteContext(context.Background(), d, newTestClient(server)); diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if mutated {
		t.Error("an already-archived link must not be PATCHed or DELETEd again")
	}
	if d.Id() != "" {
		t.Error("the resource must leave state")
	}
}

// The link half of the attributes/archive split. It is the LINK update's own
// branch (`l.Attributes`, a plain string map — not the service's), so the
// service test proves nothing about it: an archive PATCH cannot carry
// attributes, and a Terraform apply that changes both must send them as two
// PATCHes — attributes first, then the bare status.
func TestLinkUpdate_AttributesAndArchiveTravelInSeparatePatches(t *testing.T) {
	shortenPolling(t)

	var patches []Link
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PATCH" {
			var body Link
			_ = json.NewDecoder(r.Body).Decode(&body)
			patches = append(patches, body)
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(Link{Id: "lnk-1", Status: "active"})
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(Link{Id: "lnk-1", Status: "archived", ArchivedAt: "2026-08-02T09:00:00.000Z"})
	}))
	defer server.Close()

	state := linkState("active", "")
	state.Attributes["attributes.%"] = "1"
	state.Attributes["attributes.size"] = "small"
	d := linkDataWithDiff(t, state, linkConfig(map[string]any{
		"status":     "archived",
		"attributes": map[string]any{"size": "large"},
	}))

	if diags := LinkUpdateContext(context.Background(), d, newTestClient(server)); diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if len(patches) != 2 {
		t.Fatalf("expected 2 PATCHes (attributes, then status), got %d: %+v", len(patches), patches)
	}
	if patches[0].Attributes["size"] != "large" || patches[0].Status != "" {
		t.Errorf("first PATCH should carry only the attributes, got %+v", patches[0])
	}
	if patches[1].Status != "archived" || patches[1].Attributes != nil {
		t.Errorf("second PATCH should carry only the status, got %+v", patches[1])
	}
}

// The Context wrappers are what Terraform actually invokes; the older tests
// drove the inner functions directly, so the wrappers' success and error
// translation had no coverage at all.
func TestLinkCreateContext_TranslatesSuccessAndFailure(t *testing.T) {
	ok := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(Link{Id: "lnk-1", Status: "active"})
	}))
	defer ok.Close()
	d := schema.TestResourceDataRaw(t, resourceLink().Schema, map[string]any{
		"name": "link-redis", "service_id": "svc-1", "specification_id": "spec-1",
		"entity_nrn": "organization=1:account=2",
	})
	if diags := LinkCreateContext(context.Background(), d, newTestClient(ok)); diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if got := d.Get("status").(string); got != "active" {
		t.Errorf("state kept status %q, want the active the API assigned", got)
	}

	refused := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"An archived link (id lnk-9) with the same specification and service already exists - unarchive it, or request its deletion"}`))
	}))
	defer refused.Close()
	d2 := schema.TestResourceDataRaw(t, resourceLink().Schema, map[string]any{
		"name": "twin", "service_id": "svc-1", "specification_id": "spec-1",
		"entity_nrn": "organization=1:account=2",
	})
	diags := LinkCreateContext(context.Background(), d2, newTestClient(refused))
	if !diags.HasError() {
		t.Fatal("a refused create must error")
	}
	if !strings.Contains(diags[0].Summary, "unarchive it") {
		t.Errorf("diagnostic %q should carry the archived-twin aviso", diags[0].Summary)
	}
}

func TestLinkReadContext_TranslatesSuccessAndFailure(t *testing.T) {
	ok := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(Link{Id: "lnk-1", Status: "archived", ArchivedAt: "2026-08-01T10:00:00.000Z"})
	}))
	defer ok.Close()
	d, err := schema.InternalMap(resourceLink().Schema).Data(linkState("active", ""), nil)
	if err != nil {
		t.Fatalf("building resource data: %v", err)
	}
	if diags := LinkReadContext(context.Background(), d, newTestClient(ok)); diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if got := d.Get("archived_at").(string); got != "2026-08-01T10:00:00.000Z" {
		t.Errorf("read kept archived_at %q, want the API's value", got)
	}

	broken := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not json"))
	}))
	defer broken.Close()
	d2, err := schema.InternalMap(resourceLink().Schema).Data(linkState("active", ""), nil)
	if err != nil {
		t.Fatalf("building resource data: %v", err)
	}
	if diags := LinkReadContext(context.Background(), d2, newTestClient(broken)); !diags.HasError() {
		t.Fatal("an unreadable response must error")
	}
}

// Every refusal on the update path must abort the apply with the API's message
// rather than continue into the waiter (or worse, into state).
func TestLinkUpdate_RefusalsAbortTheApply(t *testing.T) {
	shortenPolling(t)
	for name, tc := range map[string]struct {
		config     map[string]any
		patchFails int // which PATCH answers 400 (1-based)
	}{
		// The attributes-first PATCH of a combined change is refused.
		"attributes patch refused": {
			config:     map[string]any{"status": "archived", "attributes": map[string]any{"size": "large"}},
			patchFails: 1,
		},
		// The status PATCH itself is refused (Z2's "use the archive action").
		// Attributes here MATCH the state, so the split does not fire and the
		// refusal comes from the bare status PATCH — omitting them entirely
		// would read as an attribute REMOVAL and take the split arm instead.
		"status patch refused": {
			config:     map[string]any{"status": "archived", "attributes": map[string]any{"size": "small"}},
			patchFails: 1,
		},
	} {
		t.Run(name, func(t *testing.T) {
			var patches int
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "PATCH" {
					patches++
					if patches >= tc.patchFails {
						w.WriteHeader(http.StatusBadRequest)
						_, _ = w.Write([]byte(`{"message":"refused by a guard"}`))
						return
					}
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(Link{Id: "lnk-1"})
					return
				}
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(Link{Id: "lnk-1", Status: "active"})
			}))
			defer server.Close()

			state := linkState("active", "")
			state.Attributes["attributes.%"] = "1"
			state.Attributes["attributes.size"] = "small"
			d := linkDataWithDiff(t, state, linkConfig(tc.config))
			diags := LinkUpdateContext(context.Background(), d, newTestClient(server))
			if !diags.HasError() {
				t.Fatal("a refused PATCH must abort the apply")
			}
			if !strings.Contains(diags[0].Summary, "refused by a guard") {
				t.Errorf("diagnostic %q should carry the API's message", diags[0].Summary)
			}
		})
	}
}

// A read that starts failing mid-wait must surface the transport error, not
// keep polling into the timeout.
func TestLinkUpdate_WaiterSurfacesAReadError(t *testing.T) {
	shortenPolling(t)
	var gets int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PATCH" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(Link{Id: "lnk-1", Status: "active"})
			return
		}
		if atomic.AddInt32(&gets, 1) >= 1 {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("not json"))
			return
		}
	}))
	defer server.Close()

	d := linkDataWithDiff(t, linkState("active", ""), linkConfig(map[string]any{"status": "archived"}))
	if diags := LinkUpdateContext(context.Background(), d, newTestClient(server)); !diags.HasError() {
		t.Fatal("a broken read during the wait must abort the apply")
	}
}

// A plain destroy whose DELETE is refused must surface the refusal.
func TestLinkDelete_RefusedDeleteAborts(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"nope"}`))
	}))
	defer server.Close()
	d, err := schema.InternalMap(resourceLink().Schema).Data(linkState("active", ""), nil)
	if err != nil {
		t.Fatalf("building resource data: %v", err)
	}
	if diags := LinkDeleteContext(context.Background(), d, newTestClient(server)); !diags.HasError() {
		t.Fatal("a refused DELETE must abort the destroy")
	}
}

// archive_on_destroy against a link already mid-archive JOINS the running
// transition instead of re-PATCHing (which the mint would refuse).
func TestLinkDelete_ArchiveOnDestroyJoinsARunningArchive(t *testing.T) {
	shortenPolling(t)
	var patched bool
	var gets int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PATCH" {
			patched = true
		}
		status, archivedAt := "archiving", ""
		if atomic.AddInt32(&gets, 1) >= 3 {
			status, archivedAt = "archived", "2026-08-07T10:00:00.000Z"
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(Link{Id: "lnk-1", Status: status, ArchivedAt: archivedAt})
	}))
	defer server.Close()

	state := linkState("archiving", "")
	state.Attributes["archive_on_destroy"] = "true"
	d, err := schema.InternalMap(resourceLink().Schema).Data(state, nil)
	if err != nil {
		t.Fatalf("building resource data: %v", err)
	}
	if diags := LinkDeleteContext(context.Background(), d, newTestClient(server)); diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if patched {
		t.Error("joining a running archive must not re-PATCH")
	}
	if d.Id() != "" {
		t.Error("the resource must leave state once the join lands")
	}
}

// The two refusal arms of archive-on-destroy: the pre-read failing, and the
// archive PATCH itself being refused.
func TestLinkDelete_ArchiveOnDestroyRefusals(t *testing.T) {
	shortenPolling(t)
	for name, handler := range map[string]http.HandlerFunc{
		"pre-read fails": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("not json"))
		},
		"archive patch refused": func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "PATCH" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"message":"Service svc-1 must be active"}`))
				return
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(Link{Id: "lnk-1", Status: "active"})
		},
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewTLSServer(handler)
			defer server.Close()
			state := linkState("active", "")
			state.Attributes["archive_on_destroy"] = "true"
			d, err := schema.InternalMap(resourceLink().Schema).Data(state, nil)
			if err != nil {
				t.Fatalf("building resource data: %v", err)
			}
			if diags := LinkDeleteContext(context.Background(), d, newTestClient(server)); !diags.HasError() {
				t.Fatal("the destroy must surface the failure")
			}
		})
	}
}

// nullOpsWithNilLink drives the defensive `current == nil` arm: the HTTP
// client never returns (nil, nil) today, so the arm is only reachable through
// the interface — which is exactly what makes it contract rather than dead
// code: any future client that DOES answer nil must drop the resource from
// state instead of panicking.
type nullOpsWithNilLink struct{ NullOps }

func (nullOpsWithNilLink) GetLink(string) (*Link, error) { return nil, nil }

func TestLinkDelete_ArchiveOnDestroyOnAVanishedLinkJustForgetsIt(t *testing.T) {
	state := linkState("active", "")
	state.Attributes["archive_on_destroy"] = "true"
	d, err := schema.InternalMap(resourceLink().Schema).Data(state, nil)
	if err != nil {
		t.Fatalf("building resource data: %v", err)
	}
	if diags := LinkDeleteContext(context.Background(), d, nullOpsWithNilLink{}); diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if d.Id() != "" {
		t.Error("a vanished link must simply leave state")
	}
}

// The wait AFTER a successful archive-on-destroy PATCH can itself fail; the
// destroy must surface that instead of pretending the archive landed.
func TestLinkDelete_ArchiveOnDestroySurfacesAFailingWait(t *testing.T) {
	shortenPolling(t)
	var gets int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PATCH" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(Link{Id: "lnk-1", Status: "active"})
			return
		}
		if atomic.AddInt32(&gets, 1) == 1 {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(Link{Id: "lnk-1", Status: "active"})
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not json"))
	}))
	defer server.Close()

	state := linkState("active", "")
	state.Attributes["archive_on_destroy"] = "true"
	d, err := schema.InternalMap(resourceLink().Schema).Data(state, nil)
	if err != nil {
		t.Fatalf("building resource data: %v", err)
	}
	if diags := LinkDeleteContext(context.Background(), d, newTestClient(server)); !diags.HasError() {
		t.Fatal("a failing wait must surface")
	}
}
