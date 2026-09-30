package nullplatform

import (
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/nullplatform/terraform-provider-nullplatform/internal/fakeplatform"
)

const functionalScopeNrn = "organization=1:account=2:namespace=3:application=7:scope=1"

func newScopeFake(t *testing.T) (*fakeplatform.Server, *fakeplatform.NrnLog) {
	t.Helper()
	fake := fakeplatform.New()
	log := fakeplatform.RegisterScope(fake)
	t.Cleanup(fake.Close)
	return fake, log
}

func scopeConfig(extra string) string {
	return fmt.Sprintf(`
provider "nullplatform" {}

resource "nullplatform_scope" "lambda" {
  scope_name                           = "functional-scope"
  null_application_id                  = 7
  capabilities_serverless_handler_name = "handler"
  capabilities_serverless_runtime_id   = "provided.al2"
%s
}
`, extra)
}

const legacyScopeNrnFields = `
  log_group_name                  = "/aws/lambda/functional"
  lambda_function_name            = "functional"
  lambda_current_function_version = "1"
  lambda_function_role            = "arn:aws:iam::123456789012:role/lambda"
  lambda_function_main_alias      = "%s"
`

func patchedKeys(patch fakeplatform.Item) string {
	keys := make([]string, 0, len(patch))
	for key := range patch {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

func TestFunctionalScope_DeprecatedNrnFieldsAreOptional(t *testing.T) {
	fake, log := newScopeFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{
				Config: scopeConfig(""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_scope.lambda", "nrn", functionalScopeNrn),
					resource.TestCheckResourceAttr("nullplatform_scope.lambda", "log_group_name", ""),
					func(*terraform.State) error {
						if len(log.Patches) != 0 {
							return fmt.Errorf("sent %d NRN patches, want none: %v", len(log.Patches), log.Patches)
						}
						return nil
					},
				),
			},
			{
				// nullplatform_provider_config writes the keys; the scope must adopt them without a diff.
				PreConfig: func() {
					fake.Seed("nrn", functionalScopeNrn, fakeplatform.Item{
						"nrn": functionalScopeNrn,
						"namespaces": fakeplatform.Item{"aws": fakeplatform.Item{
							"log_group_name":     "/aws/lambda/from-provider-config",
							"lambdaFunctionName": "from-provider-config",
						}},
					})
				},
				Config: scopeConfig(""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_scope.lambda", "log_group_name", "/aws/lambda/from-provider-config"),
					resource.TestCheckResourceAttr("nullplatform_scope.lambda", "lambda_function_name", "from-provider-config"),
					func(*terraform.State) error {
						if len(log.Patches) != 0 {
							return fmt.Errorf("sent %d NRN patches, want none: %v", len(log.Patches), log.Patches)
						}
						return nil
					},
				),
			},
			{
				// Setting one key patches only that key, never the adopted values.
				Config: scopeConfig(`  lambda_function_main_alias = "PROD"`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_scope.lambda", "lambda_function_main_alias", "PROD"),
					resource.TestCheckResourceAttr("nullplatform_scope.lambda", "log_group_name", "/aws/lambda/from-provider-config"),
					func(*terraform.State) error {
						if len(log.Patches) != 1 || patchedKeys(log.Patches[0]) != "aws.lambdaFunctionMainAlias" {
							return fmt.Errorf("NRN patches = %v, want one carrying only aws.lambdaFunctionMainAlias", log.Patches)
						}
						return nil
					},
				),
			},
		},
	})
}

func TestFunctionalScope_LegacyNrnFieldsPatchUnchanged(t *testing.T) {
	fake, log := newScopeFake(t)
	const want = "aws.lambdaCurrentFunctionVersion,aws.lambdaFunctionMainAlias,aws.lambdaFunctionName,aws.lambdaFunctionRole,aws.log_group_name"

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{
				Config: scopeConfig(fmt.Sprintf(legacyScopeNrnFields, "DEV")),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_scope.lambda", "lambda_function_main_alias", "DEV"),
					func(*terraform.State) error {
						if len(log.Patches) != 1 || patchedKeys(log.Patches[0]) != want {
							return fmt.Errorf("NRN patches = %v, want one carrying %s", log.Patches, want)
						}
						return nil
					},
				),
			},
			{
				Config: scopeConfig(fmt.Sprintf(legacyScopeNrnFields, "PROD")),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_scope.lambda", "lambda_function_main_alias", "PROD"),
					func(*terraform.State) error {
						if len(log.Patches) != 2 || patchedKeys(log.Patches[1]) != want ||
							log.Patches[1]["aws.lambdaFunctionMainAlias"] != "PROD" {
							return fmt.Errorf("NRN patches = %v, want a second one carrying %s", log.Patches, want)
						}
						return nil
					},
				),
			},
		},
	})
}

func TestFunctionalScope_FailedNrnPatchKeepsScopeTracked(t *testing.T) {
	fake, log := newScopeFake(t)
	log.Refuse = &fakeplatform.Refusal{Status: http.StatusInternalServerError, Message: "nrn unavailable"}

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{
				Config:      scopeConfig(fmt.Sprintf(legacyScopeNrnFields, "DEV")),
				ExpectError: regexp.MustCompile(`error patching nrn resource, got 500`),
			},
			{
				// The tainted scope is replaced, never duplicated next to an orphan.
				PreConfig: func() { log.Refuse = nil },
				Config:    scopeConfig(fmt.Sprintf(legacyScopeNrnFields, "DEV")),
				Check: func(*terraform.State) error {
					live := 0
					for _, scope := range fake.Items("scope") {
						if fakeplatform.Str(scope, "status") != "deleted" {
							live++
						}
					}
					if live != 1 {
						return fmt.Errorf("%d live scopes, want 1: a failed NRN patch orphaned the first one", live)
					}
					return nil
				},
			},
		},
	})
}
