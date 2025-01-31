package nullplatform

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceMetadata() *schema.Resource {
	return &schema.Resource{
		Description: "The metadata resource allows you to manage metadata for nullplatform entities",

		CreateContext: MetadataCreate,
		ReadContext:   MetadataRead,
		UpdateContext: MetadataUpdate,
		DeleteContext: MetadataDelete,

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
			"type": {
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
	parts := strings.Split(strings.TrimSpace(id), "/")
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("invalid ID format: %s (expected format: entity/entityId/type)", id)
	}

	entity := strings.TrimSpace(parts[0])
	entityId := strings.TrimSpace(parts[1])
	metadataType := strings.TrimSpace(parts[2])

	if entity == "" || entityId == "" || metadataType == "" {
		return "", "", "", fmt.Errorf("invalid ID format: %s (entity, entityId, and type cannot be empty)", id)
	}

	return entity, entityId, metadataType, nil
}
func MetadataCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)

	entity := d.Get("entity").(string)
	entityId := d.Get("entity_id").(string)
	metadataType := d.Get("type").(string)

	valueStr := d.Get("value").(string)
	var value interface{}
	if err := json.Unmarshal([]byte(valueStr), &value); err != nil {
		return diag.FromErr(fmt.Errorf("error parsing metadata value JSON: %v", err))
	}

	metadata := &Metadata{
		Value: value,
	}

	err := nullOps.CreateMetadata(entity, entityId, metadataType, metadata)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(buildMetadataId(entity, entityId, metadataType))
	return MetadataRead(ctx, d, m)
}

func MetadataRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)

	entity, entityId, metadataType, err := parseMetadataId(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	metadata, err := nullOps.GetMetadata(entity, entityId, metadataType)
	if err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("entity", entity); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("entity_id", entityId); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("type", metadataType); err != nil {
		return diag.FromErr(err)
	}

	valueJSON, err := json.Marshal(metadata.Value)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error serializing metadata value to JSON: %v", err))
	}
	if err := d.Set("value", string(valueJSON)); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func MetadataUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)

	entity, entityId, metadataType, err := parseMetadataId(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	if d.HasChange("value") {
		valueStr := d.Get("value").(string)
		var value interface{}
		if err := json.Unmarshal([]byte(valueStr), &value); err != nil {
			return diag.FromErr(fmt.Errorf("error parsing metadata value JSON: %v", err))
		}

		metadata := &Metadata{
			Value: value,
		}

		err := nullOps.UpdateMetadata(entity, entityId, metadataType, metadata)
		if err != nil {
			return diag.FromErr(err)
		}
	}

	return MetadataRead(ctx, d, m)
}

func MetadataDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)

	entity, entityId, metadataType, err := parseMetadataId(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	err = nullOps.DeleteMetadata(entity, entityId, metadataType)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
