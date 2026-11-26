package nullplatform

import (
	"encoding/base64"
	"fmt"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/nullplatform/terraform-provider-nullplatform/internal/fakeplatform"
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
				// Left out, it keeps the type the platform holds: no change.
				Config: templateConfig(""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.TestCheckResourceAttr("nullplatform_technology_template.agent", "application_type_id", "type-1"),
			},
			{
				// Taken out of its type with an empty one: null is how the API is told.
				Config: templateConfig(`  application_type_id = ""`),
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

// A template put in a type outside Terraform stays in it: a configuration
// that does not name a type plans no change.
func TestFunctionalTechnologyTemplate_ATypeSetOutsideTerraformIsKept(t *testing.T) {
	fake := newApplicationTypeFake(t)
	client := newTestClient(fake.HTTP())
	client.Token.AccessToken = tokenOfOrganization("1")
	// The type is outside Terraform too: nothing here orders its destroy.
	fake.Seed("application_type", "type-1", fakeplatform.Item{"nrn": "organization=1", "name": "AI Agent", "status": "active"})
	config := strings.Replace(templateConfig(""), applicationTypeConfig("AI Agent", agentSchema, ""), "", 1)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactoriesWith(client),
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: func() { fake.Items("template")[0]["application_type_id"] = "type-1" },
				Config:    config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_technology_template.agent", "application_type_id", "type-1"),
					func(*terraform.State) error {
						if patches := requestsTo(fake, http.MethodPatch, "/template/1"); len(patches) != 0 {
							return fmt.Errorf("want no edit of the template, got %v", patches)
						}
						return nil
					},
				),
			},
		},
	})
}
