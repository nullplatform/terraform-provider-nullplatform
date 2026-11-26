package nullplatform

import (
	"encoding/json"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/nullplatform/terraform-provider-nullplatform/internal/fakeplatform"
)

func newWorkflowFake(t *testing.T, engine fakeplatform.WorkflowEngine) (*fakeplatform.Server, *fakeplatform.WorkflowEngine) {
	t.Helper()
	fake := fakeplatform.New()
	knobs := fakeplatform.RegisterWorkflows(fake, engine)
	t.Cleanup(fake.Close)
	return fake, knobs
}

// workflowConfig is a webhook workflow; extra are more attributes of it.
func workflowConfig(message, extra string) string {
	return fmt.Sprintf(`
provider "nullplatform" {}

resource "nullplatform_workflow" "gate" {
  name = "payments-gate"
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
`, message, extra)
}

func workflowIDOf(s *terraform.State, address string) string {
	return s.RootModule().Resources[address].Primary.ID
}

func TestFunctionalWorkflow_Lifecycle(t *testing.T) {
	fake, _ := newWorkflowFake(t, fakeplatform.WorkflowEngine{})
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
				ResourceName:      "nullplatform_workflow.gate",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				// Import by key: the state keeps the id.
				ResourceName:      "nullplatform_workflow.gate",
				ImportState:       true,
				ImportStateId:     "payments-gate",
				ImportStateVerify: true,
			},
		},
	})
}

func TestFunctionalWorkflow_ImportOfAnUnknownWorkflowFails(t *testing.T) {
	fake, _ := newWorkflowFake(t, fakeplatform.WorkflowEngine{})

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
	fake, _ := newWorkflowFake(t, fakeplatform.WorkflowEngine{})
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
	fake, _ := newWorkflowFake(t, fakeplatform.WorkflowEngine{})
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
	fake, _ := newWorkflowFake(t, fakeplatform.WorkflowEngine{})
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
	fake, _ := newWorkflowFake(t, fakeplatform.WorkflowEngine{})

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
				Config:      workflowConfig("first", `  nrn = "organization=1"`) + `resource "nullplatform_workflow" "bad" {
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
	fake, _ := newWorkflowFake(t, fakeplatform.WorkflowEngine{})

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
