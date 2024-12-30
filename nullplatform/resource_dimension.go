package nullplatform

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceDimension() *schema.Resource {
	return &schema.Resource{
		Description: "The dimension resource allows you to configure a Nullplatform Dimension",

		Create: DimensionCreate,
		Read:   DimensionRead,
		Update: DimensionUpdate,
		Delete: DimensionDelete,

		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				d.Set("id", d.Id())
				return []*schema.ResourceData{d}, nil
			},
		},

		Schema: AddNRNSchema(map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The name of the dimension.",
			},
			"slug": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The slug of the dimension.",
			},
			"status": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The status of the dimension.",
			},
			"order": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "The order of the dimension.",
			},
		}),
	}
}

func DimensionCreate(d *schema.ResourceData, m interface{}) error {
	nullOps := m.(NullOps)

	var nrn string
	var err error
	if v, ok := d.GetOk("nrn"); ok {
		nrn = v.(string)
	} else {
		nrn, err = ConstructNRNFromComponents(d, nullOps)
		if err != nil {
			return fmt.Errorf("error constructing NRN: %v %s", err, nrn)
		}
	}

	dimension := &Dimension{
		Name:  d.Get("name").(string),
		NRN:   nrn,
		Order: d.Get("order").(int),
	}

	createdDimension, err := nullOps.CreateDimension(dimension)
	if err != nil {
		return err
	}

	d.SetId(strconv.Itoa(createdDimension.ID))
	return DimensionRead(d, m)
}

func DimensionRead(d *schema.ResourceData, m interface{}) error {
	nullOps := m.(NullOps)
	dimensionID := d.Id()

	dimension, err := nullOps.GetDimension(&dimensionID, nil, nil, nil, nil)
	if err != nil {
		return err
	}

	if err := d.Set("nrn", dimension.NRN); err != nil {
		return err
	}
	if err := d.Set("name", dimension.Name); err != nil {
		return err
	}
	if err := d.Set("slug", dimension.Slug); err != nil {
		return err
	}
	if err := d.Set("status", dimension.Status); err != nil {
		return err
	}
	if err := d.Set("order", dimension.Order); err != nil {
		return err
	}

	return nil
}

func DimensionUpdate(d *schema.ResourceData, m interface{}) error {
	nullOps := m.(NullOps)
	dimensionID := d.Id()

	dimension := &Dimension{}

	if d.HasChange("name") {
		dimension.Name = d.Get("name").(string)
	}
	if d.HasChange("order") {
		dimension.Order = d.Get("order").(int)
	}

	err := nullOps.UpdateDimension(dimensionID, dimension)
	if err != nil {
		return err
	}

	return DimensionRead(d, m)
}

func DimensionDelete(d *schema.ResourceData, m interface{}) error {
	nullOps := m.(NullOps)
	dimensionID := d.Id()

	err := nullOps.DeleteDimension(dimensionID)
	if err != nil {
		return err
	}

	d.SetId("")
	return nil
}
