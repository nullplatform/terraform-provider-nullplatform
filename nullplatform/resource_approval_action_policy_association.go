package nullplatform

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceApprovalActionPolicyAssociation() *schema.Resource {
	return &schema.Resource{
		Description: "The approval_action_policy_association resource allows you to manage a 1:1 association between an approval action and a policy",

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

	err := nullOps.AssociatePolicyWithAction(approvalActionId, approvalPolicyId)
	if err != nil {
		return diag.FromErr(err)
	}

	// Set a unique ID for the resource
	d.SetId(fmt.Sprintf("%s-%s", approvalActionId, approvalPolicyId))

	return ReadApprovalActionPolicyAssociation(ctx, d, m)
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
