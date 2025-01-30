package nullplatform

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
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

	var paramValue *ParameterValue
	err := retry.RetryContext(context.Background(), 1*time.Minute, func() *retry.RetryError {
		var err error
		paramValue, err = nullOps.CreateParameterValue(parameterId, newParameterValue)
		if err != nil {
			if isRetryableError(err) {
				return retry.RetryableError(err)
			}
			return retry.NonRetryableError(err)
		}
		return nil
	})

	if err != nil {
		return err
	}

	log.Printf("[DEBUG] Parameter Value Created with OriginID: %s", paramValue.Id)

	paramValueId := generateParameterValueID(paramValue, parameterId)
	d.SetId(paramValueId)

	return ParameterValueRead(d, m)
}

func ParameterValueRead(d *schema.ResourceData, m any) error {
	var parameterValue *ParameterValue

	nullOps := m.(NullOps)
	parameterId := strconv.Itoa(d.Get("parameter_id").(int))
	parameterValueId := d.Id()
	nrn := d.Get("nrn").(string)

	err := retry.RetryContext(context.Background(), 1*time.Minute, func() *retry.RetryError {
		var err error
		parameterValue, err = nullOps.GetParameterValue(parameterId, parameterValueId, &nrn)
		if err != nil {
			if isRetryableError(err) {
				return retry.RetryableError(err)
			}
			return retry.NonRetryableError(err)
		}
		return nil
	})

	if err != nil {
		// FIXME: Validate if error == 404
		if !d.IsNewResource() {
			log.Printf("[WARN] Parameter Value ID %s not found, removing value from state", parameterValueId)
			d.SetId("")
			return nil
		}
		return err
	}

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
	if d.HasChange("origin_version") || d.HasChange("value") {
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
		}

		// Updating the value means creating a new version of it
		var paramValue *ParameterValue
		err := retry.RetryContext(context.Background(), 1*time.Minute, func() *retry.RetryError {
			var err error
			paramValue, err = nullOps.CreateParameterValue(parameterId, newParameterValue)
			if err != nil {
				if isRetryableError(err) {
					return retry.RetryableError(err)
				}
				return retry.NonRetryableError(err)
			}
			return nil
		})

		if err != nil {
			return err
		}

		// The ID of the Parameter Value will change if other value is updated
		// Instead the NRN and Dimensions are composed to generate an ID
		paramValueId := generateParameterValueID(paramValue, parameterId)
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

	param, err := nullOps.GetParameter(parameterId, nil)
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
		if parameterValueId == generateParameterValueID(item, param.Id) {
			parameterValue = item
			break
		}
	}

	if parameterValue == nil {
		log.Printf("[WARN] Cannot fetch Parameter Value ID %s", parameterValueId)
		return nil
	}

	err = retry.RetryContext(context.Background(), 1*time.Minute, func() *retry.RetryError {
		err := nullOps.DeleteParameterValue(parameterId, parameterValue.Id)
		if err != nil {
			if isRetryableError(err) {
				return retry.RetryableError(err)
			}
			return retry.NonRetryableError(err)
		}
		return nil
	})

	if err != nil {
		// FIXME: Validate if error == 404
		log.Printf("[WARN] Parameter Value ID %s not found, removing from state", parameterValueId)
		d.SetId("")
		return nil
	}

	d.SetId("")

	return nil
}

func isRetryableError(err error) bool {
	if httpErr, ok := err.(interface{ StatusCode() int }); ok {
		switch httpErr.StatusCode() {
		case http.StatusRequestTimeout, http.StatusConflict, http.StatusTooManyRequests, http.StatusServiceUnavailable, http.StatusGatewayTimeout, http.StatusBadGateway:
			return true
		case http.StatusBadRequest:
			var nErr NullErrors
			if jsonErr := json.Unmarshal([]byte(err.Error()), &nErr); jsonErr == nil {
				if nErr.Message == "The parameter already exists" {
					return true
				}
			}
		}
	}
	return false
}
