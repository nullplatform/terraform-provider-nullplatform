package nullplatform

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceDimensions() *schema.Resource {
	return &schema.Resource{
		Description: "Lists the dimensions visible at an NRN: those defined at it and at its ancestors. An application's NRN also lists the dimensions defined at its account.",

		ReadContext: dataSourceDimensionsRead,
		Schema: map[string]*schema.Schema{
			"nrn": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The NRN to list dimensions at, e.g. `organization=1:account=2`.",
			},
			"dimensions": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "The dimensions visible at the NRN, ordered by ID.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id":     {Type: schema.TypeInt, Computed: true, Description: "A system-wide unique ID for the dimension."},
						"name":   {Type: schema.TypeString, Computed: true, Description: "The name of the dimension."},
						"slug":   {Type: schema.TypeString, Computed: true, Description: "The slug identifier for the dimension, e.g. `environment`."},
						"status": {Type: schema.TypeString, Computed: true, Description: "The status of the dimension."},
						"nrn":    {Type: schema.TypeString, Computed: true, Description: "The NRN the dimension is defined at."},
						"order":  {Type: schema.TypeInt, Computed: true, Description: "The position of the dimension among its NRN's dimensions."},
						"values": {
							Type:        schema.TypeList,
							Computed:    true,
							Description: "The values of the dimension.",
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"id":     {Type: schema.TypeInt, Computed: true, Description: "A system-wide unique ID for the value."},
									"name":   {Type: schema.TypeString, Computed: true, Description: "The name of the value."},
									"slug":   {Type: schema.TypeString, Computed: true, Description: "The slug identifier for the value, e.g. `production`."},
									"status": {Type: schema.TypeString, Computed: true, Description: "The status of the value."},
									"nrn":    {Type: schema.TypeString, Computed: true, Description: "The NRN the value is defined at."},
								},
							},
						},
					},
				},
			},
		},
	}
}

func dataSourceDimensionsRead(_ context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)
	nrn := d.Get("nrn").(string)

	dimensions, err := nullOps.ListDimensions(nrn)
	if err != nil {
		return diag.FromErr(err)
	}

	rows := make([]map[string]any, 0, len(dimensions))
	for _, dimension := range dimensions {
		values := make([]map[string]any, 0, len(dimension.Values))
		for _, value := range dimension.Values {
			values = append(values, map[string]any{
				"id":     value.ID,
				"name":   value.Name,
				"slug":   value.Slug,
				"status": value.Status,
				"nrn":    value.NRN,
			})
		}
		rows = append(rows, map[string]any{
			"id":     dimension.ID,
			"name":   dimension.Name,
			"slug":   dimension.Slug,
			"status": dimension.Status,
			"nrn":    dimension.NRN,
			"order":  dimension.Order,
			"values": values,
		})
	}

	if err := d.Set("dimensions", rows); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(nrn)

	return nil
}
