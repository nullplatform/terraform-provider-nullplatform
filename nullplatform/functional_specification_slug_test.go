package nullplatform

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"

	"github.com/nullplatform/terraform-provider-nullplatform/internal/fakeplatform"
)

// Real plan/apply over the fake platform: a specification created without a
// slug gets the derived one and plans clean afterwards; one created with a
// slug keeps it; changing the slug replaces the specification. The link
// specification follows the same contract.

func newSpecificationFake(t *testing.T) *fakeplatform.Server {
	t.Helper()
	fake := fakeplatform.New()
	fakeplatform.RegisterSpecifications(fake)
	t.Cleanup(fake.Close)
	return fake
}

func specificationSlugConfig(serviceSlug, linkSlug string) string {
	slugLine := func(slug string) string {
		if slug == "" {
			return ""
		}
		return fmt.Sprintf("  slug = %q\n", slug)
	}
	return fmt.Sprintf(`
resource "nullplatform_service_specification" "lambda" {
  name          = "AWS Lambda"
%s  visible_to    = ["organization=1:account=2"]
  assignable_to = "any"
  type          = "scope"
  dimensions    = jsonencode({})
  attributes    = jsonencode({})
  scopes        = jsonencode({})
  # The provider always sends selectors and reads them back as a block, so a
  # configuration declares the block to plan clean (out of this change's scope).
  selectors {
    category     = ""
    imported     = false
    provider     = ""
    sub_category = ""
  }
}

resource "nullplatform_link_specification" "access" {
  name             = "Bucket access"
%s  unique           = false
  specification_id = nullplatform_service_specification.lambda.id
  visible_to       = ["organization=1:account=2"]
  assignable_to    = "any"
  dimensions       = jsonencode({})
  attributes       = jsonencode({})
  scopes           = jsonencode({})
  selectors {
    category     = ""
    imported     = false
    provider     = ""
    sub_category = ""
  }
}
`, slugLine(serviceSlug), slugLine(linkSlug))
}

func TestFunctionalSpecificationSlug_DerivedWhenOmittedGivenWhenSet(t *testing.T) {
	fake := newSpecificationFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{
				Config: specificationSlugConfig("", ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_service_specification.lambda", "slug", "aws-lambda"),
					resource.TestCheckResourceAttr("nullplatform_link_specification.access", "slug", "bucket-access"),
				),
			},
			{
				// A slug that differs from the derived one is a new specification.
				Config: specificationSlugConfig("np-aws-lambda", "np-bucket-access"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("nullplatform_service_specification.lambda", plancheck.ResourceActionReplace),
						plancheck.ExpectResourceAction("nullplatform_link_specification.access", plancheck.ResourceActionReplace),
					},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_service_specification.lambda", "name", "AWS Lambda"),
					resource.TestCheckResourceAttr("nullplatform_service_specification.lambda", "slug", "np-aws-lambda"),
					resource.TestCheckResourceAttr("nullplatform_link_specification.access", "slug", "np-bucket-access"),
				),
			},
		},
	})
}
