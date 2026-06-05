package nullplatform

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceScopeDomain() *schema.Resource {
	return &schema.Resource{
		Description: "The scope_domain resource allows you to manage custom domains attached to a nullplatform Scope",

		CreateContext: ScopeDomainCreate,
		ReadContext:   ScopeDomainRead,
		UpdateContext: ScopeDomainUpdate,
		DeleteContext: ScopeDomainDelete,

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
				Description: "The domain name.",
			},
			"scope_id": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The ID of the scope this domain belongs to.",
			},
			"type": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The type of the scope domain.",
			},
			"status": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "The status of the scope domain.",
			},
			"selector": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "JSON string with the selector resolved for the scope domain.",
			},
		},
	}
}

func ScopeDomainCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)

	newSD := &ScopeDomain{
		Name:    d.Get("name").(string),
		ScopeId: d.Get("scope_id").(string),
		Type:    d.Get("type").(string),
	}

	sd, err := nullOps.CreateScopeDomain(newSD)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(sd.Id)
	return ScopeDomainRead(ctx, d, m)
}

func ScopeDomainRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)
	sdId := d.Id()

	sd, err := nullOps.GetScopeDomain(sdId)
	if err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("name", sd.Name); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("type", sd.Type); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("status", sd.Status); err != nil {
		return diag.FromErr(err)
	}

	// scope_id is not returned at the top level; recover it from the selector
	// (mainly to support `terraform import`).
	if scopeId, ok := sd.Selector["scopeId"]; ok {
		if err := d.Set("scope_id", fmt.Sprintf("%v", scopeId)); err != nil {
			return diag.FromErr(err)
		}
	}

	if sd.Selector != nil {
		selectorJSON, err := json.Marshal(sd.Selector)
		if err != nil {
			return diag.FromErr(fmt.Errorf("error serializing selector to JSON: %v", err))
		}
		if err := d.Set("selector", string(selectorJSON)); err != nil {
			return diag.FromErr(err)
		}
	}

	return nil
}

func ScopeDomainUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)
	sdId := d.Id()

	sd := &ScopeDomain{}

	if d.HasChange("name") {
		sd.Name = d.Get("name").(string)
	}
	if d.HasChange("status") {
		sd.Status = d.Get("status").(string)
	}

	if err := nullOps.PatchScopeDomain(sdId, sd); err != nil {
		return diag.FromErr(err)
	}

	return ScopeDomainRead(ctx, d, m)
}

func ScopeDomainDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)
	sdId := d.Id()

	if err := nullOps.DeleteScopeDomain(sdId); err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
