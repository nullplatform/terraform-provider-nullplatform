package nullplatform

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestClient(server *httptest.Server) *NullClient {
	host := strings.TrimPrefix(server.URL, "https://")
	host = strings.TrimPrefix(host, "http://")
	return &NullClient{
		Client: server.Client(),
		ApiURL: host,
		Token:  Token{AccessToken: "test-token"},
	}
}

// TestMakeRequest_ContentTypeOnlySetWhenBodyPresent verifies the fix for
// FST_ERR_CTP_EMPTY_JSON_BODY errors returned by Fastify v5+.
//
// Root cause: MakeRequest unconditionally set "Content-Type: application/json"
// regardless of whether a body was present. Fastify v3/v4 silently accepted
// bodyless requests with that header, but Fastify v5 (introduced in
// main-providers-api via chore(deps): update nullplatform packages) rejects
// them with a 400 FST_ERR_CTP_EMPTY_JSON_BODY error. DELETE requests (e.g.
// nullplatform_provider_config destroy) always pass a nil body, so they were
// the first operations to break after the Fastify upgrade.
func TestMakeRequest_ContentTypeOnlySetWhenBodyPresent(t *testing.T) {
	tests := []struct {
		name            string
		method          string
		body            *bytes.Buffer
		wantContentType bool
	}{
		{
			name:            "DELETE with nil body must not send Content-Type",
			method:          "DELETE",
			body:            nil,
			wantContentType: false,
		},
		{
			name:            "POST with JSON body must send Content-Type",
			method:          "POST",
			body:            bytes.NewBufferString(`{"key":"value"}`),
			wantContentType: true,
		},
		{
			name:            "GET with nil body must not send Content-Type",
			method:          "GET",
			body:            nil,
			wantContentType: false,
		},
		{
			name:            "PUT with JSON body must send Content-Type",
			method:          "PUT",
			body:            bytes.NewBufferString(`{"key":"value"}`),
			wantContentType: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedContentType string

			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedContentType = r.Header.Get("Content-Type")
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			client := newTestClient(server)
			_, err := client.MakeRequest(tt.method, "/test", tt.body)
			if err != nil {
				t.Fatalf("MakeRequest returned unexpected error: %v", err)
			}

			hasContentType := capturedContentType != ""
			if hasContentType != tt.wantContentType {
				t.Errorf("Content-Type header present=%v, want present=%v (got %q)",
					hasContentType, tt.wantContentType, capturedContentType)
			}
		})
	}
}
