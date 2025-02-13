package nullplatform

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceDimension() *schema.Resource {
	return &schema.Resource{
		Description: "Provides information about the Dimension",

		ReadContext: dataSourceDimensionRead,
		Schema: map[string]*schema.Schema{
			"id": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "A system-wide unique ID for the Dimension",
			},
			"name": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The Dimension name.",
			},
			"slug": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The Dimension slug.",
			},
			"status": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Possible values: [`active`, `inactive`].",
			},
			"nrn": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "A system-wide unique ID representing the resource. If id not provided nrn is mandatory",
			},
			"values": {
				Type:        schema.TypeList,
				Optional:    true,
				Description: "Values available for the given dimension",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id": {
							Type:     schema.TypeFloat,
							Computed: true,
						},
						"name": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"slug": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"nrn": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"status": {
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},
		},
	}
}
func dataSourceDimensionRead(_ context.Context, d *schema.ResourceData, unWrappedNullOps any) diag.Diagnostics {
	nullOps := unWrappedNullOps.(NullOps)

	id := strconv.Itoa(d.Get("id").(int))
	name := d.Get("name").(string)
	slug := d.Get("slug").(string)
	status := d.Get("status").(string)
	nrn := d.Get("nrn").(string)
	dimension, err := nullOps.GetDimension(&id, &name, &slug, &status, &nrn)
	if err != nil {
		return diag.FromErr(err)
	}

	if err = d.Set("name", dimension.Name); err != nil {
		return diag.FromErr(err)
	}

	if err = d.Set("status", dimension.Status); err != nil {
		return diag.FromErr(err)
	}

	if err = d.Set("slug", dimension.Slug); err != nil {
		return diag.FromErr(err)
	}

	if err = d.Set("nrn", dimension.NRN); err != nil {
		return diag.FromErr(err)
	}

	value := make([]map[string]any, len(dimension.Values))
	for i, v := range dimension.Values {
		value[i] = map[string]any{
			"id":     v.ID,
			"name":   v.Name,
			"slug":   v.Slug,
			"nrn":    v.NRN,
			"status": v.Status,
		}
	}

	if err = d.Set("values", value); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(strconv.FormatInt(int64(dimension.ID), 10))
	return nil
}
