package nullplatform

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// The API answers requested namespaces empty ({"aws":{}}); an NRN answered
// without them must read as unset keys, not panic.
func TestScopeRead_NrnWithoutAwsNamespace(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/nrn/") {
			w.Write([]byte(`{"nrn":"organization=1:scope=9","namespaces":{}}`))
			return
		}
		w.Write([]byte(`{"id":9,"nrn":"organization=1:scope=9","name":"s","capabilities":{}}`))
	}))
	defer server.Close()

	d := schema.TestResourceDataRaw(t, resourceScope().Schema, map[string]any{})
	d.SetId("9")
	if err := ScopeRead(d, newTestClient(server)); err != nil {
		t.Fatalf("ScopeRead: %v", err)
	}
	if got := d.Get("log_group_name"); got != "" {
		t.Errorf("log_group_name = %q, want empty", got)
	}
}
