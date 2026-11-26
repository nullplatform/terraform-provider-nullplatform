package nullplatform

import (
	"fmt"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/nullplatform/terraform-provider-nullplatform/internal/fakeplatform"
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

// lastPatchIs checks that the application was edited `count` times so far, the
// last of them with `body`.
func lastPatchIs(fake *fakeplatform.Server, count int, body string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		patches := requestsTo(fake, http.MethodPatch, "/application/1")
		if len(patches) != count || patches[count-1].Body != body+"\n" {
			return fmt.Errorf("edits of the application: got %v, want %d, the last one %s", patches, count, body)
		}
		return nil
	}
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
				// One value changes: it is sent, and nothing else is.
				Config: typedApplicationConfig(ofAgent, `  attributes = jsonencode({ model = "opus", autonomy = "low" })`+"\n"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_application.bot", "attributes", `{"autonomy":"low","model":"opus"}`),
					lastPatchIs(fake, 1, `{"attributes":{"model":"opus"}}`),
				),
			},
			{
				// A key taken out of the configuration is sent as null, which removes it
				// there: the plan that follows is empty.
				Config: typedApplicationConfig(ofAgent, `  attributes = jsonencode({ model = "opus" })`+"\n"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_application.bot", "attributes", `{"model":"opus"}`),
					lastPatchIs(fake, 2, `{"attributes":{"autonomy":null}}`),
				),
			},
			{
				// An object and a list come in: only what is new is sent.
				Config: typedApplicationConfig(ofAgent, `  attributes = jsonencode({ model = "opus", limits = { tokens = 1000, depth = 2 }, tools = ["search", "code"] })`+"\n"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_application.bot", "attributes", `{"limits":{"depth":2,"tokens":1000},"model":"opus","tools":["search","code"]}`),
					lastPatchIs(fake, 3, `{"attributes":{"limits":{"depth":2,"tokens":1000},"tools":["search","code"]}}`),
				),
			},
			{
				// The object changes member by member, the list whole.
				Config: typedApplicationConfig(ofAgent, `  attributes = jsonencode({ model = "opus", limits = { tokens = 2000 }, tools = ["search"] })`+"\n"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_application.bot", "attributes", `{"limits":{"tokens":2000},"model":"opus","tools":["search"]}`),
					lastPatchIs(fake, 4, `{"attributes":{"limits":{"depth":null,"tokens":2000},"tools":["search"]}}`),
				),
			},
			{
				// A null in the configuration is a key left out: the platform holds none,
				// so there is nothing to plan and nothing is sent.
				Config: typedApplicationConfig(ofAgent, `  attributes = jsonencode({ model = "opus", autonomy = null, limits = { tokens = 2000, depth = null }, tools = ["search"] })`+"\n"),
				Check:  lastPatchIs(fake, 4, `{"attributes":{"limits":{"depth":null,"tokens":2000},"tools":["search"]}}`),
			},
			{
				// Not set: what the platform holds is read, with no plan.
				Config: typedApplicationConfig(ofAgent, ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_application.bot", "attributes", `{"limits":{"tokens":2000},"model":"opus","tools":["search"]}`),
					lastPatchIs(fake, 4, `{"attributes":{"limits":{"depth":null,"tokens":2000},"tools":["search"]}}`),
				),
			},
			{
				// The type of an application is for life: another type is another
				// application. The one deleted keeps its name, so the new one cannot
				// take it.
				Config: typedApplicationConfig("  application_type_id = nullplatform_application_type.job.id\n", `  attributes = jsonencode({ model = "opus" })`+"\n"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("nullplatform_application.bot", plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
				ExpectError: regexp.MustCompile(`Application support-bot with id: 1 is already using\s+name\s+support-bot`),
			},
			{
				// Under another name it is created.
				Config: strings.Replace(typedApplicationConfig("  application_type_id = nullplatform_application_type.job.id\n", `  attributes = jsonencode({ model = "opus" })`+"\n"),
					`"support-bot"`, `"support-bot-2"`, 1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_application.bot", "id", "2"),
					resource.TestCheckResourceAttr("nullplatform_application.bot", "application_type_id", "type-2"),
					resource.TestCheckResourceAttr("nullplatform_application.bot", "attributes", `{"model":"opus"}`),
				),
			},
		},
	})
}

// An application saved before the platform took merge patches can hold a null.
// It is not in the configuration, so it is removed like any other key.
func TestFunctionalApplication_ANullThePlatformHoldsIsRemoved(t *testing.T) {
	fake := newApplicationTypeFake(t)
	config := typedApplicationConfig("  application_type_id = nullplatform_application_type.agent.id\n",
		`  attributes = jsonencode({ model = "opus" })`+"\n")

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: func() {
					fake.Items("application")[0]["attributes"] = fakeplatform.Item{"model": "opus", "autonomy": nil}
				},
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("nullplatform_application.bot", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_application.bot", "attributes", `{"model":"opus"}`),
					lastPatchIs(fake, 1, `{"attributes":{"autonomy":null}}`),
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

// The hook decides, not the type: with a hook for the creation of
// applications, one with no type waits for it too, and its answer hands it to
// the built-in workflow (pending, creating, active). With none, it goes to
// that workflow at once.
func TestFunctionalApplication_WithNoTypeWaitsForACreationHookToo(t *testing.T) {
	config := `
resource "nullplatform_application" "api" {
  name           = "my-api"
  namespace_id   = 3
  repository_url = "https://github.com/my-org/my-api"
}
`
	for _, c := range []struct {
		name  string
		fake  func(*testing.T) *fakeplatform.Server
		first string
	}{
		{"with a hook", newApplicationTypeFake, "pending_hook"},
		{"with none", newApplicationTypeFakeWithoutHooks, "pending"},
	} {
		t.Run(c.name, func(t *testing.T) {
			fake := c.fake(t)
			resource.UnitTest(t, resource.TestCase{
				ProviderFactories: functionalFactories(fake),
				Steps: []resource.TestStep{
					{Config: config, Check: resource.TestCheckResourceAttr("nullplatform_application.api", "status", c.first)},
					{Config: config},
					{Config: config},
					{Config: config, Check: resource.TestCheckResourceAttr("nullplatform_application.api", "status", "active")},
				},
			})
		})
	}
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
				ExpectError: regexp.MustCompile(`The application type cannot be used for this application:\s+The\s+application\s+type\s+type-1\s+cannot\s+be\s+used\s+in\s+namespace\s+3`),
			},
			{
				Config: `
resource "nullplatform_application" "bot" {
  name         = "support-bot"
  namespace_id = 3
  attributes   = jsonencode({ model = "fable" })
}
`,
				ExpectError: regexp.MustCompile(`attributes\s+belong\s+to\s+an\s+application\s+type,\s+and\s+this\s+application\s+has\s+none`),
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

func TestFunctionalApplication_AttributesTheSchemaRefusesAreNotCreated(t *testing.T) {
	fake := newApplicationTypeFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{{
			Config: applicationTypeConfig("AI Agent", agentSchema, "") + `
resource "nullplatform_application" "bot" {
  name                = "support-bot"
  namespace_id        = 3
  application_type_id = nullplatform_application_type.agent.id
  attributes          = jsonencode({ autonomy = "low" })
}
`,
			ExpectError: regexp.MustCompile(`Invalid attributes: data must have required property 'model'`),
		}},
	})
}

// With no hook to send, the platform creates an application of a type as
// failed: the create fails with its message, and leaves it in the state,
// tainted, for the next apply to replace.
func TestFunctionalApplication_CreatedWithNoHookToSendFails(t *testing.T) {
	fake := newApplicationTypeFakeWithoutHooks(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{{
			Config:      typedApplicationConfig("  application_type_id = nullplatform_application_type.agent.id\n", `  attributes = jsonencode({ model = "fable" })`+"\n"),
			ExpectError: regexp.MustCompile(`application 1 was created as failed: No hook could be delivered to create this\s+application`),
		}},
	})
}

// An application of a type imported, or managed from before
// application_type_id existed, keeps its type when the configuration does not
// name one.
func TestFunctionalApplication_ATypeNotConfiguredIsKept(t *testing.T) {
	fake := newApplicationTypeFake(t)
	fake.Seed("application_type", "type-1", fakeplatform.Item{"nrn": "organization=1", "name": "AI Agent", "status": "active", "last_snapshot_id": "snapshot-1"})
	fake.Seed("application", "7", fakeplatform.Item{
		"name": "support-bot", "namespace_id": 3, "status": "active", "repository_url": "https://github.com/acme/support-bot",
		"application_type_id": "type-1", "application_type_snapshot_id": "snapshot-1", "attributes": fakeplatform.Item{"model": "fable"},
		"settings": fakeplatform.Item{}, "tags": fakeplatform.Item{},
	})
	config := `
resource "nullplatform_application" "bot" {
  name         = "support-bot"
  namespace_id = 3
}
`

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{
				Config:             config,
				ResourceName:       "nullplatform_application.bot",
				ImportState:        true,
				ImportStateId:      "7",
				ImportStatePersist: true,
			},
			{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.TestCheckResourceAttr("nullplatform_application.bot", "application_type_id", "type-1"),
			},
		},
	})
}

// The platform reads the ids of a type and of a snapshot in lower case: an id
// written in upper case is the same id.
func TestFunctionalApplication_AnIDInUpperCaseIsTheSameID(t *testing.T) {
	fake := newApplicationTypeFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{
				Config: applicationTypeConfig("AI Agent", agentSchema, "") + `
resource "nullplatform_application" "bot" {
  name                         = "support-bot"
  namespace_id                 = 3
  application_type_id          = upper(nullplatform_application_type.agent.id)
  application_type_snapshot_id = upper(nullplatform_application_type.agent.last_snapshot_id)
  attributes                   = jsonencode({ model = "fable" })
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_application.bot", "application_type_id", "type-1"),
					resource.TestCheckResourceAttr("nullplatform_application.bot", "application_type_snapshot_id", "snapshot-1"),
				),
			},
		},
	})
}
