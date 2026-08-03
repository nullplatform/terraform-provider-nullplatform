package nullplatform

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestSpecificationDefaultFlagsSerialization pins the tri-state contract of
// use_default_actions and use_default_naming: an explicit true or false must
// reach the API, and an unconfigured attribute must stay out of the request
// body so the API keeps applying its own default.
//
// Root cause of the original bug: both fields were plain `bool` tagged with
// `omitempty`, and encoding/json drops such a field when it holds the zero
// value. Omitting the attribute and setting it to false produced the exact same
// payload, the backend then applied its own default (true), and disabling
// default actions from Terraform was impossible. Modelling them as *bool keeps
// omitempty working for the unset case while letting false through.
func TestSpecificationDefaultFlagsSerialization(t *testing.T) {
	boolPtr := func(v bool) *bool { return &v }

	for _, tt := range []struct {
		name string
		set  *bool
	}{
		{name: "explicit false", set: boolPtr(false)},
		{name: "explicit true", set: boolPtr(true)},
		{name: "unset", set: nil},
	} {
		t.Run("link/"+tt.name, func(t *testing.T) {
			body := captureRequestBody(t, func(c *NullClient) {
				if _, err := c.CreateLinkSpecification(&LinkSpecification{
					Name:              "test",
					UseDefaultActions: tt.set,
					UseDefaultNaming:  tt.set,
				}); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			})
			assertOptionalBoolField(t, body, "use_default_actions", tt.set)
			assertOptionalBoolField(t, body, "use_default_naming", tt.set)
		})

		t.Run("service/"+tt.name, func(t *testing.T) {
			body := captureRequestBody(t, func(c *NullClient) {
				if _, err := c.CreateServiceSpecification(&ServiceSpecification{
					Name:              "test",
					UseDefaultActions: tt.set,
					UseDefaultNaming:  tt.set,
				}); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			})
			assertOptionalBoolField(t, body, "use_default_actions", tt.set)
			assertOptionalBoolField(t, body, "use_default_naming", tt.set)
		})
	}
}

func captureRequestBody(t *testing.T, call func(*NullClient)) map[string]any {
	t.Helper()

	var raw []byte
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"spec-123"}`))
	}))
	defer server.Close()

	call(newTestClient(server))

	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("failed to decode captured request body %q: %v", raw, err)
	}
	return body
}

// assertOptionalBoolField checks that field carries *want, or is absent
// entirely when want is nil.
func assertOptionalBoolField(t *testing.T, body map[string]any, field string, want *bool) {
	t.Helper()

	got, present := body[field]

	if want == nil {
		if present {
			t.Errorf("%q present as %v in request body %v: an unconfigured attribute must be omitted so the API default applies", field, got, body)
		}
		return
	}

	if !present {
		t.Fatalf("%q missing from request body %v: a bool that must transmit false cannot be a plain bool with omitempty", field, body)
	}
	if got != *want {
		t.Errorf("%q = %v, want %v", field, got, *want)
	}
}
