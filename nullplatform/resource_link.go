package nullplatform

import (
	"context"
	"reflect"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceLink() *schema.Resource {
	return &schema.Resource{
		Description: "The link resource allows you to configure a Nullplatform Link",

		Create: LinkCreate,
		Read:   LinkRead,
		Update: LinkUpdate,
		Delete: LinkDelete,

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
				ForceNew:    true,
				Description: "Name of the entity. Must be a non-empty string and not equal to null.",
			},
			"slug": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Slug of the entity. Automatically generated from `name`.",
			},
			"service_id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Unique identifier for the entity represented as a UUID.",
			},
			"specification_id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Unique identifier for the entity represented as a UUID.",
			},
			"entity_nrn": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "NRN representing a hierarchical identifier for nullplatform resources. Value must match regular expression `^organization=[0-9]+(:account=[0-9]+)?(:namespace=[0-9]+)?(:application=[0-9]+)?(:scope=[0-9]+)?$`.",
			},
			"linkable_to": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "A list of NRN representing the visibility settings for the entity. Specifies what/who can see this entity. Value must match regular expression `^organization=[0-9]+(:account=[0-9]+)?(:namespace=[0-9]+)?(:application=[0-9]+)?(:scope=[0-9]+)?$`.",
			},
			"desired_specification_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Desired unique identifier for the associated specification.",
			},
			"attributes": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "Attributes associated with the link, should be valid against the link specification attribute schema.",
			},
			"dimensions": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "Object representing dimensions with key-value pairs.",
			},
			"selectors": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "Key-value object representing instance selectors.",
			},
			"status": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Status of the link. Should be one of: [`pending_create`, `pending`, `creating`, `updating`, `deleting`, `active`, `deleted`, `failed`]",
			},
		},
	}
}

func LinkCreate(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)

	name := d.Get("name").(string)
	specificationId := d.Get("specification_id").(string)
	serviceId := d.Get("service_id").(string)
	entityNrn := d.Get("entity_nrn").(string)
	linkableTo := d.Get("linkable_to").([]interface{})
	status := d.Get("status").(string)
	attributes := d.Get("attributes").(map[string]interface{})
	dimensions := d.Get("dimensions").(map[string]interface{})
	selectors := d.Get("selectors").(map[string]interface{})

	newLink := &Link{
		Name:            name,
		ServiceId:       serviceId,
		SpecificationId: specificationId,
		EntityNrn:       entityNrn,
		LinkableTo:      linkableTo,
		Status:          status,
		Selectors:       selectors,
		Attributes:      attributes,
		Dimensions:      dimensions,
	}

	l, err := nullOps.CreateLink(newLink)

	if err != nil {
		return err
	}

	d.SetId(l.Id)

	return nil
}

func LinkRead(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)
	linkId := d.Id()

	l, err := nullOps.GetLink(linkId)

	if err != nil {
		d.SetId("")
		return err
	}

	if err := d.Set("name", l.Name); err != nil {
		return err
	}

	if err := d.Set("slug", l.Slug); err != nil {
		return err
	}

	if err := d.Set("service_id", l.ServiceId); err != nil {
		return err
	}

	if err := d.Set("specification_id", l.SpecificationId); err != nil {
		return err
	}

	if err := d.Set("desired_specification_id", l.DesiredSpecificationId); err != nil {
		return err
	}

	if err := d.Set("entity_nrn", l.EntityNrn); err != nil {
		return err
	}

	if err := d.Set("linkable_to", l.LinkableTo); err != nil {
		return err
	}

	if err := d.Set("status", l.Status); err != nil {
		return err
	}

	if err := d.Set("dimensions", l.Dimensions); err != nil {
		return err
	}

	if err := d.Set("selectors", l.Selectors); err != nil {
		return err
	}

	if err := d.Set("attributes", l.Attributes); err != nil {
		return err
	}

	return nil
}

func LinkUpdate(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)

	linkId := d.Id()

	l := &Link{}

	if d.HasChange("name") {
		l.Name = d.Get("name").(string)
	}

	if d.HasChange("slug") {
		l.Slug = d.Get("slug").(string)
	}

	if d.HasChange("service_id") {
		l.ServiceId = d.Get("service_id").(string)
	}

	if d.HasChange("status") {
		l.Status = d.Get("status").(string)
	}

	if d.HasChange("specification_id") {
		l.SpecificationId = d.Get("specification_id").(string)
	}

	if d.HasChange("desired_specification_id") {
		l.DesiredSpecificationId = d.Get("desired_specification_id").(string)
	}

	if d.HasChange("entity_nrn") {
		l.EntityNrn = d.Get("entity_nrn").(string)
	}

	if d.HasChange("linkable_to") {
		l.LinkableTo = d.Get("linkable_to").([]interface{})
	}

	if d.HasChange("dimensions") {
		dimensions := d.Get("dimensions").(map[string]interface{})

		l.Dimensions = dimensions
	}

	if d.HasChange("attributes") {
		attributes := d.Get("attributes").(map[string]interface{})

		l.Attributes = attributes
	}

	if d.HasChange("selectors") {
		selectors := d.Get("selectors").(map[string]interface{})

		l.Selectors = selectors
	}

	if !reflect.DeepEqual(*l, Link{}) {
		err := nullOps.PatchLink(linkId, l)
		if err != nil {
			return err
		}
	}

	return nil
}

func LinkDelete(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)

	linkId := d.Id()

	err := nullOps.DeleteLink(linkId)
	if err != nil {
		return err
	}

	d.SetId("")

	return nil
}
