package nullplatform

import (
	"context"
	"strconv"
	"time"

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
				Required:    true,
				Description: "A system-wide unique ID for the Dimension",
			},
			"name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The Dimension name.",
			},
			"slug": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The Dimension slug.",
			},
			"status": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Possible values: [`active`, `inactive`].",
			},
			"nrn": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "A system-wide unique ID representing the resource. If id not provided nrn is mandatory",
			},
		},
	}
}

func dataSourceDimensionRead(_ context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)

	id := strconv.Itoa(d.Get("id").(int))
	name := d.Get("name").(string)
	slug := d.Get("slug").(string)
	status := d.Get("status").(string)
	nrn := d.Get("nrn").(string)
	dimension, err := nullOps.GetDimension(&id, &name, &slug, &status, &nrn)
	if err != nil {
		return diag.FromErr(err)
	}

	err = d.Set("name", dimension.Name)
	if err != nil {
		return diag.FromErr(err)
	}

	err = d.Set("name", dimension.Name)
	if err != nil {
		return diag.FromErr(err)
	}

	err = d.Set("status", dimension.Status)
	if err != nil {
		return diag.FromErr(err)
	}

	err = d.Set("slug", dimension.Slug)
	if err != nil {
		return diag.FromErr(err)
	}

	err = d.Set("nrn", dimension.NRN)
	if err != nil {
		return diag.FromErr(err)
	}

	//fmt.Printf("ResourceData: %+v\n", d)

	// We don't have a unique ID for this data resource so we create one using a
	// timestamp format. I've seen people use a hash of the returned API data as
	// a unique key.
	//
	// NOTE:
	// That hashcode helper is no longer available! It has been moved into an
	// internal directory meaning it's not supposed to be consumed.
	//
	// Reference:
	// https://github.com/hashicorp/terraform-plugin-sdk/blob/master/internal/helper/hashcode/hashcode.go
	//
	d.SetId(strconv.FormatInt(time.Now().Unix(), 10))

	return nil
}
