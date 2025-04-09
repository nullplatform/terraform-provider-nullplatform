package nullplatform

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceLinkSpecification() *schema.Resource {
	return &schema.Resource{
		Description: "The link_specification resource allows you to manage nullplatform Link Specifications",

		CreateContext: CreateLinkSpecification,
		ReadContext:   ReadLinkSpecification,
		UpdateContext: UpdateLinkSpecification,
		DeleteContext: DeleteLinkSpecification,

		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				d.Set("id", d.Id())
				return []*schema.ResourceData{d}, nil
			},
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the link specification",
			},
			"slug": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The computed slug for the link specification",
			},
			"unique": {
				Type:        schema.TypeBool,
				Required:    true,
				Description: "Indicates whether the service can be linked only once by an instance of this link specification",
			},
			"specification_id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Unique identifier for the associated service specification",
			},
			"visible_to": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Schema{
					Type:     schema.TypeString,
					MinItems: 1,
				},
				Description: "Array representing visibility settings for the link specification",
			},
			"dimensions": {
				Type:             schema.TypeString,
				Optional:         true,
				Default:          "{}",
				Description:      "JSON string containing dimension configurations. Example: {\"environment\": {\"required\": true}}",
				DiffSuppressFunc: suppressEquivalentJSON,
			},
			"assignable_to": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "any",
				ValidateFunc: func(val interface{}, key string) (warns []string, errs []error) {
					v := val.(string)
					valid := map[string]bool{"any": true, "dimension": true, "scope": true}
					if !valid[v] {
						errs = append(errs, fmt.Errorf("%q must be one of [any, dimension, scope], got: %s", key, v))
					}
					return
				},
				Description: "Specifies if the service can be assigned to any entity, only dimensions, or only scopes",
			},
			"attributes": {
				Type:             schema.TypeString,
				Optional:         true,
				Default:          "{}",
				Description:      "JSON string containing the attributes schema and values",
				DiffSuppressFunc: suppressEquivalentJSON,
			},
			"use_default_actions": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Indicates whether to use default actions for the link specification",
			},
			"selectors": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"category": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Category of the service specification",
						},
						"imported": {
							Type:        schema.TypeBool,
							Required:    true,
							Description: "Indicates whether the service is imported",
						},
						"provider": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Provider of the service (e.g., AWS, GCP)",
						},
						"sub_category": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Sub-category of the service",
						},
					},
				},
				Description: "Selectors for the service specification",
			},
		},
	}
}

func CreateLinkSpecification(_ context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)

	var visibleTo []string
	if v, ok := d.GetOk("visible_to"); ok {
		visibleToRaw := v.([]interface{})
		visibleTo = make([]string, len(visibleToRaw))
		for i, v := range visibleToRaw {
			visibleTo[i] = v.(string)
		}
	}

	dimensionsStr := d.Get("dimensions").(string)
	var dimensions map[string]interface{}
	if err := json.Unmarshal([]byte(dimensionsStr), &dimensions); err != nil {
		return diag.FromErr(fmt.Errorf("error parsing dimensions JSON: %v", err))
	}

	attributesStr := d.Get("attributes").(string)
	var attributes map[string]interface{}
	if err := json.Unmarshal([]byte(attributesStr), &attributes); err != nil {
		return diag.FromErr(fmt.Errorf("error parsing attributes JSON: %v", err))
	}

	selectorsList := d.Get("selectors").([]interface{})
	var selectors Selectors
	if len(selectorsList) > 0 {
		selectorsMap := selectorsList[0].(map[string]interface{})
		selectors = Selectors{
			Category:    selectorsMap["category"].(string),
			Imported:    selectorsMap["imported"].(bool),
			Provider:    selectorsMap["provider"].(string),
			SubCategory: selectorsMap["sub_category"].(string),
		}
	}

	spec := &LinkSpecification{
		Name:              d.Get("name").(string),
		Unique:            d.Get("unique").(bool),
		SpecificationId:   d.Get("specification_id").(string),
		VisibleTo:         visibleTo,
		Dimensions:        dimensions,
		AssignableTo:      d.Get("assignable_to").(string),
		Attributes:        attributes,
		Selectors:         &selectors,
		UseDefaultActions: d.Get("use_default_actions").(bool),
	}

	newSpec, err := nullOps.CreateLinkSpecification(spec)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(newSpec.Id)
	return ReadLinkSpecification(context.Background(), d, m)
}

func ReadLinkSpecification(_ context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)
	specId := d.Id()

	spec, err := nullOps.GetLinkSpecification(specId)
	if err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("name", spec.Name); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("slug", spec.Slug); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("unique", spec.Unique); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("specification_id", spec.SpecificationId); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("visible_to", spec.VisibleTo); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("use_default_actions", spec.UseDefaultActions); err != nil {
		return diag.FromErr(err)
	}

	dimensionsJSON, err := json.Marshal(spec.Dimensions)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error serializing dimensions to JSON: %v", err))
	}
	if err := d.Set("dimensions", string(dimensionsJSON)); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("assignable_to", spec.AssignableTo); err != nil {
		return diag.FromErr(err)
	}

	attributesJSON, err := json.Marshal(spec.Attributes)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error serializing attributes to JSON: %v", err))
	}
	if err := d.Set("attributes", string(attributesJSON)); err != nil {
		return diag.FromErr(err)
	}

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

	return nil
}

func UpdateLinkSpecification(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)
	specId := d.Id()

	spec := &LinkSpecification{}

	if d.HasChange("name") {
		spec.Name = d.Get("name").(string)
	}

	spec.Unique = d.Get("unique").(bool)

	if d.HasChange("visible_to") {
		if v, ok := d.GetOk("visible_to"); ok {
			visibleToRaw := v.([]interface{})
			visibleTo := make([]string, len(visibleToRaw))
			for i, v := range visibleToRaw {
				visibleTo[i] = v.(string)
			}
			spec.VisibleTo = visibleTo
		}
	}

	if d.HasChange("use_default_actions") {
		spec.UseDefaultActions = d.Get("use_default_actions").(bool)
	}

	if d.HasChange("dimensions") {
		dimensionsStr := d.Get("dimensions").(string)
		var dimensions map[string]interface{}
		if err := json.Unmarshal([]byte(dimensionsStr), &dimensions); err != nil {
			return diag.FromErr(fmt.Errorf("error parsing dimensions JSON: %v", err))
		}
		spec.Dimensions = dimensions
	}

	if d.HasChange("assignable_to") {
		spec.AssignableTo = d.Get("assignable_to").(string)
	}

	if d.HasChange("attributes") {
		attributesStr := d.Get("attributes").(string)
		var attributes map[string]interface{}
		if err := json.Unmarshal([]byte(attributesStr), &attributes); err != nil {
			return diag.FromErr(fmt.Errorf("error parsing attributes JSON: %v", err))
		}
		spec.Attributes = attributes
	}

	if d.HasChange("selectors") {
		selectorsList := d.Get("selectors").([]interface{})
		if len(selectorsList) > 0 {
			selectorsMap := selectorsList[0].(map[string]interface{})
			spec.Selectors = &Selectors{
				Category:    selectorsMap["category"].(string),
				Imported:    selectorsMap["imported"].(bool),
				Provider:    selectorsMap["provider"].(string),
				SubCategory: selectorsMap["sub_category"].(string),
			}
		}
	}

	err := nullOps.PatchLinkSpecification(specId, spec)
	if err != nil {
		return diag.FromErr(err)
	}

	return ReadLinkSpecification(ctx, d, m)
}

func DeleteLinkSpecification(_ context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)
	specId := d.Id()

	err := nullOps.DeleteLinkSpecification(specId)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
