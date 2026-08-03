package nullplatform

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
