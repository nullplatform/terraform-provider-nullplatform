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

	ApprovalPolicy, err := nullOps.CreateApprovalPolicy(newApprovalPolicy)

	if err != nil {
		return err
	}

	d.SetId(strconv.Itoa(ApprovalPolicy.Id))

	return ApprovalPolicyRead(d, m)
}

func ApprovalPolicyRead(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)
	ApprovalPolicyId := d.Id()

	ApprovalPolicy, err := nullOps.GetApprovalPolicy(ApprovalPolicyId)
	if err != nil {
		return err
	}

	if err := d.Set("nrn", ApprovalPolicy.Nrn); err != nil {
		return err
	}

	if err := d.Set("name", ApprovalPolicy.Name); err != nil {
		return err
	}

	if err := d.Set("conditions", ApprovalPolicy.Conditions); err != nil {
		return err
	}

	return nil
}

func ApprovalPolicyUpdate(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)
	ApprovalPolicyId := d.Id()

	ApprovalPolicy := &ApprovalPolicy{}

	if d.HasChange("nrn") {
		ApprovalPolicy.Nrn = d.Get("nrn").(string)
	}

	if d.HasChange("name") {
		ApprovalPolicy.Name = d.Get("name").(string)
	}

	if d.HasChange("conditions") {
		ApprovalPolicy.Conditions = d.Get("conditions").(string)
	}

	if !reflect.DeepEqual(*ApprovalPolicy, Scope{}) {
		err := nullOps.PatchApprovalPolicy(ApprovalPolicyId, ApprovalPolicy)
		if err != nil {
			return err
		}
	}

	return ApprovalPolicyRead(d, m)
}

func ApprovalPolicyDelete(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)
	ApprovalPolicyId := d.Id()

	err := nullOps.DeleteApprovalPolicy(ApprovalPolicyId)
	if err != nil {
		return err
	}

	d.SetId("")

	return nil
}
