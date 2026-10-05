package nullplatform

// Functional: a service's linkable_to always keeps its owning entity_nrn.
// The API appends the owner on create; a PATCH replaces the list as sent, so
// a provider that sends only the configured NRNs strips the owner and the
// service disappears from GET /service?nrn=<owner>. Harness in
// functional_harness_test.go.

import (
	"fmt"
	"slices"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/nullplatform/terraform-provider-nullplatform/internal/fakeplatform"
)

const linkableOwner = "organization=1:account=2"

// The stored service still lists its owner — what the API filters
// GET /service?nrn=<owner> on.
func ownerStillLinkable(fake *fakeplatform.Server) resource.TestCheckFunc {
	return func(*terraform.State) error {
		for _, svc := range fake.Items("service") {
			linkable, _ := svc["linkable_to"].([]any)
			if !slices.Contains(linkable, any(linkableOwner)) {
				return fmt.Errorf("owner %s missing from stored linkable_to %v", linkableOwner, linkable)
			}
		}
		return nil
	}
}

// Create with a linkable_to that omits the owner, then change it: the plan
// stays clean after each apply and the PATCH never strips the owner.
func TestFunctionalService_LinkableToKeepsOwner(t *testing.T) {
	fake := newFakePlatform(t, nil)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{
				Config: serviceConfig("spec-1", `  linkable_to = ["organization=1:account=3"]`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_service.db", "linkable_to.#", "2"),
					resource.TestCheckResourceAttr("nullplatform_service.db", "linkable_to.0", "organization=1:account=3"),
					resource.TestCheckResourceAttr("nullplatform_service.db", "linkable_to.1", linkableOwner),
					ownerStillLinkable(fake),
				),
			},
			{
				Config: serviceConfig("spec-1", `  linkable_to = ["organization=1:account=4"]`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_service.db", "linkable_to.#", "2"),
					resource.TestCheckResourceAttr("nullplatform_service.db", "linkable_to.0", "organization=1:account=4"),
					ownerStillLinkable(fake),
				),
			},
			{
				ResourceName:            "nullplatform_service.db",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"import", "force_destroy", "archive_on_destroy"},
			},
		},
	})
}

// Declaring the owner explicitly is not duplicated, and omitting linkable_to
// keeps what the API holds instead of planning the owner's removal forever.
func TestFunctionalService_LinkableToOwnerDeclaredOrOmitted(t *testing.T) {
	fake := newFakePlatform(t, nil)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{
				Config: serviceConfig("spec-1", fmt.Sprintf(`  linkable_to = [%q]`, linkableOwner)),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_service.db", "linkable_to.#", "1"),
					ownerStillLinkable(fake),
				),
			},
			{
				Config: serviceConfig("spec-1", ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_service.db", "linkable_to.#", "1"),
					ownerStillLinkable(fake),
				),
			},
		},
	})
}

// An entity_nrn only known after apply (typically another resource's nrn)
// leaves linkable_to unknown at plan time; the API's answer settles it.
func TestFunctionalService_LinkableToWithUnknownOwner(t *testing.T) {
	fake := newFakePlatform(t, nil)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
provider "nullplatform" {}

resource "terraform_data" "owner" {
  input = %q
}

resource "nullplatform_service" "db" {
  name             = "functional-redis"
  specification_id = "spec-1"
  entity_nrn       = terraform_data.owner.output
  linkable_to      = ["organization=1:account=3"]
  import           = true
}
`, linkableOwner),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_service.db", "linkable_to.#", "2"),
					resource.TestCheckResourceAttr("nullplatform_service.db", "linkable_to.1", linkableOwner),
					ownerStillLinkable(fake),
				),
			},
		},
	})
}
