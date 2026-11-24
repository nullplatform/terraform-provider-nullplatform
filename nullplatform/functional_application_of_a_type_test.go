package nullplatform

import (
	"fmt"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"net/http"
	"regexp"
	"strings"
	"testing"
)

// Real plan/apply over the fake platform: an application takes its type when
// it is created and keeps it, carries the attributes of the type, and one with
// no type sends what it always sent.

func typedApplicationConfig(typeLine, attributes string) string {
	return applicationTypeConfig("AI Agent", agentSchema, "") + fmt.Sprintf(`
resource "nullplatform_application_type" "job" {
  nrn    = "organization=1:account=2"
  name   = "Job"
  schema = %q

  # Created second, so the ids below are the same on every run.
  depends_on = [nullplatform_application_type.agent]
}

resource "nullplatform_application" "bot" {
  name         = "support-bot"
  namespace_id = 3
%s%s}
`, agentSchema, typeLine, attributes)
}

func TestFunctionalApplication_OfAType(t *testing.T) {
	fake := newApplicationTypeFake(t)
	ofAgent := "  application_type_id = nullplatform_application_type.agent.id\n"

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{
				Config: typedApplicationConfig(ofAgent, `  attributes = jsonencode({ model = "fable", autonomy = "low" })`+"\n"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_application.bot", "application_type_id", "type-1"),
					resource.TestCheckResourceAttr("nullplatform_application.bot", "application_type_snapshot_id", "snapshot-1"),
					resource.TestCheckResourceAttr("nullplatform_application.bot", "attributes", `{"autonomy":"low","model":"fable"}`),
					func(*terraform.State) error {
						got := requestsTo(fake, http.MethodPost, "/application")
						want := `"application_type_id":"type-1","attributes":{"autonomy":"low","model":"fable"}}` + "\n"
						if len(got) != 1 || !strings.HasSuffix(got[0].Body, want) {
							return fmt.Errorf("create body: got %v, want it to end in %s", got, want)
						}
						return nil
					},
				),
			},
			{
				// One value changes: the attributes are sent, and nothing else is.
				Config: typedApplicationConfig(ofAgent, `  attributes = jsonencode({ model = "opus", autonomy = "low" })`+"\n"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_application.bot", "attributes", `{"autonomy":"low","model":"opus"}`),
					func(*terraform.State) error {
						patches := requestsTo(fake, http.MethodPatch, "/application/1")
						if want := `{"attributes":{"autonomy":"low","model":"opus"}}` + "\n"; len(patches) != 1 || patches[0].Body != want {
							return fmt.Errorf("patch body: got %v, want %s", patches, want)
						}
						return nil
					},
				),
			},
			{
				// The platform merges: a key taken out of the configuration stays there,
				// and the plan keeps saying so.
				Config:             typedApplicationConfig(ofAgent, `  attributes = jsonencode({ model = "opus" })`+"\n"),
				ExpectNonEmptyPlan: true,
				Check:              resource.TestCheckResourceAttr("nullplatform_application.bot", "attributes", `{"autonomy":"low","model":"opus"}`),
			},
			{
				// Not set: what the platform holds is read, with no plan.
				Config: typedApplicationConfig(ofAgent, ""),
				Check:  resource.TestCheckResourceAttr("nullplatform_application.bot", "attributes", `{"autonomy":"low","model":"opus"}`),
			},
			{
				// The type of an application is for life: another type is another application.
				Config: typedApplicationConfig("  application_type_id = nullplatform_application_type.job.id\n", ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("nullplatform_application.bot", plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_application.bot", "id", "2"),
					resource.TestCheckResourceAttr("nullplatform_application.bot", "application_type_id", "type-2"),
					resource.TestCheckResourceAttr("nullplatform_application.bot", "attributes", "{}"),
				),
			},
		},
	})
}

func TestFunctionalApplication_WithNoTypeSendsWhatItAlwaysSent(t *testing.T) {
	fake := newApplicationTypeFake(t)
	config := `
resource "nullplatform_application" "api" {
  name           = "my-api"
  namespace_id   = 3
  repository_url = "https://github.com/my-org/my-api"
}
`
	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_application.api", "application_type_id", ""),
					resource.TestCheckResourceAttr("nullplatform_application.api", "application_type_snapshot_id", ""),
					resource.TestCheckResourceAttr("nullplatform_application.api", "attributes", "{}"),
					func(*terraform.State) error {
						want := `{"name":"my-api","namespace_id":3,"repository_url":"https://github.com/my-org/my-api"}` + "\n"
						if got := requestsTo(fake, http.MethodPost, "/application"); len(got) != 1 || got[0].Body != want {
							return fmt.Errorf("create body:\n got %v\nwant %s", got, want)
						}
						return nil
					},
				),
			},
			{
				Config: strings.Replace(config, "my-api\"\n", "my-api-2\"\n", 1),
				Check: func(*terraform.State) error {
					patches := requestsTo(fake, http.MethodPatch, "/application/1")
					if want := `{"name":"my-api-2"}` + "\n"; len(patches) != 1 || patches[0].Body != want {
						return fmt.Errorf("patch body: got %v, want %s", patches, want)
					}
					return nil
				},
			},
		},
	})
}

func TestFunctionalApplication_TypeRefusalsCarryTheAPIMessage(t *testing.T) {
	fake := newApplicationTypeFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{
				Config: applicationTypeConfig("AI Agent", agentSchema, `  status = "inactive"`) + `
resource "nullplatform_application" "bot" {
  name                = "support-bot"
  namespace_id        = 3
  application_type_id = nullplatform_application_type.agent.id
}
`,
				ExpectError: regexp.MustCompile(`The application type type-1 cannot be used in\s+namespace\s+3`),
			},
			{
				Config: `
resource "nullplatform_application" "bot" {
  name         = "support-bot"
  namespace_id = 3
  attributes   = jsonencode({ model = "fable" })
}
`,
				ExpectError: regexp.MustCompile(`attributes belong to an application type, and this\s+application\s+has\s+none`),
			},
			{
				Config: `
resource "nullplatform_application" "bot" {
  name         = "support-bot"
  namespace_id = 3
  attributes   = "not json"
}
`,
				ExpectError: regexp.MustCompile(`error parsing attributes JSON`),
			},
		},
	})
}

func TestFunctionalApplication_AttributesThatStopBeingJSONFailTheUpdate(t *testing.T) {
	fake := newApplicationTypeFake(t)
	config := func(attributes string) string {
		return applicationTypeConfig("AI Agent", agentSchema, "") + fmt.Sprintf(`
resource "nullplatform_application" "bot" {
  name                = "support-bot"
  namespace_id        = 3
  application_type_id = nullplatform_application_type.agent.id
  attributes          = %q
}
`, attributes)
	}

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: config(`{"model":"fable"}`)},
			{Config: config(`not json`), ExpectError: regexp.MustCompile(`error parsing attributes JSON`)},
		},
	})
}
