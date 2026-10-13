package nullplatform

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestPatchProviderConfig_ReplacesAttributes(t *testing.T) {
	var gotPath, gotMethod string
	var gotBody ProviderConfig

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	attributes := map[string]interface{}{"region": "us-east-1"}
	c := newTestClient(server)
	if err := c.PatchProviderConfig("prov-1", &ProviderConfig{Attributes: attributes}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// A PATCH would merge, so attributes removed from the configuration would never be deleted.
	if gotMethod != "PUT" {
		t.Errorf("got method %q, want PUT", gotMethod)
	}
	if gotPath != "/provider/prov-1" {
		t.Errorf("got path %q, want /provider/prov-1", gotPath)
	}
	if !reflect.DeepEqual(gotBody.Attributes, attributes) {
		t.Errorf("got attributes %v, want %v", gotBody.Attributes, attributes)
	}
}
