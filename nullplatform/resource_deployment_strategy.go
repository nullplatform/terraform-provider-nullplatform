package nullplatform

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceDeploymentStrategy() *schema.Resource {
	return &schema.Resource{
		Description: "The deployment_strategy resource allows you to manage nullplatform Deployment Strategies",

		CreateContext: DeploymentStrategyCreate,
		ReadContext:   DeploymentStrategyRead,
		UpdateContext: DeploymentStrategyUpdate,
		DeleteContext: DeploymentStrategyDelete,

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
				Description: "The name of the deployment strategy.",
			},
			"description": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "A description of the deployment strategy.",
			},
			"dimensions": {
				Type:             schema.TypeString,
				Optional:         true,
				Default:          "{}",
				Description:      "JSON string containing the dimensions the strategy applies to.",
				DiffSuppressFunc: suppressEquivalentJSON,
			},
			"parameters": {
				Type:             schema.TypeString,
				Optional:         true,
				Default:          "{}",
				Description:      "JSON string containing the strategy parameters.",
				DiffSuppressFunc: suppressEquivalentJSON,
			},
			"scope_type_ids": {
				Type:        schema.TypeList,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "List of scope type IDs this strategy is restricted to.",
			},
			"created_by": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The user that created the deployment strategy.",
			},
			"updated_by": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The user that last updated the deployment strategy.",
			},
			"created_at": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The creation timestamp of the deployment strategy.",
			},
			"updated_at": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The last update timestamp of the deployment strategy.",
			},
		}),
	}
}

func deploymentStrategyScopeTypeIds(d *schema.ResourceData) []string {
	raw := d.Get("scope_type_ids").([]interface{})
	ids := make([]string, len(raw))
	for i, v := range raw {
		ids[i] = v.(string)
	}
	return ids
}

func DeploymentStrategyCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)

	var nrn string
	var err error
	if v, ok := d.GetOk("nrn"); ok {
		nrn = v.(string)
	} else {
		nrn, err = ConstructNRNFromComponents(d, nullOps)
		if err != nil {
			return diag.FromErr(fmt.Errorf("error constructing NRN: %v %s", err, nrn))
		}
	}

	dimensionsStr := d.Get("dimensions").(string)
	var dimensions map[string]interface{}
	if err := json.Unmarshal([]byte(dimensionsStr), &dimensions); err != nil {
		return diag.FromErr(fmt.Errorf("error parsing dimensions JSON: %v", err))
	}

	parametersStr := d.Get("parameters").(string)
	var parameters map[string]interface{}
	if err := json.Unmarshal([]byte(parametersStr), &parameters); err != nil {
		return diag.FromErr(fmt.Errorf("error parsing parameters JSON: %v", err))
	}

	newDS := &DeploymentStrategy{
		Name:         d.Get("name").(string),
		Description:  d.Get("description").(string),
		Nrn:          nrn,
		Dimensions:   dimensions,
		Parameters:   parameters,
		ScopeTypeIds: deploymentStrategyScopeTypeIds(d),
	}

	ds, err := nullOps.CreateDeploymentStrategy(newDS)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(strconv.Itoa(ds.Id))
	return DeploymentStrategyRead(ctx, d, m)
}

func DeploymentStrategyRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)
	dsId := d.Id()

	ds, err := nullOps.GetDeploymentStrategy(dsId)
	if err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("name", ds.Name); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("description", ds.Description); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("nrn", ds.Nrn); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("scope_type_ids", ds.ScopeTypeIds); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("created_by", ds.CreatedBy); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("updated_by", ds.UpdatedBy); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("created_at", ds.CreatedAt); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("updated_at", ds.UpdatedAt); err != nil {
		return diag.FromErr(err)
	}

	dimensionsJSON, err := json.Marshal(ds.Dimensions)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error serializing dimensions to JSON: %v", err))
	}
	if err := d.Set("dimensions", string(dimensionsJSON)); err != nil {
		return diag.FromErr(err)
	}

	parametersJSON, err := json.Marshal(ds.Parameters)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error serializing parameters to JSON: %v", err))
	}
	if err := d.Set("parameters", string(parametersJSON)); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func DeploymentStrategyUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)
	dsId := d.Id()

	ds := &DeploymentStrategy{}

	if d.HasChange("name") {
		ds.Name = d.Get("name").(string)
	}
	if d.HasChange("description") {
		ds.Description = d.Get("description").(string)
	}
	if d.HasChange("dimensions") {
		dimensionsStr := d.Get("dimensions").(string)
		var dimensions map[string]interface{}
		if err := json.Unmarshal([]byte(dimensionsStr), &dimensions); err != nil {
			return diag.FromErr(fmt.Errorf("error parsing dimensions JSON: %v", err))
		}
		ds.Dimensions = dimensions
	}
	if d.HasChange("parameters") {
		parametersStr := d.Get("parameters").(string)
		var parameters map[string]interface{}
		if err := json.Unmarshal([]byte(parametersStr), &parameters); err != nil {
			return diag.FromErr(fmt.Errorf("error parsing parameters JSON: %v", err))
		}
		ds.Parameters = parameters
	}
	if d.HasChange("scope_type_ids") {
		ds.ScopeTypeIds = deploymentStrategyScopeTypeIds(d)
	}

	if err := nullOps.PatchDeploymentStrategy(dsId, ds); err != nil {
		return diag.FromErr(err)
	}

	return DeploymentStrategyRead(ctx, d, m)
}

func DeploymentStrategyDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)
	dsId := d.Id()

	if err := nullOps.DeleteDeploymentStrategy(dsId); err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
