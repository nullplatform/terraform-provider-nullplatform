package nullplatform

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// approvalActionHasChecklist is the API's refusal (409) of a policy
// association while the action is linked to a checklist specification: the
// only refusal the association waits out, for the unlink of the same apply.
const approvalActionHasChecklist = "Approval action already has a checklist specification; cannot also associate policies"

// policyAssociationRetryTimeout is how long an association waits for its
// action to be unlinked, and policyAssociationRetryInterval how often it
// tries.
var (
	policyAssociationRetryTimeout  = time.Minute
	policyAssociationRetryInterval = 2 * time.Second
)

func resourceApprovalActionPolicyAssociation() *schema.Resource {
	return &schema.Resource{
		Description: "The approval_action_policy_association resource allows you to manage a 1:1 association between an approval action and a policy.\n\n" +
			"~> **Note:** An approval action linked to a checklist specification takes no policies. Creating an association " +
			"waits up to a minute for the action's link to be removed, as when going back to policies in one apply. While the " +
			"action is linked, refreshing the association warns that it has no effect.",

		CreateContext: CreateApprovalActionPolicyAssociation,
		ReadContext:   ReadApprovalActionPolicyAssociation,
		DeleteContext: DeleteApprovalActionPolicyAssociation,

		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				d.Set("id", d.Id())
				return []*schema.ResourceData{d}, nil
			},
		},

		Schema: map[string]*schema.Schema{
			"approval_action_id": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The ID of the approval action to associate with the policy",
			},
			"approval_policy_id": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The ID of the approval policy to associate with the action",
			},
		},
	}
}

func CreateApprovalActionPolicyAssociation(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)

	approvalActionId := d.Get("approval_action_id").(string)
	approvalPolicyId := d.Get("approval_policy_id").(string)

	// Going back to policies, the unlink of the same apply may not have landed
	// yet: only that refusal is waited out, told by its message.
	deadline := time.Now().Add(policyAssociationRetryTimeout)
	err := nullOps.AssociatePolicyWithAction(approvalActionId, approvalPolicyId)
	for isRefusedForChecklist(err) && time.Now().Before(deadline) {
		select {
		case <-time.After(policyAssociationRetryInterval):
		case <-ctx.Done():
			return diag.FromErr(err)
		}
		err = nullOps.AssociatePolicyWithAction(approvalActionId, approvalPolicyId)
	}
	if err != nil {
		return diag.FromErr(err)
	}

	// Set a unique ID for the resource
	d.SetId(fmt.Sprintf("%s-%s", approvalActionId, approvalPolicyId))

	return ReadApprovalActionPolicyAssociation(ctx, d, m)
}

// isRefusedForChecklist tells the refusal of an action linked to a checklist
// specification from any other error.
func isRefusedForChecklist(err error) bool {
	var refused *PolicyAssociationError
	return errors.As(err, &refused) && refused.StatusCode == http.StatusConflict && refused.Message == approvalActionHasChecklist
}

func ReadApprovalActionPolicyAssociation(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)

	approvalActionId := d.Get("approval_action_id").(string)
	approvalPolicyId := d.Get("approval_policy_id").(string)

	// Get the action to verify the association still exists
	action, err := nullOps.GetApprovalAction(approvalActionId)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error getting approval action: %v", err))
	}

	// Check if the policy is still associated
	found := false
	for _, policy := range action.Policies {
		if policy != nil && fmt.Sprintf("%d", policy.Id) == approvalPolicyId {
			found = true
			break
		}
	}

	if !found {
		d.SetId("")
		return nil
	}

	// A migrated action still lists the policies it archived: the association
	// stays, and warns that it decides nothing. The state is not touched.
	if action.ChecklistSpecificationId != "" {
		return diag.Diagnostics{{
			Severity: diag.Warning,
			Summary:  "Policy association has no effect: the approval action uses a checklist specification",
			Detail: fmt.Sprintf("Approval action %s is linked to checklist specification %s, so its policies no longer take part in "+
				"its approvals and changes to this association have no effect. Declare the specification and its link in Terraform "+
				"and remove this association (see the guide \"Migrate approval policies to checklist specifications\").",
				approvalActionId, action.ChecklistSpecificationId),
		}}
	}

	return nil
}

func DeleteApprovalActionPolicyAssociation(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)

	approvalActionId := d.Get("approval_action_id").(string)
	approvalPolicyId := d.Get("approval_policy_id").(string)

	err := nullOps.DisassociatePolicyFromAction(approvalActionId, approvalPolicyId)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error disassociating policy from action: %v", err))
	}

	d.SetId("")
	return nil
}
