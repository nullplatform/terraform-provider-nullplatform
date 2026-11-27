package nullplatform

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceApplicationType() *schema.Resource {
	return &schema.Resource{
		Description: "Provides information about an existing nullplatform Application Type",
		ReadContext: dataSourceApplicationTypeRead,
		Schema: map[string]*schema.Schema{
			"id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The ID of the application type.",
			},
			"nrn": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The NRN that owns the application type. A caller outside the organization that owns the type reads its own organization here.",
			},
			"name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The name of the application type.",
			},
			"description": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "What the application type is for; empty when it has no description.",
			},
			"status": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "`active` or `inactive`. Only an active type can be given to a new application.",
			},
			"visible_to": {
				Type:        schema.TypeSet,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "The NRNs where the application type can be used. A caller outside the organization that owns the type reads only its own.",
			},
			"schema": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The JSON Schema the `attributes` of an application of this type must pass, as a JSON string with its properties in the order of the form.",
			},
			"slug": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The slug of the application type.",
			},
			"last_snapshot_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The ID of the snapshot that holds the current `schema`. New applications are created on it.",
			},
		},
	}
}

func dataSourceApplicationTypeRead(_ context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)

	applicationType, err := nullOps.GetApplicationType(d.Get("id").(string))
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(applicationType.Id)
	return diag.FromErr(setApplicationTypeState(d, applicationType))
}
