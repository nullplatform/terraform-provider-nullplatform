package nullplatform

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestDataSourceNamespaceRead_HappyPath(t *testing.T) {
	var gotPath string

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(Namespace{
			Id:        456,
			Name:      "My Namespace",
			AccountId: 123,
			Slug:      "my-namespace",
			Status:    "active",
			Nrn:       "organization=42:account=123:namespace=456",
		})
	}))
	defer server.Close()

	c := newTestClient(server)
	d := schema.TestResourceDataRaw(t, dataSourceNamespace().Schema, map[string]interface{}{"id": 456})

	diags := dataSourceNamespaceRead(context.Background(), d, c)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if gotPath != "/namespace/456" {
		t.Errorf("got path %q, want /namespace/456", gotPath)
	}
	if d.Id() != "456" {
		t.Errorf("got id %q, want 456", d.Id())
	}
	if got := d.Get("slug").(string); got != "my-namespace" {
		t.Errorf("slug = %q, want my-namespace", got)
	}
	if got := d.Get("nrn").(string); got != "organization=42:account=123:namespace=456" {
		t.Errorf("nrn = %q, want organization=42:account=123:namespace=456", got)
	}
	if got := d.Get("name").(string); got != "My Namespace" {
		t.Errorf("name = %q, want My Namespace", got)
	}
	if got := d.Get("account_id").(int); got != 123 {
		t.Errorf("account_id = %d, want 123", got)
	}
	if got := d.Get("status").(string); got != "active" {
		t.Errorf("status = %q, want active", got)
	}
}

func TestDataSourceNamespaceRead_DeletedNamespaceErrors(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(Namespace{Id: 456, Status: "deleted"})
	}))
	defer server.Close()

	c := newTestClient(server)
	d := schema.TestResourceDataRaw(t, dataSourceNamespace().Schema, map[string]interface{}{"id": 456})

	diags := dataSourceNamespaceRead(context.Background(), d, c)
	if !diags.HasError() {
		t.Fatal("expected error diagnostics for deleted namespace, got none")
	}
}
