package nullplatform_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/nullplatform/terraform-provider-nullplatform/nullplatform"
)

func TestAccResourceApprovalActionPolicyAssociation(t *testing.T) {
	applicationID := os.Getenv("NULLPLATFORM_APPLICATION_ID")

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckApprovalActionPolicyAssociationDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceApprovalActionPolicyAssociation_basic(applicationID),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckApprovalActionPolicyAssociationExists("nullplatform_approval_action_policy_association.association"),
				),
			},
		},
	})
}

func testAccCheckApprovalActionPolicyAssociationExists(n string) resource.TestCheckFunc {
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

		// Get the action to verify the association exists
		action, err := client.GetApprovalAction(rs.Primary.Attributes["approval_action_id"])
		if err != nil {
			return err
		}

		// Check if the policy is associated
		found := false
		for _, policy := range action.Policies {
			if policy != nil && fmt.Sprintf("%d", policy.Id) == rs.Primary.Attributes["approval_policy_id"] {
				found = true
				break
			}
		}

		if !found {
			return fmt.Errorf("Approval action policy association not found")
		}

		return nil
	}
}

func testAccCheckApprovalActionPolicyAssociationDestroy(s *terraform.State) error {
	client := testAccProviders["nullplatform"].Meta().(nullplatform.NullOps)
	if client == nil {
		return fmt.Errorf("provider meta is nil, ensure the provider is properly configured and initialized")
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "nullplatform_approval_action_policy_association" {
			continue
		}

		// Get the action to verify the association is gone
		action, err := client.GetApprovalAction(rs.Primary.Attributes["approval_action_id"])
		if err != nil {
			return err
		}

		// Check if the policy is still associated
		for _, policy := range action.Policies {
			if policy != nil && fmt.Sprintf("%d", policy.Id) == rs.Primary.Attributes["approval_policy_id"] {
				return fmt.Errorf("Approval action policy association still exists")
			}
		}
	}

	return nil
}

func testAccResourceApprovalActionPolicyAssociation_basic(applicationID string) string {
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

resource "nullplatform_approval_policy" "policy" {
  nrn        = data.nullplatform_application.app.nrn
  name       = "Memory <= 4Gb"
  conditions = jsonencode({
    "scope.requested_spec.memory_in_gb" = {
      "$lte" = 4
    }
  })
}

resource "nullplatform_approval_action_policy_association" "association" {
  approval_action_id = nullplatform_approval_action.action.id
  approval_policy_id = nullplatform_approval_policy.policy.id
}
`, applicationID)
}
