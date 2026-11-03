package nullplatform

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceScopes() *schema.Resource {
	return &schema.Resource{
		Description: "Lists the scopes of an application. The API lists deleted scopes too; set `status` to list only scopes in that status.",

		ReadContext: dataSourceScopesRead,
		Schema: map[string]*schema.Schema{
			"application_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "ID of the application whose scopes are listed.",
			},
			"status": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Only list scopes in this status, e.g. `active`. When omitted, scopes in every status are listed, deleted ones included.",
			},
			"scopes": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "The scopes of the application, ordered by ID.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id":             {Type: schema.TypeInt, Computed: true, Description: "A system-wide unique ID for the scope."},
						"name":           {Type: schema.TypeString, Computed: true, Description: "The name of the scope."},
						"slug":           {Type: schema.TypeString, Computed: true, Description: "The unique slug identifier for the scope."},
						"status":         {Type: schema.TypeString, Computed: true, Description: "The status of the scope, e.g. `active` or `deleted`."},
						"type":           {Type: schema.TypeString, Computed: true, Description: "The type of the scope, e.g. `web_pool_k8s` or `custom`."},
						"application_id": {Type: schema.TypeInt, Computed: true, Description: "The ID of the application that owns the scope."},
						"nrn":            {Type: schema.TypeString, Computed: true, Description: "The Nullplatform Resource Name (NRN) for the scope."},
						"domain":         {Type: schema.TypeString, Computed: true, Description: "The domain the scope is served at; empty when it has none."},
						"dimensions": {
							Type:        schema.TypeMap,
							Computed:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "The dimensions of the scope, e.g. `{ environment = \"production\" }`.",
						},
					},
				},
			},
		},
	}
}

func dataSourceScopesRead(_ context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)
	applicationID := strconv.Itoa(d.Get("application_id").(int))
	status := d.Get("status").(string)

	scopes, err := nullOps.ListScopes(applicationID, status)
	if err != nil {
		return diag.FromErr(err)
	}

	rows := make([]map[string]any, 0, len(scopes))
	for _, scope := range scopes {
		rows = append(rows, map[string]any{
			"id":             scope.Id,
			"name":           scope.Name,
			"slug":           scope.Slug,
			"status":         scope.Status,
			"type":           scope.Type,
			"application_id": scope.ApplicationId,
			"nrn":            scope.Nrn,
			"domain":         scope.Domain,
			"dimensions":     scope.Dimensions,
		})
	}

	if err := d.Set("scopes", rows); err != nil {
		return diag.FromErr(err)
	}

	id := applicationID
	if status != "" {
		id += "/" + status
	}
	d.SetId(id)

	return nil
}
