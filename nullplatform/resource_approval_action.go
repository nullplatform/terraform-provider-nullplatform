package nullplatform

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
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

		Schema: AddNRNSchema(map[string]*schema.Schema{
			"entity": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The entity to which this action applies. Example: `deployment`. Changing it replaces the action.",
			},
			"action": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The action to which this action applies. Example: `deployment:create`. Changing it replaces the action.",
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
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				Description: "The action to be taken on policy success. Possible values: [`approve`, `manual`]. When omitted, the API sets `manual`; " +
					"removing it later keeps the current value: set `manual` to go back to the default.",
			},
			"on_policy_fail": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				Description: "The action to be taken on policy failure. Possible values: [`manual`, `deny`]. When omitted, the API sets `manual`; " +
					"removing it later keeps the current value: set `manual` to go back to the default.",
			},
			"on_checklist_fail": {
				Type:         schema.TypeString,
				Optional:     true,
				Computed:     true,
				ValidateFunc: validation.StringInSlice([]string{"deny", "pending", "manual"}, false),
				Description: "What happens to a request when the checklist of the linked specification fails. Possible values: " +
					"[`deny`, `pending`, `manual`]. When omitted, linking a specification sets it: `deny` if `on_policy_fail` is " +
					"`deny`, `pending` otherwise. Unlinking leaves it as it is, and so does removing it later: set the value wanted instead.",
			},
			"checklist_specification_id": {
				Type:     schema.TypeString,
				Computed: true,
				Description: "The checklist specification version (`spec_…`) the action is linked to, if any, managed by " +
					"`nullplatform_approval_action_checklist_specification_association`. While the action is linked, `policies` " +
					"is not read back.",
			},
			"policies": {
				Type:        schema.TypeSet,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "A list of Policy IDs to associate with the action.",
			},
		}),
	}
}

func ApprovalActionCreate(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)

	var nrn string
	var err error
	if v, ok := d.GetOk("nrn"); ok {
		nrn = v.(string)
	} else {
		nrn, err = ConstructNRNFromComponents(d, nullOps)
		if err != nil {
			return fmt.Errorf("error constructing NRN: %v %s", err, nrn)
		}
	}
	entity := d.Get("entity").(string)
	action := d.Get("action").(string)
	// What the configuration omits goes out omitted: the API sets its defaults.
	// On a create there is no state, so d.Get would give "" too.
	onPolicySuccess := configuredString(d, "on_policy_success")
	onPolicyFail := configuredString(d, "on_policy_fail")
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
		OnChecklistFail: configuredString(d, "on_checklist_fail"),
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
		// Gone: deleted (the API keeps it, with status "deleted") or answered 404.
		// Any other error, a 401 included, keeps the action in the state.
		// GetApprovalAction returns a non-nil action with an error only when its
		// status is "deleted", so the status check restates the nil check.
		if errors.Is(err, errApprovalActionNotFound) || (approvalAction != nil && approvalAction.Status == "deleted") {
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

	// With a specification linked, the API keeps listing the policies the
	// action no longer uses: policies is not read.
	if err := errors.Join(d.Set("on_checklist_fail", approvalAction.OnChecklistFail),
		d.Set("checklist_specification_id", approvalAction.ChecklistSpecificationId)); err != nil || approvalAction.ChecklistSpecificationId != "" {
		return err
	}

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

	// nrn, entity, action and dimensions force a replacement: the PATCH maps
	// only the on_* callbacks, and carries only the ones that changed.
	approvalAction := &ApprovalAction{}

	if d.HasChange("on_policy_success") {
		approvalAction.OnPolicySuccess = d.Get("on_policy_success").(string)
	}

	if d.HasChange("on_policy_fail") {
		approvalAction.OnPolicyFail = d.Get("on_policy_fail").(string)
	}

	if d.HasChange("on_checklist_fail") {
		approvalAction.OnChecklistFail = d.Get("on_checklist_fail").(string)
	}

	if !reflect.DeepEqual(*approvalAction, ApprovalAction{}) {
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
