package nullplatform

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceProviderSpecification() *schema.Resource {
	return &schema.Resource{
		Description: "The provider_specification resource allows you to manage nullplatform Provider Specifications",

		CreateContext: CreateProviderSpecification,
		ReadContext:   ReadProviderSpecification,
		UpdateContext: UpdateProviderSpecification,
		DeleteContext: DeleteProviderSpecification,

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
				Description: "Name of the provider specification",
			},
			"slug": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The computed slug for the provider specification",
			},
			"icon": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Icon for the provider specification",
			},
			"description": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Description of the provider specification",
			},
			"visible_to": {
				Type:     schema.TypeList,
				Required: true,
				Elem: &schema.Schema{
					Type:     schema.TypeString,
					MinItems: 1,
				},
				Description: "List of NRNs this specification is visible to",
			},
			"spec_schema": {
				Type:             schema.TypeString,
				Required:         true,
				Description:      "JSON Schema for the provider specification. Defines settings for nullplatform integrations",
				DiffSuppressFunc: suppressEquivalentJSON,
			},
			"allow_dimensions": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Flag to allow dimensions for this specification",
			},
			"default_dimensions": {
				Type:             schema.TypeString,
				Optional:         true,
				Default:          "{}",
				Description:      "JSON object with default dimensions for the provider specification",
				DiffSuppressFunc: suppressEquivalentJSON,
			},
			"category": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Category slug to associate with this specification (e.g. logging, metrics, cloud-providers)",
			},
			"categories": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "IDs of associated categories",
			},
			"dependencies": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "IDs of associated dependencies",
			},
			"organization_id": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Organization ID that owns this specification. 0 means global (nullplatform-managed)",
			},
		},
	}
}

func CreateProviderSpecification(_ context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)

	visibleToRaw := d.Get("visible_to").([]interface{})
	visibleTo := make([]string, len(visibleToRaw))
	for i, v := range visibleToRaw {
		visibleTo[i] = v.(string)
	}

	schemaStr := d.Get("spec_schema").(string)
	var specSchema map[string]interface{}
	if err := json.Unmarshal([]byte(schemaStr), &specSchema); err != nil {
		return diag.FromErr(fmt.Errorf("error parsing spec_schema JSON: %v", err))
	}

	defaultDimensionsStr := d.Get("default_dimensions").(string)
	var defaultDimensions map[string]interface{}
	if err := json.Unmarshal([]byte(defaultDimensionsStr), &defaultDimensions); err != nil {
		return diag.FromErr(fmt.Errorf("error parsing default_dimensions JSON: %v", err))
	}

	spec := &ProviderSpecification{
		Name:              d.Get("name").(string),
		VisibleTo:         visibleTo,
		SpecSchema:        specSchema,
		AllowDimensions:   d.Get("allow_dimensions").(bool),
		DefaultDimensions: defaultDimensions,
	}

	if v, ok := d.GetOk("icon"); ok {
		spec.Icon = v.(string)
	}
	if v, ok := d.GetOk("description"); ok {
		spec.Description = v.(string)
	}
	if v, ok := d.GetOk("category"); ok {
		spec.Category = v.(string)
	}

	newSpec, err := nullOps.CreateProviderSpecification(spec)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(newSpec.Id)
	return ReadProviderSpecification(context.Background(), d, m)
}

func ReadProviderSpecification(_ context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)
	specId := d.Id()

	spec, err := nullOps.GetProviderSpecification(specId)
	if err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("name", spec.Name); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("slug", spec.Slug); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("icon", spec.Icon); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("description", spec.Description); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("visible_to", spec.VisibleTo); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("allow_dimensions", spec.AllowDimensions); err != nil {
		return diag.FromErr(err)
	}

	schemaJSON, err := json.Marshal(spec.SpecSchema)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error serializing spec_schema to JSON: %v", err))
	}
	if err := d.Set("spec_schema", string(schemaJSON)); err != nil {
		return diag.FromErr(err)
	}

	defaultDimensions := spec.DefaultDimensions
	if defaultDimensions == nil {
		defaultDimensions = map[string]interface{}{}
	}
	defaultDimensionsJSON, err := json.Marshal(defaultDimensions)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error serializing default_dimensions to JSON: %v", err))
	}
	if err := d.Set("default_dimensions", string(defaultDimensionsJSON)); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("categories", spec.Categories); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("dependencies", spec.Dependencies); err != nil {
		return diag.FromErr(err)
	}

	if spec.OrganizationId != nil {
		if err := d.Set("organization_id", *spec.OrganizationId); err != nil {
			return diag.FromErr(err)
		}
	}

	return nil
}

func UpdateProviderSpecification(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)
	specId := d.Id()

	spec := &ProviderSpecification{}

	if d.HasChange("name") {
		spec.Name = d.Get("name").(string)
	}
	if d.HasChange("icon") {
		spec.Icon = d.Get("icon").(string)
	}
	if d.HasChange("description") {
		spec.Description = d.Get("description").(string)
	}
	if d.HasChange("visible_to") {
		visibleToRaw := d.Get("visible_to").([]interface{})
		visibleTo := make([]string, len(visibleToRaw))
		for i, v := range visibleToRaw {
			visibleTo[i] = v.(string)
		}
		spec.VisibleTo = visibleTo
	}
	if d.HasChange("spec_schema") {
		schemaStr := d.Get("spec_schema").(string)
		var specSchema map[string]interface{}
		if err := json.Unmarshal([]byte(schemaStr), &specSchema); err != nil {
			return diag.FromErr(fmt.Errorf("error parsing spec_schema JSON: %v", err))
		}
		spec.SpecSchema = specSchema
	}
	if d.HasChange("allow_dimensions") {
		spec.AllowDimensions = d.Get("allow_dimensions").(bool)
	}
	if d.HasChange("default_dimensions") {
		defaultDimensionsStr := d.Get("default_dimensions").(string)
		var defaultDimensions map[string]interface{}
		if err := json.Unmarshal([]byte(defaultDimensionsStr), &defaultDimensions); err != nil {
			return diag.FromErr(fmt.Errorf("error parsing default_dimensions JSON: %v", err))
		}
		spec.DefaultDimensions = defaultDimensions
	}
	if d.HasChange("category") {
		spec.Category = d.Get("category").(string)
	}

	if err := nullOps.PatchProviderSpecification(specId, spec); err != nil {
		return diag.FromErr(err)
	}

	return ReadProviderSpecification(ctx, d, m)
}

func DeleteProviderSpecification(_ context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)
	specId := d.Id()

	if err := nullOps.DeleteProviderSpecification(specId); err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
