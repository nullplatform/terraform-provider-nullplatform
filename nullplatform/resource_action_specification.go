package nullplatform

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourceActionSpecification() *schema.Resource {
	return &schema.Resource{
		Description: "The action_specification resource allows you to manage nullplatform Action Specifications",

		CreateContext: ActionSpecificationCreate,
		ReadContext:   ActionSpecificationRead,
		UpdateContext: ActionSpecificationUpdate,
		DeleteContext: ActionSpecificationDelete,

		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				d.Set("id", d.Id())
				return []*schema.ResourceData{d}, nil
			},
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the action specification",
			},
			"slug": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The computed slug for the action specification",
			},
			"type": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				ValidateFunc: validation.StringInSlice([]string{
					"custom", "create", "update", "delete", "diagnose",
				}, false),
				Description: "Type of the action. Must be one of: custom, create, update, delete, diagnose",
			},
			"service_specification_id": {
				Type:         schema.TypeString,
				Optional:     true,
				ForceNew:     true,
				ExactlyOneOf: []string{"service_specification_id", "link_specification_id"},
				Description:  "ID of the associated service specification",
			},
			"link_specification_id": {
				Type:         schema.TypeString,
				Optional:     true,
				ForceNew:     true,
				ExactlyOneOf: []string{"service_specification_id", "link_specification_id"},
				Description:  "ID of the associated link specification",
			},
			"parameters": {
				Type:             schema.TypeString,
				Required:         true,
				Description:      "JSON string containing the parameters schema and values",
				DiffSuppressFunc: suppressEquivalentJSON,
			},
			"results": {
				Type:             schema.TypeString,
				Required:         true,
				Description:      "JSON string containing the expected results schema",
				DiffSuppressFunc: suppressEquivalentJSON,
			},
			"retryable": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether the action can be retried if the instance is in a failed state",
			},
			"icon": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Icon for the action specification",
			},
			"annotations": {
				Type:             schema.TypeString,
				Optional:         true,
				Description:      "JSON string containing annotations for the action specification",
				DiffSuppressFunc: suppressEquivalentJSON,
			},
		},
	}
}

func ActionSpecificationCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)

	parametersStr := d.Get("parameters").(string)
	var parameters map[string]interface{}
	if err := json.Unmarshal([]byte(parametersStr), &parameters); err != nil {
		return diag.FromErr(fmt.Errorf("error parsing parameters JSON: %v", err))
	}

	resultsStr := d.Get("results").(string)
	var results map[string]interface{}
	if err := json.Unmarshal([]byte(resultsStr), &results); err != nil {
		return diag.FromErr(fmt.Errorf("error parsing results JSON: %v", err))
	}

	spec := &ActionSpecification{
		Name:                   d.Get("name").(string),
		Type:                   d.Get("type").(string),
		Parameters:             parameters,
		Results:                results,
		Retryable:              d.Get("retryable").(bool),
		ServiceSpecificationId: d.Get("service_specification_id").(string),
		LinkSpecificationId:    d.Get("link_specification_id").(string),
		Icon:                   d.Get("icon").(string),
	}

	// Handle annotations if provided
	if annotationsStr, ok := d.GetOk("annotations"); ok {
		var annotations map[string]interface{}
		if err := json.Unmarshal([]byte(annotationsStr.(string)), &annotations); err != nil {
			return diag.FromErr(fmt.Errorf("error parsing annotations JSON: %v", err))
		}
		spec.Annotations = annotations
	}

	newSpec, err := nullOps.CreateActionSpecification(spec)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(newSpec.Id)
	return ActionSpecificationRead(ctx, d, m)
}

func ActionSpecificationRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)
	specId := d.Id()

	var parentType, parentId string

	if v := d.Get("service_specification_id").(string); v != "" {
		parentType = "service"
		parentId = v
	} else {
		parentType = "link"
		parentId = d.Get("link_specification_id").(string)
	}

	spec, err := nullOps.GetActionSpecification(specId, parentType, parentId)
	if err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("name", spec.Name); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("slug", spec.Slug); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("type", spec.Type); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("service_specification_id", spec.ServiceSpecificationId); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("link_specification_id", spec.LinkSpecificationId); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("retryable", spec.Retryable); err != nil {
		return diag.FromErr(err)
	}

	parametersJSON, err := json.Marshal(spec.Parameters)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error serializing parameters to JSON: %v", err))
	}
	if err := d.Set("parameters", string(parametersJSON)); err != nil {
		return diag.FromErr(err)
	}

	resultsJSON, err := json.Marshal(spec.Results)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error serializing results to JSON: %v", err))
	}
	if err := d.Set("results", string(resultsJSON)); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("icon", spec.Icon); err != nil {
		return diag.FromErr(err)
	}

	if spec.Annotations != nil {
		annotationsJSON, err := json.Marshal(spec.Annotations)
		if err != nil {
			return diag.FromErr(fmt.Errorf("error serializing annotations to JSON: %v", err))
		}
		if err := d.Set("annotations", string(annotationsJSON)); err != nil {
			return diag.FromErr(err)
		}
	}

	return nil
}

func ActionSpecificationUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)
	specId := d.Id()

	// Determine the parent type
	var parentType string
	var parentId string
	if v := d.Get("service_specification_id").(string); v != "" {
		parentType = "service"
		parentId = v
	} else {
		parentType = "link"
		parentId = d.Get("link_specification_id").(string)
	}

	spec := &ActionSpecification{}

	if d.HasChange("name") {
		spec.Name = d.Get("name").(string)
	}

	if d.HasChange("type") {
		spec.Type = d.Get("type").(string)
	}

	if d.HasChange("parameters") {
		parametersStr := d.Get("parameters").(string)
		var parameters map[string]interface{}
		if err := json.Unmarshal([]byte(parametersStr), &parameters); err != nil {
			return diag.FromErr(fmt.Errorf("error parsing parameters JSON: %v", err))
		}
		spec.Parameters = parameters
	}

	if d.HasChange("results") {
		resultsStr := d.Get("results").(string)
		var results map[string]interface{}
		if err := json.Unmarshal([]byte(resultsStr), &results); err != nil {
			return diag.FromErr(fmt.Errorf("error parsing results JSON: %v", err))
		}
		spec.Results = results
	}

	if d.HasChange("retryable") {
		spec.Retryable = d.Get("retryable").(bool)
	}

	if d.HasChange("icon") {
		spec.Icon = d.Get("icon").(string)
	}

	if d.HasChange("annotations") {
		if annotationsStr, ok := d.GetOk("annotations"); ok {
			var annotations map[string]interface{}
			if err := json.Unmarshal([]byte(annotationsStr.(string)), &annotations); err != nil {
				return diag.FromErr(fmt.Errorf("error parsing annotations JSON: %v", err))
			}
			spec.Annotations = annotations
		} else {
			spec.Annotations = nil
		}
	}

	err := nullOps.PatchActionSpecification(specId, spec, parentType, parentId)
	if err != nil {
		return diag.FromErr(err)
	}

	return ActionSpecificationRead(ctx, d, m)
}

func ActionSpecificationDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)
	specId := d.Id()

	// Determine the parent type
	var parentType string
	var parentId string
	if v := d.Get("service_specification_id").(string); v != "" {
		parentType = "service"
		parentId = v
	} else {
		parentType = "link"
		parentId = d.Get("link_specification_id").(string)
	}

	err := nullOps.DeleteActionSpecification(specId, parentType, parentId)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
