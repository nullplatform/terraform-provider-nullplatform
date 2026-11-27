package nullplatform

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

// The platform numbers its errors with a UUID, says which error in `message`,
// and what it refused in `detail`.
func TestApplicationRefusal(t *testing.T) {
	cases := []struct{ name, body, want string }{
		{"message and detail",
			`{"type":"urn:nullplatform:error:application:invalid-application-type","status":400,"code":"APPLICATION.INVALID_APPLICATION_TYPE","detail":"The application type x cannot be used in namespace 3","message":"The application type cannot be used for this application","id":"5f3c1e2a-9b7d-4c1e-8a2f-6d4b3c2a1e0f"}`,
			"The application type cannot be used for this application: The application type x cannot be used in namespace 3"},
		{"a null detail", `{"status":400,"detail":null,"message":"The application status is invalid","id":"5f3c1e2a-9b7d-4c1e-8a2f-6d4b3c2a1e0f"}`, "The application status is invalid"},
		{"a detail that repeats the message", `{"detail":"refused","message":"refused","id":0}`, "refused"},
		{"a body with no message", "<html>Bad gateway</html>\n", "<html>Bad gateway</html>"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(c.body))
			}))
			defer server.Close()
			client := newTestClient(server)

			if _, err := client.CreateApplication(&Application{Name: "my-api"}); err == nil || !strings.HasSuffix(err.Error(), "message: "+c.want) {
				t.Errorf("create: want %q, got %v", c.want, err)
			}
			if err := client.PatchApplication("7", &Application{Name: "my-api"}); err == nil || !strings.HasSuffix(err.Error(), "message: "+c.want) {
				t.Errorf("patch: want %q, got %v", c.want, err)
			}
			if err := client.DeleteApplication("7"); err == nil || !strings.HasSuffix(err.Error(), "message: "+c.want) {
				t.Errorf("delete: want %q, got %v", c.want, err)
			}
			if _, err := client.GetApplication("7"); err == nil || !strings.HasSuffix(err.Error(), ": "+c.want) {
				t.Errorf("get: want %q, got %v", c.want, err)
			}
		})
	}
}

func TestSuppressSameID(t *testing.T) {
	if !suppressSameID("application_type_id", "0d0c2f0e-6c0b-4f1e-9d57-2f2f6d6a9b10", "0D0C2F0E-6C0B-4F1E-9D57-2F2F6D6A9B10", nil) {
		t.Error("want two spellings of one id to be no diff")
	}
	if suppressSameID("application_type_id", "0d0c2f0e-6c0b-4f1e-9d57-2f2f6d6a9b10", "", nil) {
		t.Error("want an id and none to be a diff")
	}
}
