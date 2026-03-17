package nullplatform_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/nullplatform/terraform-provider-nullplatform/nullplatform"
)

func TestAccResourceProviderSpecification(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckProviderSpecificationDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceProviderSpecification_basic(),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckProviderSpecificationExists("nullplatform_provider_specification.test"),
					resource.TestCheckResourceAttr("nullplatform_provider_specification.test", "name", "acc-test-provider-spec"),
					resource.TestCheckResourceAttr("nullplatform_provider_specification.test", "allow_dimensions", "false"),
					resource.TestCheckResourceAttrSet("nullplatform_provider_specification.test", "slug"),
				),
			},
			{
				Config: testAccResourceProviderSpecification_updated(),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckProviderSpecificationExists("nullplatform_provider_specification.test"),
					resource.TestCheckResourceAttr("nullplatform_provider_specification.test", "name", "acc-test-provider-spec-updated"),
					resource.TestCheckResourceAttr("nullplatform_provider_specification.test", "description", "Updated description"),
					resource.TestCheckResourceAttr("nullplatform_provider_specification.test", "allow_dimensions", "true"),
				),
			},
			{
				ResourceName:      "nullplatform_provider_specification.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccCheckProviderSpecificationExists(n string) resource.TestCheckFunc {
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

		spec, err := client.GetProviderSpecification(rs.Primary.ID)
		if err != nil {
			return fmt.Errorf("error fetching provider specification: %s", err)
		}

		if spec.Id != rs.Primary.ID {
			return fmt.Errorf("ProviderSpecification not found")
		}

		return nil
	}
}

func testAccCheckProviderSpecificationDestroy(s *terraform.State) error {
	client := testAccProviders["nullplatform"].Meta().(nullplatform.NullOps)
	if client == nil {
		return fmt.Errorf("provider meta is nil, ensure the provider is properly configured and initialized")
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "nullplatform_provider_specification" {
			continue
		}

		_, err := client.GetProviderSpecification(rs.Primary.ID)
		if err == nil {
			return fmt.Errorf("ProviderSpecification with ID %s still exists", rs.Primary.ID)
		}
	}

	return nil
}

func testAccResourceProviderSpecification_basic() string {
	return `
resource "nullplatform_provider_specification" "test" {
  name       = "acc-test-provider-spec"
  visible_to = ["organization=1"]
  schema = jsonencode({
    type = "object"
    properties = {
      api_key = {
        type        = "string"
        description = "API Key for the provider"
      }
    }
  })
}
`
}

func testAccResourceProviderSpecification_updated() string {
	return `
resource "nullplatform_provider_specification" "test" {
  name             = "acc-test-provider-spec-updated"
  description      = "Updated description"
  visible_to       = ["organization=1"]
  allow_dimensions = true
  schema = jsonencode({
    type = "object"
    properties = {
      api_key = {
        type        = "string"
        description = "API Key for the provider"
      }
      region = {
        type        = "string"
        description = "Region for the provider"
      }
    }
  })
}
`
}
