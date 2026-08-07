package nullplatform

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// shortenPolling keeps the waiters inside the suite's timeout budget. Both
// intervals are vars for exactly this reason.
func shortenPolling(t *testing.T) {
	t.Helper()
	previousStatus, previousAction := statusPollInterval, actionPollInterval
	statusPollInterval = 10 * time.Millisecond
	actionPollInterval = 10 * time.Millisecond
	t.Cleanup(func() {
		statusPollInterval = previousStatus
		actionPollInterval = previousAction
	})
}

// archivedServiceState is the state Terraform holds for a service that was
// archived outside Terraform: the last refresh read `archived` back from the API.
func archivedServiceState() *terraform.InstanceState {
	return &terraform.InstanceState{
		ID: "svc-1",
		Attributes: map[string]string{
			"id":                 "svc-1",
			"name":               "redis-cache",
			"specification_id":   "spec-1",
			"entity_nrn":         "organization=1:account=2",
			"status":             "archived",
			"archived_at":        "2026-08-01T10:00:00.000Z",
			"import":             "true",
			"force_destroy":      "false",
			"archive_on_destroy": "false",
			"messages.#":         "0",
			"linkable_to.#":      "0",
			"selectors.#":        "0",
		},
	}
}

func serviceDiff(t *testing.T, state *terraform.InstanceState, config map[string]any) *terraform.InstanceDiff {
	t.Helper()
	diff, err := resourceService().Diff(
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

// A service archived out of band must not be silently restored by the next
// unrelated apply. This is what the removed `Default: "active"` used to do: the
// default filled the omitted attribute, the plan showed archived -> active, and
// the apply PATCHed the service back to life.
func TestServiceResource_ArchivedOutOfBandPlansClean(t *testing.T) {
	diff := serviceDiff(t, archivedServiceState(), map[string]any{
		"name":             "redis-cache",
		"specification_id": "spec-1",
		"entity_nrn":       "organization=1:account=2",
	})

	if diff != nil && !diff.Empty() {
		t.Fatalf("a configuration without `status` must plan clean against an archived service, got %s", diff.GoString())
	}
}

// The same guarantee stated as the thing that actually hurt: no `status`
// attribute may appear in the plan, whatever else the diff carries.
func TestServiceResource_ArchivedOutOfBandNeverPlansAStatusChange(t *testing.T) {
	state := archivedServiceState()
	// A pre-upgrade state: `archive_on_destroy` did not exist when it was written.
	delete(state.Attributes, "archive_on_destroy")

	diff := serviceDiff(t, state, map[string]any{
		"name":             "redis-cache",
		"specification_id": "spec-1",
		"entity_nrn":       "organization=1:account=2",
	})

	if diff == nil {
		return
	}
	if attr, planned := diff.Attributes["status"]; planned {
		t.Fatalf("planned a status change %q -> %q on an archived service the configuration says nothing about", attr.Old, attr.New)
	}
}

// Restoring stays possible, but only as something the operator wrote down.
func TestServiceResource_ExplicitActiveOnArchivedServiceIsADiff(t *testing.T) {
	diff := serviceDiff(t, archivedServiceState(), map[string]any{
		"name":             "redis-cache",
		"specification_id": "spec-1",
		"entity_nrn":       "organization=1:account=2",
		"status":           "active",
	})

	if diff == nil || diff.Empty() {
		t.Fatal("an explicit `status = \"active\"` on an archived service must show a diff")
	}
	attr, ok := diff.Attributes["status"]
	if !ok {
		t.Fatalf("expected a `status` attribute diff, got %s", diff.GoString())
	}
	if attr.Old != "archived" || attr.New != "active" {
		t.Errorf("got status diff %q -> %q, want archived -> active", attr.Old, attr.New)
	}
}

// The `status` default only ever meant "what a brand new service is created as".
// Keeping it out of the schema must not change what a create plans.
func TestServiceResource_CreateStillDefaultsStatusToActive(t *testing.T) {
	var gotBody Service
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(Service{Id: "svc-1", Name: gotBody.Name, Status: "active"})
	}))
	defer server.Close()

	d := schema.TestResourceDataRaw(t, resourceService().Schema, map[string]any{
		"name":             "redis-cache",
		"specification_id": "spec-1",
		"entity_nrn":       "organization=1:account=2",
	})

	if diags := ServiceCreateContext(context.Background(), d, newTestClient(server)); diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if gotBody.Status != "active" {
		t.Errorf("POST /service sent status %q, want active", gotBody.Status)
	}
	if got := d.Get("status").(string); got != "active" {
		t.Errorf("state kept status %q after create, want active", got)
	}
}

// Action-driven creates still POST `pending` so the specification's create
// action owns the transition to active.
func TestServiceResource_CreateInActionModeStillPostsPending(t *testing.T) {
	shortenPolling(t)
	var gotBody Service
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && r.URL.Path == "/service":
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(Service{Id: "svc-1", SpecificationId: "spec-1", Status: "pending"})
		case r.URL.Path == "/service_specification/spec-1/action_specification":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"results": []*ActionSpecification{{Id: "as-1", Type: "create"}}})
		case r.Method == "POST" && r.URL.Path == "/service/svc-1/action":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(ActionInstance{Id: "act-1", Status: "success"})
		case r.URL.Path == "/service/svc-1/action/act-1":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(ActionInstance{Id: "act-1", Status: "success"})
		case r.Method == "GET" && r.URL.Path == "/service/svc-1":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(Service{Id: "svc-1", Status: "active"})
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	d := schema.TestResourceDataRaw(t, resourceService().Schema, map[string]any{
		"name":             "redis-cache",
		"specification_id": "spec-1",
		"entity_nrn":       "organization=1:account=2",
		"import":           false,
	})

	if diags := ServiceCreateContext(context.Background(), d, newTestClient(server)); diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if gotBody.Status != "pending" {
		t.Errorf("POST /service sent status %q, want pending", gotBody.Status)
	}
	// The create action landed the service on active; state must show that
	// rather than the transient `pending` the POST returned.
	if got := d.Get("status").(string); got != "active" {
		t.Errorf("state kept status %q after the create action, want active", got)
	}
}

func serviceDataWithDiff(t *testing.T, state *terraform.InstanceState, config map[string]any) *schema.ResourceData {
	t.Helper()
	diff := serviceDiff(t, state, config)
	d, err := schema.InternalMap(resourceService().Schema).Data(state, diff)
	if err != nil {
		t.Fatalf("building resource data: %v", err)
	}
	return d
}

func TestServiceStatusTransition_ClassifiesTheVerb(t *testing.T) {
	activeState := func() *terraform.InstanceState {
		s := archivedServiceState()
		s.Attributes["status"] = "active"
		s.Attributes["archived_at"] = ""
		return s
	}

	cases := []struct {
		name           string
		state          *terraform.InstanceState
		config         map[string]any
		wantTransition string
		wantFrom       string
	}{
		{
			name:           "archived on an active service archives it",
			state:          activeState(),
			config:         map[string]any{"status": "archived"},
			wantTransition: "archive",
			wantFrom:       "active",
		},
		{
			name:           "active on an archived service restores it",
			state:          archivedServiceState(),
			config:         map[string]any{"status": "active"},
			wantTransition: "unarchive",
			wantFrom:       "archived",
		},
		{
			// The legacy direct write: `active` on a service that is not
			// archived has never been a lifecycle verb and must not start
			// polling.
			name:           "active on a failed service stays an ordinary update",
			state:          func() *terraform.InstanceState { s := activeState(); s.Attributes["status"] = "failed"; return s }(),
			config:         map[string]any{"status": "active"},
			wantTransition: "",
		},
		{
			name:           "a name change is not a status transition",
			state:          activeState(),
			config:         map[string]any{"name": "renamed"},
			wantTransition: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			config := map[string]any{
				"name":             "redis-cache",
				"specification_id": "spec-1",
				"entity_nrn":       "organization=1:account=2",
			}
			for k, v := range tc.config {
				config[k] = v
			}
			d := serviceDataWithDiff(t, tc.state, config)

			transition, from := statusTransition(d)
			if transition != tc.wantTransition {
				t.Errorf("got transition %q, want %q", transition, tc.wantTransition)
			}
			if tc.wantTransition != "" && from != tc.wantFrom {
				t.Errorf("got from-status %q, want %q", from, tc.wantFrom)
			}
		})
	}
}

// statusSequenceServer answers every GET /service/{id} with the next status in
// the list, repeating the last one forever.
func statusSequenceServer(t *testing.T, statuses ...string) (*httptest.Server, *int32) {
	t.Helper()
	var calls int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		index := int(atomic.AddInt32(&calls, 1)) - 1
		if index >= len(statuses) {
			index = len(statuses) - 1
		}
		s := Service{Id: "svc-1", Status: statuses[index]}
		if statuses[index] == "archived" {
			s.ArchivedAt = "2026-08-02T09:00:00.000Z"
		}
		if statuses[index] == "failed" {
			s.Messages = []any{map[string]any{"severity": "error", "message": "provider blew up"}}
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(s)
	}))
	return server, &calls
}

// An unmanaged specification flips the status inside the PATCH, so the waiter
// must return on its first read and never pay a poll interval.
func TestWaitForServiceStatusTerminal_SynchronousFlipCostsOneRead(t *testing.T) {
	shortenPolling(t)
	server, calls := statusSequenceServer(t, "archived")
	defer server.Close()

	s, err := waitForServiceStatusTerminal(context.Background(), newTestClient(server), "svc-1", "archive", "active", time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Status != "archived" {
		t.Errorf("got status %q, want archived", s.Status)
	}
	if s.ArchivedAt == "" {
		t.Error("expected archived_at to come back with the archived service")
	}
	if got := atomic.LoadInt32(calls); got != 1 {
		t.Errorf("made %d reads, want exactly 1", got)
	}
}

// A managed archive action mints and returns; the instance sits in `archiving`
// until the agent reports success.
func TestWaitForServiceStatusTerminal_WaitsOutTheManagedArchive(t *testing.T) {
	shortenPolling(t)
	server, calls := statusSequenceServer(t, "active", "archiving", "archiving", "archived")
	defer server.Close()

	s, err := waitForServiceStatusTerminal(context.Background(), newTestClient(server), "svc-1", "archive", "active", time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Status != "archived" {
		t.Errorf("got status %q, want archived", s.Status)
	}
	if got := atomic.LoadInt32(calls); got < 4 {
		t.Errorf("made %d reads, want at least 4 (the whole sequence)", got)
	}
}

// There is no `unarchiving` status: a restore transits `updating` and lands on
// the ordinary active success path.
func TestWaitForServiceStatusTerminal_RestoreTransitsUpdating(t *testing.T) {
	shortenPolling(t)
	server, _ := statusSequenceServer(t, "archived", "updating", "active")
	defer server.Close()

	s, err := waitForServiceStatusTerminal(context.Background(), newTestClient(server), "svc-1", "unarchive", "archived", time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Status != "active" {
		t.Errorf("got status %q, want active", s.Status)
	}
}

func TestWaitForServiceStatusTerminal_FailedTransitionSurfacesTheMessage(t *testing.T) {
	shortenPolling(t)
	server, _ := statusSequenceServer(t, "archiving", "failed")
	defer server.Close()

	_, err := waitForServiceStatusTerminal(context.Background(), newTestClient(server), "svc-1", "archive", "active", time.Minute)
	if err == nil {
		t.Fatal("expected an error when the archive ends in failed")
	}
	if !strings.Contains(err.Error(), "provider blew up") {
		t.Errorf("error %q should carry the service's error message", err.Error())
	}
}

// A failed service is archivable, so `failed` cannot double as the failure
// signal when that is where the transition started.
func TestWaitForServiceStatusTerminal_ArchivingAFailedServiceIsNotAFailure(t *testing.T) {
	shortenPolling(t)
	server, _ := statusSequenceServer(t, "failed", "archiving", "archived")
	defer server.Close()

	s, err := waitForServiceStatusTerminal(context.Background(), newTestClient(server), "svc-1", "archive", "failed", time.Minute)
	if err != nil {
		t.Fatalf("archiving a failed service must succeed, got %v", err)
	}
	if s.Status != "archived" {
		t.Errorf("got status %q, want archived", s.Status)
	}
}

// An update that only touches metadata must stay exactly as fast as it was: one
// PATCH, no polling.
func TestServiceUpdate_NonStatusChangeDoesNotPoll(t *testing.T) {
	var patches, gets int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "PATCH":
			atomic.AddInt32(&patches, 1)
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(Service{Id: "svc-1", Status: "active"})
		case "GET":
			atomic.AddInt32(&gets, 1)
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(Service{Id: "svc-1", Status: "active"})
		}
	}))
	defer server.Close()

	state := archivedServiceState()
	state.Attributes["status"] = "active"
	d := serviceDataWithDiff(t, state, map[string]any{
		"name":             "renamed",
		"specification_id": "spec-1",
		"entity_nrn":       "organization=1:account=2",
	})

	if diags := ServiceUpdateContext(context.Background(), d, newTestClient(server)); diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if got := atomic.LoadInt32(&patches); got != 1 {
		t.Errorf("made %d PATCHes, want 1", got)
	}
	if got := atomic.LoadInt32(&gets); got != 0 {
		t.Errorf("made %d GETs, want 0 — a metadata update must not poll", got)
	}
}

// Archiving through an apply: PATCH the status, then wait for the transition and
// record where it landed.
func TestServiceUpdate_ArchiveWaitsAndRecordsArchivedAt(t *testing.T) {
	shortenPolling(t)

	var patched Service
	var gets int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PATCH" {
			_ = json.NewDecoder(r.Body).Decode(&patched)
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(Service{Id: "svc-1", Status: "active"})
			return
		}
		status := "archiving"
		archivedAt := ""
		if atomic.AddInt32(&gets, 1) >= 2 {
			status, archivedAt = "archived", "2026-08-02T09:00:00.000Z"
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(Service{Id: "svc-1", Status: status, ArchivedAt: archivedAt})
	}))
	defer server.Close()

	state := archivedServiceState()
	state.Attributes["status"] = "active"
	state.Attributes["archived_at"] = ""
	d := serviceDataWithDiff(t, state, map[string]any{
		"name":             "redis-cache",
		"specification_id": "spec-1",
		"entity_nrn":       "organization=1:account=2",
		"status":           "archived",
	})

	if diags := ServiceUpdateContext(context.Background(), d, newTestClient(server)); diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if patched.Status != "archived" {
		t.Errorf("PATCH sent status %q, want archived", patched.Status)
	}
	if got := d.Get("status").(string); got != "archived" {
		t.Errorf("state kept status %q, want archived", got)
	}
	if got := d.Get("archived_at").(string); got != "2026-08-02T09:00:00.000Z" {
		t.Errorf("state kept archived_at %q, want the timestamp the API reported", got)
	}
}

func destroyResourceData(t *testing.T, raw map[string]any) *schema.ResourceData {
	t.Helper()
	d := schema.TestResourceDataRaw(t, resourceService().Schema, raw)
	d.SetId("svc-1")
	return d
}

// archive_on_destroy turns `terraform destroy` into an archive: the record must
// survive, so no DELETE may be issued.
func TestServiceDelete_ArchiveOnDestroyArchivesInsteadOfDeleting(t *testing.T) {
	shortenPolling(t)

	var patched Service
	var deletes int32
	var gets int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "DELETE":
			atomic.AddInt32(&deletes, 1)
			w.WriteHeader(http.StatusNoContent)
		case "PATCH":
			_ = json.NewDecoder(r.Body).Decode(&patched)
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(Service{Id: "svc-1"})
		default:
			status := "active"
			if atomic.AddInt32(&gets, 1) >= 2 {
				status = "archived"
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(Service{Id: "svc-1", Status: status})
		}
	}))
	defer server.Close()

	d := destroyResourceData(t, map[string]any{
		"name":               "redis-cache",
		"specification_id":   "spec-1",
		"entity_nrn":         "organization=1:account=2",
		"import":             true,
		"archive_on_destroy": true,
	})

	if diags := ServiceDeleteContext(context.Background(), d, newTestClient(server)); diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if patched.Status != "archived" {
		t.Errorf("PATCH sent status %q, want archived", patched.Status)
	}
	if got := atomic.LoadInt32(&deletes); got != 0 {
		t.Errorf("made %d DELETEs, want 0 — archive_on_destroy must not delete the record", got)
	}
	if d.Id() != "" {
		t.Error("the resource must be dropped from state after the archive")
	}
}

// The escape hatch outranks the flag: someone applying force_destroy wants the
// record gone, not preserved.
func TestServiceDelete_ForceDestroyBeatsArchiveOnDestroy(t *testing.T) {
	var deletes int32
	var patches int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			atomic.AddInt32(&deletes, 1)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		atomic.AddInt32(&patches, 1)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(Service{Id: "svc-1"})
	}))
	defer server.Close()

	d := destroyResourceData(t, map[string]any{
		"name":               "redis-cache",
		"specification_id":   "spec-1",
		"entity_nrn":         "organization=1:account=2",
		"import":             false,
		"force_destroy":      true,
		"archive_on_destroy": true,
	})

	if diags := ServiceDeleteContext(context.Background(), d, newTestClient(server)); diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if got := atomic.LoadInt32(&deletes); got != 1 {
		t.Errorf("made %d DELETEs, want 1", got)
	}
	if got := atomic.LoadInt32(&patches); got != 0 {
		t.Errorf("made %d PATCHes, want 0", got)
	}
}

// Re-archiving an archived service is refused by the API, so a destroy that
// finds one already archived just drops it from state.
func TestServiceDelete_ArchiveOnDestroyIsANoOpOnAnArchivedService(t *testing.T) {
	var writes int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			atomic.AddInt32(&writes, 1)
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(Service{Id: "svc-1", Status: "archived"})
	}))
	defer server.Close()

	d := destroyResourceData(t, map[string]any{
		"name":               "redis-cache",
		"specification_id":   "spec-1",
		"entity_nrn":         "organization=1:account=2",
		"import":             true,
		"archive_on_destroy": true,
	})

	if diags := ServiceDeleteContext(context.Background(), d, newTestClient(server)); diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if got := atomic.LoadInt32(&writes); got != 0 {
		t.Errorf("made %d write requests, want 0 on an already archived service", got)
	}
	if d.Id() != "" {
		t.Error("the resource must be dropped from state")
	}
}

// The delete action never runs on a service that is mid-archive, and the mint's
// refusal is an opaque 400. Say what is wrong instead of relaying it.
func TestServiceDelete_MidArchiveServiceGetsAnActionableError(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("unexpected %s request while the service is archiving", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(Service{Id: "svc-1", Status: "archiving"})
	}))
	defer server.Close()

	d := destroyResourceData(t, map[string]any{
		"name":             "redis-cache",
		"specification_id": "spec-1",
		"entity_nrn":       "organization=1:account=2",
		"import":           false,
	})

	diags := ServiceDeleteContext(context.Background(), d, newTestClient(server))
	if !diags.HasError() {
		t.Fatal("expected an error when destroying a service that is being archived")
	}
	if !strings.Contains(diags[0].Summary, "force_destroy") {
		t.Errorf("the error should point at the escape hatch, got %q", diags[0].Summary)
	}
}

// Delete from archive runs the ordinary delete action: the API's status guard
// admits archived, so nothing special happens here.
func TestServiceDelete_ArchivedServiceUsesTheDeleteAction(t *testing.T) {
	shortenPolling(t)
	var mintedActions int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/service/svc-1":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(Service{Id: "svc-1", Status: "archived"})
		case r.URL.Path == "/service_specification/spec-1/action_specification":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"results": []*ActionSpecification{{Id: "as-9", Type: "delete"}}})
		case r.Method == "POST" && r.URL.Path == "/service/svc-1/action":
			atomic.AddInt32(&mintedActions, 1)
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(ActionInstance{Id: "act-1", Status: "success"})
		case r.URL.Path == "/service/svc-1/action/act-1":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(ActionInstance{Id: "act-1", Status: "success"})
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	d := destroyResourceData(t, map[string]any{
		"name":             "redis-cache",
		"specification_id": "spec-1",
		"entity_nrn":       "organization=1:account=2",
		"import":           false,
	})

	if diags := ServiceDeleteContext(context.Background(), d, newTestClient(server)); diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if got := atomic.LoadInt32(&mintedActions); got != 1 {
		t.Errorf("minted %d delete actions, want 1", got)
	}
	if d.Id() != "" {
		t.Error("the resource must be dropped from state")
	}
}

// The API refuses an archive request that carries attributes (the minted
// action's parameters are `{}` and the direct flip writes none). Changing both
// in one apply is ordinary Terraform, so the provider sends the attributes on
// their own first instead of handing the operator a 400.
func TestServiceUpdate_AttributesAndArchiveTravelInSeparatePatches(t *testing.T) {
	shortenPolling(t)

	var patches []Service
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PATCH" {
			var body Service
			_ = json.NewDecoder(r.Body).Decode(&body)
			patches = append(patches, body)
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(Service{Id: "svc-1", Status: "active"})
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(Service{Id: "svc-1", Status: "archived", ArchivedAt: "2026-08-02T09:00:00.000Z"})
	}))
	defer server.Close()

	state := archivedServiceState()
	state.Attributes["status"] = "active"
	state.Attributes["archived_at"] = ""
	state.Attributes["attributes.%"] = "1"
	state.Attributes["attributes.size"] = "small"
	d := serviceDataWithDiff(t, state, map[string]any{
		"name":             "redis-cache",
		"specification_id": "spec-1",
		"entity_nrn":       "organization=1:account=2",
		"status":           "archived",
		"attributes":       map[string]any{"size": "large"},
	})

	if diags := ServiceUpdateContext(context.Background(), d, newTestClient(server)); diags.HasError() {
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

// An ordinary attribute change is still one PATCH — the split is reserved for
// the combination the API refuses.
func TestServiceUpdate_AttributesWithoutATransitionStaySingle(t *testing.T) {
	var patches int
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PATCH" {
			patches++
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(Service{Id: "svc-1", Status: "active"})
	}))
	defer server.Close()

	state := archivedServiceState()
	state.Attributes["status"] = "active"
	state.Attributes["archived_at"] = ""
	state.Attributes["attributes.%"] = "1"
	state.Attributes["attributes.size"] = "small"
	d := serviceDataWithDiff(t, state, map[string]any{
		"name":             "redis-cache",
		"specification_id": "spec-1",
		"entity_nrn":       "organization=1:account=2",
		"attributes":       map[string]any{"size": "large"},
	})

	if diags := ServiceUpdateContext(context.Background(), d, newTestClient(server)); diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if patches != 1 {
		t.Errorf("expected a single PATCH, got %d", patches)
	}
}

// approvalSequenceServer answers each successive GET with the next
// (status, actions_in_progress) pair, holding the last one thereafter.
func approvalSequenceServer(t *testing.T, steps []Service) *httptest.Server {
	t.Helper()
	var calls int32
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		i := int(atomic.AddInt32(&calls, 1)) - 1
		if i >= len(steps) {
			i = len(steps) - 1
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(steps[i])
	}))
}

// An archive behind an approval policy parks its action in `pending_create` and
// the service stays `active` — which must read as "waiting", not "stuck". When
// the approval lands, the action leaves `pending_create` for a RUNNING status
// while the service still reads `active` for a beat; mistaking that window for
// a denial was a real bug in the first draft of this waiter, which keyed the
// denial check on "no longer parked" instead of "no longer present".
func TestWaitForServiceStatusTerminal_ApprovedMidWaitCompletes(t *testing.T) {
	shortenPolling(t)
	parked := []ActionInProgress{{Id: "act-1", Type: "archive", Status: "pending_create"}}
	approved := []ActionInProgress{{Id: "act-1", Type: "archive", Status: "in_progress"}}
	server := approvalSequenceServer(t, []Service{
		{Id: "svc-1", Status: "active", ActionsInProgress: parked},
		{Id: "svc-1", Status: "active", ActionsInProgress: approved},
		{Id: "svc-1", Status: "archiving", ActionsInProgress: approved},
		{Id: "svc-1", Status: "archived", ArchivedAt: "2026-08-07T09:00:00.000Z"},
	})
	defer server.Close()

	s, err := waitForServiceStatusTerminal(context.Background(), newTestClient(server), "svc-1", "archive", "active", time.Minute)
	if err != nil {
		t.Fatalf("an approved archive must complete, got: %v", err)
	}
	if s.Status != "archived" {
		t.Errorf("got status %q, want archived", s.Status)
	}
}

// A denied (or hand-cancelled) approval removes the action from
// actions_in_progress while the status never moves. Nothing will ever move
// again, so the waiter must say so promptly instead of running out the timeout.
func TestWaitForServiceStatusTerminal_DeniedApprovalFailsFast(t *testing.T) {
	shortenPolling(t)
	parked := []ActionInProgress{{Id: "act-1", Type: "archive", Status: "pending_create"}}
	server := approvalSequenceServer(t, []Service{
		{Id: "svc-1", Status: "active", ActionsInProgress: parked},
		{Id: "svc-1", Status: "active"},
	})
	defer server.Close()

	_, err := waitForServiceStatusTerminal(context.Background(), newTestClient(server), "svc-1", "archive", "active", time.Minute)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "act-1") || !strings.Contains(err.Error(), "cancelled while waiting") {
		t.Errorf("error %q should name the cancelled action and the reason", err.Error())
	}
	if !strings.Contains(err.Error(), "Re-run apply") {
		t.Errorf("error %q should tell the operator how to try again", err.Error())
	}
}

// An approval nobody answers holds the instance at its from-status until the
// update timeout. The timeout error must carry the reason — the platform is not
// stuck, an action is waiting for a human — instead of a generic "timed out
// waiting for state".
func TestWaitForServiceStatusTerminal_UnansweredApprovalExplainsTheTimeout(t *testing.T) {
	shortenPolling(t)
	parked := []ActionInProgress{{Id: "act-1", Type: "archive", Status: "pending_create"}}
	server := approvalSequenceServer(t, []Service{
		{Id: "svc-1", Status: "active", ActionsInProgress: parked},
	})
	defer server.Close()

	_, err := waitForServiceStatusTerminal(context.Background(), newTestClient(server), "svc-1", "archive", "active", 100*time.Millisecond)
	if err == nil {
		t.Fatal("expected a timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "waiting for approval") || !strings.Contains(err.Error(), "act-1") {
		t.Errorf("timeout error %q should explain the parked approval", err.Error())
	}
}

// A parked action of a DIFFERENT type (a custom verb behind its own approval)
// must not be blamed for this wait: the transition match is what keeps the
// explanation honest.
func TestWaitForServiceStatusTerminal_UnrelatedParkedActionIsNotBlamed(t *testing.T) {
	shortenPolling(t)
	unrelated := []ActionInProgress{{Id: "act-9", Type: "custom", Status: "pending_create"}}
	server := approvalSequenceServer(t, []Service{
		{Id: "svc-1", Status: "active", ActionsInProgress: unrelated},
		{Id: "svc-1", Status: "archiving", ActionsInProgress: unrelated},
		{Id: "svc-1", Status: "archived", ArchivedAt: "2026-08-07T09:00:00.000Z"},
	})
	defer server.Close()

	s, err := waitForServiceStatusTerminal(context.Background(), newTestClient(server), "svc-1", "archive", "active", time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Status != "archived" {
		t.Errorf("got status %q, want archived", s.Status)
	}
}
