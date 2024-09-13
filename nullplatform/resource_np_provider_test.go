package nullplatform_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/nullplatform/terraform-provider-nullplatform/nullplatform"
)

func TestAccResourceNpProvider(t *testing.T) {
	var npProvider nullplatform.NpProvider
	specificationID := os.Getenv("NULLPLATFORM_SPECIFICATION_ID")

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckNpProviderDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceNpProvider_basic(specificationID),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckNpProviderExists("nullplatform_np_provider.test_provider", &npProvider),
					resource.TestCheckResourceAttr("nullplatform_np_provider.test_provider", "attributes.api_key", "test-api-key"),
					resource.TestCheckResourceAttr("nullplatform_np_provider.test_provider", "attributes.region", "us-west-2"),
					resource.TestCheckResourceAttr("nullplatform_np_provider.test_provider", "dimensions.environment", "dev"),
				),
			},
		},
	})
}

func testAccCheckNpProviderExists(n string, np *nullplatform.NpProvider) resource.TestCheckFunc {
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

		foundNpProvider, err := client.GetNpProvider(rs.Primary.ID)
		if err != nil {
			return err
		}

		if foundNpProvider.Id != rs.Primary.ID {
			return fmt.Errorf("NpProvider not found")
		}

		*np = *foundNpProvider

		return nil
	}
}

func testAccCheckNpProviderDestroy(s *terraform.State) error {
	client := testAccProviders["nullplatform"].Meta().(nullplatform.NullOps)
	if client == nil {
		return fmt.Errorf("provider meta is nil, ensure the provider is properly configured and initialized")
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "nullplatform_np_provider" {
			continue
		}

		_, err := client.GetNpProvider(rs.Primary.ID)
		if err == nil {
			return fmt.Errorf("NpProvider with ID %s still exists", rs.Primary.ID)
		}
	}

	return nil
}

func testAccResourceNpProvider_basic(specificationID string) string {
	return fmt.Sprintf(`
resource "nullplatform_np_provider" "test_provider" {
  nrn              = "nrn:null:np_provider:test"
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
