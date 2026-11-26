package nullplatform

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/nullplatform/terraform-provider-nullplatform/internal/fakeplatform"
)

const linkSpecificationExternalConfig = `
resource "nullplatform_service_specification" "publisher" {
  name          = "API Publisher"
  visible_to    = ["organization=1:account=2"]
  assignable_to = "any"
  type          = "dependency"
  dimensions    = jsonencode({})
  attributes    = jsonencode({})
  scopes        = jsonencode({})
  selectors {
    category     = ""
    imported     = false
    provider     = ""
    sub_category = ""
  }
}

resource "nullplatform_link_specification" "connect" {
  name             = "Connect"
  unique           = false
  specification_id = nullplatform_service_specification.publisher.id
  visible_to       = ["organization=1:account=2"]
  assignable_to    = "any"
  dimensions       = jsonencode({})
  attributes       = jsonencode({})
  scopes           = jsonencode({})
  external = jsonencode({
    action        = "list-plans"
    output_schema = { type = "object" }
  })
  selectors {
    category     = ""
    imported     = false
    provider     = ""
    sub_category = ""
  }
}
`

func dropStoredLinkSpecificationExternal(fake *fakeplatform.Server) {
	for _, item := range fake.Items("link_specification") {
		delete(item, "external")
	}
}

func storedLinkSpecificationExternalAction(fake *fakeplatform.Server, want string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		items := fake.Items("link_specification")
		if len(items) != 1 {
			return fmt.Errorf("expected one stored link specification, got %d", len(items))
		}
		external, _ := items[0]["external"].(map[string]any)
		if got := fmt.Sprint(external["action"]); got != want {
			return fmt.Errorf("stored external action is %q, want %q", got, want)
		}
		return nil
	}
}

func TestFunctionalLinkSpecificationExternal_SentAgainWhenTheAPIDoesNotHaveIt(t *testing.T) {
	fake := newSpecificationFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{
				Config: linkSpecificationExternalConfig,
				Check:  storedLinkSpecificationExternalAction(fake, "list-plans"),
			},
			{
				PreConfig: func() { dropStoredLinkSpecificationExternal(fake) },
				Config:    linkSpecificationExternalConfig,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("nullplatform_link_specification.connect", plancheck.ResourceActionUpdate),
					},
				},
				Check: storedLinkSpecificationExternalAction(fake, "list-plans"),
			},
			{
				Config: linkSpecificationExternalConfig,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}
