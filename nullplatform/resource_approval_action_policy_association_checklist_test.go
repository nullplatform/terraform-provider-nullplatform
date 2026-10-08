package nullplatform

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// shortenPolicyAssociationRetry makes an association refused for a linked
// action try again at once, and give up after timeout.
func shortenPolicyAssociationRetry(t *testing.T, timeout time.Duration) {
	t.Helper()
	previousTimeout, previousInterval := policyAssociationRetryTimeout, policyAssociationRetryInterval
	policyAssociationRetryTimeout, policyAssociationRetryInterval = timeout, 10*time.Millisecond
	t.Cleanup(func() {
		policyAssociationRetryTimeout, policyAssociationRetryInterval = previousTimeout, previousInterval
	})
}

// linkedActionWithPolicy is action 1 linked to version 1, and policy 1.
func linkedActionWithPolicy(t *testing.T) *NullClient {
	t.Helper()
	_, _, client := newLinkFake(t)
	newAPIAction(t, client)
	if _, err := client.CreateApprovalPolicy(&ApprovalPolicy{Nrn: approvalNrn, Name: "p1", Conditions: map[string]any{}, Selector: map[string]any{}}); err != nil {
		t.Fatal(err)
	}
	if err := client.LinkChecklistSpecification("1", specVersionID(1)); err != nil {
		t.Fatal(err)
	}
	return client
}

func associationData(t *testing.T) *schema.ResourceData {
	return schema.TestResourceDataRaw(t, resourceApprovalActionPolicyAssociation().Schema,
		map[string]any{"approval_action_id": "1", "approval_policy_id": "1"})
}

// countAssociations counts the association requests that reach the client.
type countAssociations struct {
	NullOps
	posts *int
}

func (o countAssociations) AssociatePolicyWithAction(actionID, policyID string) error {
	*o.posts++
	return o.NullOps.AssociatePolicyWithAction(actionID, policyID)
}

// Going back to policies, an association waits for the unlink of the same
// apply (here, after two refusals) and then associates.
func TestPolicyAssociation_RetriesWhileLinked(t *testing.T) {
	shortenPolicyAssociationRetry(t, time.Minute)
	_, approvals, client := newLinkFake(t)
	newAPIAction(t, client)
	if _, err := client.CreateApprovalPolicy(&ApprovalPolicy{Nrn: approvalNrn, Name: "p1", Conditions: map[string]any{}, Selector: map[string]any{}}); err != nil {
		t.Fatal(err)
	}
	if err := client.LinkChecklistSpecification("1", specVersionID(1)); err != nil {
		t.Fatal(err)
	}
	approvals.UnlinkAfter("1", 2)
	posts := 0

	d := associationData(t)
	diags := CreateApprovalActionPolicyAssociation(context.Background(), d, countAssociations{client, &posts})
	if diags.HasError() || d.Id() != "1-1" || posts != 3 {
		t.Errorf("diags %v, id %q after %d associations; want 1-1 after two refusals and the association", diags, d.Id(), posts)
	}
}

// An action still linked when the minute runs out fails with the message the
// provider has always shown.
func TestPolicyAssociation_TimeoutKeepsTheProvidersMessage(t *testing.T) {
	shortenPolicyAssociationRetry(t, 50*time.Millisecond)
	client := linkedActionWithPolicy(t)

	diags := CreateApprovalActionPolicyAssociation(context.Background(), associationData(t), client)
	want := "error associating approval policy with action, got status code: 409, " + approvalActionHasChecklist
	if !diags.HasError() || diags[0].Summary != want {
		t.Errorf("diags %v, want %s", diags, want)
	}
}

// otherConflict refuses every association with a 409 that is not a link.
type otherRefusal struct {
	NullOps
	refusal *PolicyAssociationError
	posts   *int
}

func (o otherRefusal) AssociatePolicyWithAction(string, string) error {
	*o.posts++
	return o.refusal
}

// Any other refusal fails at once, never waited out: another 409, or the
// link's text with another status.
func TestPolicyAssociation_OnlyTheLinkIsWaitedOut(t *testing.T) {
	shortenPolicyAssociationRetry(t, 200*time.Millisecond)
	for _, refusal := range []*PolicyAssociationError{
		{StatusCode: http.StatusConflict, Message: "Some other conflict"},
		{StatusCode: http.StatusBadRequest, Message: approvalActionHasChecklist},
	} {
		client := linkedActionWithPolicy(t)
		posts := 0

		diags := CreateApprovalActionPolicyAssociation(context.Background(), associationData(t), otherRefusal{client, refusal, &posts})
		if !diags.HasError() || posts != 1 {
			t.Errorf("%d %s: diags %v after %d associations, want the refusal after one", refusal.StatusCode, refusal.Message, diags, posts)
		}
	}
}

// A cancelled apply stops the wait at once, with the refusal.
func TestPolicyAssociation_CancelStopsTheWait(t *testing.T) {
	shortenPolicyAssociationRetry(t, time.Minute)
	client := linkedActionWithPolicy(t)
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	diags := CreateApprovalActionPolicyAssociation(cancelled, associationData(t), client)
	if want := "error associating approval policy with action, got status code: 409, " + approvalActionHasChecklist; !diags.HasError() || diags[0].Summary != want || time.Since(start) > 5*time.Second {
		t.Errorf("diags %v after %s, want %s at once", diags, time.Since(start), want)
	}
}

// The refresh of an association whose action is linked warns and keeps
// it in the state; an action without a link reads as before.
func TestPolicyAssociation_WarnsWhenTheActionIsLinked(t *testing.T) {
	_, approvals, client := newLinkFake(t)
	newAPIAction(t, client)
	withLivePolicies(t, client, 1)
	d := associationData(t)
	d.SetId("1-1")

	if diags := ReadApprovalActionPolicyAssociation(context.Background(), d, client); len(diags) != 0 || d.Id() != "1-1" {
		t.Fatalf("unlinked: diags %v, id %q; want none and the association kept", diags, d.Id())
	}

	approvals.Migrate("1", specVersionID(1))
	diags := ReadApprovalActionPolicyAssociation(context.Background(), d, client)
	want := diag.Diagnostic{
		Severity: diag.Warning,
		Summary:  "Policy association has no effect: the approval action uses a checklist specification",
		Detail: "Approval action 1 is linked to checklist specification " + specVersionID(1) + ", so its policies no longer take " +
			"part in its approvals and changes to this association have no effect. Declare the specification and its link in " +
			"Terraform and remove this association (see the guide \"Migrate approval policies to checklist specifications\").",
	}
	if len(diags) != 1 || diags[0].Severity != want.Severity || diags[0].Summary != want.Summary || diags[0].Detail != want.Detail || d.Id() != "1-1" {
		t.Errorf("linked: diags %+v, id %q; want the warning and the association kept", diags, d.Id())
	}
}
