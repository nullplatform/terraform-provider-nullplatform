package nullplatform_test

import (
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/nullplatform/terraform-provider-nullplatform/nullplatform"
)

func TestAccResourceNamespace(t *testing.T) {
	var namespaceID string
	accountID := os.Getenv("NULLPLATFORM_ACCOUNT_ID")

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckNamespaceDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceNamespaceConfig(accountID, "test-namespace"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckNamespaceExists("nullplatform_namespace.test"),
					resource.TestCheckResourceAttr("nullplatform_namespace.test", "name", "test-namespace"),
					resource.TestCheckResourceAttrSet("nullplatform_namespace.test", "slug"),
					resource.TestCheckResourceAttrSet("nullplatform_namespace.test", "nrn"),
					captureNamespaceID("nullplatform_namespace.test", &namespaceID),
				),
			},
			{
				// A rename must be applied in place (PATCH), never by
				// destroying and recreating the namespace.
				Config: testAccResourceNamespaceConfig(accountID, "renamed-test-namespace"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckNamespaceExists("nullplatform_namespace.test"),
					resource.TestCheckResourceAttr("nullplatform_namespace.test", "name", "renamed-test-namespace"),
					checkNamespaceIDUnchanged("nullplatform_namespace.test", &namespaceID),
				),
			},
			{
				ResourceName:      "nullplatform_namespace.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func captureNamespaceID(n string, id *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("not found: %s", n)
		}
		*id = rs.Primary.ID
		return nil
	}
}

func checkNamespaceIDUnchanged(n string, id *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("not found: %s", n)
		}
		if rs.Primary.ID != *id {
			return fmt.Errorf("namespace was replaced on rename: ID changed from %s to %s", *id, rs.Primary.ID)
		}
		return nil
	}
}

func testAccCheckNamespaceExists(n string) resource.TestCheckFunc {
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

		foundNamespace, err := client.GetNamespace(rs.Primary.ID)
		if err != nil {
			return err
		}

		if strconv.Itoa(foundNamespace.Id) != rs.Primary.ID {
			return fmt.Errorf("Namespace not found")
		}

		return nil
	}
}

func testAccCheckNamespaceDestroy(s *terraform.State) error {
	client := testAccProviders["nullplatform"].Meta().(nullplatform.NullOps)
	if client == nil {
		return fmt.Errorf("provider meta is nil, ensure the provider is properly configured and initialized")
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "nullplatform_namespace" {
			continue
		}

		namespace, err := client.GetNamespace(rs.Primary.ID)
		if err == nil && namespace != nil && namespace.Status != "inactive" {
			return fmt.Errorf("Namespace with ID %s still exists", rs.Primary.ID)
		}
	}

	return nil
}

func testAccResourceNamespaceConfig(accountID, name string) string {
	return fmt.Sprintf(`
resource "nullplatform_namespace" "test" {
  name       = %[2]q
  account_id = %[1]s
}
`, accountID, name)
}
