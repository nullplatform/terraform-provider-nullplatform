package nullplatform

import (
	"context"
	"encoding/json"
	"log"
	"reflect"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceParameterValue() *schema.Resource {
	return &schema.Resource{
		Description: "The parameter value resource allows you to manage an application or scope parameter value.",

		Create: ParameterValueCreate,
		Read:   ParameterValueRead,
		Update: ParameterValueUpdate,
		Delete: ParameterValueDelete,

		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				d.Set("id", d.Id())
				return []*schema.ResourceData{d}, nil
			},
		},

		Schema: map[string]*schema.Schema{
			"parameter_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "The ID of the parameter.",
			},
			"origin_version": {
				Type:        schema.TypeInt,
				Optional:    true,
				Computed:    true,
				Description: "Use when you want to create a new value copying the other values from a specific-version (roll back).",
			},
			"nrn": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The NRN of the application or scope to which the value will apply to (when setting dimensions, the NRN must be at app-level).",
			},
			"value": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The content of the value. Can't exceed 2KB for environment variables and 2MB for files.",
			},
			"dimensions": {
				Type:     schema.TypeMap,
				ForceNew: true,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "The dimensions of the value.",
			},
		},
	}
}

func ParameterValueCreate(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)

	// FIXME: This code is duplicated in Scope
	dimensionsMap := d.Get("dimensions").(map[string]any)
	// Convert the dimensions to a map[string]string
	dimensions := make(map[string]string)
	for key, value := range dimensionsMap {
		dimensions[key] = value.(string)
	}

	parameterId := d.Get("parameter_id").(int)

	newParameterValue := &ParameterValue{
		OriginVersion: d.Get("origin_version").(int),
		Nrn:           d.Get("nrn").(string),
		Value:         d.Get("value").(string),
		Dimensions:    dimensions,
	}

	paramValue, err := nullOps.CreateParameterValue(parameterId, newParameterValue)
	if err != nil {
		return err
	}

	log.Printf("[DEBUG] Parameter Value Created with OriginID: %d", paramValue.Id)

	paramValueId := generateParameterValueID(paramValue)
	d.SetId(paramValueId)

	return ParameterValueRead(d, m)
}

func ParameterValueRead(d *schema.ResourceData, m any) error {
	var parameterValue *ParameterValue

	nullOps := m.(NullOps)
	parameterId := strconv.Itoa(d.Get("parameter_id").(int))
	parameterValueId := d.Id()

	param, err := nullOps.GetParameter(parameterId)
	if err != nil {
		// FIXME: Validate if error == 404
		if !d.IsNewResource() {
			log.Printf("[WARN] Parameter ID %s not found, removing value from state", d.Id())
			d.SetId("")
			return nil
		}
		return err
	}

	for _, item := range param.Values {
		if parameterValueId == generateParameterValueID(item) {
			parameterValue = item

			// -------- DEBUG
			// Convert struct to JSON
			jsonData, err := json.Marshal(item)
			if err != nil {
				return err
			}
			// Print JSON string
			//log.Println(string(jsonData))
			// -------- DEBUG
			log.Printf("[DEBUG] Found Parameter Value ID: %s, %s", parameterValueId, string(jsonData))

			break
		}
	}

	if parameterValue == nil {
		log.Printf("[WARN] Cannot fetch Parameter Value ID %s", parameterValueId)
		return nil
	}

	/*if err := d.Set("origin_version", parameterValue.OriginVersion); err != nil {
		return err
	}*/

	if err := d.Set("nrn", parameterValue.Nrn); err != nil {
		return err
	}

	if err := d.Set("value", parameterValue.Value); err != nil {
		return err
	}

	if err := d.Set("dimensions", parameterValue.Dimensions); err != nil {
		return err
	}

	return nil
}

func ParameterValueUpdate(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)

	// FIXME: This code is duplicated in Scope
	dimensionsMap := d.Get("dimensions").(map[string]any)
	// Convert the dimensions to a map[string]string
	dimensions := make(map[string]string)
	for key, value := range dimensionsMap {
		dimensions[key] = value.(string)
	}

	parameterId := d.Get("parameter_id").(int)

	newParameterValue := &ParameterValue{}

	if d.HasChange("origin_version") {
		newParameterValue.OriginVersion = d.Get("origin_version").(int)
	}

	if d.HasChange("value") {
		newParameterValue.Value = d.Get("value").(string)
	}

	// The ID of the Parameter Value will change if other value is updated
	// Instead the NRN and Dimensions are composed to generate an ID
	if !reflect.DeepEqual(*newParameterValue, ParameterValue{}) {
		newParameterValue.Nrn = d.Get("nrn").(string)
		// Update the value means creating a new version of it
		paramValue, err := nullOps.CreateParameterValue(parameterId, newParameterValue)
		if err != nil {
			return err
		}
		// -------- DEBUG
		// Convert struct to JSON
		jsonData, err := json.Marshal(paramValue)
		if err != nil {
			return err
		}
		// Print JSON string
		log.Println("[DEBUG] Creating new Parameter Value version: ", string(jsonData))
		// -------- DEBUG
		//d.Set("new_id", paramValue.Id)
		paramValueId := generateParameterValueID(paramValue)
		d.SetId(paramValueId)
	}

	return nil
}

func ParameterValueDelete(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)
	parameterId := strconv.Itoa(d.Get("parameter_id").(int))
	parameterValueId := d.Id()

	// FIXME: Most of this logic is duplicated in `ParameterValueRead`
	var parameterValue *ParameterValue

	param, err := nullOps.GetParameter(parameterId)
	if err != nil {
		// FIXME: Validate if error == 404
		if !d.IsNewResource() {
			log.Printf("[WARN] Parameter ID %s not found, removing value from state", d.Id())
			d.SetId("")
			return nil
		}
		return err
	}

	for _, item := range param.Values {
		// -------- DEBUG
		// Convert struct to JSON
		jsonData, err := json.Marshal(item)
		if err != nil {
			return err
		}
		// Print JSON string
		log.Println(string(jsonData))
		// -------- DEBUG

		if parameterValueId == generateParameterValueID(item) {
			parameterValue = item
			break
		}
	}

	if parameterValue == nil {
		log.Printf("[WARN] Cannot fetch Parameter Value ID %s", parameterValueId)
		return nil
	}

	err = nullOps.DeleteParameterValue(parameterId, strconv.Itoa(parameterValue.Id))
	if err != nil {
		// FIXME: Validate if error == 404
		log.Printf("[WARN] Parameter Value ID %s not found, removing from state", parameterValueId)
		d.SetId("")
		return nil
	}

	d.SetId("")

	return nil
}
