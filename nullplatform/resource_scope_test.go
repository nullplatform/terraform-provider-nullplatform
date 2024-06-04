package nullplatform_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// TestResourceScope_basic tests the basic lifecycle of the Scope resource
func TestResourceScope_basic(t *testing.T) {
	applicationID := os.Getenv("NULLPLATFORM_APPLICATION_ID")

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckResourceScopeDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccScopeConfigBasic(applicationID),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckResourceScopeExists("nullplatform_scope.test"),
					resource.TestCheckResourceAttr("nullplatform_scope.test", "scope_name", "acc-test-scope"),
					resource.TestCheckResourceAttr("nullplatform_scope.test", "capabilities_serverless_runtime_id", "provided.al2"),
					resource.TestCheckResourceAttr("nullplatform_scope.test", "capabilities_serverless_handler_name", "handler"),
					resource.TestCheckResourceAttr("nullplatform_scope.test", "capabilities_serverless_ephemeral_storage", "512"),
					resource.TestCheckResourceAttr("nullplatform_scope.test", "log_group_name", "/aws/lambda/acc-test-lambda"),
					resource.TestCheckResourceAttr("nullplatform_scope.test", "lambda_function_name", "acc-test-lambda"),
					resource.TestCheckResourceAttr("nullplatform_scope.test", "lambda_current_function_version", "1"),
					resource.TestCheckResourceAttr("nullplatform_scope.test", "lambda_function_role", "arn:aws:iam::123456789012:role/lambda-role"),
					resource.TestCheckResourceAttr("nullplatform_scope.test", "lambda_function_main_alias", "DEV"),
					resource.TestCheckResourceAttr("nullplatform_scope.test", "null_application_id", applicationID),
				),
			},
		},
	})
}

func testAccCheckResourceScopeExists(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return nil
		}
		if rs.Primary.ID == "" {
			return nil
		}

		// Additional checks can be added here to verify the resource's state in the backend system
		return nil
	}
}

func testAccCheckResourceScopeDestroy(s *terraform.State) error {
	// Check that the resource has been destroyed in the backend system
	return nil
}

func testAccScopeConfigBasic(applicationID string) string {
	return fmt.Sprintf(`
resource "nullplatform_scope" "test" {
  null_application_id                       = %s
  scope_name                                = "acc-test-scope"
  capabilities_serverless_runtime_id        = "provided.al2"
  capabilities_serverless_handler_name      = "handler"
  capabilities_serverless_timeout           = 10
  capabilities_serverless_memory            = 1024
  capabilities_serverless_ephemeral_storage = 512
  log_group_name                            = "/aws/lambda/acc-test-lambda"
  lambda_function_name                      = "acc-test-lambda"
  lambda_current_function_version           = "1"
  lambda_function_role                      = "arn:aws:iam::123456789012:role/lambda-role"
  lambda_function_main_alias                = "DEV"
}
`, applicationID)
}
