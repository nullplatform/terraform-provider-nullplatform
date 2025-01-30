package nullplatform

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceUser() *schema.Resource {
	return &schema.Resource{
		Description: "The user resource allows you to manage users in nullplatform",

		CreateContext: CreateUser,
		ReadContext:   ReadUser,
		UpdateContext: UpdateUser,
		DeleteContext: DeleteUser,

		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				d.Set("id", d.Id())
				return []*schema.ResourceData{d}, nil
			},
		},

		Schema: map[string]*schema.Schema{
			"email": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The email address of the user.",
			},
			"first_name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The first name of the user.",
			},
			"last_name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The last name of the user.",
			},
			"avatar": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The URL of the user's avatar.",
			},
			"organization_id": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "The ID of the organization the user belongs to.",
			},
		},
	}
}

func CreateUser(_ context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)

	orgIDStr, err := nullOps.GetOrganizationIDFromToken()
	if err != nil {
		return diag.FromErr(fmt.Errorf("error getting organization ID: %v", err))
	}

	orgID, err := strconv.Atoi(orgIDStr)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error converting organization ID to integer: %v", err))
	}

	newUser := &User{
		Email:          d.Get("email").(string),
		FirstName:      d.Get("first_name").(string),
		LastName:       d.Get("last_name").(string),
		Avatar:         d.Get("avatar").(string),
		OrganizationID: orgID,
	}

	user, err := nullOps.CreateUser(newUser)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(strconv.Itoa(user.ID))
	return ReadUser(context.Background(), d, m)
}

func ReadUser(_ context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)

	user, err := nullOps.GetUser(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("email", user.Email); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("first_name", user.FirstName); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("last_name", user.LastName); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("avatar", user.Avatar); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("organization_id", user.OrganizationID); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func UpdateUser(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)

	updateUser := &User{
		FirstName: d.Get("first_name").(string),
		LastName:  d.Get("last_name").(string),
		Avatar:    d.Get("avatar").(string),
	}

	err := nullOps.UpdateUser(d.Id(), updateUser)
	if err != nil {
		return diag.FromErr(err)
	}

	return ReadUser(ctx, d, m)
}

func DeleteUser(_ context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)

	err := nullOps.DeleteUser(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
