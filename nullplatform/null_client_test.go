package nullplatform

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
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

func TestMakeRequest_RetryOnGet(t *testing.T) {
	tests := []struct {
		name             string
		method           string
		failuresBefore2xx int32
		respondStatus    int
		wantAttempts     int32
		wantStatus       int
	}{
		{
			name:             "GET retries on 503 until success",
			method:           "GET",
			failuresBefore2xx: 2,
			respondStatus:    http.StatusServiceUnavailable,
			wantAttempts:     3,
			wantStatus:       http.StatusOK,
		},
		{
			name:             "GET stops after max retries",
			method:           "GET",
			failuresBefore2xx: 10,
			respondStatus:    http.StatusBadGateway,
			wantAttempts:     4,
			wantStatus:       http.StatusBadGateway,
		},
		{
			name:             "GET does not retry on 200",
			method:           "GET",
			failuresBefore2xx: 0,
			respondStatus:    http.StatusOK,
			wantAttempts:     1,
			wantStatus:       http.StatusOK,
		},
		{
			name:             "GET does not retry on 404",
			method:           "GET",
			failuresBefore2xx: 10,
			respondStatus:    http.StatusNotFound,
			wantAttempts:     1,
			wantStatus:       http.StatusNotFound,
		},
		{
			name:             "DELETE never retries on 503",
			method:           "DELETE",
			failuresBefore2xx: 10,
			respondStatus:    http.StatusServiceUnavailable,
			wantAttempts:     1,
			wantStatus:       http.StatusServiceUnavailable,
		},
		{
			name:             "POST never retries on 502",
			method:           "POST",
			failuresBefore2xx: 10,
			respondStatus:    http.StatusBadGateway,
			wantAttempts:     1,
			wantStatus:       http.StatusBadGateway,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var attempts int32

			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n := atomic.AddInt32(&attempts, 1)
				if n <= tt.failuresBefore2xx {
					w.WriteHeader(tt.respondStatus)
					return
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			client := newTestClient(server)
			res, err := client.MakeRequest(tt.method, "/test", nil)
			if err != nil {
				t.Fatalf("MakeRequest returned unexpected error: %v", err)
			}
			defer res.Body.Close()

			if got := atomic.LoadInt32(&attempts); got != tt.wantAttempts {
				t.Errorf("attempts = %d, want %d", got, tt.wantAttempts)
			}
			if res.StatusCode != tt.wantStatus {
				t.Errorf("status = %d, want %d", res.StatusCode, tt.wantStatus)
			}
		})
	}
}
