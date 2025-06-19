package nullplatform

import (
	"log"
	"reflect"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceService() *schema.Resource {
	return &schema.Resource{
		Description: "The service resource allows you to configure a Nullplatform Service",

		Create: ServiceCreate,
		Read:   ServiceRead,
		Update: ServiceUpdate,
		Delete: ServiceDelete,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Name of the entity. Must be a non-empty string and not equal to null.",
			},
			"specification_id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Unique identifier for the entity represented as a UUID.",
			},
			"entity_nrn": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "NRN representing a hierarchical identifier for nullplatform resourcesValue must match regular expression `^organization=[0-9]+(:account=[0-9]+)?(:namespace=[0-9]+)?(:application=[0-9]+)?(:scope=[0-9]+)?$`.",
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
				Optional:    true,
				Description: "Desired unique identifier for the associated specification.",
			},
			"messages": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Schema{
					Type: schema.TypeMap,
					Elem: &schema.Schema{
						Type: schema.TypeString,
					},
				},
				Description: "A message and its severity level",
			},
			"attributes": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "Attributes associated with the service, should be valid against the service specification attribute schema.",
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
			"status": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "active",
				Description: "Status of the service. Should be one of: [`pending_create`, `pending`, `creating`, `updating`, `deleting`, `active`, `deleted`, `failed`]",
			},
		},
	}
}

func ServiceCreate(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)

	name := d.Get("name").(string)
	specificationId := d.Get("specification_id").(string)
	entityNrn := d.Get("entity_nrn").(string)
	linkableTo := d.Get("linkable_to").([]interface{})
	desiredSpecificationId := d.Get("desired_specification_id").(string)
	status := d.Get("status").(string)
	messages := d.Get("messages").([]interface{})
	attributes := d.Get("attributes").(map[string]interface{})
	dimensions := d.Get("dimensions").(map[string]interface{})
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

	newService := &Service{
		Name:                   name,
		SpecificationId:        specificationId,
		DesiredSpecificationId: desiredSpecificationId,
		EntityNrn:              entityNrn,
		LinkableTo:             linkableTo,
		Status:                 status,
		Messages:               messages,
		Selectors:              &selectors,
		Attributes:             attributes,
		Dimensions:             dimensions,
	}

	s, err := nullOps.CreateService(newService)

	if err != nil {
		return err
	}

	d.SetId(s.Id)

	return nil
}

func ServiceRead(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)
	serviceID := d.Id()

	s, err := nullOps.GetService(serviceID)

	if err != nil {
		d.SetId("")
		return err
	}

	if err := d.Set("name", s.Name); err != nil {
		return err
	}

	if err := d.Set("specification_id", s.SpecificationId); err != nil {
		return err
	}

	if err := d.Set("desired_specification_id", s.DesiredSpecificationId); err != nil {
		return err
	}

	if err := d.Set("entity_nrn", s.EntityNrn); err != nil {
		return err
	}

	if err := d.Set("linkable_to", s.LinkableTo); err != nil {
		return err
	}

	if err := d.Set("status", s.Status); err != nil {
		return err
	}

	if err := d.Set("dimensions", s.Dimensions); err != nil {
		return err
	}

	if err := d.Set("messages", s.Messages); err != nil {
		return err
	}

	selectors := []map[string]interface{}{
		{
			"category":     s.Selectors.Category,
			"imported":     s.Selectors.Imported,
			"provider":     s.Selectors.Provider,
			"sub_category": s.Selectors.SubCategory,
		},
	}
	if err := d.Set("selectors", selectors); err != nil {
		return err
	}

	if err := d.Set("attributes", s.Attributes); err != nil {
		return err
	}

	return nil
}

func ServiceUpdate(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)

	serviceID := d.Id()

	log.Println("serviceID:", serviceID)

	ps := &Service{}

	if d.HasChange("name") {
		ps.Name = d.Get("name").(string)
	}

	if d.HasChange("status") {
		ps.Status = d.Get("status").(string)
	}

	if d.HasChange("specification_id") {
		ps.SpecificationId = d.Get("specification_id").(string)
	}

	if d.HasChange("entity_nrn") {
		ps.EntityNrn = d.Get("entity_nrn").(string)
	}

	if d.HasChange("linkable_to") {
		ps.LinkableTo = d.Get("linkable_to").([]interface{})
	}

	if d.HasChange("dimensions") {
		dimensions := d.Get("dimensions").(map[string]interface{})

		ps.Dimensions = dimensions
	}

	if d.HasChange("attributes") {
		attributes := d.Get("attributes").(map[string]interface{})

		ps.Attributes = attributes
	}

	if d.HasChange("selectors") {
		selectorsList := d.Get("selectors").([]interface{})
		if len(selectorsList) > 0 {
			selectorsMap := selectorsList[0].(map[string]interface{})
			ps.Selectors = &Selectors{
				Category:    selectorsMap["category"].(string),
				Imported:    selectorsMap["imported"].(bool),
				Provider:    selectorsMap["provider"].(string),
				SubCategory: selectorsMap["sub_category"].(string),
			}
		}
	}

	if !reflect.DeepEqual(*ps, Service{}) {
		err := nullOps.PatchService(serviceID, ps)
		if err != nil {
			return err
		}
	}

	return nil
}

func ServiceDelete(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)

	serviceID := d.Id()

	err := nullOps.DeleteService(serviceID)
	if err != nil {
		return err
	}

	d.SetId("")

	return nil
}
