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

	d.SetId(strconv.Itoa(approvalAction.Id))

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
