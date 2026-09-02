package nullplatform

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/nullplatform/terraform-provider-nullplatform/internal/fakeplatform"
)

func newApiKeyFake(t *testing.T) (*fakeplatform.Server, *fakeplatform.ApiKeyLog) {
	t.Helper()
	fake := fakeplatform.New()
	log := fakeplatform.RegisterApiKey(fake)
	t.Cleanup(fake.Close)
	return fake, log
}

func apiKeyConfig(extra string) string {
	return fmt.Sprintf(`
provider "nullplatform" {}

resource "nullplatform_api_key" "agent" {
  name = "AGENT"

  grants {
    nrn       = "organization=1:account=2"
    role_slug = "controlplane:agent"
  }
%s
}
`, extra)
}

func TestFunctionalApiKey_InternalMarkedOnCreate(t *testing.T) {
	fake, log := newApiKeyFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{
				Config: apiKeyConfig(`  internal = true`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_api_key.agent", "internal", "true"),
					resource.TestCheckResourceAttrSet("nullplatform_api_key.agent", "api_key"),
					func(*terraform.State) error {
						if want := []any{true}; fmt.Sprint(log.Internal) != fmt.Sprint(want) {
							return fmt.Errorf("creates carried internal = %v, want %v", log.Internal, want)
						}
						return nil
					},
				),
			},
		},
	})
}

func TestFunctionalApiKey_InternalUnsetLeavesApiDefault(t *testing.T) {
	fake, log := newApiKeyFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{
				Config: apiKeyConfig(""),
				Check: func(*terraform.State) error {
					if len(log.Internal) != 1 || log.Internal[0] != nil {
						return fmt.Errorf("creates carried internal = %v, want one entry, absent", log.Internal)
					}
					return nil
				},
			},
		},
	})
}

func TestFunctionalApiKey_InternalChangeReplacesKey(t *testing.T) {
	fake, log := newApiKeyFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: apiKeyConfig(`  internal = true`)},
			{
				Config: apiKeyConfig(`  internal = false`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("nullplatform_api_key.agent", plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_api_key.agent", "internal", "false"),
					func(*terraform.State) error {
						if want := []any{true, false}; fmt.Sprint(log.Internal) != fmt.Sprint(want) {
							return fmt.Errorf("creates carried internal = %v, want %v", log.Internal, want)
						}
						return nil
					},
				),
			},
		},
	})
}

func TestFunctionalApiKey_ImportCannotRecoverInternal(t *testing.T) {
	fake, _ := newApiKeyFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: apiKeyConfig(`  internal = true`)},
			{
				ResourceName: "nullplatform_api_key.agent",
				ImportState:  true,
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					if len(states) != 1 {
						return fmt.Errorf("expected one imported instance, got %d", len(states))
					}
					if got, present := states[0].Attributes["internal"]; present {
						return fmt.Errorf("imported internal = %q, want it absent — the API does not expose the mark", got)
					}
					if got := states[0].Attributes["api_key"]; got != "" {
						return fmt.Errorf("imported api_key = %q, want it empty — the secret is returned only on create", got)
					}
					return nil
				},
			},
		},
	})
}
