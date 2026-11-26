package nullplatform

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/nullplatform/terraform-provider-nullplatform/internal/fakeplatform"
)

// triggerNamed matches the failure of the checklist trigger "start" in an
// activation's error; the line breaks of the CLI's output may fall anywhere.
var triggerNamed = regexp.MustCompile(`(?s)activating\s+alias\s+live\s+of\s+workflow\s+wf_\d+\s+on\s+revision\s+\d+\s+failed.*trigger\s+start\s+\(np-checklist-trigger\)`)

func denyChannels(report bool) fakeplatform.WorkflowEngine {
	return fakeplatform.WorkflowEngine{ReportActivationError: report, DeniedChannelNRNs: []string{functionalChecklistNRN}}
}

func liftDenial(fake *fakeplatform.Server) {
	fakeplatform.SetWorkflowEngine(fake, func(e *fakeplatform.WorkflowEngine) { e.DeniedChannelNRNs = nil })
}

// aliasWantedIn is a PreConfig that fails the test unless the fake's alias
// "live" of the only workflow is as wanted.
func aliasWantedIn(t *testing.T, fake *fakeplatform.Server, revision int, active bool) func() {
	return func() {
		alias, err := aliasOf(fake, onlyWorkflowID(t, fake))
		if err != nil {
			t.Fatal(err)
		}
		if err := aliasIs(alias, revision, active); err != nil {
			t.Fatal(err)
		}
	}
}

// An activation the API refuses on create names the trigger, with the API
// reporting its activation error or not. The alias exists, inactive, and is
// tainted: the next plan replaces it, and the replacement activates.
func TestFunctionalWorkflowAlias_ActivationDeniedOnCreate(t *testing.T) {
	for _, report := range []bool{false, true} {
		t.Run(fmt.Sprintf("activationError=%t", report), func(t *testing.T) {
			fake := newWorkflowFake(t, denyChannels(report))

			resource.UnitTest(t, resource.TestCase{
				ProviderFactories: functionalFactories(fake),
				Steps: []resource.TestStep{
					{
						Config:      checklistAliasConfig(functionalChecklistNRN, true),
						ExpectError: triggerNamed,
					},
					{
						PreConfig:          aliasWantedIn(t, fake, 1, false),
						Config:             checklistAliasConfig(functionalChecklistNRN, true),
						PlanOnly:           true,
						ExpectNonEmptyPlan: true,
						ConfigPlanChecks: resource.ConfigPlanChecks{PostApplyPostRefresh: []plancheck.PlanCheck{
							plancheck.ExpectResourceAction("nullplatform_workflow.gate", plancheck.ResourceActionNoop),
							plancheck.ExpectResourceAction("nullplatform_workflow_alias.live", plancheck.ResourceActionReplace),
						}},
					},
					{
						// The denial lifted, the same configuration recovers.
						PreConfig: func() { liftDenial(fake) },
						Config:    checklistAliasConfig(functionalChecklistNRN, true),
						Check:     channelCountIs(fake, 1),
					},
				},
			})
		})
	}
}

// A new revision whose activation the API refuses leaves the alias on it,
// inactive: the apply fails and the next plan activates it.
func TestFunctionalWorkflowAlias_ActivationDeniedOnUpdate(t *testing.T) {
	fake := newWorkflowFake(t, denyChannels(true))

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: workflowAliasConfig("first", true)},
			{
				// The new definition adds a checklist trigger at a denied NRN.
				Config:      checklistAliasConfig(functionalChecklistNRN, true),
				ExpectError: triggerNamed,
			},
			{
				PreConfig:          aliasWantedIn(t, fake, 2, false),
				Config:             checklistAliasConfig(functionalChecklistNRN, true),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
				ConfigPlanChecks: resource.ConfigPlanChecks{PostApplyPostRefresh: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("nullplatform_workflow.gate", plancheck.ResourceActionNoop),
					plancheck.ExpectResourceAction("nullplatform_workflow_alias.live", plancheck.ResourceActionUpdate),
				}},
			},
			{
				PreConfig: func() { liftDenial(fake) },
				Config:    checklistAliasConfig(functionalChecklistNRN, true),
				Check:     channelCountIs(fake, 1),
			},
		},
	})
}

// A deactivation that cannot release a channel keeps the alias: the apply
// fails, and the plan after it shows what is left to do.
func TestFunctionalWorkflowAlias_ReleaseFailureKeepsTheAlias(t *testing.T) {
	fake := newWorkflowFake(t, fakeplatform.WorkflowEngine{
		ReportReleaseFailures:   true,
		UnreleasableChannelNRNs: []string{functionalChecklistNRN},
	})

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		// The destroy deactivates again, which releases the channel.
		CheckDestroy: channelCountIs(fake, 0),
		Steps: []resource.TestStep{
			{
				Config: checklistAliasConfig(functionalChecklistNRN, true),
				Check:  channelCountIs(fake, 1),
			},
			{
				Config:      checklistAliasConfig(functionalChecklistNRN, false),
				ExpectError: regexp.MustCompile(`(?s)could not release all its\s+triggers`),
			},
			{
				// The API marked the alias inactive, as configured: the plan is empty and
				// the channel is still registered.
				PreConfig: func() {
					fakeplatform.SetWorkflowEngine(fake, func(e *fakeplatform.WorkflowEngine) { e.UnreleasableChannelNRNs = nil })
				},
				Config:           checklistAliasConfig(functionalChecklistNRN, false),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
				Check:            channelCountIs(fake, 1),
			},
		},
	})
}

// The API refuses the delete of an alias whose channel it could not release
// (409 alias-not-released): the destroy fails with its detail and the alias
// stays. Once the release is possible the destroy completes.
func TestFunctionalWorkflowAlias_DestroyRefusedWhileUnreleased(t *testing.T) {
	fake := newWorkflowFake(t, fakeplatform.WorkflowEngine{
		UnreleasedOnDelete:      true,
		UnreleasableChannelNRNs: []string{functionalChecklistNRN},
	})

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		CheckDestroy: func(*terraform.State) error {
			if n := len(fake.Items(fakeplatform.WorkflowDefinitions)); n != 0 {
				return fmt.Errorf("%d workflows after the destroy", n)
			}
			return channelCountIs(fake, 0)(nil)
		},
		Steps: []resource.TestStep{
			{
				Config: checklistAliasConfig(functionalChecklistNRN, true),
				Check:  channelCountIs(fake, 1),
			},
			{
				Destroy:     true,
				Config:      checklistAliasConfig(functionalChecklistNRN, true),
				ExpectError: regexp.MustCompile(`(?s)alias\s+'live'\s+of\s+workflow\s+'wf_\d+'\s+still\s+holds\s+unreleased\s+trigger\s+channels`),
			},
			{
				// The alias was kept, deactivated, with its channel still registered.
				PreConfig: func() {
					aliasWantedIn(t, fake, 1, false)()
					if n := len(fakeplatform.WorkflowChannels(fake)); n != 1 {
						t.Fatalf("%d channels after the refused destroy, want 1", n)
					}
					fakeplatform.SetWorkflowEngine(fake, func(e *fakeplatform.WorkflowEngine) { e.UnreleasableChannelNRNs = nil })
				},
				Config: checklistAliasConfig(functionalChecklistNRN, true),
				Check:  channelCountIs(fake, 1),
			},
		},
	})
}

// aliasActiveOnce answers its first delete of an alias with the API's
// alias-active refusal, as when the alias was activated again since the
// provider deactivated it.
type aliasActiveOnce struct {
	NullOps
	deletes atomic.Int32
}

func (o *aliasActiveOnce) DeleteWorkflowAlias(id, name string) error {
	if o.deletes.Add(1) == 1 {
		return &WorkflowError{
			Op: "deleting workflow alias", StatusCode: 409, Type: "https://workflow-system.dev/errors/alias-active",
			Message: fmt.Sprintf("Alias '%s' is active; deactivate it before deleting", name),
		}
	}
	return o.NullOps.DeleteWorkflowAlias(id, name)
}

// deactivationsOf counts the deactivations the fake received.
func deactivationsOf(fake *fakeplatform.Server) int {
	n := 0
	for _, request := range fake.Requests() {
		if request.Method == "POST" && strings.HasSuffix(request.Path, "/deactivate") {
			n++
		}
	}
	return n
}

// The API refuses the delete of an alias that was activated again after the
// deactivation: the provider deactivates once more, deletes, and succeeds.
func TestFunctionalWorkflowAlias_DestroyDeactivatesAgainWhenReactivated(t *testing.T) {
	fake := newWorkflowFake(t, fakeplatform.WorkflowEngine{})
	ops := &aliasActiveOnce{NullOps: newTestClient(fake.HTTP())}

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactoriesWith(ops),
		CheckDestroy: func(*terraform.State) error {
			if n := ops.deletes.Load(); n != 2 {
				return fmt.Errorf("%d delete attempts, want 2 (one refused, one retried)", n)
			}
			if n := deactivationsOf(fake); n != 2 {
				return fmt.Errorf("%d deactivations, want 2 (one before each delete)", n)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{Config: workflowAliasConfig("first", true)},
		},
	})
}

// The API refuses to delete a workflow while an alias made outside Terraform is
// active; the error carries its message, and the workflow stays.
func TestFunctionalWorkflow_DestroyRefusedWhileAnAliasIsActive(t *testing.T) {
	fake := newWorkflowFake(t, fakeplatform.WorkflowEngine{})
	var id string
	client := func() *NullClient { return newTestClient(fake.HTTP()) }

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{
				Config: workflowConfig("first", ""),
				Check:  func(s *terraform.State) error { id = workflowIDOf(s, "nullplatform_workflow.gate"); return nil },
			},
			{
				PreConfig: func() {
					if _, err := client().CreateWorkflowAlias(id, "live", 1); err != nil {
						t.Fatalf("creating the alias outside Terraform: %v", err)
					}
					if _, err := client().ActivateWorkflowAlias(id, "live"); err != nil {
						t.Fatalf("activating the alias outside Terraform: %v", err)
					}
				},
				Destroy:     true,
				Config:      workflowConfig("first", ""),
				ExpectError: regexp.MustCompile(`(?s)cannot\s+delete\s+workflow\s+'wf_\d+':\s+alias\s+'live'\s+is\s+still\s+active`),
			},
			{
				// Released, the workflow can go: the framework's destroy follows.
				PreConfig: func() {
					if _, err := client().DeactivateWorkflowAlias(id, "live"); err != nil {
						t.Fatalf("deactivating the alias outside Terraform: %v", err)
					}
					if err := client().DeleteWorkflowAlias(id, "live"); err != nil {
						t.Fatalf("deleting the alias outside Terraform: %v", err)
					}
				},
				Config: workflowConfig("first", ""),
				Check: func(*terraform.State) error {
					if _, err := workflowRecordOf(fake, id); err != nil {
						return fmt.Errorf("the refused destroy lost the workflow: %w", err)
					}
					return nil
				},
			},
		},
	})
}

// readBackFailsOnce fails the first read of a workflow or an alias that is
// armed, as a network error right after a create succeeded.
type readBackFailsOnce struct {
	NullOps
	workflow, alias atomic.Bool
	failures        atomic.Int32
}

func (o *readBackFailsOnce) GetWorkflow(ref string) (*WorkflowWithRevision, error) {
	if o.workflow.CompareAndSwap(true, false) {
		o.failures.Add(1)
		return nil, errors.New("connection reset by peer")
	}
	return o.NullOps.GetWorkflow(ref)
}

func (o *readBackFailsOnce) GetWorkflowAlias(ref, name string) (*WorkflowAlias, error) {
	if o.alias.CompareAndSwap(true, false) {
		o.failures.Add(1)
		return nil, errors.New("connection reset by peer")
	}
	return o.NullOps.GetWorkflowAlias(ref, name)
}

func (o *readBackFailsOnce) failedOnce() resource.TestCheckFunc {
	return func(*terraform.State) error {
		if n := o.failures.Load(); n != 1 {
			return fmt.Errorf("%d reads failed, want 1", n)
		}
		return nil
	}
}

// A read that fails after the create succeeded only warns: the workflow is in
// the state with its id, and the next refresh completes it.
func TestFunctionalWorkflow_CreateReadBackFailureKeepsTheResource(t *testing.T) {
	fake := newWorkflowFake(t, fakeplatform.WorkflowEngine{})
	ops := &readBackFailsOnce{NullOps: newTestClient(fake.HTTP())}
	ops.workflow.Store(true)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactoriesWith(ops),
		Steps: []resource.TestStep{
			{
				Config: workflowConfig("first", ""),
				Check: resource.ComposeTestCheckFunc(
					ops.failedOnce(),
					resource.TestMatchResourceAttr("nullplatform_workflow.gate", "id", regexp.MustCompile(`^wf_\d+$`)),
					resource.TestCheckResourceAttr("nullplatform_workflow.gate", "revision", "1"),
				),
			},
			{
				Config: workflowConfig("first", ""),
				Check:  resource.TestCheckResourceAttrSet("nullplatform_workflow.gate", "nrn"),
			},
		},
	})
}

func TestFunctionalWorkflowAlias_CreateReadBackFailureKeepsTheResource(t *testing.T) {
	fake := newWorkflowFake(t, fakeplatform.WorkflowEngine{})
	ops := &readBackFailsOnce{NullOps: newTestClient(fake.HTTP())}
	ops.alias.Store(true)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactoriesWith(ops),
		Steps: []resource.TestStep{
			{
				Config: workflowAliasConfig("first", true),
				Check: resource.ComposeTestCheckFunc(
					ops.failedOnce(),
					resource.TestMatchResourceAttr("nullplatform_workflow_alias.live", "id", regexp.MustCompile(`^wf_\d+/live$`)),
					func(*terraform.State) error {
						alias, err := aliasOf(fake, onlyWorkflowID(t, fake))
						if err != nil {
							return err
						}
						return aliasIs(alias, 1, true)
					},
				),
			},
			{
				Config: workflowAliasConfig("first", true),
				Check:  resource.TestCheckResourceAttrSet("nullplatform_workflow_alias.live", "webhook_urls.start"),
			},
		},
	})
}
