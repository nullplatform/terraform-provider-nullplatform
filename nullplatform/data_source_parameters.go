package nullplatform

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceParameters() *schema.Resource {
	return &schema.Resource{
		Description: "Lists the parameters visible at an NRN: those defined at it and at its ancestors. The listing describes the parameters and never their values — the provider asks the API to leave values out, so secret values are never read.",

		ReadContext: dataSourceParametersRead,
		Schema: map[string]*schema.Schema{
			"nrn": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The NRN to list parameters at, e.g. `organization=1:account=2:namespace=3:application=4`.",
			},
			"parameters": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "The parameters visible at the NRN, without their values.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id":        {Type: schema.TypeInt, Computed: true, Description: "A system-wide unique ID for the parameter."},
						"name":      {Type: schema.TypeString, Computed: true, Description: "The name of the parameter."},
						"nrn":       {Type: schema.TypeString, Computed: true, Description: "The NRN the parameter is defined at."},
						"type":      {Type: schema.TypeString, Computed: true, Description: "The type of the parameter: `environment` or `file`."},
						"encoding":  {Type: schema.TypeString, Computed: true, Description: "The encoding of the parameter's values, e.g. `plaintext` or `base64`."},
						"variable":  {Type: schema.TypeString, Computed: true, Description: "The environment variable name of an `environment` parameter; empty for `file` parameters."},
						"secret":    {Type: schema.TypeBool, Computed: true, Description: "Whether the parameter's values are secret."},
						"read_only": {Type: schema.TypeBool, Computed: true, Description: "Whether the parameter is read-only."},
					},
				},
			},
		},
	}
}

func dataSourceParametersRead(_ context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)
	nrn := d.Get("nrn").(string)

	parameters, err := nullOps.ListParameters(nrn)
	if err != nil {
		return diag.FromErr(err)
	}

	rows := make([]map[string]any, 0, len(parameters))
	for _, parameter := range parameters {
		rows = append(rows, map[string]any{
			"id":        parameter.Id,
			"name":      parameter.Name,
			"nrn":       parameter.Nrn,
			"type":      parameter.Type,
			"encoding":  parameter.Encoding,
			"variable":  parameter.Variable,
			"secret":    parameter.Secret,
			"read_only": parameter.ReadOnly,
		})
	}

	if err := d.Set("parameters", rows); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(nrn)

	return nil
}
