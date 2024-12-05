package nullplatform

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceMetadataSpecification() *schema.Resource {
	return &schema.Resource{
		Description: "The metadata_specification resource allows you to manage entity metadata specifications",

		Create: MetadataSpecificationCreate,
		Read:   MetadataSpecificationRead,
		Update: MetadataSpecificationUpdate,
		Delete: MetadataSpecificationDelete,

		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				d.Set("id", d.Id())
				return []*schema.ResourceData{d}, nil
			},
		},

		Schema: AddNRNSchema(map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Metadata specification name",
			},
			"description": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Metadata specification description",
			},
			"entity": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Parent entity that holds metadata information",
			},
			"metadata": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Entity metadata key",
			},
			"schema": {
				Type:             schema.TypeString,
				Required:         true,
				Description:      "JSON schema definition for the metadata specification",
				DiffSuppressFunc: suppressEquivalentJSON,
			},
		}),
	}
}

func MetadataSpecificationCreate(d *schema.ResourceData, m interface{}) error {
	nullOps := m.(NullOps)

	nrn, err := ConstructNRNFromComponents(d, nullOps)
	if err != nil {
		return fmt.Errorf("error constructing NRN: %v", err)
	}

	schemaJSON := d.Get("schema").(string)
	var schemaMap map[string]interface{}
	if err := json.Unmarshal([]byte(schemaJSON), &schemaMap); err != nil {
		return fmt.Errorf("error parsing schema JSON: %v", err)
	}

	spec := &MetadataSpecification{
		Name:        d.Get("name").(string),
		Description: d.Get("description").(string),
		Nrn:         nrn,
		Entity:      d.Get("entity").(string),
		Metadata:    d.Get("metadata").(string),
		Schema:      schemaMap,
	}

	created, err := nullOps.CreateMetadataSpecification(spec)
	if err != nil {
		return err
	}

	d.SetId(created.Id)
	return MetadataSpecificationRead(d, m)
}

func MetadataSpecificationRead(d *schema.ResourceData, m interface{}) error {
	nullOps := m.(NullOps)

	spec, err := nullOps.GetMetadataSpecification(d.Id())
	if err != nil {
		return err
	}

	if err := d.Set("name", spec.Name); err != nil {
		return err
	}
	if err := d.Set("description", spec.Description); err != nil {
		return err
	}
	if err := d.Set("nrn", spec.Nrn); err != nil {
		return err
	}
	if err := d.Set("entity", spec.Entity); err != nil {
		return err
	}
	if err := d.Set("metadata", spec.Metadata); err != nil {
		return err
	}

	schemaJSON, err := json.Marshal(spec.Schema)
	if err != nil {
		return fmt.Errorf("error serializing schema to JSON: %v", err)
	}
	if err := d.Set("schema", string(schemaJSON)); err != nil {
		return err
	}

	return nil
}

func MetadataSpecificationUpdate(d *schema.ResourceData, m interface{}) error {
	nullOps := m.(NullOps)

	spec := &MetadataSpecification{
		Name:        d.Get("name").(string),
		Description: d.Get("description").(string),
	}

	if d.HasChange("schema") {
		schemaJSON := d.Get("schema").(string)
		var schemaMap map[string]interface{}
		if err := json.Unmarshal([]byte(schemaJSON), &schemaMap); err != nil {
			return fmt.Errorf("error parsing schema JSON: %v", err)
		}
		spec.Schema = schemaMap
	}

	_, err := nullOps.UpdateMetadataSpecification(d.Id(), spec)
	if err != nil {
		return err
	}

	return MetadataSpecificationRead(d, m)
}

func MetadataSpecificationDelete(d *schema.ResourceData, m interface{}) error {
	nullOps := m.(NullOps)

	err := nullOps.DeleteMetadataSpecification(d.Id())
	if err != nil {
		return err
	}

	d.SetId("")
	return nil
}
