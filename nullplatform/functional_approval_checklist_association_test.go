package nullplatform

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/nullplatform/terraform-provider-nullplatform/internal/fakeplatform"
)

const (
	linkAddress   = "nullplatform_approval_action_checklist_specification_association.deployment_create"
	actionAddress = "nullplatform_approval_action.deployment_create"
	linkPath      = APPROVAL_ACTION_PATH + "/1/checklist_specification"
)

func newLinkFake(t *testing.T) (*fakeplatform.Server, *fakeplatform.Approvals, *NullClient) {
	t.Helper()
	fake := fakeplatform.New()
	approvals := fakeplatform.RegisterApproval(fake)
	fakeplatform.RegisterChecklist(fake)
	t.Cleanup(fake.Close)
	return fake, approvals, newTestClient(fake.HTTP())
}

// shortenLinkRetries makes a link refused for live policies try again at once.
func shortenLinkRetries(t *testing.T) {
	t.Helper()
	previous := checklistLinkRetryInterval
	checklistLinkRetryInterval = 10 * time.Millisecond
	t.Cleanup(func() { checklistLinkRetryInterval = previous })
}

// linkedConfig is a specification, an action in checklist mode and their
// link; actionAttributes and linkAttributes go inside those blocks.
func linkedConfig(actionAttributes, linkAttributes string) string {
	return linkedSpecConfig(`  name       = "spec"
  definition = `+specHCLV1, actionAttributes, linkAttributes)
}

// linkedSpecConfig is linkedConfig with the specification's attributes.
func linkedSpecConfig(specAttributes, actionAttributes, linkAttributes string) string {
	return fmt.Sprintf(`
provider "nullplatform" {}

resource "nullplatform_checklist_specification" "spec" {
  nrn = %[1]q
%[2]s
}

resource "nullplatform_approval_action" "deployment_create" {
  nrn        = %[1]q
  entity     = "deployment"
  action     = "deployment:create"
  dimensions = { environment = "production" }
%[3]s
}

resource "nullplatform_approval_action_checklist_specification_association" "deployment_create" {
  approval_action_id         = nullplatform_approval_action.deployment_create.id
  checklist_specification_id = nullplatform_checklist_specification.spec.current_version_id
%[4]s
}
`, approvalNrn, specAttributes, actionAttributes, linkAttributes)
}

// linkOnlyConfig links existing objects by their ids.
func linkOnlyConfig(actionID, specID, attributes string) string {
	return fmt.Sprintf(`
provider "nullplatform" {}

resource "nullplatform_approval_action_checklist_specification_association" "deployment_create" {
  approval_action_id         = %q
  checklist_specification_id = %q
%s
}
`, actionID, specID, attributes)
}

// newAPIAction creates action 1 and specification version 1 through the API,
// outside Terraform: the action with the API's defaults.
func newAPIAction(t *testing.T, client *NullClient) {
	t.Helper()
	if _, err := client.CreateChecklistSpecification(&ChecklistSpecification{Nrn: approvalNrn, Name: "spec", Definition: json.RawMessage(specJSONV1)}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.CreateApprovalAction(&ApprovalAction{Nrn: approvalNrn, Entity: "deployment", Action: "deployment:create",
		Dimensions: map[string]string{"environment": "production"}}); err != nil {
		t.Fatal(err)
	}
}

// withLivePolicies gives action 1 live associations to as many new policies.
func withLivePolicies(t *testing.T, client *NullClient, count int) {
	t.Helper()
	for i := 1; i <= count; i++ {
		policy, err := client.CreateApprovalPolicy(&ApprovalPolicy{Nrn: approvalNrn, Name: fmt.Sprintf("p%d", i), Conditions: map[string]any{}, Selector: map[string]any{}})
		if err != nil {
			t.Fatal(err)
		}
		if err := client.AssociatePolicyWithAction("1", fmt.Sprint(policy.Id)); err != nil {
			t.Fatal(err)
		}
	}
}

func linkPosts(fake *fakeplatform.Server) int {
	posts := 0
	for _, r := range requestsOf(fake, http.MethodPost) {
		if r.Path == linkPath {
			posts++
		}
	}
	return posts
}

// A specification, an action and their link made through the API import into
// a configuration that declares neither on_policy_* nor on_checklist_fail
// (seeded by the link), and plan nothing.
func TestFunctionalChecklistAssociation_Import(t *testing.T) {
	fake, _, client := newLinkFake(t)
	newAPIAction(t, client)
	if err := client.LinkChecklistSpecification("1", specVersionID(1)); err != nil {
		t.Fatal(err)
	}
	imports := fmt.Sprintf(`
import {
  to = nullplatform_checklist_specification.spec
  id = %q
}
import {
  to = %s
  id = "1"
}
import {
  to = %s
  id = "1"
}
`, specVersionID(1), actionAddress, linkAddress)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{{
			Config: linkedConfig("", "") + imports,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr(actionAddress, "on_policy_success", "manual"),
				resource.TestCheckResourceAttr(actionAddress, "on_checklist_fail", "pending"),
				resource.TestCheckResourceAttr(actionAddress, "checklist_specification_id", specVersionID(1)),
				resource.TestCheckResourceAttr(linkAddress, "checklist_specification_id", specVersionID(1)),
			),
		}},
	})
}

// An action with no link has nothing to import; a missing one is not readable.
func TestFunctionalChecklistAssociation_ImportUnlinkedRefused(t *testing.T) {
	fake, _, client := newLinkFake(t)
	newAPIAction(t, client)
	importLink := func(id string, expect *regexp.Regexp) resource.TestStep {
		return resource.TestStep{Config: linkOnlyConfig(id, specVersionID(1), ""), ResourceName: linkAddress,
			ImportState: true, ImportStateId: id, ExpectError: expect}
	}

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			importLink("1", wrapped("approval action 1 is not linked to a checklist specification; there is nothing to import")),
			importLink("999999999", wrapped("approval action 999999999 not found or not readable with this API key")),
		},
	})
}

// A create over an action linked to another version — by the API, a
// migration, another state or a second block — never takes the link over: it
// asks for an import.
func TestFunctionalChecklistAssociation_AlreadyLinkedRequiresImport(t *testing.T) {
	fake, _, client := newLinkFake(t)
	newAPIAction(t, client)
	if _, err := client.PatchChecklistSpecification(specVersionID(1), &ChecklistSpecification{Definition: json.RawMessage(specJSONV2)}); err != nil {
		t.Fatal(err)
	}
	if err := client.LinkChecklistSpecification("1", specVersionID(2)); err != nil {
		t.Fatal(err)
	}

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{{
			Config: linkOnlyConfig("1", specVersionID(1), ""),
			ExpectError: wrapped("approval action 1 is already linked to checklist specification " + specVersionID(2) +
				"; import the link instead: terraform import nullplatform_approval_action_checklist_specification_association.<name> 1"),
		}},
	})
}

// lostLink is a link the API made after the plan whose answer never reached
// the provider: a POST cut on its way back.
type lostLink struct{ client *NullClient }

func (l lostLink) CheckPlan(_ context.Context, _ plancheck.CheckPlanRequest, resp *plancheck.CheckPlanResponse) {
	resp.Error = l.client.LinkChecklistSpecification("1", specVersionID(1))
}

// An action already linked to the desired version is adopted, with no second
// link: what a link whose answer was lost leaves converges without asking for
// an import.
func TestFunctionalChecklistAssociation_SameLinkAdopted(t *testing.T) {
	fake, _, client := newLinkFake(t)
	newAPIAction(t, client)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{{
			Config:           linkOnlyConfig("1", specVersionID(1), ""),
			ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{lostLink{client}}},
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr(linkAddress, "checklist_specification_id", specVersionID(1)),
				func(*terraform.State) error {
					if posts := linkPosts(fake); posts != 1 {
						return fmt.Errorf("sent %d links, want only the lost one", posts)
					}
					return nil
				},
			),
		}},
	})
}

// An action deleted outside Terraform takes its link out of the state too:
// the plan offers the action and the link again.
func TestFunctionalChecklistAssociation_ActionDeletedOutside(t *testing.T) {
	fake, _, client := newLinkFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: linkedConfig("", "")},
			{
				PreConfig: func() {
					if err := client.DeleteApprovalAction("1"); err != nil {
						t.Fatal(err)
					}
				},
				Config: linkedConfig("", ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(actionAddress, plancheck.ResourceActionCreate),
					plancheck.ExpectResourceAction(linkAddress, plancheck.ResourceActionCreate),
				}},
				Check: resource.TestCheckResourceAttr(linkAddress, "id", "2"),
			},
		},
	})
}

// The specification's id is not a version: refused in the plan when it
// is known, and before any link request when it is not known until apply.
func TestFunctionalChecklistAssociation_RejectsSpecificationID(t *testing.T) {
	fake, _, client := newLinkFake(t)
	e3 := func(id string) *regexp.Regexp {
		return wrapped(`checklist_specification_id "` + id + `" is the id of a nullplatform_checklist_specification, not a ` +
			`specification version; reference nullplatform_checklist_specification.<name>.current_version_id`)
	}

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: linkOnlyConfig("1", approvalNrn+"/spec", ""), ExpectError: e3(approvalNrn + "/spec")},
			{
				Config:      strings.Replace(linkedConfig("", ""), "spec.current_version_id", "spec.id", 1),
				ExpectError: e3(approvalNrn + "/spec"),
			},
			{
				PreConfig: func() {
					if posts := linkPosts(fake); posts != 0 {
						t.Errorf("sent %d links, want none", posts)
					}
					if _, err := client.GetApprovalAction("1"); err != nil {
						t.Errorf("the action the refused apply created: %v", err)
					}
				},
				Config: linkedConfig("", ""),
			},
		},
	})
}

// A specification's id is refused also on an action already linked to another
// version: the value is refused before the action is read, and no import is
// asked for.
func TestFunctionalChecklistAssociation_RejectsSpecificationIDBeforeTheLink(t *testing.T) {
	fake, _, client := newLinkFake(t)
	newAPIAction(t, client)
	if err := client.LinkChecklistSpecification("1", specVersionID(1)); err != nil {
		t.Fatal(err)
	}

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{{
			Config: fmt.Sprintf(`
provider "nullplatform" {}

resource "nullplatform_checklist_specification" "other" {
  nrn        = %q
  name       = "other"
  definition = %s
}

resource "nullplatform_approval_action_checklist_specification_association" "deployment_create" {
  approval_action_id         = "1"
  checklist_specification_id = nullplatform_checklist_specification.other.id
}
`, approvalNrn, specHCLV1),
			ExpectError: wrapped(`checklist_specification_id "` + approvalNrn + `/other" is the id of a nullplatform_checklist_specification, ` +
				`not a specification version; reference nullplatform_checklist_specification.<name>.current_version_id`),
		}},
	})
}

// A link undone outside Terraform leaves the state and the plan links again;
// removing the block unlinks, and the action keeps its fail path.
func TestFunctionalChecklistAssociation_Unlink(t *testing.T) {
	fake, _, client := newLinkFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: linkedConfig("", "")},
			{
				PreConfig: func() {
					if err := client.UnlinkChecklistSpecification("1"); err != nil {
						t.Fatal(err)
					}
				},
				Config: linkedConfig("", ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(linkAddress, plancheck.ResourceActionCreate),
				}},
			},
			{
				Config: strings.Split(linkedConfig("", ""), `resource "nullplatform_approval_action_checklist_specification_association"`)[0],
				Check: func(*terraform.State) error {
					action, err := client.GetApprovalAction("1")
					if err != nil || action.ChecklistSpecificationId != "" || action.OnChecklistFail != "pending" {
						return fmt.Errorf("action %+v, %v; want unlinked, on_checklist_fail still pending", action, err)
					}
					return nil
				},
			},
		},
	})
}

// While the action still has live policies the link tries again, and links
// once they are gone (the fake releases them after two refusals).
func TestFunctionalChecklistAssociation_RetriesWhilePoliciesLive(t *testing.T) {
	shortenLinkRetries(t)
	fake, approvals, client := newLinkFake(t)
	newAPIAction(t, client)
	withLivePolicies(t, client, 1)
	approvals.ReleasePoliciesAfter("1", 2)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{{
			Config: linkOnlyConfig("1", specVersionID(1), ""),
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr(linkAddress, "checklist_specification_id", specVersionID(1)),
				func(*terraform.State) error {
					if posts := linkPosts(fake); posts != 3 {
						return fmt.Errorf("sent %d links, want 3: two refused, then the link", posts)
					}
					return nil
				},
			),
		}},
	})
}

// Policies still live when timeouts.create runs out fail the link, naming
// them.
func TestFunctionalChecklistAssociation_TimeoutListsPolicies(t *testing.T) {
	shortenLinkRetries(t)
	fake, _, client := newLinkFake(t)
	newAPIAction(t, client)
	withLivePolicies(t, client, 2)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{{
			Config: linkOnlyConfig("1", specVersionID(1), `  timeouts {
    create = "1s"
  }`),
			ExpectError: wrapped("approval action 1 still has policies 1, 2 after 1s: Approval action already has policies; " +
				"cannot also assign a checklist specification. Remove those policy associations (one that is not in this " +
				"configuration must be imported or deleted) and apply again"),
		}},
	})
}
