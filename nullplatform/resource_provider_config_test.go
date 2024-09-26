package nullplatform_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/nullplatform/terraform-provider-nullplatform/nullplatform"
)

func TestAccResourceProviderConfig(t *testing.T) {
	var providerConfig nullplatform.ProviderConfig
	specificationID := os.Getenv("NULLPLATFORM_SPECIFICATION_ID")

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckProviderConfigDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceProviderConfig_basic(specificationID),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckProviderConfigExists("nullplatform_provider_config.test_provider", &providerConfig),
					resource.TestCheckResourceAttr("nullplatform_provider_config.test_provider", "attributes.api_key", "test-api-key"),
					resource.TestCheckResourceAttr("nullplatform_provider_config.test_provider", "attributes.region", "us-west-2"),
					resource.TestCheckResourceAttr("nullplatform_provider_config.test_provider", "dimensions.environment", "dev"),
				),
			},
		},
	})
}

func testAccCheckProviderConfigExists(n string, pc *nullplatform.ProviderConfig) resource.TestCheckFunc {
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

		foundProviderConfig, err := client.GetProviderConfig(rs.Primary.ID)
		if err != nil {
			return err
		}

		if foundProviderConfig.Id != rs.Primary.ID {
			return fmt.Errorf("ProviderConfig not found")
		}

		*pc = *foundProviderConfig

		return nil
	}
}

func testAccCheckProviderConfigDestroy(s *terraform.State) error {
	client := testAccProviders["nullplatform"].Meta().(nullplatform.NullOps)
	if client == nil {
		return fmt.Errorf("provider meta is nil, ensure the provider is properly configured and initialized")
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "nullplatform_provider_config" {
			continue
		}

		_, err := client.GetProviderConfig(rs.Primary.ID)
		if err == nil {
			return fmt.Errorf("ProviderConfig with ID %s still exists", rs.Primary.ID)
		}
	}

	return nil
}

func testAccResourceProviderConfig_basic(specificationID string) string {
	return fmt.Sprintf(`
resource "nullplatform_provider_config" "test_provider" {
  nrn              = "nrn:null:provider_config:test"
  specification_id = "%s"
  dimensions = {
    environment = "dev"
  }
  attributes = {
    api_key = "test-api-key"
    region  = "us-west-2"
  }
}
`, specificationID)
}
