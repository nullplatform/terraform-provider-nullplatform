package nullplatform

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDeleteService_ForceQueryParam(t *testing.T) {
	tests := []struct {
		name      string
		force     bool
		wantQuery string
	}{
		{name: "without force", force: false, wantQuery: ""},
		{name: "with force", force: true, wantQuery: "force=true"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotQuery string
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotQuery = r.URL.RawQuery
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()

			c := newTestClient(server)
			if err := c.DeleteService("svc-123", tt.force); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(gotQuery, tt.wantQuery) {
				t.Errorf("got query %q, want it to contain %q", gotQuery, tt.wantQuery)
			}
		})
	}
}

// Archive and restore refusals all arrive as a 400 whose body is the only thing
// that names the guard that rejected the request. Terraform shows the operator
// the error, so the body has to travel in it.
func TestPatchService_SurfacesTheRefusalMessage(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"Service has non-archived links and cannot be archived; archive its links first"}`))
	}))
	defer server.Close()

	c := newTestClient(server)
	err := c.PatchService("svc-123", &Service{Status: "archived"})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("error %q should carry the status code", err.Error())
	}
	if !strings.Contains(err.Error(), "non-archived links") {
		t.Errorf("error %q should carry the API's message", err.Error())
	}
}

// archived_at rides the ordinary read contract, and is null while the service is
// not archived.
func TestGetService_DecodesArchivedAt(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{name: "archived", body: `{"id":"svc-1","status":"archived","archived_at":"2026-08-02T09:00:00.000Z"}`, want: "2026-08-02T09:00:00.000Z"},
		{name: "not archived", body: `{"id":"svc-1","status":"active","archived_at":null}`, want: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()

			s, err := newTestClient(server).GetService("svc-1")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if s.ArchivedAt != tc.want {
				t.Errorf("got archived_at %q, want %q", s.ArchivedAt, tc.want)
			}
		})
	}
}
