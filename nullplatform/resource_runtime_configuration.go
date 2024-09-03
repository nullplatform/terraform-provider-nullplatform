package nullplatform

import (
	"context"
	"reflect"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceRuntimeConfiguration() *schema.Resource {
	return &schema.Resource{
		Description: "The runtime configuration resource allows you to configure a nullplatform Runtime Configurations",

		Create: RuntimeConfigurationCreate,
		Read:   RuntimeConfigurationRead,
		Update: RuntimeConfigurationUpdate,
		Delete: RuntimeConfigurationeDelete,

		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				d.Set("id", d.Id())
				return []*schema.ResourceData{d}, nil
			},
		},

		Schema: map[string]*schema.Schema{
			"nrn": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "A system-wide unique ID representing the resource.",
			},
			"dimensions": {
				Type:     schema.TypeMap,
				ForceNew: true,
				Required: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "A key-value map with the runtime configuration dimensions that apply to this scope.",
			},
			"values": {
				Type:     schema.TypeMap,
				Required: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "The set values that this runtime configuration holds.",
			},
		},
	}
}

func RuntimeConfigurationCreate(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)

	dimensionsMap := d.Get("dimensions").(map[string]any)
	// Convert the dimensions to a map[string]string
	dimensions := make(map[string]string)
	for key, value := range dimensionsMap {
		dimensions[key] = value.(string)
	}

	valuesMap := d.Get("values").(map[string]any)
	values := make(map[string]string)
	for key, value := range valuesMap {
		values[key] = value.(string)
	}

	newRuntimeConfig := &RuntimeConfiguration{
		Nrn:        d.Get("nrn").(string),
		Dimensions: dimensions,
		Values:     RuntimeConfigurationValues{AWS: values},
	}

	rc, err := nullOps.CreateRuntimeConfiguration(newRuntimeConfig)

	if err != nil {
		return err
	}

	d.SetId(strconv.Itoa(rc.Id))

	return RuntimeConfigurationRead(d, m)
}

func RuntimeConfigurationRead(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)
	runtimeConfigId := d.Id()

	rc, err := nullOps.GetRuntimeConfiguration(runtimeConfigId)

	if err != nil {
		return err
	}

	if err := d.Set("nrn", rc.Nrn); err != nil {
		return err
	}

	if err := d.Set("dimensions", rc.Dimensions); err != nil {
		return err
	}

	if err := d.Set("values", rc.Values.AWS); err != nil {
		return err
	}

	return nil
}

func RuntimeConfigurationUpdate(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)
	runtimeConfigId := d.Id()

	rc := &RuntimeConfiguration{}

	if d.HasChange("nrn") {
		rc.Nrn = d.Get("nrn").(string)
	}

	if d.HasChange("dimensions") {
		dimensionsMap := d.Get("dimensions").(map[string]interface{})

		// Convert the dimensions to a map[string]string
		dimensions := make(map[string]string)
		for key, value := range dimensionsMap {
			dimensions[key] = value.(string)
		}

		rc.Dimensions = dimensions
	}

	if d.HasChange("values") {
		valuesMap := d.Get("values").(map[string]any)
		values := make(map[string]string)
		for key, value := range valuesMap {
			values[key] = value.(string)
		}

		rc.Values = RuntimeConfigurationValues{AWS: values}
	}

	if !reflect.DeepEqual(*rc, RuntimeConfiguration{}) {
		err := nullOps.PatchRuntimeConfiguration(runtimeConfigId, rc)
		if err != nil {
			return err
		}
	}

	return RuntimeConfigurationRead(d, m)
}

func RuntimeConfigurationeDelete(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)
	runtimeConfigId := d.Id()

	err := nullOps.DeleteRuntimeConfiguration(runtimeConfigId)
	if err != nil {
		return err
	}

	d.SetId("")

	return nil
}
