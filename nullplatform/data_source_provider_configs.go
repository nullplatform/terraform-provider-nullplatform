package nullplatform

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceProviderConfigs() *schema.Resource {
	return &schema.Resource{
		Description: "Lists the provider configurations defined exactly at an NRN. Configurations defined at its ancestors or descendants are not listed: an account's NRN lists the account's configurations, its namespaces' NRNs do not. The listing does not include the configurations' attributes.",

		ReadContext: dataSourceProviderConfigsRead,
		Schema: map[string]*schema.Schema{
			"nrn": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The NRN to list provider configurations at, e.g. `organization=1:account=2`.",
			},
			"provider_configs": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "The provider configurations defined at the NRN.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id":               {Type: schema.TypeString, Computed: true, Description: "A system-wide unique ID for the provider configuration."},
						"nrn":              {Type: schema.TypeString, Computed: true, Description: "The NRN the provider configuration is defined at."},
						"specification_id": {Type: schema.TypeString, Computed: true, Description: "The ID of the provider specification the configuration implements."},
						"dimensions": {
							Type:        schema.TypeMap,
							Computed:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "The dimensions the provider configuration applies to; empty when it applies without dimension scoping.",
						},
					},
				},
			},
		},
	}
}

func dataSourceProviderConfigsRead(_ context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)
	nrn := d.Get("nrn").(string)

	configs, err := nullOps.ListProviderConfigs(nrn)
	if err != nil {
		return diag.FromErr(err)
	}

	rows := make([]map[string]any, 0, len(configs))
	for _, config := range configs {
		rows = append(rows, map[string]any{
			"id":               config.Id,
			"nrn":              config.Nrn,
			"specification_id": config.SpecificationId,
			"dimensions":       config.Dimensions,
		})
	}

	if err := d.Set("provider_configs", rows); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(nrn)

	return nil
}
