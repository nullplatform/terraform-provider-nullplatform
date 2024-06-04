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

func TestAccResourceParameter(t *testing.T) {
	var parameter nullplatform.Parameter
	applicationID := os.Getenv("NULLPLATFORM_APPLICATION_ID")

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckParameterDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceParameterConfig_basic(applicationID),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckParameterExists("nullplatform_parameter.parameter", &parameter),
					resource.TestCheckResourceAttr("nullplatform_parameter.parameter", "name", "Log Level"),
					resource.TestCheckResourceAttr("nullplatform_parameter.parameter", "variable", "LOG_LEVEL"),
				),
			},
			{
				Config: testAccResourceParameterConfig_update(applicationID),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckParameterExists("nullplatform_parameter.parameter", &parameter),
					resource.TestCheckResourceAttr("nullplatform_parameter.parameter", "name", "Log Level"),
					resource.TestCheckResourceAttr("nullplatform_parameter.parameter", "variable", "LOG_LEVEL_UPDATE"),
				),
			},
		},
	})
}

func testAccCheckParameterExists(n string, parameter *nullplatform.Parameter) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("no Parameter ID is set")
		}

		client := testAccProviders["nullplatform"].Meta().(nullplatform.NullOps)
		if client == nil {
			return fmt.Errorf("provider meta is nil, ensure the provider is properly configured and initialized")
		}

		foundParameter, err := client.GetParameter(rs.Primary.ID)
		if err != nil {
			return err
		}

		if strconv.Itoa(foundParameter.Id) != rs.Primary.ID {
			return fmt.Errorf("Parameter not found")
		}

		*parameter = *foundParameter

		return nil
	}
}

func testAccCheckParameterDestroy(s *terraform.State) error {
	client := testAccProviders["nullplatform"].Meta().(nullplatform.NullOps)
	if client == nil {
		return fmt.Errorf("provider meta is nil, ensure the provider is properly configured and initialized")
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "nullplatform_parameter" {
			continue
		}

		_, err := client.GetParameter(rs.Primary.ID)
		if err == nil {
			return fmt.Errorf("Parameter still exists")
		}
	}

	return nil
}

func testAccResourceParameterConfig_basic(applicationID string) string {
	return fmt.Sprintf(`
data "nullplatform_application" "app" {
  id = %s
}

resource "nullplatform_parameter" "parameter" {
  nrn      = data.nullplatform_application.app.nrn
  name     = "Log Level"
  variable = "LOG_LEVEL"
}
`, applicationID)
}

func testAccResourceParameterConfig_update(applicationID string) string {
	return fmt.Sprintf(`
data "nullplatform_application" "app" {
  id = %s
}

resource "nullplatform_parameter" "parameter" {
  nrn      = data.nullplatform_application.app.nrn
  name     = "Log Level"
  variable = "LOG_LEVEL_UPDATE"
}
`, applicationID)
}
