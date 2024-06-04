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
			{
				Config: testAccResourceParameterConfig_import_if_created(applicationID),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckParameterExists("nullplatform_parameter.parameter", &parameter),
					resource.TestCheckResourceAttr("nullplatform_parameter.parameter", "name", "Log Level"),
					resource.TestCheckResourceAttr("nullplatform_parameter.parameter", "variable", "LOG_LEVEL_UPDATE"),
					resource.TestCheckResourceAttr("nullplatform_parameter.parameter_import_if_created", "name", "Log Level"),
					resource.TestCheckResourceAttr("nullplatform_parameter.parameter_import_if_created", "variable", "LOG_LEVEL_UPDATE"),
					resource.TestCheckResourceAttr("nullplatform_parameter.parameter_import_if_created_different_variable", "name", "Another Level"),
					resource.TestCheckResourceAttr("nullplatform_parameter.parameter_import_if_created_different_variable", "variable", "ANOTHER_LEVEL"),
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

		// when import_if_created=true resources won't be deleted
		importIfCreated, _ := strconv.ParseBool(rs.Primary.Attributes["import_if_created"])

		_, err := client.GetParameter(rs.Primary.ID)
		if err == nil && !importIfCreated {
			return fmt.Errorf("Parameter with ID %s still exists", rs.Primary.ID)
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

func testAccResourceParameterConfig_import_if_created(applicationID string) string {
	return fmt.Sprintf(`
data "nullplatform_application" "app" {
	id = %s
}

# Should create a new Parameter
resource "nullplatform_parameter" "parameter" {
	nrn      = data.nullplatform_application.app.nrn
	name     = "Log Level"
	variable = "LOG_LEVEL_UPDATE"
}

# Should avoid creating and import the ID
resource "nullplatform_parameter" "parameter_import_if_created" {
	nrn      = data.nullplatform_application.app.nrn
	name     = "Log Level"
	variable = "LOG_LEVEL_UPDATE"
	import_if_created = true

	depends_on = [
		nullplatform_parameter.parameter
	]
}

# Should create a new Parameter even though "import_if_created = true"
resource "nullplatform_parameter" "parameter_import_if_created_different_variable" {
	nrn      = data.nullplatform_application.app.nrn
	name     = "Another Level"
	variable = "ANOTHER_LEVEL"
	import_if_created = true

	depends_on = [
		nullplatform_parameter.parameter
	]
}
`, applicationID)
}
