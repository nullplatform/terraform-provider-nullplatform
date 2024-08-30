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

func TestAccResourceNotificationChannel(t *testing.T) {
	applicationID := os.Getenv("NULLPLATFORM_APPLICATION_ID")

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckNotificationChannelDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceNotificationChannel_basic(applicationID),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckNotificationChannelExists("nullplatform_notification_channel.channel"),
					resource.TestCheckResourceAttr("nullplatform_notification_channel.channel", "type", "slack"),
					resource.TestCheckResourceAttr("nullplatform_notification_channel.channel", "source.0", "approval"),
					resource.TestCheckTypeSetElemAttr("nullplatform_notification_channel.channel", "configuration.0.channels.*", "nullplatform-approvals"),
					resource.TestCheckTypeSetElemAttr("nullplatform_notification_channel.channel", "configuration.0.channels.*", "other-channel"),
				),
			},
		},
	})
}

func testAccCheckNotificationChannelExists(n string) resource.TestCheckFunc {
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

		foundNotificationChannel, err := client.GetNotificationChannel(rs.Primary.ID)
		if err != nil {
			return err
		}

		if strconv.Itoa(foundNotificationChannel.Id) != rs.Primary.ID {
			return fmt.Errorf("Approval policy not found")
		}

		return nil
	}
}

func testAccCheckNotificationChannelDestroy(s *terraform.State) error {
	client := testAccProviders["nullplatform"].Meta().(nullplatform.NullOps)
	if client == nil {
		return fmt.Errorf("provider meta is nil, ensure the provider is properly configured and initialized")
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "nullplatform_notification_channel" {
			continue
		}

		_, err := client.GetNotificationChannel(rs.Primary.ID)
		if err == nil {
			return fmt.Errorf("Approval policy with ID %s still exists", rs.Primary.ID)
		}
	}

	return nil
}

func testAccResourceNotificationChannel_basic(applicationID string) string {
	return fmt.Sprintf(`
data "nullplatform_application" "app" {
  id = %s
}

resource "nullplatform_notification_channel" "channel" {
  nrn    = data.nullplatform_application.app.nrn
  type   = "slack"
  source = ["approval"]
  configuration {
    channels = ["nullplatform-approvals", "other-channel"]
  }
}
`, applicationID)
}
