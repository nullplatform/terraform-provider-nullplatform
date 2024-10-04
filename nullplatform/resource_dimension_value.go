package nullplatform

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceDimensionValue() *schema.Resource {
	return &schema.Resource{
		Description: "The dimension_value resource allows you to configure a Nullplatform Dimension Value",

		CreateContext: resourceDimensionValueCreate,
		ReadContext:   resourceDimensionValueRead,
		UpdateContext: resourceDimensionValueUpdate,
		DeleteContext: resourceDimensionValueDelete,

		Importer: &schema.ResourceImporter{
			StateContext: resourceDimensionValueImport,
		},

		Schema: map[string]*schema.Schema{
			"dimension_id": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The ID of the parent dimension.",
			},
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The name of the dimension value.",
			},
			"nrn": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The NRN (Null Resource Name) of the dimension value.",
			},
			"slug": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The slug of the dimension value.",
			},
			"status": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The status of the dimension value.",
			},
		},
	}
}

func resourceDimensionValueCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	c := m.(NullOps)

	dimensionValue := &DimensionValue{
		Name: d.Get("name").(string),
		NRN:  d.Get("nrn").(string),
	}

	dimensionID := d.Get("dimension_id").(string)

	createdValue, err := c.CreateDimensionValue(dimensionID, dimensionValue)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(fmt.Sprintf("%s:%d", dimensionID, createdValue.ID))

	return resourceDimensionValueRead(ctx, d, m)
}

func resourceDimensionValueRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	c := m.(NullOps)

	dimensionID, valueID, err := splitDimensionValueID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	value, err := c.GetDimensionValue(dimensionID, valueID)
	if err != nil {
		return diag.FromErr(err)
	}

	d.Set("dimension_id", dimensionID)
	d.Set("name", value.Name)
	d.Set("nrn", value.NRN)
	d.Set("slug", value.Slug)
	d.Set("status", value.Status)

	return nil
}

func resourceDimensionValueUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	c := m.(NullOps)

	dimensionID, valueID, err := splitDimensionValueID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	dimensionValue := &DimensionValue{
		Name: d.Get("name").(string),
		NRN:  d.Get("nrn").(string),
	}

	err = c.UpdateDimensionValue(dimensionID, valueID, dimensionValue)
	if err != nil {
		return diag.FromErr(err)
	}

	return resourceDimensionValueRead(ctx, d, m)
}

func resourceDimensionValueDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	c := m.(NullOps)

	dimensionID, valueID, err := splitDimensionValueID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	err = c.DeleteDimensionValue(dimensionID, valueID)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")

	return nil
}

func resourceDimensionValueImport(ctx context.Context, d *schema.ResourceData, m interface{}) ([]*schema.ResourceData, error) {
	dimensionID, valueID, err := splitDimensionValueID(d.Id())
	if err != nil {
		return nil, err
	}

	d.Set("dimension_id", dimensionID)
	d.SetId(fmt.Sprintf("%s:%s", dimensionID, valueID))

	return []*schema.ResourceData{d}, nil
}

func splitDimensionValueID(id string) (string, string, error) {
	parts := strings.SplitN(id, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid ID format: %s (expected dimension_id:value_id)", id)
	}
	return parts[0], parts[1], nil
}
