package nullplatform

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceServiceSpecification() *schema.Resource {
	return &schema.Resource{
		Description: "Provides information about an existing nullplatform Service Specification",
		ReadContext: dataSourceServiceSpecificationRead,
		Schema: map[string]*schema.Schema{
			"id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The ID of the service specification.",
			},
			"name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Name of the service specification.",
			},
			"slug": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The computed slug for the service specification.",
			},
			"visible_to": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Array representing visibility settings for the service specification.",
			},
			"dimensions": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "JSON string containing dimension configurations.",
			},
			"assignable_to": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Specifies if the service can be assigned to any entity, only dimensions, or only scopes.",
			},
			"type": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Type of the service specification.",
			},
			"attributes": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "JSON string containing the attributes schema and values.",
			},
			"use_default_actions": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Indicates whether the service specification uses default actions.",
			},
			"scopes": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "JSON string containing scope configurations.",
			},
			"selectors": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"category": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"imported": {
							Type:     schema.TypeBool,
							Computed: true,
						},
						"provider": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"sub_category": {
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
				Description: "Selectors for the service specification.",
			},
		},
	}
}

func dataSourceServiceSpecificationRead(_ context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)

	spec, err := nullOps.GetServiceSpecification(d.Get("id").(string))
	if err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("name", spec.Name); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("slug", spec.Slug); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("visible_to", spec.VisibleTo); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("use_default_actions", spec.UseDefaultActions); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("assignable_to", spec.AssignableTo); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("type", spec.Type); err != nil {
		return diag.FromErr(err)
	}

	dimensionsJSON, err := json.Marshal(spec.Dimensions)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error serializing dimensions to JSON: %v", err))
	}
	if err := d.Set("dimensions", string(dimensionsJSON)); err != nil {
		return diag.FromErr(err)
	}

	attributesJSON, err := json.Marshal(spec.Attributes)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error serializing attributes to JSON: %v", err))
	}
	if err := d.Set("attributes", string(attributesJSON)); err != nil {
		return diag.FromErr(err)
	}

	scopesJSON, err := json.Marshal(spec.Scopes)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error serializing scopes to JSON: %v", err))
	}
	if err := d.Set("scopes", string(scopesJSON)); err != nil {
		return diag.FromErr(err)
	}

	if spec.Selectors != nil {
		selectors := []map[string]interface{}{
			{
				"category":     spec.Selectors.Category,
				"imported":     spec.Selectors.Imported,
				"provider":     spec.Selectors.Provider,
				"sub_category": spec.Selectors.SubCategory,
			},
		}
		if err := d.Set("selectors", selectors); err != nil {
			return diag.FromErr(err)
		}
	}

	d.SetId(d.Get("id").(string))

	return nil
}
