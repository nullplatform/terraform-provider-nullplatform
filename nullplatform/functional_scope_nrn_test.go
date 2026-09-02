package nullplatform

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/nullplatform/terraform-provider-nullplatform/internal/fakeplatform"
)

const scopeNrn = "organization=1:account=2:namespace=3:application=4"

func newScopeFake(t *testing.T) (*fakeplatform.Server, *fakeplatform.ScopeLog) {
	t.Helper()
	fake := fakeplatform.New()
	log := fakeplatform.RegisterScope(fake, scopeNrn, fakeplatform.Item{
		"s3_assets_bucket":             "seeded-bucket",
		"scope_workflow_role":          "seeded-workflow-role",
		"log_group_name":               "seeded-log-group",
		"lambdaFunctionName":           "seeded-function",
		"lambdaCurrentFunctionVersion": "7",
		"lambdaFunctionRole":           "seeded-function-role",
		"lambdaFunctionMainAlias":      "seeded-main-alias",
		"log_reader_role":              "seeded-reader-role",
		"lambdaFunctionWarmAlias":      "seeded-warm-alias",
	})
	t.Cleanup(fake.Close)
	return fake, log
}

func scopeConfig(extra string) string {
	return fmt.Sprintf(`
provider "nullplatform" {}

resource "nullplatform_scope" "s" {
  scope_name          = "functional-scope"
  null_application_id = 4

  capabilities_serverless_runtime_id   = "nodejs20.x"
  capabilities_serverless_handler_name = "index.handler"
%s
}
`, extra)
}

// The NRN attributes are deprecated in favour of nullplatform_provider_config, so a scope
// must be declarable without them — and the plan that follows must be clean even though
// the NRN holds values for every one of them.
func TestFunctionalScope_DeprecatedNrnAttributesAreOptional(t *testing.T) {
	fake, _ := newScopeFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{
				Config: scopeConfig(""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_scope.s", "log_group_name", "seeded-log-group"),
					resource.TestCheckResourceAttr("nullplatform_scope.s", "s3_assets_bucket", "seeded-bucket"),
				),
			},
		},
	})
}

// With nothing to patch, no PATCH is sent: the empty-payload guard has to actually guard.
func TestFunctionalScope_NoNrnPatchWithoutDeprecatedAttributes(t *testing.T) {
	fake, log := newScopeFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{
				Config: scopeConfig(""),
				Check: func(*terraform.State) error {
					if len(log.NrnPatches) != 0 {
						return fmt.Errorf("NRN patches sent = %v, want none", log.NrnPatches)
					}
					return nil
				},
			},
		},
	})
}

// A declared attribute still reaches the NRN: the deprecation does not disable the path.
func TestFunctionalScope_DeclaredNrnAttributeIsPatched(t *testing.T) {
	fake, log := newScopeFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{
				Config: scopeConfig(`  log_group_name = "declared-log-group"`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_scope.s", "log_group_name", "declared-log-group"),
					func(*terraform.State) error {
						if len(log.NrnPatches) != 1 {
							return fmt.Errorf("NRN patches sent = %d, want 1", len(log.NrnPatches))
						}
						if got := log.NrnPatches[0]["aws.log_group_name"]; got != "declared-log-group" {
							return fmt.Errorf("patched log group = %v, want %q", got, "declared-log-group")
						}
						return nil
					},
				),
			},
		},
	})
}
