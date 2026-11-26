package nullplatform

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/nullplatform/terraform-provider-nullplatform/internal/fakeplatform"
)

// Real destroy over the fake platform: the platform deletes an application by
// moving it to `deleting`, and refuses a DELETE of one that is not.

func TestFunctionalApplication_DestroyGoesThroughDeleting(t *testing.T) {
	fake := newApplicationTypeFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		// One destroy takes the application and then its type: the type is
		// refused while an application of it exists, so the application must be
		// deleted, not only asked to be, before the type is.
		CheckDestroy: func(*terraform.State) error {
			asked := requestsTo(fake, http.MethodPatch, "/application/1")
			if len(asked) != 1 || asked[0].Body != `{"status":"deleting"}`+"\n" {
				return fmt.Errorf("want the application moved to deleting, got %v", asked)
			}
			if deletes := requestsTo(fake, http.MethodDelete, "/application/1"); len(deletes) != 0 {
				return fmt.Errorf("want no DELETE of the application, got %v", deletes)
			}
			if left := fake.Items("application"); len(left) != 1 || fakeplatform.Str(left[0], "status") != "deleted" {
				return fmt.Errorf("want the application deleted, got %v", left)
			}
			if types := fake.Items("application_type"); len(types) != 0 {
				return fmt.Errorf("want no type left, got %v", types)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				// The create does not wait for the type's workflow.
				Config: snapshotConfig(agentSchema, "", lowAutonomy),
				Check:  resource.TestCheckResourceAttr("nullplatform_application.bot", "status", "pending_hook"),
			},
			{
				Config: snapshotConfig(agentSchema, "", lowAutonomy),
				Check: resource.ComposeTestCheckFunc(
					// The type's workflow created it, and reported its repository:
					// read, with nothing to plan.
					resource.TestCheckResourceAttr("nullplatform_application.bot", "status", "active"),
					resource.TestCheckResourceAttr("nullplatform_application.bot", "repository_url", "https://github.com/acme/support-bot"),
				),
			},
		},
	})
}

func TestFunctionalApplication_WithNoTypeIsDestroyedTheSameWay(t *testing.T) {
	fake := newApplicationTypeFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		CheckDestroy: func(*terraform.State) error {
			if left := fake.Items("application"); len(left) != 1 || fakeplatform.Str(left[0], "status") != "deleted" {
				return fmt.Errorf("want the application deleted, got %v", left)
			}
			return nil
		},
		Steps: []resource.TestStep{{Config: `
resource "nullplatform_application" "api" {
  name           = "my-api"
  namespace_id   = 3
  repository_url = "https://github.com/my-org/my-api"
}
`}},
	})
}
