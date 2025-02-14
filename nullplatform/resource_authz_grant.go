package nullplatform

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceAuthzGrant() *schema.Resource {
	return &schema.Resource{
		Description: "The authz_grant resource allows you to manage authorization grants in nullplatform",

		DeprecationMessage: "This resource is deprecated and will be removed in a future version. Please use the `nullplatform_user_role` resource instead.",

		CreateContext: CreateAuthzGrant,
		ReadContext:   ReadAuthzGrant,
		DeleteContext: DeleteAuthzGrant,

		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				d.Set("id", d.Id())
				return []*schema.ResourceData{d}, nil
			},
		},

		Schema: AddNRNSchema(map[string]*schema.Schema{
			"user_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "The ID of the user to grant permissions to.",
			},
			"role_slug": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The slug of the role to grant.",
			},
		}),
	}
}

func CreateAuthzGrant(_ context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)

	var nrn string
	var err error
	if v, ok := d.GetOk("nrn"); ok {
		nrn = v.(string)
	} else {
		nrn, err = ConstructNRNFromComponents(d, nullOps)
		if err != nil {
			return diag.FromErr(fmt.Errorf("error constructing NRN: %v %s", err, nrn))
		}
	}

	grant := &AuthzGrant{
		UserID:   d.Get("user_id").(int),
		RoleSlug: d.Get("role_slug").(string),
		NRN:      nrn,
	}

	newGrant, err := nullOps.CreateAuthzGrant(grant)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(strconv.Itoa(newGrant.ID))
	return ReadAuthzGrant(context.Background(), d, m)
}

func ReadAuthzGrant(_ context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)

	grant, err := nullOps.GetAuthzGrant(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("user_id", grant.UserID); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("role_slug", grant.RoleSlug); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("nrn", grant.NRN); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func DeleteAuthzGrant(_ context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)

	err := nullOps.DeleteAuthzGrant(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
