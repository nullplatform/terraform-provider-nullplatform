package nullplatform

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceServiceAction() *schema.Resource {
	return &schema.Resource{
		Description: "The service_action resource allows you to trigger and manage actions executed against a nullplatform Service",

		CreateContext: ServiceActionCreate,
		ReadContext:   ServiceActionRead,
		DeleteContext: ServiceActionDelete,

		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				parts := strings.SplitN(d.Id(), "/", 2)
				if len(parts) != 2 {
					return nil, fmt.Errorf("invalid import ID %q, expected format <service_id>/<action_id>", d.Id())
				}
				d.Set("service_id", parts[0])
				d.SetId(parts[1])
				return []*schema.ResourceData{d}, nil
			},
		},

		Schema: map[string]*schema.Schema{
			"service_id": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "ID of the service the action is executed against",
			},
			"specification_id": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "ID of the action specification to execute",
			},
			"parameters": {
				Type:             schema.TypeString,
				Optional:         true,
				ForceNew:         true,
				Description:      "JSON string containing the parameters for the action",
				DiffSuppressFunc: suppressEquivalentJSON,
			},
			"status": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The current status of the action",
			},
			"results": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "JSON string containing the results produced by the action",
			},
		},
	}
}

func ServiceActionCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)

	serviceID := d.Get("service_id").(string)

	action := &ActionInstance{
		SpecificationId: d.Get("specification_id").(string),
		ServiceId:       serviceID,
	}

	if parametersStr, ok := d.GetOk("parameters"); ok {
		var parameters map[string]interface{}
		if err := json.Unmarshal([]byte(parametersStr.(string)), &parameters); err != nil {
			return diag.FromErr(fmt.Errorf("error parsing parameters JSON: %v", err))
		}
		action.Parameters = parameters
	}

	newAction, err := nullOps.CreateServiceAction(serviceID, action)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(newAction.Id)
	return ServiceActionRead(ctx, d, m)
}

func ServiceActionRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)

	serviceID := d.Get("service_id").(string)
	actionID := d.Id()

	action, err := nullOps.GetServiceAction(serviceID, actionID)
	if err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("specification_id", action.SpecificationId); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("status", action.Status); err != nil {
		return diag.FromErr(err)
	}

	if action.Parameters != nil {
		parametersJSON, err := json.Marshal(action.Parameters)
		if err != nil {
			return diag.FromErr(fmt.Errorf("error serializing parameters to JSON: %v", err))
		}
		if err := d.Set("parameters", string(parametersJSON)); err != nil {
			return diag.FromErr(err)
		}
	}

	if action.Results != nil {
		resultsJSON, err := json.Marshal(action.Results)
		if err != nil {
			return diag.FromErr(fmt.Errorf("error serializing results to JSON: %v", err))
		}
		if err := d.Set("results", string(resultsJSON)); err != nil {
			return diag.FromErr(err)
		}
	}

	return nil
}

func ServiceActionDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)

	serviceID := d.Get("service_id").(string)
	actionID := d.Id()

	if err := nullOps.DeleteServiceAction(serviceID, actionID); err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
