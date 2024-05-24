package nullplatform

import (
	"context"
	"log"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceScope() *schema.Resource {
	return &schema.Resource{
		Description: "Provides information about the Scope",
		ReadContext: dataSourceScopeRead,
		Schema: map[string]*schema.Schema{
			"id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "A system-wide unique ID for the Scope.",
			},
			"nrn": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "A system-wide unique ID representing the resource.",
			},
			"name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The Scope name.",
			},
			"dimensions": {
				Type:     schema.TypeMap,
				Computed: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "A key-value map with the runtime configuration dimensions that apply to this scope.",
			},
			"runtime_configurations": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Schema{
					Type: schema.TypeInt,
				},
				Description: "List of the runtime configurations that apply to this scope based on its dimensions and values.",
			},
		},
	}
}

func dataSourceScopeRead(_ context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)

	log.Print("\n\n--- Terraform 'read data source Scope' operation begin ---\n\n")

	s, err := nullOps.GetScope(d.Get("id").(string))

	if err != nil {
		return diag.FromErr(err)
	}

	err = d.Set("name", s.Name)

	if err != nil {
		return diag.FromErr(err)
	}

	err = d.Set("nrn", s.Nrn)

	if err != nil {
		return diag.FromErr(err)
	}

	err = d.Set("dimensions", s.Dimensions)

	if err != nil {
		return diag.FromErr(err)
	}

	err = d.Set("runtime_configurations", s.RuntimeConfigurations)

	if err != nil {
		return diag.FromErr(err)
	}

	//fmt.Printf("ResourceData: %+v\n", d)

	// We don't have a unique ID for this data resource so we create one using a
	// timestamp format. I've seen people use a hash of the returned API data as
	// a unique key.
	//
	// NOTE:
	// That hashcode helper is no longer available! It has been moved into an
	// internal directory meaning it's not supposed to be consumed.
	//
	// Reference:
	// https://github.com/hashicorp/terraform-plugin-sdk/blob/master/internal/helper/hashcode/hashcode.go
	//
	d.SetId(strconv.FormatInt(time.Now().Unix(), 10))

	log.Print("\n\n--- Terraform 'read data source Scope' operation ends ---\n\n")

	return nil
}
