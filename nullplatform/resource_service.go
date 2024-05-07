package nullplatform

import (
	"log"
	"reflect"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceService() *schema.Resource {
	return &schema.Resource{
		Create: ServiceCreate,
		Read:   ServiceRead,
		Update: ServiceUpdate,
		Delete: ServiceDelete,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"specification_id": {
				Type:     schema.TypeString,
				Required: true,
			},
			"entity_nrn": {
				Type:     schema.TypeString,
				Required: true,
			},
			"linkable_to": {
        Type:     schema.TypeList,
        Optional: true,
        Elem: &schema.Schema{
          Type: schema.TypeString,
        },
      },
			"desired_specification_id": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"messages": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"attributes": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"dimensions": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"selectors": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"status": {
				Type:     schema.TypeString,
				Optional: true,
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
	messagesMap := d.Get("messages").(map[string]interface{})
	attributesMap := d.Get("attributes").(map[string]interface{})
	dimensionsMap := d.Get("dimensions").(map[string]interface{})
	selectorsMap := d.Get("selectors").(map[string]interface{})


	dimensions := make(map[string]string)
	for key, value := range dimensionsMap {
		dimensions[key] = value.(string)
	}

	attributes := make(map[string]string)
	for key, value := range attributesMap {
		attributes[key] = value.(string)
	}

	selectors := make(map[string]string)
	for key, value := range selectorsMap {
		selectors[key] = value.(string)
	}

	messages := make(map[string]string)
	for key, value := range messagesMap {
		messages[key] = value.(string)
	}

	newService := &Service{
		Name:                    name,
		SpecificationId:         specificationId,
		DesiredSpecificationId:  desiredSpecificationId,
		EntityNrn:               entityNrn,
		LinkableTo:              linkableTo,
		Status:                  status,
		Messages:                messages,
		Selectors:               selectors,
		Attributes:              attributes,
		Dimensions:              dimensions,
	}

	s, err := nullOps.CreateService(newService)

	if err != nil {
		return err
	}

	d.SetId(strconv.Itoa(s.Id))

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

	if err := d.Set("selectors", s.Selectors); err != nil {
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
		dimensionsMap := d.Get("dimensions").(map[string]interface{})

		dimensions := make(map[string]string)
		for key, value := range dimensionsMap {
			dimensions[key] = value.(string)
		}

		ps.Dimensions = dimensions
	}

	if d.HasChange("attributes") {
		attributesMap := d.Get("attributes").(map[string]interface{})


		attributes := make(map[string]string)
		for key, value := range attributesMap {
			attributes[key] = value.(string)
		}

		ps.Attributes = attributes
	}

	if d.HasChange("selectors") {
		selectorsMap := d.Get("selectors").(map[string]interface{})


		selectors := make(map[string]string)
		for key, value := range selectorsMap {
			selectors[key] = value.(string)
		}

		ps.Selectors = selectors
	}

	if d.HasChange("messages") {
		messagesMap := d.Get("messages").(map[string]interface{})


		messages := make(map[string]string)
		for key, value := range messagesMap {
			messages[key] = value.(string)
		}

		ps.Messages = messages
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

	pService := &Service{
		Status: "deleting",
	}

	err := nullOps.PatchService(serviceID, pService)
	if err != nil {
		return err
	}

	pService.Status = "deleted"

	err = nullOps.PatchService(serviceID, pService)
	if err != nil {
		return err
	}

	d.SetId("")

	return nil
}
