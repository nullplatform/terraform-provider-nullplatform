package nullplatform

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceAuthzGrant() *schema.Resource {
	return &schema.Resource{
		Description: "The authz_grant resource allows you to manage authorization grants in nullplatform",

		Create: AuthzGrantCreate,
		Read:   AuthzGrantRead,
		Delete: AuthzGrantDelete,

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

func AuthzGrantCreate(d *schema.ResourceData, m interface{}) error {
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

	grant := &AuthzGrant{
		UserID:   d.Get("user_id").(int),
		RoleSlug: d.Get("role_slug").(string),
		NRN:      nrn,
	}

	newGrant, err := nullOps.CreateAuthzGrant(grant)
	if err != nil {
		return err
	}

	d.SetId(strconv.Itoa(newGrant.ID))
	return AuthzGrantRead(d, m)
}

func AuthzGrantRead(d *schema.ResourceData, m interface{}) error {
	nullOps := m.(NullOps)

	grant, err := nullOps.GetAuthzGrant(d.Id())
	if err != nil {
		return err
	}

	d.Set("user_id", grant.UserID)
	d.Set("role_slug", grant.RoleSlug)
	d.Set("nrn", grant.NRN)

	return nil
}

func AuthzGrantDelete(d *schema.ResourceData, m interface{}) error {
	nullOps := m.(NullOps)

	err := nullOps.DeleteAuthzGrant(d.Id())
	if err != nil {
		return err
	}

	d.SetId("")
	return nil
}
