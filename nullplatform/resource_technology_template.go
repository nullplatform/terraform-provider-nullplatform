package nullplatform

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceTechnologyTemplate() *schema.Resource {
	return &schema.Resource{
		Description: "The technology_template resource allows you to manage nullplatform Technology Templates",

		CreateContext: TechnologyTemplateCreate,
		ReadContext:   TechnologyTemplateRead,
		UpdateContext: TechnologyTemplateUpdate,
		DeleteContext: TechnologyTemplateDelete,

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
				Description: "Name of the technology template",
			},
			"url": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "URL of the template repository",
			},
			"account": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Account ID the template belongs to. If not specified, it will be a global template",
			},
			"provider_config": {
				Type:     schema.TypeMap,
				Required: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "Provider configuration for the template",
			},
			"components": {
				Type:     schema.TypeList,
				Required: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"type": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Type of the component (e.g., language, framework)",
						},
						"id": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Identifier of the component",
						},
						"version": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Version of the component",
						},
						"metadata": {
							Type:             schema.TypeString,
							Optional:         true,
							Default:          "{}",
							Description:      "JSON string containing component metadata",
							DiffSuppressFunc: suppressEquivalentJSON,
						},
					},
				},
				Description: "List of components that make up the template",
			},
			"tags": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "List of tags associated with the template",
			},
			"metadata": {
				Type:             schema.TypeString,
				Optional:         true,
				Default:          "{}",
				Description:      "JSON string containing template metadata",
				DiffSuppressFunc: suppressEquivalentJSON,
			},
			"rules": {
				Type:             schema.TypeString,
				Optional:         true,
				Default:          "{}",
				Description:      "JSON string containing template rules",
				DiffSuppressFunc: suppressEquivalentJSON,
			},
		},
	}
}

func TechnologyTemplateCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)

	client := nullOps.(*NullClient)
	organizationID, err := client.GetOrganizationIDFromToken()
	if err != nil {
		return diag.FromErr(fmt.Errorf("error getting organization ID from token: %v", err))
	}

	metadataStr := d.Get("metadata").(string)
	var metadata map[string]interface{}
	if err := json.Unmarshal([]byte(metadataStr), &metadata); err != nil {
		return diag.FromErr(fmt.Errorf("error parsing metadata JSON: %v", err))
	}

	rulesStr := d.Get("rules").(string)
	var rules map[string]interface{}
	if err := json.Unmarshal([]byte(rulesStr), &rules); err != nil {
		return diag.FromErr(fmt.Errorf("error parsing rules JSON: %v", err))
	}

	componentsRaw := d.Get("components").([]interface{})
	components := make([]map[string]interface{}, len(componentsRaw))
	for i, c := range componentsRaw {
		comp := c.(map[string]interface{})
		componentMetadata := map[string]interface{}{}
		if metadataStr, ok := comp["metadata"].(string); ok && metadataStr != "" {
			if err := json.Unmarshal([]byte(metadataStr), &componentMetadata); err != nil {
				return diag.FromErr(fmt.Errorf("error parsing component metadata JSON: %v", err))
			}
		}

		components[i] = map[string]interface{}{
			"type":     comp["type"],
			"id":       comp["id"],
			"version":  comp["version"],
			"metadata": componentMetadata,
		}
	}

	tagsRaw := d.Get("tags").([]interface{})
	tags := make([]string, len(tagsRaw))
	for i, t := range tagsRaw {
		tags[i] = t.(string)
	}

	template := &TechnologyTemplate{
		Name:         d.Get("name").(string),
		Organization: json.Number(organizationID),
		Account:      json.Number(d.Get("account").(string)),
		URL:          d.Get("url").(string),
		Provider:     d.Get("provider_config").(map[string]interface{}),
		Status:       "active",
		Components:   components,
		Tags:         tags,
		Metadata:     metadata,
		Rules:        rules,
	}

	newTemplate, err := nullOps.CreateTechnologyTemplate(template)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(newTemplate.GetId())

	return TechnologyTemplateRead(ctx, d, m)
}

func TechnologyTemplateRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)
	templateId := d.Id()

	template, err := nullOps.GetTechnologyTemplate(templateId)
	if err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("name", template.Name); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("url", template.URL); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("account", template.GetAccount()); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("provider_config", template.Provider); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("tags", template.Tags); err != nil {
		return diag.FromErr(err)
	}

	components := make([]map[string]interface{}, len(template.Components))
	for i, comp := range template.Components {
		metadata, err := json.Marshal(comp["metadata"])
		if err != nil {
			return diag.FromErr(fmt.Errorf("error serializing component metadata to JSON: %v", err))
		}

		components[i] = map[string]interface{}{
			"type":     comp["type"],
			"id":       comp["id"],
			"version":  comp["version"],
			"metadata": string(metadata),
		}
	}
	if err := d.Set("components", components); err != nil {
		return diag.FromErr(err)
	}

	metadataJSON, err := json.Marshal(template.Metadata)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error serializing metadata to JSON: %v", err))
	}
	if err := d.Set("metadata", string(metadataJSON)); err != nil {
		return diag.FromErr(err)
	}

	rulesJSON, err := json.Marshal(template.Rules)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error serializing rules to JSON: %v", err))
	}
	if err := d.Set("rules", string(rulesJSON)); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func TechnologyTemplateUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)
	templateId := d.Id()

	template := &TechnologyTemplate{}

	if d.HasChange("name") {
		template.Name = d.Get("name").(string)
	}

	if d.HasChange("url") {
		template.URL = d.Get("url").(string)
	}

	if d.HasChange("provider_config") {
		template.Provider = d.Get("provider_config").(map[string]interface{})
	}

	if d.HasChange("components") {
		componentsRaw := d.Get("components").([]interface{})
		components := make([]map[string]interface{}, len(componentsRaw))
		for i, c := range componentsRaw {
			comp := c.(map[string]interface{})
			componentMetadata := map[string]interface{}{}
			if metadataStr, ok := comp["metadata"].(string); ok && metadataStr != "" {
				if err := json.Unmarshal([]byte(metadataStr), &componentMetadata); err != nil {
					return diag.FromErr(fmt.Errorf("error parsing component metadata JSON: %v", err))
				}
			}

			components[i] = map[string]interface{}{
				"type":     comp["type"],
				"id":       comp["id"],
				"version":  comp["version"],
				"metadata": componentMetadata,
			}
		}
		template.Components = components
	}

	if d.HasChange("tags") {
		tagsRaw := d.Get("tags").([]interface{})
		tags := make([]string, len(tagsRaw))
		for i, t := range tagsRaw {
			tags[i] = t.(string)
		}
		template.Tags = tags
	}

	if d.HasChange("metadata") {
		metadataStr := d.Get("metadata").(string)
		var metadata map[string]interface{}
		if err := json.Unmarshal([]byte(metadataStr), &metadata); err != nil {
			return diag.FromErr(fmt.Errorf("error parsing metadata JSON: %v", err))
		}
		template.Metadata = metadata
	}

	if d.HasChange("rules") {
		rulesStr := d.Get("rules").(string)
		var rules map[string]interface{}
		if err := json.Unmarshal([]byte(rulesStr), &rules); err != nil {
			return diag.FromErr(fmt.Errorf("error parsing rules JSON: %v", err))
		}
		template.Rules = rules
	}

	err := nullOps.PatchTechnologyTemplate(templateId, template)
	if err != nil {
		return diag.FromErr(err)
	}

	return TechnologyTemplateRead(ctx, d, m)
}

func TechnologyTemplateDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)
	templateId := d.Id()

	template := &TechnologyTemplate{
		Status: "inactive",
	}

	err := nullOps.PatchTechnologyTemplate(templateId, template)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
