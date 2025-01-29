package nullplatform

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceMetadata() *schema.Resource {
	return &schema.Resource{
		Description: "The metadata resource allows you to manage metadata for nullplatform entities",

		Create: MetadataCreate,
		Read:   MetadataRead,
		Update: MetadataUpdate,
		Delete: MetadataDelete,

		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				d.Set("id", d.Id())
				return []*schema.ResourceData{d}, nil
			},
		},

		Schema: map[string]*schema.Schema{
			"entity": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Entity type that holds the metadata (e.g. application, deployment)",
			},
			"entity_id": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "ID of the entity that holds the metadata",
			},
			"metadata_type": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Type of metadata to manage (e.g. coverage, frameworks, vulnerabilities)",
			},
			"value": {
				Type:             schema.TypeString,
				Required:         true,
				Description:      "JSON string containing the metadata value",
				DiffSuppressFunc: suppressEquivalentJSON,
			},
		},
	}
}

func buildMetadataId(entity, entityId, metadataType string) string {
	return fmt.Sprintf("%s/%s/%s", entity, entityId, metadataType)
}

func parseMetadataId(id string) (string, string, string, error) {
	var entity, entityId, metadataType string
	_, err := fmt.Sscanf(id, "%s/%s/%s", &entity, &entityId, &metadataType)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid ID format: %s", id)
	}
	return entity, entityId, metadataType, nil
}

func MetadataCreate(d *schema.ResourceData, m interface{}) error {
	nullOps := m.(NullOps)

	entity := d.Get("entity").(string)
	entityId := d.Get("entity_id").(string)
	metadataType := d.Get("metadata_type").(string)

	valueStr := d.Get("value").(string)
	var value interface{}
	if err := json.Unmarshal([]byte(valueStr), &value); err != nil {
		return fmt.Errorf("error parsing metadata value JSON: %v", err)
	}

	metadata := &Metadata{
		Value: value,
	}

	err := nullOps.CreateMetadata(entity, entityId, metadataType, metadata)
	if err != nil {
		return err
	}

	d.SetId(buildMetadataId(entity, entityId, metadataType))
	return MetadataRead(d, m)
}

func MetadataRead(d *schema.ResourceData, m interface{}) error {
	nullOps := m.(NullOps)

	entity, entityId, metadataType, err := parseMetadataId(d.Id())
	if err != nil {
		return err
	}

	metadata, err := nullOps.GetMetadata(entity, entityId, metadataType)
	if err != nil {
		return err
	}

	if err := d.Set("entity", entity); err != nil {
		return err
	}
	if err := d.Set("entity_id", entityId); err != nil {
		return err
	}
	if err := d.Set("metadata_type", metadataType); err != nil {
		return err
	}

	valueJSON, err := json.Marshal(metadata.Value)
	if err != nil {
		return fmt.Errorf("error serializing metadata value to JSON: %v", err)
	}
	if err := d.Set("value", string(valueJSON)); err != nil {
		return err
	}

	return nil
}

func MetadataUpdate(d *schema.ResourceData, m interface{}) error {
	nullOps := m.(NullOps)

	entity, entityId, metadataType, err := parseMetadataId(d.Id())
	if err != nil {
		return err
	}

	if d.HasChange("value") {
		valueStr := d.Get("value").(string)
		var value interface{}
		if err := json.Unmarshal([]byte(valueStr), &value); err != nil {
			return fmt.Errorf("error parsing metadata value JSON: %v", err)
		}

		metadata := &Metadata{
			Value: value,
		}

		err := nullOps.UpdateMetadata(entity, entityId, metadataType, metadata)
		if err != nil {
			return err
		}
	}

	return MetadataRead(d, m)
}

func MetadataDelete(d *schema.ResourceData, m interface{}) error {
	nullOps := m.(NullOps)

	entity, entityId, metadataType, err := parseMetadataId(d.Id())
	if err != nil {
		return err
	}

	err = nullOps.DeleteMetadata(entity, entityId, metadataType)
	if err != nil {
		return err
	}

	d.SetId("")
	return nil
}
