package nullplatform

// Functional tests: the HashiCorp-recommended harness (terraform-plugin-testing)
// driving REAL `terraform plan / apply / import / destroy` cycles against the
// in-process provider, backed by internal/fakeplatform — a generic stateful
// REST engine plus the archive behavior module. Only ConfigureContextFunc is
// swapped, for a client that trusts the fake's TLS certificate.
//
//   - `resource.UnitTest` is `resource.Test` without the TF_ACC gate: the
//     framework's own mode for mock-backed runs. No credentials, every
//     `go test`, CI included.
//   - after every apply the framework re-plans and fails on any diff, so the
//     perpetual-diff regression class is checked on every step for free.
//   - a specification's ARCHIVE CHAIN is registered per test and keyed on
//     specification_id — the same document the real API resolves it from.
import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/nullplatform/terraform-provider-nullplatform/internal/fakeplatform"
)

func newFakePlatform(t *testing.T, chains map[string]fakeplatform.Chain) *fakeplatform.Server {
	t.Helper()
	fake := fakeplatform.New()
	fakeplatform.RegisterArchive(fake, chains)
	t.Cleanup(fake.Close)
	return fake
}

// functionalFactories serves the REAL provider with only its configure swapped
// for a client that trusts the fake's certificate.
func functionalFactories(fake *fakeplatform.Server) map[string]func() (*schema.Provider, error) {
	return map[string]func() (*schema.Provider, error){
		"nullplatform": func() (*schema.Provider, error) {
			provider := Provider()
			provider.ConfigureContextFunc = func(_ context.Context, _ *schema.ResourceData) (any, diag.Diagnostics) {
				return newTestClient(fake.HTTP()), nil
			}
			return provider, nil
		},
	}
}

func serviceConfig(spec, extra string) string {
	return fmt.Sprintf(`
provider "nullplatform" {}

resource "nullplatform_service" "db" {
  name             = "functional-redis"
  specification_id = %q
  entity_nrn       = "organization=1:account=2"
  import           = true
%s
}
`, spec, extra)
}
