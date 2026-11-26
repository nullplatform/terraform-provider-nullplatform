package nullplatform

import (
	"fmt"
	"regexp"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"

	"github.com/nullplatform/terraform-provider-nullplatform/internal/fakeplatform"
)

const functionalChecklistNRN = "organization=1:account=2"

// workflowAliasConfig is a webhook workflow with the alias "live" on its
// latest revision.
func workflowAliasConfig(message string, active bool) string {
	return workflowConfig(message, "") + workflowAliasBlock(active)
}

// workflowAliasBlock is the alias "live" of the workflow "gate", on its latest
// revision.
func workflowAliasBlock(active bool) string {
	return fmt.Sprintf(`
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

// aliasIs checks a copy of the fake's alias: the fake answers numbers as
// json.Number.
func aliasIs(alias fakeplatform.Item, revision int, active bool) error {
	if fmt.Sprint(alias["revision"]) != strconv.Itoa(revision) || alias["active"] != active {
		return fmt.Errorf("alias = revision %v active %v, want revision %d active %t", alias["revision"], alias["active"], revision, active)
	}
	return nil
}

func TestFunctionalWorkflowAlias_Lifecycle(t *testing.T) {
	fake := newWorkflowFake(t, fakeplatform.WorkflowEngine{})
	var id string
	remember := func(s *terraform.State) error { id = workflowIDOf(s, "nullplatform_workflow.gate"); return nil }
	aliasHolds := func(revision int, active bool) resource.TestCheckFunc {
		return func(*terraform.State) error {
			alias, err := aliasOf(fake, id)
			if err != nil {
				return err
			}
			return aliasIs(alias, revision, active)
		}
	}

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{
				Config: workflowAliasConfig("first", true),
				Check: resource.ComposeTestCheckFunc(
					remember,
					aliasHolds(1, true),
					resource.TestCheckResourceAttr("nullplatform_workflow_alias.live", "webhook_urls.%", "1"),
					resource.TestCheckResourceAttrSet("nullplatform_workflow_alias.live", "webhook_urls.start"),
				),
			},
			{
				// One apply: the new revision, the alias moved onto it, active.
				Config: workflowAliasConfig("second", true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_workflow_alias.live", "revision", "2"),
					aliasHolds(2, true),
				),
			},
			{
				Config: workflowAliasConfig("second", false),
				Check: resource.ComposeTestCheckFunc(
					aliasHolds(2, false),
					resource.TestCheckResourceAttr("nullplatform_workflow_alias.live", "webhook_urls.%", "0"),
				),
			},
			{
				// Inactive, a new revision moves the alias without activating it.
				Config: workflowAliasConfig("third", false),
				Check:  aliasHolds(3, false),
			},
			{
				Config: workflowAliasConfig("third", true),
				Check:  aliasHolds(3, true),
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
	fake := newWorkflowFake(t, fakeplatform.WorkflowEngine{})

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
	fake := newWorkflowFake(t, fakeplatform.WorkflowEngine{})

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
	fake := newWorkflowFake(t, fakeplatform.WorkflowEngine{})
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
	fake := newWorkflowFake(t, fakeplatform.WorkflowEngine{})
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

// channelCountIs checks how many notification channels the fake holds.
func channelCountIs(fake *fakeplatform.Server, want int) resource.TestCheckFunc {
	return func(*terraform.State) error {
		if n := len(fakeplatform.WorkflowChannels(fake)); n != want {
			return fmt.Errorf("%d channels, want %d", n, want)
		}
		return nil
	}
}

// workflowAliasURLConfig is workflowAliasConfig plus an output reading the
// alias's webhook URL, or "none" while it has none.
func workflowAliasURLConfig(message string, active bool) string {
	return workflowAliasConfig(message, active) + `
output "url" {
  value     = try(nullplatform_workflow_alias.live.webhook_urls["start"], "none")
  sensitive = true
}
`
}

// webhook_urls is planned unknown when active changes, so what reads it is
// planned unknown too and takes the URL the apply registers.
func TestFunctionalWorkflowAlias_WebhookURLFollowsActivation(t *testing.T) {
	fake := newWorkflowFake(t, fakeplatform.WorkflowEngine{})
	unknown := resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
		plancheck.ExpectUnknownValue("nullplatform_workflow_alias.live", tfjsonpath.New("webhook_urls")),
		plancheck.ExpectUnknownOutputValue("url"),
	}}

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{
				Config: workflowAliasURLConfig("first", false),
				Check:  resource.TestCheckOutput("url", "none"),
			},
			{
				Config:           workflowAliasURLConfig("first", true),
				ConfigPlanChecks: unknown,
				Check:            resource.TestMatchOutput("url", regexp.MustCompile(`/workflows/webhooks/wf_\d+-payments-gate/live/[0-9a-f]+$`)),
			},
			{
				Config:           workflowAliasURLConfig("first", false),
				ConfigPlanChecks: unknown,
				Check:            resource.TestCheckOutput("url", "none"),
			},
		},
	})
}

// A checklist trigger's activation registers exactly one channel, at the
// trigger's NRN, however often the alias is activated; destroying releases it.
func TestFunctionalWorkflowAlias_ChecklistTriggerOwnsOneChannel(t *testing.T) {
	fake := newWorkflowFake(t, fakeplatform.WorkflowEngine{})

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		CheckDestroy: func(*terraform.State) error {
			return channelCountIs(fake, 0)(nil)
		},
		Steps: []resource.TestStep{
			{
				Config: checklistAliasConfig(functionalChecklistNRN, true),
				Check: resource.ComposeTestCheckFunc(
					channelCountIs(fake, 1),
					func(*terraform.State) error {
						if nrn := fakeplatform.Str(fakeplatform.WorkflowChannels(fake)[0], "nrn"); nrn != functionalChecklistNRN {
							return fmt.Errorf("the channel is at %q, want %q", nrn, functionalChecklistNRN)
						}
						return nil
					},
				),
			},
			{
				Config: checklistAliasConfig(functionalChecklistNRN, false),
				Check:  channelCountIs(fake, 0),
			},
			{
				Config: checklistAliasConfig(functionalChecklistNRN, true),
				Check:  channelCountIs(fake, 1),
			},
		},
	})
}

// Import blocks (plannable import): the import plans no change, by the
// workflow's id and by its key.
func TestFunctionalWorkflowAlias_ImportBlock(t *testing.T) {
	fake := newWorkflowFake(t, fakeplatform.WorkflowEngine{})

	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_5_0)},
		ProviderFactories:      functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: workflowAliasConfig("first", true)},
			{
				ResourceName:    "nullplatform_workflow_alias.live",
				ImportState:     true,
				ImportStateKind: resource.ImportBlockWithID,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					return workflowIDOf(s, "nullplatform_workflow.gate") + "/live", nil
				},
			},
			{
				ResourceName:    "nullplatform_workflow_alias.live",
				ImportState:     true,
				ImportStateKind: resource.ImportBlockWithID,
				ImportStateId:   "payments-gate/live",
			},
		},
	})
}
