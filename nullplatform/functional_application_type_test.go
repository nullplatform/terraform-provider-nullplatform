package nullplatform

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"

	"github.com/nullplatform/terraform-provider-nullplatform/internal/fakeplatform"
)

// Real plan/apply/import/destroy over the fake platform: an application type
// keeps the order of its schema, cuts a snapshot when the schema changes, is
// deleted on destroy, and gives applications and templates their type.

func newApplicationTypeFake(t *testing.T) *fakeplatform.Server {
	t.Helper()
	fake := fakeplatform.New()
	fakeplatform.RegisterApplicationTypes(fake)
	t.Cleanup(fake.Close)
	return fake
}

// Properties out of alphabetical order, as a form would have them.
const (
	agentSchema          = `{"type":"object","properties":{"model":{"type":"string"},"autonomy":{"type":"string"}},"required":["model"]}`
	agentSchemaReordered = `{"type":"object","properties":{"autonomy":{"type":"string"},"model":{"type":"string"}},"required":["model"]}`
	agentSchemaSpaced    = `{ "type": "object", "properties": { "model": { "type": "string" }, "autonomy": { "type": "string" } }, "required": ["model"] }`
)

func applicationTypeConfig(name, typeSchema, extra string) string {
	return fmt.Sprintf(`
resource "nullplatform_application_type" "agent" {
  nrn    = "organization=1:account=2"
  name   = %q
  schema = %q
%s
}
`, name, typeSchema, extra)
}

func requestsTo(fake *fakeplatform.Server, method, path string) []fakeplatform.Request {
	var matching []fakeplatform.Request
	for _, request := range requestsOf(fake, method) {
		if request.Path == path {
			matching = append(matching, request)
		}
	}
	return matching
}

func TestFunctionalApplicationType_Lifecycle(t *testing.T) {
	fake := newApplicationTypeFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		// Destroy deletes the type, and so does replacing it.
		CheckDestroy: func(*terraform.State) error {
			if types := fake.Items("application_type"); len(types) != 0 {
				return fmt.Errorf("want no type left, got %v", types)
			}
			deletes := requestsOf(fake, http.MethodDelete)
			if len(deletes) != 2 || deletes[0].Path != "/application_type/type-1" || deletes[1].Path != "/application_type/type-2" {
				return fmt.Errorf("want one DELETE per type, got %v", deletes)
			}
			if retired := requestsTo(fake, http.MethodPatch, "/application_type/type-2"); len(retired) != 0 {
				return fmt.Errorf("want no PATCH on destroy, got %v", retired)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: applicationTypeConfig("AI Agent", agentSchema, ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_application_type.agent", "slug", "ai-agent"),
					resource.TestCheckResourceAttr("nullplatform_application_type.agent", "status", "active"),
					resource.TestCheckResourceAttr("nullplatform_application_type.agent", "visible_to.#", "1"),
					resource.TestCheckResourceAttr("nullplatform_application_type.agent", "visible_to.0", "organization=1:account=2"),
					resource.TestCheckResourceAttr("nullplatform_application_type.agent", "description", ""),
					resource.TestCheckResourceAttr("nullplatform_application_type.agent", "last_snapshot_id", "snapshot-1"),
					resource.TestCheckResourceAttr("nullplatform_application_type.agent", "schema", agentSchema),
					func(*terraform.State) error {
						// What the API is not told, it decides: no status, visible_to or description.
						want := `{"nrn":"organization=1:account=2","name":"AI Agent","schema":` + agentSchema + `}`
						if got := requestsTo(fake, http.MethodPost, "/application_type"); len(got) != 1 || got[0].Body != want {
							return fmt.Errorf("create body:\n got %v\nwant %s", got, want)
						}
						return nil
					},
				),
			},
			{
				// The same schema written with other whitespace is no change.
				Config: applicationTypeConfig("AI Agent", agentSchemaSpaced, ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				ResourceName:      "nullplatform_application_type.agent",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				// Everything but the schema: one PATCH naming only what changed, no new snapshot.
				Config: applicationTypeConfig("Agent", agentSchema, `
  description = "Runs a model"
  status      = "inactive"
  visible_to  = ["organization=1:account=2", "organization=1:account=3"]
`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("nullplatform_application_type.agent", plancheck.ResourceActionUpdate),
						plancheck.ExpectKnownValue("nullplatform_application_type.agent", tfjsonpath.New("last_snapshot_id"), knownvalue.StringExact("snapshot-1")),
					},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_application_type.agent", "name", "Agent"),
					// A slug is for life.
					resource.TestCheckResourceAttr("nullplatform_application_type.agent", "slug", "ai-agent"),
					resource.TestCheckResourceAttr("nullplatform_application_type.agent", "description", "Runs a model"),
					resource.TestCheckResourceAttr("nullplatform_application_type.agent", "status", "inactive"),
					resource.TestCheckResourceAttr("nullplatform_application_type.agent", "visible_to.#", "2"),
					resource.TestCheckResourceAttr("nullplatform_application_type.agent", "last_snapshot_id", "snapshot-1"),
					func(*terraform.State) error {
						patches := requestsTo(fake, http.MethodPatch, "/application_type/type-1")
						if len(patches) != 1 || strings.Contains(patches[0].Body, "schema") || strings.Contains(patches[0].Body, "nrn") {
							return fmt.Errorf("want one patch without schema or nrn, got %v", patches)
						}
						return nil
					},
				),
			},
			{
				// The description is cleared, not left behind.
				Config: applicationTypeConfig("Agent", agentSchema, `
  status     = "inactive"
  visible_to = ["organization=1:account=2", "organization=1:account=3"]
`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_application_type.agent", "description", ""),
					func(*terraform.State) error {
						patches := requestsTo(fake, http.MethodPatch, "/application_type/type-1")
						if want := `{"description":""}`; patches[len(patches)-1].Body != want {
							return fmt.Errorf("patch body: got %s, want %s", patches[len(patches)-1].Body, want)
						}
						return nil
					},
				),
			},
			{
				// A different owner is a different type: the old one is deleted first,
				// which gives its name back.
				Config: strings.Replace(applicationTypeConfig("Agent", agentSchema, ""),
					`nrn    = "organization=1:account=2"`, `nrn    = "organization=1:account=9"`, 1),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("nullplatform_application_type.agent", plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_application_type.agent", "id", "type-2"),
					resource.TestCheckResourceAttr("nullplatform_application_type.agent", "nrn", "organization=1:account=9"),
				),
			},
		},
	})
}

func TestFunctionalApplicationType_SchemaOrderIsKeptAndIsAChange(t *testing.T) {
	fake := newApplicationTypeFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{
				Config: applicationTypeConfig("AI Agent", agentSchemaSpaced, ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_application_type.agent", "last_snapshot_id", "snapshot-1"),
					func(*terraform.State) error {
						// Sent in the order written: "model" before "autonomy".
						got := requestsTo(fake, http.MethodPost, "/application_type")
						if len(got) != 1 || !strings.Contains(got[0].Body, `"schema":`+agentSchema) {
							return fmt.Errorf("want the schema sent in the order written, got %v", got)
						}
						return nil
					},
				),
			},
			{
				// Only the order of the properties changes: a new schema, a new snapshot,
				// and the plan says the snapshot is about to change.
				Config: applicationTypeConfig("AI Agent", agentSchemaReordered, ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("nullplatform_application_type.agent", plancheck.ResourceActionUpdate),
						plancheck.ExpectUnknownValue("nullplatform_application_type.agent", tfjsonpath.New("last_snapshot_id")),
					},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_application_type.agent", "schema", agentSchemaReordered),
					resource.TestCheckResourceAttr("nullplatform_application_type.agent", "last_snapshot_id", "snapshot-2"),
					func(*terraform.State) error {
						patches := requestsTo(fake, http.MethodPatch, "/application_type/type-1")
						if want := `{"schema":` + agentSchemaReordered + `}`; len(patches) != 1 || patches[0].Body != want {
							return fmt.Errorf("patch body: got %v, want %s", patches, want)
						}
						return nil
					},
				),
			},
		},
	})
}

// callFake sends one request to the fake, as something other than Terraform
// would to the API.
func callFake(t *testing.T, fake *fakeplatform.Server, method, path string) {
	t.Helper()
	request, err := http.NewRequest(method, fake.URL()+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := fake.HTTP().Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode >= 300 {
		t.Fatalf("%s %s: status %d", method, path, response.StatusCode)
	}
}

func TestFunctionalApplicationType_DeletedOutsideTerraformIsCreatedAgain(t *testing.T) {
	fake := newApplicationTypeFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: applicationTypeConfig("AI Agent", agentSchema, "")},
			{
				PreConfig: func() { callFake(t, fake, http.MethodDelete, "/application_type/type-1") },
				Config:    applicationTypeConfig("AI Agent", agentSchema, ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("nullplatform_application_type.agent", plancheck.ResourceActionCreate),
					},
				},
				// The name was given back with the delete.
				Check: resource.TestCheckResourceAttr("nullplatform_application_type.agent", "id", "type-2"),
			},
		},
	})
}

func TestFunctionalApplicationType_DestroyIsRefusedWhileSomethingUsesTheType(t *testing.T) {
	fake := newApplicationTypeFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: applicationTypeConfig("AI Agent", agentSchema, "")},
			{
				// An application Terraform does not manage has the type.
				PreConfig: func() {
					fake.Seed("application", "7", fakeplatform.Item{"name": "Support bot", "application_type_id": "type-1"})
				},
				Config:      applicationTypeConfig("AI Agent", agentSchema, ""),
				Destroy:     true,
				ExpectError: regexp.MustCompile(`status code 409(.|\s)*1 application of this type still exists`),
			},
			{
				PreConfig: func() {
					// Its workflow finished deleting it.
					fake.Seed("application", "7", fakeplatform.Item{"name": "Support bot", "application_type_id": "type-1", "status": "deleted"})
					fake.Seed("template", "3", fakeplatform.Item{"name": "Agent template", "application_type_id": "type-1"})
				},
				Config:      applicationTypeConfig("AI Agent", agentSchema, ""),
				Destroy:     true,
				ExpectError: regexp.MustCompile(`status code 409(.|\s)*1 template still belongs to it`),
			},
			{
				// Once nothing uses it, the type is still in the state and goes.
				PreConfig: func() { callFake(t, fake, http.MethodDelete, "/template/3") },
				Config:    applicationTypeConfig("AI Agent", agentSchema, ""),
				Check:     resource.TestCheckResourceAttr("nullplatform_application_type.agent", "id", "type-1"),
			},
		},
	})
}

func TestFunctionalApplicationType_RefusalsCarryTheAPIMessage(t *testing.T) {
	fake := newApplicationTypeFake(t)
	// An inactive type still holds its name in its organization.
	fake.Seed("application_type", "type-1", fakeplatform.Item{"nrn": "organization=1:account=7", "name": "AI Agent", "status": "inactive"})
	// Another organization's type does not.
	fake.Seed("application_type", "type-2", fakeplatform.Item{"nrn": "organization=5", "name": "Job", "status": "active"})

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{
				Config:      applicationTypeConfig("AI Agent", agentSchema, ""),
				ExpectError: regexp.MustCompile(`There is already an application type named \\"AI Agent\\" where this\s+one\s+would\s+be\s+visible`),
			},
			{
				Config:      applicationTypeConfig("AI Agent", `["not", "an", "object"]`, ""),
				ExpectError: regexp.MustCompile(`"schema" must be a JSON object`),
			},
			{
				Config:      applicationTypeConfig("Job", agentSchema, "  visible_to = []"),
				ExpectError: regexp.MustCompile(`Attribute visible_to requires 1 item minimum`),
			},
			{
				Config: applicationTypeConfig("Job", agentSchema, ""),
				Check:  resource.TestCheckResourceAttr("nullplatform_application_type.agent", "slug", "job"),
			},
		},
	})
}

func TestFunctionalApplicationType_DataSource(t *testing.T) {
	fake := newApplicationTypeFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{{
			Config: applicationTypeConfig("AI Agent", agentSchema, `  description = "Runs a model"`) + `
data "nullplatform_application_type" "agent" {
  id = nullplatform_application_type.agent.id
}
`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("data.nullplatform_application_type.agent", "id", "type-1"),
				resource.TestCheckResourceAttr("data.nullplatform_application_type.agent", "nrn", "organization=1:account=2"),
				resource.TestCheckResourceAttr("data.nullplatform_application_type.agent", "name", "AI Agent"),
				resource.TestCheckResourceAttr("data.nullplatform_application_type.agent", "slug", "ai-agent"),
				resource.TestCheckResourceAttr("data.nullplatform_application_type.agent", "description", "Runs a model"),
				resource.TestCheckResourceAttr("data.nullplatform_application_type.agent", "status", "active"),
				resource.TestCheckResourceAttr("data.nullplatform_application_type.agent", "visible_to.#", "1"),
				resource.TestCheckResourceAttr("data.nullplatform_application_type.agent", "schema", agentSchema),
				resource.TestCheckResourceAttr("data.nullplatform_application_type.agent", "last_snapshot_id", "snapshot-1"),
			),
		}},
	})
}
