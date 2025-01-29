package nullplatform

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceLinkSpecification() *schema.Resource {
	return &schema.Resource{
		Description: "The link_specification resource allows you to manage nullplatform Link Specifications",

		Create: LinkSpecificationCreate,
		Read:   LinkSpecificationRead,
		Update: LinkSpecificationUpdate,
		Delete: LinkSpecificationDelete,

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
					Type: schema.TypeString,
				},
				Description: "Array representing visibility settings for the link specification",
			},
			"dimensions": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeMap,
					Elem: &schema.Schema{
						Type: schema.TypeString,
					},
				},
				Description: "Object defining required dimensions and their allowed values",
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
			"selectors": {
				Type:             schema.TypeString,
				Optional:         true,
				Default:          "{}",
				Description:      "JSON string containing link specification selectors",
				DiffSuppressFunc: suppressEquivalentJSON,
			},
		},
	}
}

func LinkSpecificationCreate(d *schema.ResourceData, m interface{}) error {
	nullOps := m.(NullOps)

	// Handle visible_to if present
	var visibleTo []string
	if v, ok := d.GetOk("visible_to"); ok {
		visibleToRaw := v.([]interface{})
		visibleTo = make([]string, len(visibleToRaw))
		for i, v := range visibleToRaw {
			visibleTo[i] = v.(string)
		}
	}

	// Parse attributes JSON
	attributesStr := d.Get("attributes").(string)
	var attributes map[string]interface{}
	if err := json.Unmarshal([]byte(attributesStr), &attributes); err != nil {
		return fmt.Errorf("error parsing attributes JSON: %v", err)
	}

	// Parse selectors JSON
	selectorsStr := d.Get("selectors").(string)
	var selectors map[string]interface{}
	if err := json.Unmarshal([]byte(selectorsStr), &selectors); err != nil {
		return fmt.Errorf("error parsing selectors JSON: %v", err)
	}

	// Parse dimensions
	dimensionsRaw := d.Get("dimensions").(map[string]interface{})
	dimensions := make(map[string]interface{})
	for k, v := range dimensionsRaw {
		dimensions[k] = v
	}

	spec := &LinkSpecification{
		Name:            d.Get("name").(string),
		Unique:          d.Get("unique").(bool),
		SpecificationId: d.Get("specification_id").(string),
		VisibleTo:       visibleTo,
		Dimensions:      dimensions,
		AssignableTo:    d.Get("assignable_to").(string),
		Attributes:      attributes,
		Selectors:       selectors,
	}

	newSpec, err := nullOps.CreateLinkSpecification(spec)
	if err != nil {
		return err
	}

	d.SetId(newSpec.Id)
	return LinkSpecificationRead(d, m)
}

func LinkSpecificationRead(d *schema.ResourceData, m interface{}) error {
	nullOps := m.(NullOps)
	specId := d.Id()

	spec, err := nullOps.GetLinkSpecification(specId)
	if err != nil {
		return err
	}

	if err := d.Set("name", spec.Name); err != nil {
		return err
	}
	if err := d.Set("unique", spec.Unique); err != nil {
		return err
	}
	if err := d.Set("specification_id", spec.SpecificationId); err != nil {
		return err
	}
	if err := d.Set("visible_to", spec.VisibleTo); err != nil {
		return err
	}
	if err := d.Set("dimensions", spec.Dimensions); err != nil {
		return err
	}
	if err := d.Set("assignable_to", spec.AssignableTo); err != nil {
		return err
	}

	attributesJSON, err := json.Marshal(spec.Attributes)
	if err != nil {
		return fmt.Errorf("error serializing attributes to JSON: %v", err)
	}
	if err := d.Set("attributes", string(attributesJSON)); err != nil {
		return err
	}

	selectorsJSON, err := json.Marshal(spec.Selectors)
	if err != nil {
		return fmt.Errorf("error serializing selectors to JSON: %v", err)
	}
	if err := d.Set("selectors", string(selectorsJSON)); err != nil {
		return err
	}

	return nil
}

func LinkSpecificationUpdate(d *schema.ResourceData, m interface{}) error {
	nullOps := m.(NullOps)
	specId := d.Id()

	spec := &LinkSpecification{}

	if d.HasChange("name") {
		spec.Name = d.Get("name").(string)
	}

	if d.HasChange("unique") {
		spec.Unique = d.Get("unique").(bool)
	}

	if d.HasChange("visible_to") {
		visibleToRaw := d.Get("visible_to").([]interface{})
		visibleTo := make([]string, len(visibleToRaw))
		for i, v := range visibleToRaw {
			visibleTo[i] = v.(string)
		}
		spec.VisibleTo = visibleTo
	}

	if d.HasChange("dimensions") {
		dimensionsRaw := d.Get("dimensions").(map[string]interface{})
		dimensions := make(map[string]interface{})
		for k, v := range dimensionsRaw {
			dimensions[k] = v
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
			return fmt.Errorf("error parsing attributes JSON: %v", err)
		}
		spec.Attributes = attributes
	}

	if d.HasChange("selectors") {
		selectorsStr := d.Get("selectors").(string)
		var selectors map[string]interface{}
		if err := json.Unmarshal([]byte(selectorsStr), &selectors); err != nil {
			return fmt.Errorf("error parsing selectors JSON: %v", err)
		}
		spec.Selectors = selectors
	}

	err := nullOps.PatchLinkSpecification(specId, spec)
	if err != nil {
		return err
	}

	return LinkSpecificationRead(d, m)
}

func LinkSpecificationDelete(d *schema.ResourceData, m interface{}) error {
	nullOps := m.(NullOps)
	specId := d.Id()

	err := nullOps.DeleteLinkSpecification(specId)
	if err != nil {
		return err
	}

	d.SetId("")
	return nil
}
