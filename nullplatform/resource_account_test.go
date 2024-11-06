package nullplatform_test

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/nullplatform/terraform-provider-nullplatform/nullplatform"
)

func TestAccResourceAccount(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAccountDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceAccount_basic(),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAccountExists("nullplatform_account.test"),
					resource.TestCheckResourceAttr("nullplatform_account.test", "name", "test-account"),
					resource.TestCheckResourceAttrSet("nullplatform_account.test", "organization_id"),
					resource.TestCheckResourceAttr("nullplatform_account.test", "repository_prefix", "test-prefix"),
					resource.TestCheckResourceAttr("nullplatform_account.test", "repository_provider", "github"),
					resource.TestCheckResourceAttr("nullplatform_account.test", "slug", "test-account"),
				),
			},
			{
				Config: testAccResourceAccount_update(),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAccountExists("nullplatform_account.test"),
					resource.TestCheckResourceAttr("nullplatform_account.test", "name", "updated-test-account"),
					resource.TestCheckResourceAttrSet("nullplatform_account.test", "organization_id"),
					resource.TestCheckResourceAttr("nullplatform_account.test", "repository_prefix", "updated-prefix"),
					resource.TestCheckResourceAttr("nullplatform_account.test", "repository_provider", "gitlab"),
					resource.TestCheckResourceAttr("nullplatform_account.test", "slug", "updated-test-account"),
				),
			},
			{
				// Test importing the resource
				ResourceName:      "nullplatform_account.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccCheckAccountExists(n string) resource.TestCheckFunc {
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

		foundAccount, err := client.GetAccount(rs.Primary.ID)
		if err != nil {
			return err
		}

		if strconv.Itoa(foundAccount.Id) != rs.Primary.ID {
			return fmt.Errorf("Account not found")
		}

		return nil
	}
}

func testAccCheckAccountDestroy(s *terraform.State) error {
	client := testAccProviders["nullplatform"].Meta().(nullplatform.NullOps)
	if client == nil {
		return fmt.Errorf("provider meta is nil, ensure the provider is properly configured and initialized")
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "nullplatform_account" {
			continue
		}

		account, err := client.GetAccount(rs.Primary.ID)
		if err == nil && account.Status != "inactive" {
			return fmt.Errorf("Account with ID %s still exists and is not inactive", rs.Primary.ID)
		}
	}

	return nil
}

func testAccResourceAccount_basic() string {
	return `
resource "nullplatform_account" "test" {
  name                = "test-account"
  repository_prefix   = "test-prefix"
  repository_provider = "github"
  slug               = "test-account"
}
`
}

func testAccResourceAccount_update() string {
	return `
resource "nullplatform_account" "test" {
  name                = "updated-test-account"
  repository_prefix   = "updated-prefix"
  repository_provider = "gitlab"
  slug               = "updated-test-account"
}
`
}
