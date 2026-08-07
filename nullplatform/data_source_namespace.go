package nullplatform

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceNamespace() *schema.Resource {
	return &schema.Resource{
		Description: "Provides information about a Namespace",

		ReadContext: dataSourceNamespaceRead,
		Schema: map[string]*schema.Schema{
			"id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "A system-wide unique ID for the Namespace",
			},
			"name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The name of the namespace",
			},
			"account_id": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "The ID of the account that owns this namespace",
			},
			"slug": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The unique slug identifier for the namespace",
			},
			"status": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The status of the namespace",
			},
			"nrn": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The Nullplatform Resource Name (NRN) for the namespace",
			},
		},
	}
}

func dataSourceNamespaceRead(_ context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)

	namespace, err := nullOps.GetNamespace(strconv.Itoa(d.Get("id").(int)))
	if err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("name", namespace.Name); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("account_id", namespace.AccountId); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("slug", namespace.Slug); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("status", namespace.Status); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("nrn", namespace.Nrn); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(strconv.Itoa(namespace.Id))

	return nil
}
