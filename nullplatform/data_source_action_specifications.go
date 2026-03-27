package nullplatform

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceActionSpecifications() *schema.Resource {
	return &schema.Resource{
		Description: "Lists all action specifications for a given service specification",
		ReadContext: dataSourceActionSpecificationsRead,
		Schema: map[string]*schema.Schema{
			"service_specification_id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "ID of the service specification to list action specifications for.",
			},
			"action_specifications": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "List of action specifications belonging to the service specification.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id": {
							Type:     schema.TypeString,
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
						"type": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"retryable": {
							Type:     schema.TypeBool,
							Computed: true,
						},
						"icon": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"parameters": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"results": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"annotations": {
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},
		},
	}
}

func dataSourceActionSpecificationsRead(_ context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)
	serviceSpecId := d.Get("service_specification_id").(string)

	specs, err := nullOps.ListActionSpecifications(serviceSpecId)
	if err != nil {
		return diag.FromErr(err)
	}

	actionSpecs := make([]map[string]interface{}, 0, len(specs))
	for _, spec := range specs {
		parametersJSON, err := json.Marshal(spec.Parameters)
		if err != nil {
			return diag.FromErr(fmt.Errorf("error serializing parameters for %s: %v", spec.Id, err))
		}

		resultsJSON, err := json.Marshal(spec.Results)
		if err != nil {
			return diag.FromErr(fmt.Errorf("error serializing results for %s: %v", spec.Id, err))
		}

		annotationsJSON, err := json.Marshal(spec.Annotations)
		if err != nil {
			return diag.FromErr(fmt.Errorf("error serializing annotations for %s: %v", spec.Id, err))
		}

		actionSpecs = append(actionSpecs, map[string]interface{}{
			"id":          spec.Id,
			"name":        spec.Name,
			"slug":        spec.Slug,
			"type":        spec.Type,
			"retryable":   spec.Retryable,
			"icon":        spec.Icon,
			"parameters":  string(parametersJSON),
			"results":     string(resultsJSON),
			"annotations": string(annotationsJSON),
		})
	}

	if err := d.Set("action_specifications", actionSpecs); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(serviceSpecId)

	return nil
}
