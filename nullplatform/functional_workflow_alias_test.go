package nullplatform

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/nullplatform/terraform-provider-nullplatform/internal/fakeplatform"
)

const functionalChecklistNRN = "organization=1:account=2"

// workflowAliasConfig is a webhook workflow with the alias "live" on its
// latest revision.
func workflowAliasConfig(message string, active bool) string {
	return workflowConfig(message, "") + fmt.Sprintf(`
resource "nullplatform_workflow_alias" "live" {
  workflow_id = nullplatform_workflow.gate.id
  name        = "live"
  revision    = nullplatform_workflow.gate.revision
  active      = %t
}
`, active)
}

// checklistAliasConfig is a workflow whose trigger is a checklist item: its
// activation creates a notification channel at nrn.
func checklistAliasConfig(nrn string, active bool) string {
	return fmt.Sprintf(`
provider "nullplatform" {}

resource "nullplatform_workflow" "gate" {
  name = "payments-gate"
  key  = "payments-gate"
  definition = jsonencode({
    steps = {
      start = { id = "start", name = "On checklist", pluginType = "np-checklist-trigger", config = {
        kind = "release", nrn = %q, mode = "start", pathPrefix = "/payments"
      } }
      say = { id = "say", name = "Say", pluginType = "log", config = { message = "hello" } }
    }
    connections = [{ id = "c1", from = "start", to = "say" }]
  })
}

resource "nullplatform_workflow_alias" "live" {
  workflow_id = nullplatform_workflow.gate.id
  name        = "live"
  revision    = nullplatform_workflow.gate.revision
  active      = %t
}
`, nrn, active)
}

func aliasOf(fake *fakeplatform.Server, id string) (fakeplatform.Item, error) {
	alias, ok := fakeplatform.WorkflowAlias(fake, id, "live")
	if !ok {
		return nil, fmt.Errorf("workflow %s has no alias live", id)
	}
	return alias, nil
}

func TestFunctionalWorkflowAlias_Lifecycle(t *testing.T) {
	fake, _ := newWorkflowFake(t, fakeplatform.WorkflowEngine{})
	var id string
	remember := func(s *terraform.State) error { id = workflowIDOf(s, "nullplatform_workflow.gate"); return nil }
	aliasIs := func(revision int, active bool) resource.TestCheckFunc {
		return func(*terraform.State) error {
			alias, err := aliasOf(fake, id)
			if err != nil {
				return err
			}
			if alias["revision"] != revision || alias["active"] != active {
				return fmt.Errorf("alias = revision %v active %v, want revision %d active %t", alias["revision"], alias["active"], revision, active)
			}
			return nil
		}
	}

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{
				Config: workflowAliasConfig("first", true),
				Check: resource.ComposeTestCheckFunc(
					remember,
					aliasIs(1, true),
					resource.TestCheckResourceAttr("nullplatform_workflow_alias.live", "webhook_urls.%", "1"),
					resource.TestCheckResourceAttrSet("nullplatform_workflow_alias.live", "webhook_urls.start"),
				),
			},
			{
				// One apply: the new revision, the alias moved onto it, active.
				Config: workflowAliasConfig("second", true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_workflow_alias.live", "revision", "2"),
					aliasIs(2, true),
				),
			},
			{
				Config: workflowAliasConfig("second", false),
				Check: resource.ComposeTestCheckFunc(
					aliasIs(2, false),
					resource.TestCheckResourceAttr("nullplatform_workflow_alias.live", "webhook_urls.%", "0"),
				),
			},
			{
				// Inactive, a new revision moves the alias without activating it.
				Config: workflowAliasConfig("third", false),
				Check:  aliasIs(3, false),
			},
			{
				Config: workflowAliasConfig("third", true),
				Check:  aliasIs(3, true),
			},
			{
				ResourceName:      "nullplatform_workflow_alias.live",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				// Import by the workflow's key: the state keeps its id.
				ResourceName:      "nullplatform_workflow_alias.live",
				ImportState:       true,
				ImportStateId:     "payments-gate/live",
				ImportStateVerify: true,
			},
		},
	})
}

func TestFunctionalWorkflowAlias_ImportRefusals(t *testing.T) {
	fake, _ := newWorkflowFake(t, fakeplatform.WorkflowEngine{})

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: workflowAliasConfig("first", true)},
			{
				ResourceName:  "nullplatform_workflow_alias.live",
				ImportState:   true,
				ImportStateId: "no-separator",
				ExpectError:   regexp.MustCompile(`is not "<workflow id or key>/<alias>"`),
			},
			{
				ResourceName:  "nullplatform_workflow_alias.live",
				ImportState:   true,
				ImportStateId: "payments-gate/missing",
				ExpectError:   regexp.MustCompile(`cannot import alias "missing" of workflow "payments-gate"`),
			},
		},
	})
}

func TestFunctionalWorkflowAlias_LatestCannotBeManaged(t *testing.T) {
	fake, _ := newWorkflowFake(t, fakeplatform.WorkflowEngine{})

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{
				Config: workflowConfig("first", "") + `
resource "nullplatform_workflow_alias" "latest" {
  workflow_id = nullplatform_workflow.gate.id
  name        = "latest"
  revision    = 1
}
`,
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`expected name to not be any of`),
			},
		},
	})
}

// An alias that exists already is not adopted silently: the error names the
// import.
func TestFunctionalWorkflowAlias_ExistingAliasPointsToImport(t *testing.T) {
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
					if _, err := newTestClient(fake.HTTP()).CreateWorkflowAlias(id, "live", 1); err != nil {
						t.Fatalf("creating the alias outside Terraform: %v", err)
					}
				},
				Config:      workflowAliasConfig("first", true),
				ExpectError: regexp.MustCompile(`(?s)to manage it, import it: terraform import`),
			},
		},
	})
}

// An alias deleted outside Terraform leaves the state on refresh and is
// created again.
func TestFunctionalWorkflowAlias_DeletedOutsideIsRecreated(t *testing.T) {
	fake, _ := newWorkflowFake(t, fakeplatform.WorkflowEngine{})
	var id string

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{
				Config: workflowAliasConfig("first", true),
				Check:  func(s *terraform.State) error { id = workflowIDOf(s, "nullplatform_workflow.gate"); return nil },
			},
			{
				PreConfig: func() {
					client := newTestClient(fake.HTTP())
					if _, err := client.DeactivateWorkflowAlias(id, "live"); err != nil {
						t.Fatalf("deactivating the alias outside Terraform: %v", err)
					}
					if err := client.DeleteWorkflowAlias(id, "live"); err != nil {
						t.Fatalf("deleting the alias outside Terraform: %v", err)
					}
				},
				Config: workflowAliasConfig("first", true),
				Check: func(*terraform.State) error {
					alias, err := aliasOf(fake, id)
					if err != nil {
						return err
					}
					if alias["active"] != true {
						return fmt.Errorf("the recreated alias is not active: %v", alias)
					}
					return nil
				},
			},
		},
	})
}

// An alias deactivated outside Terraform is drift: the next apply activates it.
func TestFunctionalWorkflowAlias_DeactivatedOutsideIsActivated(t *testing.T) {
	fake, _ := newWorkflowFake(t, fakeplatform.WorkflowEngine{})
	var id string

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{
				Config: workflowAliasConfig("first", true),
				Check:  func(s *terraform.State) error { id = workflowIDOf(s, "nullplatform_workflow.gate"); return nil },
			},
			{
				PreConfig: func() {
					if _, err := newTestClient(fake.HTTP()).DeactivateWorkflowAlias(id, "live"); err != nil {
						t.Fatalf("deactivating the alias outside Terraform: %v", err)
					}
				},
				Config: workflowAliasConfig("first", true),
				Check: func(*terraform.State) error {
					if alias, err := aliasOf(fake, id); err != nil || alias["active"] != true {
						return fmt.Errorf("alias = %v, %v: want it active", alias, err)
					}
					return nil
				},
			},
		},
	})
}

func TestFunctionalWorkflowAlias_ActivationFailureIsReported(t *testing.T) {
	fake, _ := newWorkflowFake(t, fakeplatform.WorkflowEngine{
		ReportActivationError: true,
		DeniedChannelNRNs:     []string{functionalChecklistNRN},
	})

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{
				Config:      checklistAliasConfig(functionalChecklistNRN, true),
				ExpectError: regexp.MustCompile(`(?s)activating alias live of workflow wf_\d+ on revision 1 failed`),
			},
		},
	})
}

// A deactivation that cannot release a channel keeps the alias: the apply
// fails, and the plan after it shows what is left to do.
func TestFunctionalWorkflowAlias_ReleaseFailureKeepsTheAlias(t *testing.T) {
	fake, engine := newWorkflowFake(t, fakeplatform.WorkflowEngine{
		ReportReleaseFailures:   true,
		UnreleasableChannelNRNs: []string{functionalChecklistNRN},
	})

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{
				Config: checklistAliasConfig(functionalChecklistNRN, true),
				Check: func(*terraform.State) error {
					if n := len(fakeplatform.WorkflowChannels(fake)); n != 1 {
						return fmt.Errorf("%d channels after the activation, want 1", n)
					}
					return nil
				},
			},
			{
				Config:      checklistAliasConfig(functionalChecklistNRN, false),
				ExpectError: regexp.MustCompile(`(?s)could not release all its\s+triggers`),
			},
			{
				// The release is allowed again: the same apply now deactivates.
				PreConfig: func() { engine.UnreleasableChannelNRNs = nil },
				Config:    checklistAliasConfig(functionalChecklistNRN, false),
				Check: func(*terraform.State) error {
					if n := len(fakeplatform.WorkflowChannels(fake)); n != 0 {
						return fmt.Errorf("%d channels after the deactivation, want 0", n)
					}
					return nil
				},
			},
		},
	})
}

// The API refuses the delete of an alias that was activated again after the
// deactivation: the provider deactivates once more and deletes.
func TestFunctionalWorkflowAlias_DestroyDeactivatesAgainWhenReactivated(t *testing.T) {
	fake, _ := newWorkflowFake(t, fakeplatform.WorkflowEngine{RefuseActiveAliasDelete: true})

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: workflowAliasConfig("first", true)},
		},
	})
}
