package nullplatform

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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
