package nullplatform_test

import (
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/nullplatform/terraform-provider-nullplatform/nullplatform"
)

func TestAccResourceApprovalAction(t *testing.T) {
	applicationID := os.Getenv("NULLPLATFORM_APPLICATION_ID")

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckApprovalActionDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceApprovalAction_basic(applicationID),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckApprovalActionExists("nullplatform_approval_action.action"),
					resource.TestCheckResourceAttr("nullplatform_approval_action.action", "entity", "deployment"),
					resource.TestCheckResourceAttr("nullplatform_approval_action.action", "action", "deployment:create"),
					resource.TestCheckResourceAttr("nullplatform_approval_action.action", "dimensions.environment", "prod"),
					resource.TestCheckResourceAttr("nullplatform_approval_action.action", "on_policy_success", "approve"),
					resource.TestCheckResourceAttr("nullplatform_approval_action.action", "on_policy_fail", "deny"),
				),
			},
			{
				Config: testAccResourceApprovalAction_withPolicyAssociation(applicationID),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckApprovalActionExists("nullplatform_approval_action.action"),
					resource.TestCheckResourceAttrSet("nullplatform_approval_action.action", "policies.0"),
				),
			},
		},
	})
}

func testAccCheckApprovalActionExists(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("No ID is set for the resource")
		}

		client := testAccProviders["nullplatform"].Meta().(nullplatform.NullOps)
		if client == nil {
			return fmt.Errorf("provider meta is nil, ensure the provider is properly configured and initialized")
		}

		foundApprovalAction, err := client.GetApprovalAction(rs.Primary.ID)
		if err != nil {
			return err
		}

		if strconv.Itoa(foundApprovalAction.Id) != rs.Primary.ID {
			return fmt.Errorf("Approval action not found")
		}

		return nil
	}
}

func testAccCheckApprovalActionDestroy(s *terraform.State) error {
	client := testAccProviders["nullplatform"].Meta().(nullplatform.NullOps)
	if client == nil {
		return fmt.Errorf("provider meta is nil, ensure the provider is properly configured and initialized")
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "nullplatform_approval_action" {
			continue
		}

		_, err := client.GetApprovalAction(rs.Primary.ID)
		if err == nil {
			return fmt.Errorf("Approval action with ID %s still exists", rs.Primary.ID)
		}
	}

	return nil
}

func testAccResourceApprovalAction_basic(applicationID string) string {
	return fmt.Sprintf(`
data "nullplatform_application" "app" {
  id = %s
}

resource "nullplatform_approval_action" "action" {
  nrn        = data.nullplatform_application.app.nrn
  entity     = "deployment"
  action     = "deployment:create"
  dimensions = {
	environment = "prod"
  }
  on_policy_success = "approve"
  on_policy_fail    = "deny"
}
`, applicationID)
}

func testAccResourceApprovalAction_withPolicyAssociation(applicationID string) string {
	return fmt.Sprintf(`
data "nullplatform_application" "app" {
  id = %s
}

resource "nullplatform_approval_action" "action" {
  nrn        = data.nullplatform_application.app.nrn
  entity     = "deployment"
  action     = "deployment:create"
  dimensions = {
	environment = "prod"
  }
  on_policy_success = "approve"
  on_policy_fail    = "deny"
  policies          = [nullplatform_approval_policy.policy.id]
}

resource "nullplatform_approval_policy" "policy" {
  nrn        = data.nullplatform_application.app.nrn
  name       = "Memory <= 4Gb"
  conditions = jsonencode({
    "scope.requested_spec.memory_in_gb" = {
      "$lte" = 4
    }
  })
}
`, applicationID)
}
