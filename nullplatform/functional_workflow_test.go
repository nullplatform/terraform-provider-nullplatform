package nullplatform

import (
	"encoding/json"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"

	"github.com/nullplatform/terraform-provider-nullplatform/internal/fakeplatform"
)

// workflowConfig is a webhook workflow; extra are more attributes of it.
func workflowConfig(message, extra string) string {
	return workflowNamed("payments-gate", message, extra)
}

// workflowNamed is workflowConfig under another name; the key stays the same.
func workflowNamed(name, message, extra string) string {
	return fmt.Sprintf(`
provider "nullplatform" {}

resource "nullplatform_workflow" "gate" {
  name = %[3]q
  key  = "payments-gate"
%[2]s
  definition = jsonencode({
    steps = {
      start = { id = "start", name = "On webhook", pluginType = "webhook", config = { path = "/gate", mode = "start" } }
      say   = { id = "say", name = "Say", pluginType = "log", config = { message = %[1]q } }
    }
    connections = [{ id = "c1", from = "start", to = "say" }]
  })
}
`, message, extra, name)
}

func workflowIDOf(s *terraform.State, address string) string {
	return s.RootModule().Resources[address].Primary.ID
}

func TestFunctionalWorkflow_Lifecycle(t *testing.T) {
	fake := newWorkflowFake(t, fakeplatform.WorkflowEngine{})
	var id string
	remember := func(s *terraform.State) error { id = workflowIDOf(s, "nullplatform_workflow.gate"); return nil }

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		CheckDestroy: func(*terraform.State) error {
			if revisions := fakeplatform.WorkflowRevisions(fake, id); revisions != nil {
				return fmt.Errorf("workflow %s still exists after destroy", id)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: workflowConfig("first", `  description = "gates payments"`+"\n"+`  path        = "/governance/releases"`),
				Check: resource.ComposeTestCheckFunc(
					remember,
					resource.TestCheckResourceAttr("nullplatform_workflow.gate", "revision", "1"),
					resource.TestCheckResourceAttr("nullplatform_workflow.gate", "description", "gates payments"),
					resource.TestCheckResourceAttr("nullplatform_workflow.gate", "path", "/governance/releases"),
					resource.TestCheckResourceAttrSet("nullplatform_workflow.gate", "nrn"),
				),
			},
			{
				// A new definition saves revision 2.
				Config: workflowConfig("second", `  description = "gates payments"`+"\n"+`  path        = "/governance/releases"`),
				Check:  resource.TestCheckResourceAttr("nullplatform_workflow.gate", "revision", "2"),
			},
			{
				// The record's fields change in place: no revision.
				Config: workflowConfig("second", `  description = "gates payment releases"`+"\n"+`  path        = "/governance"`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_workflow.gate", "revision", "2"),
					resource.TestCheckResourceAttr("nullplatform_workflow.gate", "description", "gates payment releases"),
					resource.TestCheckResourceAttr("nullplatform_workflow.gate", "path", "/governance"),
					func(*terraform.State) error {
						if n := len(fakeplatform.WorkflowRevisions(fake, id)); n != 2 {
							return fmt.Errorf("the workflow has %d revisions, want 2", n)
						}
						return nil
					},
				),
			},
			{
				// An import holds the definition the API stores, with the step types it
				// fills in: equivalent to the written one (the import-block test proves
				// its plan is a no-op), not the same string.
				ResourceName:            "nullplatform_workflow.gate",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"definition"},
			},
			{
				// Import by key: the state keeps the id.
				ResourceName:            "nullplatform_workflow.gate",
				ImportState:             true,
				ImportStateId:           "payments-gate",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"definition"},
			},
		},
	})
}

func TestFunctionalWorkflow_ImportOfAnUnknownWorkflowFails(t *testing.T) {
	fake := newWorkflowFake(t, fakeplatform.WorkflowEngine{})

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: workflowConfig("first", "")},
			{
				ResourceName:  "nullplatform_workflow.gate",
				ImportState:   true,
				ImportStateId: "no-such-workflow",
				ExpectError:   regexp.MustCompile(`cannot import workflow "no-such-workflow"`),
			},
		},
	})
}

// A workflow deleted outside Terraform leaves the state on refresh: the next
// plan offers its creation, and a new id comes out of the apply.
func TestFunctionalWorkflow_DeletedOutsideIsRecreated(t *testing.T) {
	fake := newWorkflowFake(t, fakeplatform.WorkflowEngine{})
	var id string

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{
				Config: workflowConfig("first", ""),
				Check:  func(s *terraform.State) error { id = workflowIDOf(s, "nullplatform_workflow.gate"); return nil },
			},
			{
				PreConfig: func() {
					if err := newTestClient(fake.HTTP()).DeleteWorkflow(id); err != nil {
						t.Fatalf("deleting the workflow outside Terraform: %v", err)
					}
				},
				Config: workflowConfig("first", ""),
				Check: func(s *terraform.State) error {
					if now := workflowIDOf(s, "nullplatform_workflow.gate"); now == id {
						return fmt.Errorf("the workflow kept id %s: it was not recreated", id)
					}
					return nil
				},
			},
		},
	})
}

// A revision saved outside Terraform is drift: the plan shows it and the next
// apply saves the configured definition as a new revision.
func TestFunctionalWorkflow_RevisionSavedOutsideIsDrift(t *testing.T) {
	fake := newWorkflowFake(t, fakeplatform.WorkflowEngine{})
	var id string

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{
				Config: workflowConfig("first", ""),
				Check:  func(s *terraform.State) error { id = workflowIDOf(s, "nullplatform_workflow.gate"); return nil },
			},
			{
				PreConfig: func() {
					body := map[string]json.RawMessage{
						"name":        json.RawMessage(`"payments-gate"`),
						"steps":       json.RawMessage(`{"start":{"id":"start","name":"On webhook","pluginType":"webhook","config":{"path":"/gate","mode":"start"}}}`),
						"connections": json.RawMessage(`[]`),
					}
					if _, err := newTestClient(fake.HTTP()).PutWorkflowRevision(id, body); err != nil {
						t.Fatalf("saving a revision outside Terraform: %v", err)
					}
				},
				Config: workflowConfig("first", ""),
				Check:  resource.TestCheckResourceAttr("nullplatform_workflow.gate", "revision", "3"),
			},
		},
	})
}

func TestFunctionalWorkflow_DefinitionValidation(t *testing.T) {
	fake := newWorkflowFake(t, fakeplatform.WorkflowEngine{})
	withDefinition := func(definition string) string {
		return fmt.Sprintf(`
provider "nullplatform" {}

resource "nullplatform_workflow" "gate" {
  name       = "payments-gate"
  definition = %q
}
`, definition)
	}

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{
				Config:      withDefinition(`[]`),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`definition must be a JSON object`),
			},
			{
				Config:      withDefinition(`{"connections":[]}`),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`definition must have steps`),
			},
			{
				Config:      withDefinition(`{"steps":{},"name":"x"}`),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`definition must not have "name"`),
			},
			{
				Config:      withDefinition(`{"steps":{},"triggers":[]}`),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`definition must not have "triggers"`),
			},
		},
	})
}

func TestFunctionalWorkflow_PathAndKeyValidation(t *testing.T) {
	fake := newWorkflowFake(t, fakeplatform.WorkflowEngine{})

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{
				Config:      workflowConfig("first", `  path = "/governance/"`),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`must be written as the API stores it`),
			},
			{
				Config:      workflowConfig("first", `  path = ""`),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`must not be empty`),
			},
			{
				Config: workflowConfig("first", `  nrn = "organization=1"`) + `resource "nullplatform_workflow" "bad" {
  name       = "bad"
  key        = "Bad Key"
  definition = jsonencode({ steps = {} })
}`,
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`must be a lowercase slug`),
			},
		},
	})
}

func TestFunctionalWorkflow_CreateRefusedSurfacesTheAPIsMessage(t *testing.T) {
	fake := newWorkflowFake(t, fakeplatform.WorkflowEngine{})

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: workflowConfig("first", "")},
			{
				// The key is taken: the same key under another resource.
				Config: workflowConfig("first", "") + `
resource "nullplatform_workflow" "twin" {
  name       = "twin"
  key        = "payments-gate"
  definition = jsonencode({ steps = {} })
}
`,
				ExpectError: regexp.MustCompile(`(?s)creating workflow`),
			},
		},
	})
}

// workflowWrittenConfig is the workflow "gate" with its definition written by
// hand, as JSON: the spelling is the test's.
func workflowWrittenConfig(definition string) string {
	return fmt.Sprintf(`
provider "nullplatform" {}

resource "nullplatform_workflow" "gate" {
  name = "payments-gate"
  key  = "payments-gate"
  definition = <<-JSON
%s
JSON
}
`, definition)
}

// workflowRecordOf is the record the fake holds for a workflow.
func workflowRecordOf(fake *fakeplatform.Server, id string) (fakeplatform.Item, error) {
	for _, item := range fake.Items(fakeplatform.WorkflowDefinitions) {
		if record, _ := item["workflow"].(fakeplatform.Item); fakeplatform.Str(record, "id") == id {
			return record, nil
		}
	}
	return nil, fmt.Errorf("the fake holds no workflow %s", id)
}

// onlyWorkflowID is the id of the one workflow the fake holds: for a test whose
// failing step leaves no state to read it from.
func onlyWorkflowID(t *testing.T, fake *fakeplatform.Server) string {
	t.Helper()
	items := fake.Items(fakeplatform.WorkflowDefinitions)
	if len(items) != 1 {
		t.Fatalf("the fake holds %d workflows, want 1", len(items))
	}
	return fakeplatform.Str(items[0]["workflow"].(fakeplatform.Item), "id")
}

// revisionCountIs checks how many revisions the fake holds for the workflow
// whose id the pointer holds.
func revisionCountIs(fake *fakeplatform.Server, id *string, want int) resource.TestCheckFunc {
	return func(*terraform.State) error {
		if n := len(fakeplatform.WorkflowRevisions(fake, *id)); n != want {
			return fmt.Errorf("the workflow has %d revisions, want %d", n, want)
		}
		return nil
	}
}

// A definition spelled otherwise (key order, spacing) and a step "type" the API
// fills in are not a change: no plan, no revision.
func TestFunctionalWorkflow_SpellingOnlyChangesPlanNothing(t *testing.T) {
	fake := newWorkflowFake(t, fakeplatform.WorkflowEngine{})
	var id string
	nothing := resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
		plancheck.ExpectEmptyPlan(),
		plancheck.ExpectResourceAction("nullplatform_workflow.gate", plancheck.ResourceActionNoop),
	}}
	reordered := `{
  "connections": [ { "to": "say", "from": "start", "id": "c1" } ],
  "steps": {
    "say":   { "pluginType": "log", "name": "Say", "id": "say", "config": { "message": "first" } },
    "start": { "config": { "mode": "start", "path": "/gate" }, "pluginType": "webhook", "name": "On webhook", "id": "start" }
  }
}`
	typed := `{
  "steps": {
    "start": { "id": "start", "name": "On webhook", "type": "trigger", "pluginType": "webhook", "config": { "path": "/gate", "mode": "start" } },
    "say":   { "id": "say", "name": "Say", "type": "module", "pluginType": "log", "config": { "message": "first" } }
  },
  "connections": [ { "id": "c1", "from": "start", "to": "say" } ]
}`

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{
				Config: workflowConfig("first", ""),
				Check:  func(s *terraform.State) error { id = workflowIDOf(s, "nullplatform_workflow.gate"); return nil },
			},
			{
				Config:           workflowWrittenConfig(reordered),
				ConfigPlanChecks: nothing,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_workflow.gate", "revision", "1"),
					revisionCountIs(fake, &id, 1),
				),
			},
			{
				// The type the API filled in, now written down: the same definition.
				Config:           workflowWrittenConfig(typed),
				ConfigPlanChecks: nothing,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_workflow.gate", "revision", "1"),
					revisionCountIs(fake, &id, 1),
				),
			},
		},
	})
}

// Renaming, removing the description and moving the folder or the owner change
// the record in place: no revision, and the alias on the workflow plans nothing.
func TestFunctionalWorkflow_RecordOnlyChangesSaveNoRevision(t *testing.T) {
	fake := newWorkflowFake(t, fakeplatform.WorkflowEngine{})
	var id string

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{
				Config: workflowNamed("payments-gate", "first", `  description = "gates payments"`+"\n"+`  path        = "/governance"`+"\n"+`  nrn         = "organization=1"`) +
					workflowAliasBlock(true),
				Check: func(s *terraform.State) error { id = workflowIDOf(s, "nullplatform_workflow.gate"); return nil },
			},
			{
				Config: workflowNamed("payments-gate-renamed", "first", `  path = "/release-gates"`+"\n"+`  nrn  = "organization=1:account=2"`) +
					workflowAliasBlock(true),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("nullplatform_workflow.gate", plancheck.ResourceActionUpdate),
					plancheck.ExpectResourceAction("nullplatform_workflow_alias.live", plancheck.ResourceActionNoop),
				}},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_workflow.gate", "revision", "1"),
					resource.TestCheckResourceAttr("nullplatform_workflow_alias.live", "revision", "1"),
					revisionCountIs(fake, &id, 1),
					func(*terraform.State) error {
						record, err := workflowRecordOf(fake, id)
						if err != nil {
							return err
						}
						want := map[string]string{"name": "payments-gate-renamed", "description": "", "path": "/release-gates", "nrn": "organization=1:account=2"}
						for key, value := range want {
							if got := fakeplatform.Str(record, key); got != value {
								return fmt.Errorf("the fake's record has %s %q, want %q", key, got, value)
							}
						}
						alias, err := aliasOf(fake, id)
						if err != nil {
							return err
						}
						return aliasIs(alias, 1, true)
					},
				),
			},
		},
	})
}

// Import blocks (plannable import): the import plans no change, by id and by key.
func TestFunctionalWorkflow_ImportBlock(t *testing.T) {
	fake := newWorkflowFake(t, fakeplatform.WorkflowEngine{})

	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_5_0)},
		ProviderFactories:      functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: workflowConfig("first", `  description = "gates payments"`)},
			{
				ResourceName:    "nullplatform_workflow.gate",
				ImportState:     true,
				ImportStateKind: resource.ImportBlockWithID,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					return workflowIDOf(s, "nullplatform_workflow.gate"), nil
				},
			},
			{
				ResourceName:    "nullplatform_workflow.gate",
				ImportState:     true,
				ImportStateKind: resource.ImportBlockWithID,
				ImportStateId:   "payments-gate",
			},
		},
	})
}
