package nullplatform

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceScopeType() *schema.Resource {
	return &schema.Resource{
		Description: "Provides information about an existing nullplatform Scope Type",
		ReadContext: dataSourceScopeTypeRead,
		Schema: map[string]*schema.Schema{
			"id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The ID of the scope type.",
			},
			"nrn": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The NRN of the scope type.",
			},
			"type": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The type of scope type.",
			},
			"name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The display name of the scope type.",
			},
			"status": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Whether this scope type is enabled to be used.",
			},
			"description": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Short description of how the scope type works or what it does.",
			},
			"provider_type": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Defines the source entity that implements the scope type.",
			},
			"provider_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The ID of the entity that implements the scope type.",
			},
		},
	}
}

func dataSourceScopeTypeRead(_ context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)

	st, err := nullOps.GetScopeType(d.Get("id").(string))
	if err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("nrn", st.Nrn); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("type", st.Type); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("name", st.Name); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("status", st.Status); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("description", st.Description); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("provider_type", st.ProviderType); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("provider_id", st.ProviderId); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(fmt.Sprintf("%d", st.Id))

	return nil
}
