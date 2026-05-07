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
