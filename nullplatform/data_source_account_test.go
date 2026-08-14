package nullplatform

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestDataSourceAccountRead_HappyPath(t *testing.T) {
	var gotPath string

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(Account{
			Id:                 123,
			Name:               "My Account",
			OrganizationId:     42,
			RepositoryPrefix:   "my-prefix",
			RepositoryProvider: "github",
			Slug:               "my-account",
			Status:             "active",
			Nrn:                "organization=42:account=123",
			Settings:           map[string]interface{}{"key": "value"},
		})
	}))
	defer server.Close()

	c := newTestClient(server)
	d := schema.TestResourceDataRaw(t, dataSourceAccount().Schema, map[string]interface{}{"id": 123})

	diags := dataSourceAccountRead(context.Background(), d, c)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if gotPath != "/account/123" {
		t.Errorf("got path %q, want /account/123", gotPath)
	}
	if d.Id() != "123" {
		t.Errorf("got id %q, want 123", d.Id())
	}
	if got := d.Get("slug").(string); got != "my-account" {
		t.Errorf("slug = %q, want my-account", got)
	}
	if got := d.Get("nrn").(string); got != "organization=42:account=123" {
		t.Errorf("nrn = %q, want organization=42:account=123", got)
	}
	if got := d.Get("name").(string); got != "My Account" {
		t.Errorf("name = %q, want My Account", got)
	}
	if got := d.Get("organization_id").(int); got != 42 {
		t.Errorf("organization_id = %d, want 42", got)
	}
	if got := d.Get("status").(string); got != "active" {
		t.Errorf("status = %q, want active", got)
	}
	if got := d.Get("settings").(string); got != `{"key":"value"}` {
		t.Errorf("settings = %q, want {\"key\":\"value\"}", got)
	}
}

func TestDataSourceAccountRead_DeletedAccountErrors(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(Account{Id: 123, Status: "deleted"})
	}))
	defer server.Close()

	c := newTestClient(server)
	d := schema.TestResourceDataRaw(t, dataSourceAccount().Schema, map[string]interface{}{"id": 123})

	diags := dataSourceAccountRead(context.Background(), d, c)
	if !diags.HasError() {
		t.Fatal("expected error diagnostics for deleted account, got none")
	}
}
