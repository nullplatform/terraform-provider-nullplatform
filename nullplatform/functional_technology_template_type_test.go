package nullplatform

import (
	"encoding/base64"
	"fmt"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"net/http"
	"strings"
	"testing"
)

// Real plan/apply over the fake platform: a template is put in an application
// type, moved into one and taken out of it.

// tokenOfOrganization is an unsigned token naming an organization, which is
// all the template resource reads from it.
func tokenOfOrganization(id string) string {
	encode := base64.RawURLEncoding.EncodeToString
	return encode([]byte(`{"alg":"none"}`)) + "." +
		encode([]byte(`{"cognito:groups":["@nullplatform/organization=`+id+`"]}`)) + "."
}

func templateConfig(typeLine string) string {
	return applicationTypeConfig("AI Agent", agentSchema, "") + fmt.Sprintf(`
resource "nullplatform_technology_template" "agent" {
  name = "Agent starter"
  url  = "https://github.com/my-org/agent-starter"
%s
  provider_config = {
    repository = "agent-starter"
  }

  components {
    type    = "language"
    id      = "python"
    version = "3.12"
  }

  # Both read back as sent only when they hold something (not this change's concern).
  metadata = jsonencode({ kind = "starter" })
  rules    = jsonencode({ visibility = "all" })
}
`, typeLine)
}

func TestFunctionalTechnologyTemplate_OfAType(t *testing.T) {
	fake := newApplicationTypeFake(t)
	client := newTestClient(fake.HTTP())
	client.Token.AccessToken = tokenOfOrganization("1")

	lastTemplateBody := func(method, path string) string {
		requests := requestsTo(fake, method, path)
		if len(requests) == 0 {
			return ""
		}
		return requests[len(requests)-1].Body
	}

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactoriesWith(client),
		Steps: []resource.TestStep{
			{
				// No type: the request does not name one.
				Config: templateConfig(""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_technology_template.agent", "application_type_id", ""),
					func(*terraform.State) error {
						if body := lastTemplateBody(http.MethodPost, "/template"); strings.Contains(body, "application_type_id") {
							return fmt.Errorf("want no application_type_id in %s", body)
						}
						return nil
					},
				),
			},
			{
				Config: templateConfig("  application_type_id = nullplatform_application_type.agent.id"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_technology_template.agent", "application_type_id", "type-1"),
					func(*terraform.State) error {
						body := lastTemplateBody(http.MethodPatch, "/template/1")
						if !strings.Contains(body, `"application_type_id":"type-1"`) {
							return fmt.Errorf("want the type in %s", body)
						}
						return nil
					},
				),
			},
			{
				// Taken out of its type: null is how the API is told.
				Config: templateConfig(""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_technology_template.agent", "application_type_id", ""),
					func(*terraform.State) error {
						body := lastTemplateBody(http.MethodPatch, "/template/1")
						if !strings.Contains(body, `"application_type_id":null`) {
							return fmt.Errorf("want a null type in %s", body)
						}
						return nil
					},
				),
			},
		},
	})
}

func TestFunctionalTechnologyTemplate_CreatedInAType(t *testing.T) {
	fake := newApplicationTypeFake(t)
	client := newTestClient(fake.HTTP())
	client.Token.AccessToken = tokenOfOrganization("1")

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactoriesWith(client),
		Steps: []resource.TestStep{{
			Config: templateConfig("  application_type_id = nullplatform_application_type.agent.id"),
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("nullplatform_technology_template.agent", "application_type_id", "type-1"),
				func(*terraform.State) error {
					got := requestsTo(fake, http.MethodPost, "/template")
					if len(got) != 1 || !strings.Contains(got[0].Body, `"application_type_id":"type-1"`) {
						return fmt.Errorf("want the type in the create body, got %v", got)
					}
					return nil
				},
			),
		}},
	})
}
