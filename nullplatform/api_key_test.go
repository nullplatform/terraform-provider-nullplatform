package nullplatform

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateApiKey_InternalFlag(t *testing.T) {
	internalTrue, internalFalse := true, false

	tests := []struct {
		name       string
		internal   *bool
		wantInBody bool
		wantValue  bool
	}{
		{
			name:       "unset leaves the API default alone",
			internal:   nil,
			wantInBody: false,
		},
		{
			name:       "true marks the key as internal",
			internal:   &internalTrue,
			wantInBody: true,
			wantValue:  true,
		},
		{
			name:       "explicit false is sent, not dropped",
			internal:   &internalFalse,
			wantInBody: true,
			wantValue:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got map[string]any

			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				raw, _ := io.ReadAll(r.Body)
				if err := json.Unmarshal(raw, &got); err != nil {
					t.Fatalf("request body is not valid JSON: %v", err)
				}

				w.WriteHeader(http.StatusCreated)
				w.Write([]byte(`{"id":123,"name":"test","api_key":"1.abc","masked_api_key":"1.abxxxxxabc"}`))
			}))
			defer server.Close()

			client := newTestClient(server)

			_, err := client.CreateApiKey(&CreateApiKeyRequestBody{
				Name:     "test",
				Grants:   []ApiKeyGrant{{NRN: "organization=1:account=1"}},
				Internal: tt.internal,
			})
			if err != nil {
				t.Fatalf("CreateApiKey returned an error: %v", err)
			}

			value, present := got["internal"]

			if present != tt.wantInBody {
				t.Fatalf("internal present in body = %v, want %v (body: %v)", present, tt.wantInBody, got)
			}

			if tt.wantInBody && value != tt.wantValue {
				t.Fatalf("internal = %v, want %v", value, tt.wantValue)
			}
		})
	}
}

func TestPatchApiKey_NeverSendsInternal(t *testing.T) {
	var got map[string]any

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("request body is not valid JSON: %v", err)
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":123,"name":"renamed"}`))
	}))
	defer server.Close()

	client := newTestClient(server)

	err := client.PatchApiKey(123, &PatchApiKeyRequestBody{Name: "renamed"})
	if err != nil {
		t.Fatalf("PatchApiKey returned an error: %v", err)
	}

	if _, present := got["internal"]; present {
		t.Fatalf("patch body must not contain internal, got: %v", got)
	}
}
