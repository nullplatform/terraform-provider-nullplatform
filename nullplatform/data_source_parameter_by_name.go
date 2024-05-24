package nullplatform

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceParameterByName() *schema.Resource {
	return &schema.Resource{
		Description: "Provides information about the Parameter by Name and NRN.",
		ReadContext: dataSourceParameterByNameRead,
		Schema: map[string]*schema.Schema{
			"id": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "A system-wide unique ID representing the resource.",
			},
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Definition name of the variable.",
			},
			"nrn": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The NRN of the application to which the parameter belongs to.",
			},
			"type": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Possible values: [`environment`, `file`]",
			},
			"encoding": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Possible values: [`plaintext`, `base64`]",
			},
			"variable": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The name of the environment variable. Required when `type = environment`.",
			},
			"destination_path": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The full path for file. Required when `type = file`.",
			},
			"secret": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "`true` if the value is a secret, `false` otherwise",
			},
			"read_only": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "`true` if the value is a secret, `false` otherwise",
			},
			/*
				"values": {
					Type:     schema.TypeList,
					Computed: true,
					Elem: &schema.Schema{
						Type: schema.TypeInt,
					},
					Description: "List of unique IDs representing the values",
				},
			*/
		},
	}
}

func dataSourceParameterByNameRead(_ context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)

	parameterList, err := nullOps.GetParameterList(d.Get("nrn").(string))
	if err != nil {
		return diag.FromErr(err)
	}

	param := &Parameter{
		Name: d.Get("name").(string),
		Nrn:  d.Get("nrn").(string),
	}

	paramRes, paramExists := parameterExists(parameterList, param)
	if paramExists {
		err = d.Set("name", paramRes.Name)
		if err != nil {
			return diag.FromErr(err)
		}

		err = d.Set("nrn", paramRes.Nrn)
		if err != nil {
			return diag.FromErr(err)
		}

		err = d.Set("type", paramRes.Type)
		if err != nil {
			return diag.FromErr(err)
		}

		err = d.Set("encoding", paramRes.Encoding)
		if err != nil {
			return diag.FromErr(err)
		}

		err = d.Set("variable", paramRes.Variable)
		if err != nil {
			return diag.FromErr(err)
		}

		err = d.Set("destination_path", paramRes.DestinationPath)
		if err != nil {
			return diag.FromErr(err)
		}

		err = d.Set("secret", paramRes.Secret)
		if err != nil {
			return diag.FromErr(err)
		}

		err = d.Set("read_only", paramRes.ReadOnly)
		if err != nil {
			return diag.FromErr(err)
		}

		d.SetId(strconv.Itoa(paramRes.Id))
	}

	return nil
}
