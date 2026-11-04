package nullplatform

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceServices() *schema.Resource {
	return &schema.Resource{
		Description: "Lists the services visible at an NRN: those attached to it and to its ancestors, never those attached to its descendants. An application's NRN lists the application's services; a namespace's NRN does not.",

		ReadContext: dataSourceServicesRead,
		Schema: map[string]*schema.Schema{
			"nrn": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The NRN to list services at, e.g. `organization=1:account=2:namespace=3:application=4`.",
			},
			"services": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "The services visible at the NRN, ordered by creation time.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id":               {Type: schema.TypeString, Computed: true, Description: "A system-wide unique ID for the service."},
						"name":             {Type: schema.TypeString, Computed: true, Description: "The name of the service."},
						"slug":             {Type: schema.TypeString, Computed: true, Description: "The slug identifier for the service."},
						"status":           {Type: schema.TypeString, Computed: true, Description: "The status of the service, e.g. `active`."},
						"specification_id": {Type: schema.TypeString, Computed: true, Description: "The ID of the service specification the service implements."},
						"entity_nrn":       {Type: schema.TypeString, Computed: true, Description: "The NRN of the entity the service is attached to."},
						"dimensions": {
							Type:        schema.TypeMap,
							Computed:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "The dimensions of the service.",
						},
					},
				},
			},
		},
	}
}

func dataSourceServicesRead(_ context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)
	nrn := d.Get("nrn").(string)

	services, err := nullOps.ListServices(nrn)
	if err != nil {
		return diag.FromErr(err)
	}

	rows := make([]map[string]any, 0, len(services))
	for _, service := range services {
		dimensions := make(map[string]string, len(service.Dimensions))
		for key, value := range service.Dimensions {
			dimensions[key] = fmt.Sprint(value)
		}
		rows = append(rows, map[string]any{
			"id":               service.Id,
			"name":             service.Name,
			"slug":             service.Slug,
			"status":           service.Status,
			"specification_id": service.SpecificationId,
			"entity_nrn":       service.EntityNrn,
			"dimensions":       dimensions,
		})
	}

	if err := d.Set("services", rows); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(nrn)

	return nil
}
