package nullplatform

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceUserRole() *schema.Resource {
	return &schema.Resource{
		Description: "The user_role resource allows you to manage authorization grants in nullplatform",

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
