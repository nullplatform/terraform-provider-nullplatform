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

// TestResourceScope tests the lifecycle of the Scope resource, including recreation after deletion
func TestResourceScope(t *testing.T) {
	var scopeID int
	applicationID := os.Getenv("NULLPLATFORM_APPLICATION_ID")

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckResourceScopeDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccScopeConfig_basic(applicationID),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckResourceScopeExists("nullplatform_scope.test", &scopeID),
					resource.TestCheckResourceAttr("nullplatform_scope.test", "scope_name", "acc-test-scope"),
					resource.TestCheckResourceAttr("nullplatform_scope.test", "capabilities_serverless_runtime_id", "provided.al2"),
					resource.TestCheckResourceAttr("nullplatform_scope.test", "capabilities_serverless_runtime_platform", "x86_64"),
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
			{
				// Step to delete the resource and ensure Terraform recreates it
				PreConfig: func() {
					client := testAccProviders["nullplatform"].Meta().(nullplatform.NullOps)
					scopeIDStr := strconv.Itoa(scopeID)
					err := client.DeleteScope(scopeIDStr)
					if err != nil {
						t.Fatalf("Error deleting scope: %s", err)
					}
				},
				Config: testAccScopeConfig_basic(applicationID),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckResourceScopeExists("nullplatform_scope.test", &scopeID),
				),
			},
			{
				Config: testAccScopeConfig_basicArm64(applicationID),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckResourceScopeExists("nullplatform_scope.test", &scopeID),
					resource.TestCheckResourceAttr("nullplatform_scope.test", "scope_name", "acc-test-scope"),
					resource.TestCheckResourceAttr("nullplatform_scope.test", "capabilities_serverless_runtime_id", "provided.al2"),
					resource.TestCheckResourceAttr("nullplatform_scope.test", "capabilities_serverless_runtime_platform", "arm_64"),
				),
			},
			{
				Config: testAccScopeConfig_basicWithAssetName(applicationID),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckResourceScopeExists("nullplatform_scope.test", &scopeID),
					resource.TestCheckResourceAttr("nullplatform_scope.test", "scope_name", "acc-test-scope"),
					resource.TestCheckResourceAttr("nullplatform_scope.test", "scope_asset_name", "the-secret-algo-lambda-asset"),
				),
			},
		},
	})
}

func testAccCheckResourceScopeExists(n string, scopeID *int) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Resource not found: %s", n)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("No ID is set for the resource")
		}

		client := testAccProviders["nullplatform"].Meta().(nullplatform.NullOps)
		scope, err := client.GetScope(rs.Primary.ID)
		if err != nil {
			return fmt.Errorf("error fetching scope: %s", err)
		}

		*scopeID = scope.Id

		return nil
	}
}

func testAccCheckResourceScopeDestroy(s *terraform.State) error {
	client := testAccProviders["nullplatform"].Meta().(nullplatform.NullOps)
	if client == nil {
		return fmt.Errorf("provider meta is nil, ensure the provider is properly configured and initialized")
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "nullplatform_scope" {
			continue
		}

		_, err := client.GetScope(rs.Primary.ID)
		if err == nil {
			return fmt.Errorf("scope with ID %s still exists", rs.Primary.ID)
		}
	}

	return nil
}

func testAccScopeConfig_basic(applicationID string) string {
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

func testAccScopeConfig_basicArm64(applicationID string) string {
	return fmt.Sprintf(`
resource "nullplatform_scope" "test" {
  null_application_id                       = %s
  scope_name                                = "acc-test-scope"
  capabilities_serverless_runtime_id        = "provided.al2"
  capabilities_serverless_runtime_platform  = "arm_64"
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

func testAccScopeConfig_basicWithAssetName(applicationID string) string {
	return fmt.Sprintf(`
resource "nullplatform_scope" "test" {
  null_application_id                       = %s
  scope_name                                = "acc-test-scope"
  scope_asset_name                          = "the-secret-algo-lambda-asset"
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
