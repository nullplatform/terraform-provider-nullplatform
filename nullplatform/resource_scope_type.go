package nullplatform

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourceScopeType() *schema.Resource {
	return &schema.Resource{
		Description: "The scope_type resource allows you to configure a nullplatform Scope Type",

		Create: ScopeTypeCreate,
		Read:   ScopeTypeRead,
		Update: ScopeTypeUpdate,
		Delete: ScopeTypeDelete,

		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				d.Set("id", d.Id())
				return []*schema.ResourceData{d}, nil
			},
		},

		Schema: AddNRNSchema(map[string]*schema.Schema{
			"type": {
				Type:         schema.TypeString,
				Optional:     true,
				Default:      "custom",
				ForceNew:     true,
				ValidateFunc: validation.StringInSlice([]string{"custom"}, false),
				Description:  "The type of scope type.",
			},
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The display name shown to developers to identify the scope type.",
			},
			"status": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Whether this scope type is enabled to be used. Always set to 'active' on creation.",
			},
			"description": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Short description of how the scope type works or what it does.",
			},
			"provider_type": {
				Type:         schema.TypeString,
				Optional:     true,
				Default:      "service",
				ForceNew:     true,
				ValidateFunc: validation.StringInSlice([]string{"service"}, false),
				Description:  "Defines the source entity that implements the scope type.",
			},
			"provider_id": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The ID of the entity that implements the scope type.",
			},
		}),
	}
}

func ScopeTypeCreate(d *schema.ResourceData, m interface{}) error {
	nullOps := m.(NullOps)

	var nrn string
	var err error
	if v, ok := d.GetOk("nrn"); ok {
		nrn = v.(string)
	} else {
		nrn, err = ConstructNRNFromComponents(d, nullOps)
		if err != nil {
			return fmt.Errorf("error constructing NRN: %v %s", err, nrn)
		}
	}

	newScopeType := &ScopeType{
		Nrn:          nrn,
		Type:         d.Get("type").(string),
		Name:         d.Get("name").(string),
		Status:       "active", // Always set to active on creation
		Description:  d.Get("description").(string),
		ProviderType: d.Get("provider_type").(string),
		ProviderId:   d.Get("provider_id").(string),
	}

	st, err := nullOps.CreateScopeType(newScopeType)

	if err != nil {
		return err
	}

	d.SetId(fmt.Sprintf("%d", st.Id))

	return ScopeTypeRead(d, m)
}

func ScopeTypeRead(d *schema.ResourceData, m interface{}) error {
	nullOps := m.(NullOps)
	scopeTypeId := d.Id()

	st, err := nullOps.GetScopeType(scopeTypeId)
	if err != nil {
		return err
	}

	if err := d.Set("nrn", st.Nrn); err != nil {
		return err
	}

	if err := d.Set("type", st.Type); err != nil {
		return err
	}

	if err := d.Set("name", st.Name); err != nil {
		return err
	}

	if err := d.Set("status", st.Status); err != nil {
		return err
	}

	if err := d.Set("description", st.Description); err != nil {
		return err
	}

	if err := d.Set("provider_type", st.ProviderType); err != nil {
		return err
	}

	if err := d.Set("provider_id", st.ProviderId); err != nil {
		return err
	}

	return nil
}

func ScopeTypeUpdate(d *schema.ResourceData, m interface{}) error {
	nullOps := m.(NullOps)
	scopeTypeId := d.Id()

	updateScopeType := &ScopeType{}

	if d.HasChange("name") {
		updateScopeType.Name = d.Get("name").(string)
	}

	if d.HasChange("description") {
		updateScopeType.Description = d.Get("description").(string)
	}

	err := nullOps.PatchScopeType(scopeTypeId, updateScopeType)
	if err != nil {
		return err
	}

	return ScopeTypeRead(d, m)
}

func ScopeTypeDelete(d *schema.ResourceData, m interface{}) error {
	nullOps := m.(NullOps)
	scopeTypeId := d.Id()

	err := nullOps.DeleteScopeType(scopeTypeId)
	if err != nil {
		return err
	}

	d.SetId("")
	return nil
}
