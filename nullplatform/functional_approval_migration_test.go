package nullplatform

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/nullplatform/terraform-provider-nullplatform/internal/fakeplatform"
)

// specBlock is a specification for an action declared as
// approvalActionConfig's, and linkBlock links them; specAndLinkBlocks moves
// the action to the specification.
var (
	specBlock = fmt.Sprintf(`
resource "nullplatform_checklist_specification" "spec" {
  nrn        = %q
  name       = "spec"
  definition = %s
}
`, approvalNrn, specHCLV1)
	specAndLinkBlocks = specBlock + linkBlock("")
)

func linkBlock(attributes string) string {
	return `
resource "nullplatform_approval_action_checklist_specification_association" "deployment_create" {
  approval_action_id         = nullplatform_approval_action.deployment_create.id
  checklist_specification_id = nullplatform_checklist_specification.spec.current_version_id
` + attributes + `
}
`
}

// migratedImports declares what a migration by API linked: the specification
// and the link.
var migratedImports = fmt.Sprintf(`
import {
  to = nullplatform_checklist_specification.spec
  id = %q
}
import {
  to = %s
  id = "1"
}
`, specVersionID(1), linkAddress)

// migrateByAPI is the migration endpoint's result on action 1: a new
// specification linked and the associations archived.
func migrateByAPI(t *testing.T, client *NullClient, approvals *fakeplatform.Approvals) func() {
	return func() {
		spec, err := client.CreateChecklistSpecification(&ChecklistSpecification{Nrn: approvalNrn, Name: "spec", Definition: json.RawMessage(specJSONV1)})
		if err != nil {
			t.Fatal(err)
		}
		approvals.Migrate("1", spec.Id)
	}
}

// migrated checks the action ended linked to the specification.
func migrated(client *NullClient) resource.TestCheckFunc {
	return func(*terraform.State) error {
		action, err := client.GetApprovalAction("1")
		if err != nil || action.ChecklistSpecificationId != specVersionID(1) {
			return fmt.Errorf("action %+v, %v; want it on %s", action, err, specVersionID(1))
		}
		return nil
	}
}

// writeOrder is the position of each write in the fake's log.
func writeOrder(fake *fakeplatform.Server, method, path string) int {
	for i, r := range fake.Requests() {
		if r.Method == method && r.Path == path {
			return i
		}
	}
	return -1
}

// An action with association resources moves to a specification in one apply
// that deletes its associations and links the specification; the link waits
// out the associations still live.
func TestFunctionalApprovalMigration_AssociationsActionUnchanged(t *testing.T) {
	shortenLinkRetries(t)
	fake, _, client := newLinkFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: approvalActionConfig(approvalAttributes, ignorePolicies, associationBlock)},
			{
				Config: approvalActionConfig(approvalAttributes, ignorePolicies, specAndLinkBlocks),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("nullplatform_approval_action_policy_association.coverage", plancheck.ResourceActionDestroy),
					plancheck.ExpectResourceAction(specAddress, plancheck.ResourceActionCreate),
					plancheck.ExpectResourceAction(linkAddress, plancheck.ResourceActionCreate),
					plancheck.ExpectResourceAction(actionAddress, plancheck.ResourceActionNoop),
				}},
				Check: migrated(client),
			},
		},
	})
}

// With on_checklist_fail set too, the order is fixed: the
// associations go, the action is patched with only that, and then the link.
func TestFunctionalApprovalMigration_AssociationsActionChanged(t *testing.T) {
	shortenLinkRetries(t)
	fake, _, client := newLinkFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: approvalActionConfig(approvalAttributes, ignorePolicies, associationBlock)},
			{
				Config: approvalActionConfig(approvalAttributes, ignorePolicies+"\n  on_checklist_fail = \"manual\"", specAndLinkBlocks),
				Check: resource.ComposeTestCheckFunc(migrated(client), func(*terraform.State) error {
					unassociated := writeOrder(fake, http.MethodDelete, APPROVAL_ACTION_PATH+"/1/policy/1")
					patched := writeOrder(fake, http.MethodPatch, APPROVAL_ACTION_PATH+"/1")
					linked := writeOrder(fake, http.MethodPost, linkPath)
					patches := requestsOf(fake, http.MethodPatch)
					if unassociated < 0 || patched < unassociated || linked < patched ||
						len(patches) != 1 || patches[0].Body != `{"on_checklist_fail":"manual"}`+"\n" {
						return fmt.Errorf("order %d, %d, %d (patches %v); want the association, a PATCH of on_checklist_fail alone, the link",
							unassociated, patched, linked, patches)
					}
					return nil
				}),
			},
		},
	})
}

// An action with inline policies drops them before it is linked,
// in a fixed order: the link never meets a live policy, and no empty PATCH.
func TestFunctionalApprovalMigration_InlinePolicies(t *testing.T) {
	fake, _, client := newLinkFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: approvalActionConfig(approvalAttributes, `  policies = [nullplatform_approval_policy.coverage.id]`, "")},
			{
				Config: approvalActionConfig(approvalAttributes, "", specAndLinkBlocks),
				Check: resource.ComposeTestCheckFunc(migrated(client), func(*terraform.State) error {
					if posts, patches := linkPosts(fake), requestsOf(fake, http.MethodPatch); posts != 1 || len(patches) != 0 ||
						writeOrder(fake, http.MethodDelete, APPROVAL_ACTION_PATH+"/1/policy/1") > writeOrder(fake, http.MethodPost, linkPath) {
						return fmt.Errorf("%d links, patches %v; want one link after the policy left, no PATCH", posts, patches)
					}
					return nil
				}),
			},
		},
	})
}

// An action migrated with the API declares its specification and link by
// importing them; the archived association's delete changes nothing.
func TestFunctionalApprovalMigration_APIMigratedDeclaresSpec(t *testing.T) {
	fake, approvals, client := newLinkFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: approvalActionConfig(approvalAttributes, ignorePolicies, associationBlock)},
			{
				PreConfig: migrateByAPI(t, client, approvals),
				Config:    approvalActionConfig(approvalAttributes, ignorePolicies, specAndLinkBlocks+migratedImports),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("nullplatform_approval_action_policy_association.coverage", plancheck.ResourceActionDestroy),
					plancheck.ExpectResourceAction(specAddress, plancheck.ResourceActionNoop),
					plancheck.ExpectResourceAction(linkAddress, plancheck.ResourceActionNoop),
				}},
				Check: resource.ComposeTestCheckFunc(migrated(client), func(*terraform.State) error {
					if posts := linkPosts(fake); posts != 0 {
						return fmt.Errorf("sent %d links, want none: the link is imported", posts)
					}
					return nil
				}),
			},
			// Unlinked and linked again, in one apply each. The migration's archived
			// associations are still there, and they do not block the link: only
			// live ones do.
			{Config: approvalActionConfig(approvalAttributes, ignorePolicies, specBlock)},
			{
				Config: approvalActionConfig(approvalAttributes, ignorePolicies, specBlock+linkBlock(`  timeouts {
    create = "2s"
  }`)),
				Check: migrated(client),
			},
		},
	})
}

// An action the API migrated, whose configuration listed its policies inline,
// takes ignore_changes in their place when its link is declared. Unlinked
// alone, it plans nothing more: the API keeps listing the archived policy, and
// the action no longer declares it. No DELETE goes out for it either, which
// would change nothing.
func TestFunctionalApprovalMigration_APIMigratedInlineUnlinks(t *testing.T) {
	fake, approvals, client := newLinkFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: approvalActionConfig(approvalAttributes, `  policies = [nullplatform_approval_policy.coverage.id]`, "")},
			{
				PreConfig: migrateByAPI(t, client, approvals),
				Config:    approvalActionConfig(approvalAttributes, ignorePolicies, specAndLinkBlocks+migratedImports),
				Check:     migrated(client),
			},
			{
				Config: approvalActionConfig(approvalAttributes, ignorePolicies, specBlock),
				ConfigPlanChecks: resource.ConfigPlanChecks{PostApplyPostRefresh: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				}},
				Check: func(*terraform.State) error {
					if deletes := writeCount(fake, http.MethodDelete, APPROVAL_ACTION_PATH+"/1/policy/1"); deletes != 0 {
						return fmt.Errorf("sent %d deletes of the archived association, want none", deletes)
					}
					return nil
				},
			},
			// What the guide warns about: refreshed unlinked, the state lists the
			// archived policy, and none is live.
			{
				RefreshState: true,
				Check: resource.ComposeTestCheckFunc(resource.TestCheckResourceAttr(actionAddress, "policies.#", "1"), func(*terraform.State) error {
					if live := approvals.LivePolicies("1"); len(live) != 0 {
						return fmt.Errorf("live associations with policies %v, want none", live)
					}
					return nil
				}),
			},
		},
	})
}

// That action goes back with an association, with the
// ignore_changes it took at the link or from policies still listed: the
// association always sends its POST. The warning rows are what the guide
// warns about, not a way back: listing the policy again plans nothing, since
// the state still holds it and the API keeps listing the archived row after
// the unlink, so no POST goes out and the action is left with no live policy.
func TestFunctionalApprovalMigration_APIMigratedInlineGoesBack(t *testing.T) {
	inline := `  policies = [nullplatform_approval_policy.coverage.id]`
	for name, back := range map[string]struct {
		linked, extra, after string
		live                 []string
	}{
		"association, ignore_changes from the link":             {ignorePolicies, ignorePolicies, specBlock + associationBlock, []string{"1"}},
		"association, policies still listed":                    {inline, ignorePolicies, specBlock + associationBlock, []string{"1"}},
		"the guide's warning: listed again":                     {inline, inline, specBlock, []string{}},
		"the guide's warning: listed in ignore_changes's place": {ignorePolicies, inline, specBlock, []string{}},
	} {
		t.Run(name, func(t *testing.T) {
			shortenPolicyAssociationRetry(t, time.Minute)
			fake, approvals, client := newLinkFake(t)

			resource.UnitTest(t, resource.TestCase{
				ProviderFactories: functionalFactories(fake),
				Steps: []resource.TestStep{
					{Config: approvalActionConfig(approvalAttributes, inline, "")},
					{
						PreConfig: migrateByAPI(t, client, approvals),
						Config:    approvalActionConfig(approvalAttributes, back.linked, specAndLinkBlocks+migratedImports),
						Check:     migrated(client),
					},
					{
						Config: approvalActionConfig(approvalAttributes, back.extra, back.after),
						ConfigPlanChecks: resource.ConfigPlanChecks{PostApplyPostRefresh: []plancheck.PlanCheck{
							plancheck.ExpectEmptyPlan(),
						}},
						Check: func(*terraform.State) error {
							action, err := client.GetApprovalAction("1")
							if err != nil || action.ChecklistSpecificationId != "" || len(action.Policies) != 1 {
								return fmt.Errorf("action %+v, %v; want unlinked, listing its policy", action, err)
							}
							if unlinks := writeCount(fake, http.MethodDelete, linkPath); unlinks != 1 {
								return fmt.Errorf("sent %d unlinks, want 1", unlinks)
							}
							if live := approvals.LivePolicies("1"); !reflect.DeepEqual(live, back.live) {
								return fmt.Errorf("live associations with policies %v, want %v", live, back.live)
							}
							if posts := writeCount(fake, http.MethodPost, APPROVAL_ACTION_PATH+"/1/policy"); len(back.live) == 0 && posts != 1 {
								return fmt.Errorf("sent %d associations, want only the create's", posts)
							}
							return nil
						},
					},
				},
			})
		})
	}
}

// Back to policies in one apply that removes the link and adds an
// association. The fake lands the unlink only after the association was
// refused once, so the association waits for the unlink under Terraform
// whatever the order. With inline policies there is no race: the
// link goes before the action is updated.
func TestFunctionalApprovalMigration_BackToPolicies(t *testing.T) {
	for name, back := range map[string]struct {
		config            string
		unlinkAfter, sent int
	}{
		"association":     {approvalActionConfig(approvalAttributes, ignorePolicies, specBlock+associationBlock), 1, 2},
		"inline policies": {approvalActionConfig(approvalAttributes, `  policies = [nullplatform_approval_policy.coverage.id]`, specBlock), 0, 1},
	} {
		// Terraform's unlink goes out once either way: under UnlinkAfter the fake
		// only lands it later.
		t.Run(name, func(t *testing.T) {
			shortenPolicyAssociationRetry(t, time.Minute)
			fake, approvals, client := newLinkFake(t)

			resource.UnitTest(t, resource.TestCase{
				ProviderFactories: functionalFactories(fake),
				Steps: []resource.TestStep{
					{Config: approvalActionConfig(approvalAttributes, ignorePolicies, specAndLinkBlocks)},
					{
						PreConfig: func() { approvals.UnlinkAfter("1", back.unlinkAfter) },
						Config:    back.config,
						Check: func(*terraform.State) error {
							action, err := client.GetApprovalAction("1")
							if err != nil || action.ChecklistSpecificationId != "" || len(action.Policies) != 1 {
								return fmt.Errorf("action %+v, %v; want unlinked with its policy", action, err)
							}
							if sent := writeCount(fake, http.MethodPost, APPROVAL_ACTION_PATH+"/1/policy"); sent != back.sent {
								return fmt.Errorf("sent %d associations, want %d", sent, back.sent)
							}
							if unlinks := writeCount(fake, http.MethodDelete, linkPath); unlinks != 1 {
								return fmt.Errorf("sent %d unlinks, want 1", unlinks)
							}
							return nil
						},
					},
				},
			})
		})
	}
}

// writeCount counts the fake's requests of a method on a path.
func writeCount(fake *fakeplatform.Server, method, path string) int {
	count := 0
	for _, r := range fake.Requests() {
		if r.Method == method && r.Path == path {
			count++
		}
	}
	return count
}
