package nullplatform

import (
	"context"
	"reflect"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceApprovalAction() *schema.Resource {
	return &schema.Resource{
		Description: "The approval action resource allows you to configure a nullplatform action for the approval workflow",

		Create: ApprovalActionCreate,
		Read:   ApprovalActionRead,
		Update: ApprovalActionUpdate,
		Delete: ApprovalActionDelete,

		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				d.Set("id", d.Id())
				return []*schema.ResourceData{d}, nil
			},
		},

		Schema: map[string]*schema.Schema{
			"nrn": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The NRN of the resource (including children resources) where the action will apply.",
			},
			"entity": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The entity to which this action applies. Example: `deployment`.",
			},
			"action": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The action to which this action applies. Example: `deployment:create`",
			},
			"dimensions": {
				Type:     schema.TypeMap,
				ForceNew: true,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "A key-value map with the runtime configuration dimensions that apply to this scope.",
			},
			"on_policy_success": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The action to be taken on policy success. Possible values: [`approve`, `manual`]",
			},
			"on_policy_fail": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The action to be taken on policy failure. Possible values: [`manual`, `deny`]",
			},
			"policies": {
				Type:        schema.TypeSet,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "A list of Policy IDs to associate with the action.",
			},
		},
	}
}

func ApprovalActionCreate(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)

	nrn := d.Get("nrn").(string)
	entity := d.Get("entity").(string)
	action := d.Get("action").(string)
	onPolicySuccess := d.Get("on_policy_success").(string)
	onPolicyFail := d.Get("on_policy_fail").(string)
	policies := d.Get("policies").(*schema.Set)

	dimensionsMap := d.Get("dimensions").(map[string]any)
	// Convert the dimensions to a map[string]string
	dimensions := make(map[string]string)
	for key, value := range dimensionsMap {
		dimensions[key] = value.(string)
	}

	newApprovalAction := &ApprovalAction{
		Nrn:             nrn,
		Entity:          entity,
		Action:          action,
		Dimensions:      dimensions,
		OnPolicySuccess: onPolicySuccess,
		OnPolicyFail:    onPolicyFail,
	}

	approvalAction, err := nullOps.CreateApprovalAction(newApprovalAction)
	if err != nil {
		return err
	}

	approvalActionId := strconv.Itoa(approvalAction.Id)
	d.SetId(approvalActionId)

	for _, policyId := range policies.List() {
		err := nullOps.AssociatePolicyWithAction(approvalActionId, policyId.(string))
		if err != nil {
			return err
		}
	}

	return ApprovalActionRead(d, m)
}

func ApprovalActionRead(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)
	approvalActionId := d.Id()

	approvalAction, err := nullOps.GetApprovalAction(approvalActionId)
	if err != nil {
		if approvalAction.Status == "deleted" {
			d.SetId("")
			return nil
		}
		return err
	}

	if err := d.Set("nrn", approvalAction.Nrn); err != nil {
		return err
	}

	if err := d.Set("entity", approvalAction.Entity); err != nil {
		return err
	}

	if err := d.Set("action", approvalAction.Action); err != nil {
		return err
	}

	if err := d.Set("dimensions", approvalAction.Dimensions); err != nil {
		return err
	}

	if err := d.Set("on_policy_success", approvalAction.OnPolicySuccess); err != nil {
		return err
	}

	if err := d.Set("on_policy_fail", approvalAction.OnPolicyFail); err != nil {
		return err
	}

	// Convert ApprovalPolicy.Id to a set of strings
	policyIds := make([]string, len(approvalAction.Policies))
	for i, policy := range approvalAction.Policies {
		if policy != nil {
			policyIds[i] = strconv.Itoa(policy.Id)
		}
	}

	if err := d.Set("policies", policyIds); err != nil {
		return err
	}

	return nil
}

func ApprovalActionUpdate(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)
	approvalActionId := d.Id()

	approvalAction := &ApprovalAction{}

	if d.HasChange("nrn") {
		approvalAction.Nrn = d.Get("nrn").(string)
	}

	if d.HasChange("entity") {
		approvalAction.Entity = d.Get("entity").(string)
	}

	if d.HasChange("action") {
		approvalAction.Entity = d.Get("action").(string)
	}

	if d.HasChange("dimensions") {
		dimensionsMap := d.Get("dimensions").(map[string]interface{})

		// Convert the dimensions to a map[string]string
		dimensions := make(map[string]string)
		for key, value := range dimensionsMap {
			dimensions[key] = value.(string)
		}

		approvalAction.Dimensions = dimensions
	}

	if d.HasChange("on_policy_success") {
		approvalAction.OnPolicySuccess = d.Get("on_policy_success").(string)
	}

	if d.HasChange("on_policy_fail") {
		approvalAction.OnPolicyFail = d.Get("on_policy_fail").(string)
	}

	if !reflect.DeepEqual(*approvalAction, Scope{}) {
		err := nullOps.PatchApprovalAction(approvalActionId, approvalAction)
		if err != nil {
			return err
		}
	}

	if d.HasChange("policies") {
		var oldSet, newSet *schema.Set

		oldPolicies, newPolicies := d.GetChange("policies")

		if oldPolicies != nil {
			oldSet = oldPolicies.(*schema.Set)
		} else {
			oldSet = schema.NewSet(schema.HashString, nil)
		}

		if newPolicies != nil {
			newSet = newPolicies.(*schema.Set)
		} else {
			newSet = schema.NewSet(schema.HashString, nil)
		}

		// Remove policies
		for _, policyId := range oldSet.Difference(newSet).List() {
			err := nullOps.DisassociatePolicyFromAction(approvalActionId, policyId.(string))
			if err != nil {
				return err
			}
		}

		// Add new policies
		for _, policyId := range newSet.Difference(oldSet).List() {
			err := nullOps.AssociatePolicyWithAction(approvalActionId, policyId.(string))
			if err != nil {
				return err
			}
		}
	}

	return ApprovalActionRead(d, m)
}

func ApprovalActionDelete(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)
	approvalActionId := d.Id()

	err := nullOps.DeleteApprovalAction(approvalActionId)
	if err != nil {
		return err
	}

	d.SetId("")

	return nil
}
