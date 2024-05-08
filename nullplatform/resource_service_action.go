package nullplatform

import (
	"log"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceActionService() *schema.Resource {
	return &schema.Resource{
		Create: ActionServiceCreate,
		Read:   ActionServiceRead,
		Update: ActionServiceUpdate,
		Delete: ActionServiceDelete,

		Schema: map[string]*schema.Schema{
			"service_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"specification_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"parameters": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
		},
	}
}

func ActionServiceCreate(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)

	serviceId := d.Get("service_id").(string)
	name := d.Get("name").(string)
	specificationId := d.Get("specification_id").(string)
	parameters := d.Get("parameters").(map[string]interface{})

	newAction := &ActionService{
		Name:            name,
		SpecificationId: specificationId,
		Parameters:      parameters,
	}

	s, err := nullOps.CreateServiceAction(newAction, serviceId, "create")

	if err != nil {
		return err
	}

	d.SetId(s.Id)

	return nil
}


func ActionServiceRead(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)
	actionID := d.Id()
	serviceID := d.Get("service_id").(string)

	as, err := nullOps.GetServiceAction(actionID, serviceID)

	if err != nil {
		d.SetId("")
		return err
	}

	if err := d.Set("name", as.Name); err != nil {
		return err
	}

	if err := d.Set("specification_id", as.SpecificationId); err != nil {
		return err
	}

	if err := d.Set("status", as.Status); err != nil {
		return err
	}

	if err := d.Set("parameters", as.Parameters); err != nil {
		return err
	}

	if err := d.Set("results", as.Results); err != nil {
		return err
	}

	return nil
}

func ActionServiceUpdate(d *schema.ResourceData, m any) error {
	actionId := d.Id()

	log.Println("Action ID:", actionId)

	as := &ActionService{}

	if d.HasChange("name") {
		as.Name = d.Get("name").(string)
	}

	if d.HasChange("status") {
		as.Status = d.Get("status").(string)
	}

	if d.HasChange("specification_id") {
		as.SpecificationId = d.Get("specification_id").(string)
	}

	if d.HasChange("parameters") {
		parameters := d.Get("parameters").(map[string]interface{})

		as.Parameters = parameters
	}

	if d.HasChange("results") {
		results := d.Get("results").(map[string]interface{})

		as.Results = results
	}

	return nil
}

func ActionServiceDelete(d *schema.ResourceData, m any) error {
	d.SetId("")
	return nil
}
