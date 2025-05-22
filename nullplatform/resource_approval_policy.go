package nullplatform

import (
	"context"
	"encoding/json"
	"fmt"
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

		Schema: AddNRNSchema(map[string]*schema.Schema{
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
			"selector": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "{}",
				Description: "The selector criteria that determines which policies apply, as a JSON object. Defaults to an empty object.",
			},
		}),
	}
}

func ApprovalPolicyCreate(d *schema.ResourceData, m any) error {
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
	name := d.Get("name").(string)
	conditionsJSON := d.Get("conditions").(string)
	selectorJSON := d.Get("selector").(string)

	var conditions interface{}
	if err := json.Unmarshal([]byte(conditionsJSON), &conditions); err != nil {
		return fmt.Errorf("error parsing conditions JSON: %v", err)
	}

	var selector interface{}
	if err := json.Unmarshal([]byte(selectorJSON), &selector); err != nil {
		return fmt.Errorf("error parsing selector JSON: %v", err)
	}

	newApprovalPolicy := &ApprovalPolicy{
		Nrn:        nrn,
		Name:       name,
		Conditions: conditions,
		Selector:   selector,
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

	conditionsJSON, err := json.Marshal(approvalPolicy.Conditions)
	if err != nil {
		return fmt.Errorf("error serializing conditions to JSON: %v", err)
	}

	if err := d.Set("conditions", string(conditionsJSON)); err != nil {
		return err
	}

	var selectorJSON []byte
	if approvalPolicy.Selector == nil {
		selectorJSON = []byte("{}")
	} else {
		selectorJSON, err = json.Marshal(approvalPolicy.Selector)
		if err != nil {
			return fmt.Errorf("error serializing selector to JSON: %v", err)
		}
	}

	if err := d.Set("selector", string(selectorJSON)); err != nil {
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
		conditionsJSON := d.Get("conditions").(string)
		var conditions interface{}
		if err := json.Unmarshal([]byte(conditionsJSON), &conditions); err != nil {
			return fmt.Errorf("error parsing conditions JSON: %v", err)
		}
		approvalPolicy.Conditions = conditions
	}

	if d.HasChange("selector") {
		selectorJSON := d.Get("selector").(string)
		var selector interface{}
		if err := json.Unmarshal([]byte(selectorJSON), &selector); err != nil {
			return fmt.Errorf("error parsing selector JSON: %v", err)
		}
		approvalPolicy.Selector = selector
	}

	if !reflect.DeepEqual(*approvalPolicy, ApprovalPolicy{}) {
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

	err := nullOps.DeleteApprovalPolicy(approvalPolicyId)
	if err != nil {
		return err
	}

	d.SetId("")

	return nil
}
