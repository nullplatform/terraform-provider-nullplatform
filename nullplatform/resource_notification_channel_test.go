package nullplatform_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
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

// TestNotificationChannelUpdate_SendsSource is a regression test: updating the
// `source` attribute produced a plan diff but the PATCH request body never
// included the field, so the API value was never updated.
func TestNotificationChannelUpdate_SendsSource(t *testing.T) {
	var patchBody map[string]interface{}

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPatch:
			if err := json.NewDecoder(r.Body).Decode(&patchBody); err != nil {
				t.Errorf("failed to decode PATCH body: %v", err)
			}
			w.WriteHeader(http.StatusNoContent)
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{
				"id": 123,
				"nrn": "organization=1:account=2",
				"type": "http",
				"source": ["approval", "entity"],
				"configuration": {"url": "https://hooks.example.com/webhook/xyz"},
				"status": "active",
				"filters": {}
			}`)
		default:
			t.Errorf("unexpected request method: %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "https://")
	client := &nullplatform.NullClient{
		Client: server.Client(),
		ApiURL: host,
		Token:  nullplatform.Token{AccessToken: "test-token"},
	}

	channelSchema := nullplatform.Provider().ResourcesMap["nullplatform_notification_channel"].Schema
	d := schema.TestResourceDataRaw(t, channelSchema, map[string]interface{}{
		"nrn":    "organization=1:account=2",
		"type":   "http",
		"source": []interface{}{"approval", "entity"},
		"configuration": []interface{}{
			map[string]interface{}{
				"http": []interface{}{
					map[string]interface{}{
						"url": "https://hooks.example.com/webhook/xyz",
					},
				},
			},
		},
	})
	d.SetId("123")

	if err := nullplatform.NotificationChannelUpdate(d, client); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, ok := patchBody["source"]
	if !ok {
		t.Fatalf("PATCH body is missing the source field: %v", patchBody)
	}
	want := []interface{}{"approval", "entity"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got source %v, want %v", got, want)
	}
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
