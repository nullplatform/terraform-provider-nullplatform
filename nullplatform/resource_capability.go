package nullplatform

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceCapability() *schema.Resource {
	return &schema.Resource{
		Description: "The capability resource allows you to manage nullplatform Capabilities",

		CreateContext: CapabilityCreate,
		ReadContext:   CapabilityRead,
		UpdateContext: CapabilityUpdate,
		DeleteContext: CapabilityDelete,

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
				Description: "The name of the capability. Maximum length is 60 characters.",
			},
			"slug": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "A unique slug for the capability. Generated from the name if not provided.",
			},
			"description": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "A description of the capability. Maximum length is 2048 characters.",
			},
			"target": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The target of the capability.",
			},
			"definition": {
				Type:             schema.TypeString,
				Required:         true,
				Description:      "JSON string containing the capability definition (a JSON schema).",
				DiffSuppressFunc: suppressEquivalentJSON,
			},
			"status": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "The status of the capability.",
			},
			"created_at": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The creation timestamp of the capability.",
			},
			"updated_at": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The last update timestamp of the capability.",
			},
		},
	}
}

func CapabilityCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)

	definitionStr := d.Get("definition").(string)
	var definition map[string]interface{}
	if err := json.Unmarshal([]byte(definitionStr), &definition); err != nil {
		return diag.FromErr(fmt.Errorf("error parsing definition JSON: %v", err))
	}

	newCapability := &CapabilityEntity{
		Name:        d.Get("name").(string),
		Slug:        d.Get("slug").(string),
		Description: d.Get("description").(string),
		Target:      d.Get("target").(string),
		Definition:  definition,
		Status:      d.Get("status").(string),
	}

	capability, err := nullOps.CreateCapability(newCapability)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(strconv.Itoa(capability.Id))
	return CapabilityRead(ctx, d, m)
}

func CapabilityRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)
	capabilityId := d.Id()

	capability, err := nullOps.GetCapability(capabilityId)
	if err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("name", capability.Name); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("slug", capability.Slug); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("description", capability.Description); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("target", capability.Target); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("status", capability.Status); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("created_at", capability.CreatedAt); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("updated_at", capability.UpdatedAt); err != nil {
		return diag.FromErr(err)
	}

	if capability.Definition != nil {
		definitionJSON, err := json.Marshal(capability.Definition)
		if err != nil {
			return diag.FromErr(fmt.Errorf("error serializing definition to JSON: %v", err))
		}
		if err := d.Set("definition", string(definitionJSON)); err != nil {
			return diag.FromErr(err)
		}
	}

	return nil
}

func CapabilityUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)
	capabilityId := d.Id()

	capability := &CapabilityEntity{}

	if d.HasChange("name") {
		capability.Name = d.Get("name").(string)
	}
	if d.HasChange("description") {
		capability.Description = d.Get("description").(string)
	}
	if d.HasChange("target") {
		capability.Target = d.Get("target").(string)
	}
	if d.HasChange("status") {
		capability.Status = d.Get("status").(string)
	}
	if d.HasChange("definition") {
		definitionStr := d.Get("definition").(string)
		var definition map[string]interface{}
		if err := json.Unmarshal([]byte(definitionStr), &definition); err != nil {
			return diag.FromErr(fmt.Errorf("error parsing definition JSON: %v", err))
		}
		capability.Definition = definition
	}

	if err := nullOps.PatchCapability(capabilityId, capability); err != nil {
		return diag.FromErr(err)
	}

	return CapabilityRead(ctx, d, m)
}

func CapabilityDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)
	capabilityId := d.Id()

	if err := nullOps.DeleteCapability(capabilityId); err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
