package nullplatform_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/nullplatform/terraform-provider-nullplatform/nullplatform"
)

func TestAccResourceParameterValue(t *testing.T) {
	var parameterValue nullplatform.ParameterValue
	applicationID := os.Getenv("NULLPLATFORM_APPLICATION_ID")

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckParameterDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceParameterValueConfig_basic(applicationID),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckParameterValueExists("nullplatform_parameter_value.parameter_value", &parameterValue),
					resource.TestCheckResourceAttr("nullplatform_parameter_value.parameter_value", "value", "DEBUG"),
				),
			},
			{
				Config: testAccResourceParameterValueConfig_update(applicationID),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckParameterValueExists("nullplatform_parameter_value.parameter_value", &parameterValue),
					resource.TestCheckResourceAttr("nullplatform_parameter_value.parameter_value", "value", "INFO"),
				),
			},
			{
				Config: testAccResourceParameterValueConfig_emptyValue(applicationID),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckParameterValueExists("nullplatform_parameter_value.parameter_value", &parameterValue),
					resource.TestCheckResourceAttr("nullplatform_parameter_value.parameter_value", "value", ""),
				),
			},
		},
	})
}

func testAccCheckParameterValueExists(n string, parameterValue *nullplatform.ParameterValue) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("no Parameter Value ID is set")
		}

		client := testAccProviders["nullplatform"].Meta().(nullplatform.NullOps)
		if client == nil {
			return fmt.Errorf("provider meta is nil, ensure the provider is properly configured and initialized")
		}

		foundParameterValue, err := client.GetParameterValue(rs.Primary.Attributes["parameter_id"], rs.Primary.ID, nil)
		if err != nil {
			return err
		}

		if foundParameterValue.GeneratedId != rs.Primary.ID {
			return fmt.Errorf("Parameter Value ID %s not found", rs.Primary.ID)
		}

		*parameterValue = *foundParameterValue

		return nil
	}
}

func testAccResourceParameterValueConfig_basic(applicationID string) string {
	return fmt.Sprintf(`
data "nullplatform_application" "app" {
  id = %s
}

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

resource "nullplatform_parameter" "parameter" {
  nrn      = data.nullplatform_application.app.nrn
  name     = "Log Level"
  variable = "LOG_LEVEL"
}

resource "nullplatform_parameter_value" "parameter_value" {
  parameter_id = nullplatform_parameter.parameter.id
  nrn          = nullplatform_scope.test.nrn
  value        = "DEBUG"
}
`, applicationID, applicationID)
}

func testAccResourceParameterValueConfig_update(applicationID string) string {
	return fmt.Sprintf(`
data "nullplatform_application" "app" {
  id = %s
}

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

resource "nullplatform_parameter" "parameter" {
  nrn      = data.nullplatform_application.app.nrn
  name     = "Log Level"
  variable = "LOG_LEVEL"
}

resource "nullplatform_parameter_value" "parameter_value" {
  parameter_id = nullplatform_parameter.parameter.id
  nrn          = nullplatform_scope.test.nrn
  value        = "INFO"
}
`, applicationID, applicationID)
}

func testAccResourceParameterValueConfig_emptyValue(applicationID string) string {
	return fmt.Sprintf(`
data "nullplatform_application" "app" {
  id = %s
}

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
  
  resource "nullplatform_parameter" "parameter" {
	nrn      = data.nullplatform_application.app.nrn
	name     = "Log Level"
	variable = "LOG_LEVEL"
  }
  
  resource "nullplatform_parameter_value" "parameter_value" {
	parameter_id = nullplatform_parameter.parameter.id
	nrn          = nullplatform_scope.test.nrn
	value        = ""
  }
  `, applicationID, applicationID)
}
