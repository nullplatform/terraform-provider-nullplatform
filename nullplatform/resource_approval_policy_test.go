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

func TestAccResourceApprovalPolicy(t *testing.T) {
	applicationID := os.Getenv("NULLPLATFORM_APPLICATION_ID")

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckApprovalPolicyDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceApprovalPolicy_basic(applicationID),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckApprovalPolicyExists("nullplatform_approval_policy.policy"),
					resource.TestCheckResourceAttr("nullplatform_approval_policy.policy", "name", "Memory <= 4Gb"),
					//resource.TestCheckResourceAttr("nullplatform_approval_policy.policy", "conditions", ""),
				),
			},
		},
	})
}

func testAccCheckApprovalPolicyExists(n string) resource.TestCheckFunc {
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

		foundApprovalPolicy, err := client.GetApprovalPolicy(rs.Primary.ID)
		if err != nil {
			return err
		}

		if strconv.Itoa(foundApprovalPolicy.Id) != rs.Primary.ID {
			return fmt.Errorf("Approval policy not found")
		}

		return nil
	}
}

func testAccCheckApprovalPolicyDestroy(s *terraform.State) error {
	client := testAccProviders["nullplatform"].Meta().(nullplatform.NullOps)
	if client == nil {
		return fmt.Errorf("provider meta is nil, ensure the provider is properly configured and initialized")
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "nullplatform_approval_policy" {
			continue
		}

		_, err := client.GetApprovalPolicy(rs.Primary.ID)
		if err == nil {
			return fmt.Errorf("Approval policy with ID %s still exists", rs.Primary.ID)
		}
	}

	return nil
}

func testAccResourceApprovalPolicy_basic(applicationID string) string {
	return fmt.Sprintf(`
data "nullplatform_application" "app" {
  id = %s
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
