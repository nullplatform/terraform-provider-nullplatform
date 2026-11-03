package nullplatform

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestCreateApplication_HappyPath(t *testing.T) {
	var gotPath, gotMethod string
	var gotBody Application

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(Application{Id: 7, Name: gotBody.Name, NamespaceId: gotBody.NamespaceId, Status: "pending"})
	}))
	defer server.Close()

	c := newTestClient(server)
	got, err := c.CreateApplication(&Application{Name: "my-api", NamespaceId: 42, RepositoryUrl: "https://example.com/repo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != "POST" {
		t.Errorf("got method %q, want POST", gotMethod)
	}
	if gotPath != "/application" {
		t.Errorf("got path %q, want /application", gotPath)
	}
	if gotBody.NamespaceId != 42 {
		t.Errorf("body.namespace_id = %d, want 42", gotBody.NamespaceId)
	}
	if got.Id != 7 {
		t.Errorf("got id %d, want 7", got.Id)
	}
}

func TestPatchApplication_HappyPath(t *testing.T) {
	var gotPath, gotMethod string
	var gotBody Application

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := newTestClient(server)
	err := c.PatchApplication("7", &Application{Name: "renamed"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != "PATCH" {
		t.Errorf("got method %q, want PATCH", gotMethod)
	}
	if gotPath != "/application/7" {
		t.Errorf("got path %q, want /application/7", gotPath)
	}
	if gotBody.Name != "renamed" {
		t.Errorf("body.name = %q, want renamed", gotBody.Name)
	}
}

func TestDeleteApplication_NotFoundIsTolerated(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	c := newTestClient(server)
	if err := c.DeleteApplication("missing"); err != nil {
		t.Fatalf("expected nil error on 404, got %v", err)
	}
}

// The API answers a deleted application with a 404 whose body has a numeric
// "status"; decoding it into Application used to fail before the status check.
const deletedApplicationBody = `{"type":"urn:nullplatform:error:nrn:terminal-entity-not-found","title":"Can't generate an NRN for this node specification","status":404,"code":"NRN.TERMINAL_ENTITY_NOT_FOUND","message":"Can't generate an NRN for this node specification","id":0}`

func TestGetApplication_NotFoundReturnsResourceNotFoundError(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(deletedApplicationBody))
	}))
	defer server.Close()

	c := newTestClient(server)
	_, err := c.GetApplication("835430870")
	nf, ok := IsResourceNotFoundError(err)
	if !ok {
		t.Fatalf("expected ResourceNotFoundError, got %v", err)
	}
	if nf.ID != 835430870 {
		t.Errorf("got id %d, want 835430870", nf.ID)
	}
}

func TestApplicationRead_DeletedApplicationIsRemovedFromState(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(deletedApplicationBody))
	}))
	defer server.Close()

	d := schema.TestResourceDataRaw(t, resourceApplication().Schema, map[string]interface{}{})
	d.SetId("835430870")

	if diags := ApplicationRead(context.Background(), d, newTestClient(server)); diags.HasError() {
		t.Fatalf("expected no error, got %v", diags)
	}
	if d.Id() != "" {
		t.Errorf("expected the resource to be removed from state, id is %q", d.Id())
	}
}

func TestApplicationRead_StatusDeletedIsRemovedFromState(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(Application{Id: 7, Name: "gone", Status: "deleted"})
	}))
	defer server.Close()

	d := schema.TestResourceDataRaw(t, resourceApplication().Schema, map[string]interface{}{})
	d.SetId("7")

	if diags := ApplicationRead(context.Background(), d, newTestClient(server)); diags.HasError() {
		t.Fatalf("expected no error, got %v", diags)
	}
	if d.Id() != "" {
		t.Errorf("expected the resource to be removed from state, id is %q", d.Id())
	}
}

func TestGetApplication_ServerErrorIsReturned(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"boom"}`))
	}))
	defer server.Close()

	c := newTestClient(server)
	_, err := c.GetApplication("7")
	if err == nil {
		t.Fatal("expected error on 500, got nil")
	}
	if _, ok := IsResourceNotFoundError(err); ok {
		t.Fatalf("a 500 must not be reported as not found, got %v", err)
	}
}

func TestGetApplication_InvalidBodyIsReturned(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not json`))
	}))
	defer server.Close()

	c := newTestClient(server)
	if _, err := c.GetApplication("7"); err == nil {
		t.Fatal("expected decode error, got nil")
	}
}
