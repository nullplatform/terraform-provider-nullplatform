package nullplatform

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateCapability_HappyPath(t *testing.T) {
	var gotPath, gotMethod string

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(CapabilityEntity{Id: 11, Name: "cpu", Target: "scope"})
	}))
	defer server.Close()

	c := newTestClient(server)
	got, err := c.CreateCapability(&CapabilityEntity{Name: "cpu", Target: "scope", Definition: map[string]interface{}{"type": "object"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != "POST" || gotPath != "/capability" {
		t.Errorf("got %s %s, want POST /capability", gotMethod, gotPath)
	}
	if got.Id != 11 {
		t.Errorf("got id %d, want 11", got.Id)
	}
}

func TestCreateDeploymentStrategy_HappyPath(t *testing.T) {
	var gotPath, gotMethod string
	var gotBody DeploymentStrategy

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(DeploymentStrategy{Id: 5, Name: gotBody.Name, Nrn: gotBody.Nrn})
	}))
	defer server.Close()

	c := newTestClient(server)
	got, err := c.CreateDeploymentStrategy(&DeploymentStrategy{Name: "rolling", Nrn: "organization=1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != "POST" || gotPath != "/deployment_strategy" {
		t.Errorf("got %s %s, want POST /deployment_strategy", gotMethod, gotPath)
	}
	if gotBody.Nrn != "organization=1" {
		t.Errorf("body.nrn = %q, want organization=1", gotBody.Nrn)
	}
	if got.Id != 5 {
		t.Errorf("got id %d, want 5", got.Id)
	}
}

func TestCreateScopeDomain_HappyPath(t *testing.T) {
	var gotPath, gotMethod string
	var gotBody ScopeDomain

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(ScopeDomain{Id: "uuid-1", Name: gotBody.Name, Type: gotBody.Type, Status: "active"})
	}))
	defer server.Close()

	c := newTestClient(server)
	got, err := c.CreateScopeDomain(&ScopeDomain{Name: "api.example.com", ScopeId: "99", Type: "custom"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != "POST" || gotPath != "/scope_domain" {
		t.Errorf("got %s %s, want POST /scope_domain", gotMethod, gotPath)
	}
	if gotBody.ScopeId != "99" {
		t.Errorf("body.scope_id = %q, want 99", gotBody.ScopeId)
	}
	if got.Id != "uuid-1" {
		t.Errorf("got id %q, want uuid-1", got.Id)
	}
}
