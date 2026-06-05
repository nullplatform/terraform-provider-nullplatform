package nullplatform

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreateServiceAction_HappyPath(t *testing.T) {
	var gotPath, gotMethod string
	var gotBody ActionInstance

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(ActionInstance{Id: "act-1", Status: "pending", SpecificationId: "spec-1"})
	}))
	defer server.Close()

	c := newTestClient(server)
	got, err := c.CreateServiceAction("svc-9", &ActionInstance{
		SpecificationId: "spec-1",
		Parameters:      map[string]interface{}{"endpoint": "x"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Id != "act-1" {
		t.Errorf("got id %q, want act-1", got.Id)
	}
	if gotMethod != "POST" {
		t.Errorf("got method %q, want POST", gotMethod)
	}
	if gotPath != "/service/svc-9/action" {
		t.Errorf("got path %q, want /service/svc-9/action", gotPath)
	}
	if gotBody.SpecificationId != "spec-1" {
		t.Errorf("body.specification_id = %q, want spec-1", gotBody.SpecificationId)
	}
}

func TestCreateServiceAction_NonOkSurfaceStatusAndBody(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":"FST_ERR","message":"bad params"}`))
	}))
	defer server.Close()

	c := newTestClient(server)
	_, err := c.CreateServiceAction("svc-9", &ActionInstance{SpecificationId: "spec-1"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "status=400") {
		t.Errorf("error %q should contain status=400", err.Error())
	}
	if !strings.Contains(err.Error(), "bad params") {
		t.Errorf("error %q should include the response body", err.Error())
	}
}

func TestGetServiceAction_HappyPath(t *testing.T) {
	var gotPath, gotMethod string

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(ActionInstance{Id: "act-1", Status: "success"})
	}))
	defer server.Close()

	c := newTestClient(server)
	got, err := c.GetServiceAction("svc-9", "act-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != "GET" {
		t.Errorf("got method %q, want GET", gotMethod)
	}
	if gotPath != "/service/svc-9/action/act-1" {
		t.Errorf("got path %q, want /service/svc-9/action/act-1", gotPath)
	}
	if got.Status != "success" {
		t.Errorf("got status %q, want success", got.Status)
	}
}

func TestGetServiceAction_NonOkSurfaceStatusAndBody(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"not found"}`))
	}))
	defer server.Close()

	c := newTestClient(server)
	_, err := c.GetServiceAction("svc-9", "missing")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "status=404") {
		t.Errorf("error %q should contain status=404", err.Error())
	}
}

func TestPatchServiceAction_HappyPath(t *testing.T) {
	var gotPath, gotMethod string

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(ActionInstance{Id: "act-1", Status: "in_progress"})
	}))
	defer server.Close()

	c := newTestClient(server)
	got, err := c.PatchServiceAction("svc-9", "act-1", &ActionInstance{Status: "in_progress"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != "PATCH" {
		t.Errorf("got method %q, want PATCH", gotMethod)
	}
	if gotPath != "/service/svc-9/action/act-1" {
		t.Errorf("got path %q, want /service/svc-9/action/act-1", gotPath)
	}
	if got.Status != "in_progress" {
		t.Errorf("got status %q, want in_progress", got.Status)
	}
}

func TestDeleteServiceAction_HappyPath(t *testing.T) {
	var gotPath, gotMethod string

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	c := newTestClient(server)
	err := c.DeleteServiceAction("svc-9", "act-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != "DELETE" {
		t.Errorf("got method %q, want DELETE", gotMethod)
	}
	if gotPath != "/service/svc-9/action/act-1" {
		t.Errorf("got path %q, want /service/svc-9/action/act-1", gotPath)
	}
}

func TestDeleteServiceAction_NotFoundIsTolerated(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	c := newTestClient(server)
	if err := c.DeleteServiceAction("svc-9", "missing"); err != nil {
		t.Fatalf("expected nil error on 404, got %v", err)
	}
}
