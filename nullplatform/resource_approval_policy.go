package nullplatform

import (
	"context"
	"reflect"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceApprovalPolicy() *schema.Resource {
	return &schema.Resource{
		Description: "The approval policy resource allows you to configure a nullplatform policy for the approval workflow",

		Create: ApprovalPolicyCreate,
		Read:   ApprovalPolicyRead,
		Update: ApprovalPolicyUpdate,
		Delete: ApprovalPolicyDelete,

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
				Description: "The NRN of the resource (including children resources) where the policy will apply.",
			},
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The name of the policy.",
			},
			"conditions": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The conditions that the policy applies to, as a JSON object.",
			},
		},
	}
}

func ApprovalPolicyCreate(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)

	nrn := d.Get("nrn").(string)
	name := d.Get("name").(string)
	conditions := d.Get("conditions").(string)

	newApprovalPolicy := &ApprovalPolicy{
		Nrn:        nrn,
		Name:       name,
		Conditions: conditions,
	}

	approvalPolicy, err := nullOps.CreateApprovalPolicy(newApprovalPolicy)

	if err != nil {
		return err
	}

	d.SetId(strconv.Itoa(approvalPolicy.Id))

	return ApprovalPolicyRead(d, m)
}

func ApprovalPolicyRead(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)
	approvalPolicyId := d.Id()

	approvalPolicy, err := nullOps.GetApprovalPolicy(approvalPolicyId)
	if err != nil {
		if approvalPolicy.Status == "deleted" {
			d.SetId("")
			return nil
		}
		return err
	}
	if err := d.Set("nrn", approvalPolicy.Nrn); err != nil {
		return err
	}

	if err := d.Set("name", approvalPolicy.Name); err != nil {
		return err
	}

	if err := d.Set("conditions", approvalPolicy.Conditions); err != nil {
		return err
	}

	return nil
}

func ApprovalPolicyUpdate(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)
	approvalPolicyId := d.Id()

	approvalPolicy := &ApprovalPolicy{}

	if d.HasChange("nrn") {
		approvalPolicy.Nrn = d.Get("nrn").(string)
	}

	if d.HasChange("name") {
		approvalPolicy.Name = d.Get("name").(string)
	}

	if d.HasChange("conditions") {
		approvalPolicy.Conditions = d.Get("conditions").(string)
	}

	if !reflect.DeepEqual(*approvalPolicy, Scope{}) {
		err := nullOps.PatchApprovalPolicy(approvalPolicyId, approvalPolicy)
		if err != nil {
			return err
		}
	}

	return ApprovalPolicyRead(d, m)
}

func ApprovalPolicyDelete(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)
	approvalPolicyId := d.Id()
	approvalPolicyNrn := d.Get("nrn").(string)

	err := nullOps.DeleteApprovalPolicy(approvalPolicyNrn, approvalPolicyId)
	if err != nil {
		return err
	}

	d.SetId("")

	return nil
}
