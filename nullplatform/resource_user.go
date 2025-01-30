// resource_user.go
package nullplatform

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceUser() *schema.Resource {
	return &schema.Resource{
		Description: "The user resource allows you to manage users in nullplatform",

		Create: UserCreate,
		Read:   UserRead,
		Update: UserUpdate,
		Delete: UserDelete,

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

func UserCreate(d *schema.ResourceData, m interface{}) error {
	nullOps := m.(NullOps)
	client := nullOps.(*NullClient)

	orgIDStr, err := client.GetOrganizationIDFromToken()

	if err != nil {
		return fmt.Errorf("error getting organization ID: %v", err)
	}

	orgID, err := strconv.Atoi(orgIDStr)

	if err != nil {
		return fmt.Errorf("error converting organization ID to integer: %v", err)
	}

	newUser := &User{
		Email:          d.Get("email").(string),
		FirstName:      d.Get("first_name").(string),
		LastName:       d.Get("last_name").(string),
		Avatar:         d.Get("avatar").(string),
		OrganizationID: orgID,
	}

	user, err := client.CreateUser(newUser)
	if err != nil {
		return err
	}

	d.SetId(user.ID)
	return UserRead(d, m)
}

func UserRead(d *schema.ResourceData, m interface{}) error {
	nullOps := m.(NullOps)
	client := nullOps.(*NullClient)

	user, err := client.GetUser(d.Id())
	if err != nil {
		return err
	}

	d.Set("email", user.Email)
	d.Set("first_name", user.FirstName)
	d.Set("last_name", user.LastName)
	d.Set("avatar", user.Avatar)
	d.Set("organization_id", user.OrganizationID)

	return nil
}

func UserUpdate(d *schema.ResourceData, m interface{}) error {
	nullOps := m.(NullOps)
	client := nullOps.(*NullClient)

	updateUser := &User{
		FirstName: d.Get("first_name").(string),
		LastName:  d.Get("last_name").(string),
		Avatar:    d.Get("avatar").(string),
	}

	err := client.UpdateUser(d.Id(), updateUser)
	if err != nil {
		return err
	}

	return UserRead(d, m)
}

func UserDelete(d *schema.ResourceData, m interface{}) error {
	nullOps := m.(NullOps)
	client := nullOps.(*NullClient)

	err := client.DeleteUser(d.Id())
	if err != nil {
		return err
	}

	d.SetId("")
	return nil
}
