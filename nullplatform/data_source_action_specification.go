package nullplatform

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceActionSpecification() *schema.Resource {
	return &schema.Resource{
		Description: "Provides information about an existing nullplatform Action Specification",
		ReadContext: dataSourceActionSpecificationRead,
		Schema: map[string]*schema.Schema{
			"id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The ID of the action specification.",
			},
			"service_specification_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "ID of the associated service specification. Required if not using link_specification_id.",
			},
			"link_specification_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "ID of the associated link specification. Required if not using service_specification_id.",
			},
			"name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Name of the action specification.",
			},
			"slug": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The computed slug for the action specification.",
			},
			"type": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Type of the action.",
			},
			"parameters": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "JSON string containing the parameters schema and values.",
			},
			"results": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "JSON string containing the expected results schema.",
			},
			"retryable": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether the action can be retried if the instance is in a failed state.",
			},
			"icon": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Icon for the action specification.",
			},
			"annotations": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "JSON string containing annotations for the action specification.",
			},
		},
	}
}

func dataSourceActionSpecificationRead(_ context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)

	var parentType, parentId string
	if v := d.Get("service_specification_id").(string); v != "" {
		parentType = "service"
		parentId = v
	} else {
		parentType = "link"
		parentId = d.Get("link_specification_id").(string)
	}

	spec, err := nullOps.GetActionSpecification(d.Get("id").(string), parentType, parentId)
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
	if err := d.Set("icon", spec.Icon); err != nil {
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

	if spec.Annotations != nil {
		annotationsJSON, err := json.Marshal(spec.Annotations)
		if err != nil {
			return diag.FromErr(fmt.Errorf("error serializing annotations to JSON: %v", err))
		}
		if err := d.Set("annotations", string(annotationsJSON)); err != nil {
			return diag.FromErr(err)
		}
	}

	d.SetId(d.Get("id").(string))

	return nil
}
