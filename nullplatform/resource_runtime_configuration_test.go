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

func TestAccResourceRuntimeConfiguration(t *testing.T) {
	var runtimeConfig int
	applicationID := os.Getenv("NULLPLATFORM_APPLICATION_ID")

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckRuntimeConfigurationDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceRuntimeConfiguration_basic(applicationID),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRuntimeConfigurationExists("nullplatform_runtime_configuration.runtime_config", &runtimeConfig),
					resource.TestCheckResourceAttr("nullplatform_runtime_configuration.runtime_config", "values.scope_workflow_role", "arn:aws:iam::012345678901:role/scope_workflow_role"),
					resource.TestCheckResourceAttr("nullplatform_runtime_configuration.runtime_config", "values.application_workflow_role", "arn:aws:iam::012345678901:role/application_workflow_role"),
					resource.TestCheckResourceAttr("nullplatform_runtime_configuration.runtime_config", "values.log_reader_role", "arn:aws:iam::012345678901:role/log_reader_role"),
				),
			},
		},
	})
}

func testAccCheckRuntimeConfigurationExists(n string, rc *int) resource.TestCheckFunc {
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

		foundRuntimeConfig, err := client.GetRuntimeConfiguration(rs.Primary.ID)
		if err != nil {
			return err
		}

		if strconv.Itoa(foundRuntimeConfig.Id) != rs.Primary.ID {
			return fmt.Errorf("Runtime Configuration not found")
		}

		*rc = foundRuntimeConfig.Id

		return nil
	}
}

func testAccCheckRuntimeConfigurationDestroy(s *terraform.State) error {
	client := testAccProviders["nullplatform"].Meta().(nullplatform.NullOps)
	if client == nil {
		return fmt.Errorf("provider meta is nil, ensure the provider is properly configured and initialized")
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "nullplatform_runtime_configuration" {
			continue
		}

		_, err := client.GetRuntimeConfiguration(rs.Primary.ID)
		if err == nil {
			return fmt.Errorf("Runtime Configuration with ID %s still exists", rs.Primary.ID)
		}
	}

	return nil
}

func testAccResourceRuntimeConfiguration_basic(applicationID string) string {
	return fmt.Sprintf(`
data "nullplatform_application" "app" {
  id = %s
}

resource "nullplatform_runtime_configuration" "runtime_config" {
  nrn        = data.nullplatform_application.app.nrn
  dimensions = {
	environment = "dev"
  }
  values = {
	scope_workflow_role       = "arn:aws:iam::012345678901:role/scope_workflow_role"
	application_workflow_role = "arn:aws:iam::012345678901:role/application_workflow_role"
	log_reader_role           = "arn:aws:iam::012345678901:role/log_reader_role"
  }
}
`, applicationID)
}
