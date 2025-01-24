package nullplatform

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourceApiKey() *schema.Resource {
	return &schema.Resource{
		Description: "The API key resource allows you to configure an API key for the nullplatform API.",

		CreateContext: CreateApiKey,
		ReadContext:   ReadApiKey,
		UpdateContext: UpdateApiKey,
		DeleteContext: DeleteApiKey,

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
				Description: "The name of the API key.",
			},
			"api_key": {
				Type:        schema.TypeString,
				Computed:    true,
				Sensitive:   true,
				Description: "The API key value (only available after creation).",
			},
			"masked_api_key": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The masked version of the API key.",
			},
			"owner_id": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "The ID of the user who owns the API key.",
			},
			"grants": {
				Type:        schema.TypeSet,
				Required:    true,
				Description: "List of grants associated with the API key.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nrn": {
							Type:         schema.TypeString,
							Required:     true,
							Description:  "The NRN for the grant.",
							ValidateFunc: validation.StringIsNotEmpty,
						},
						"role_id": {
							Type:        schema.TypeInt,
							Optional:    true,
							Computed:    true,
							Description: "The ID of the role. (Either role_id or role_slug must be set)",
						},
						"role_slug": {
							Type:        schema.TypeString,
							Optional:    true,
							Computed:    true,
							Description: "The slug of the role. (Either role_id or role_slug must be set)",
						},
					},
				},
			},
			"tags": {
				Type:        schema.TypeSet,
				Optional:    true,
				Description: "List of tags of the API key.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"key": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "The key of the tag.",
						},
						"value": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "The value of the tag.",
						},
					},
				},
			},
			"last_used_at": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Timestamp of the last usage of the API key.",
			},
			"created_at": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Timestamp when the API key was created.",
			},
			"updated_at": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Timestamp when the API key was last updated.",
			},
		},
	}
}

func ReadApiKey(_ context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)

	apiKeyId, err := strconv.ParseInt(d.Id(), 10, 64)
	if err != nil {
		return diag.Errorf("failed to parse API key ID: %v", err)
	}

	apiKey, err := nullOps.GetApiKey(apiKeyId)
	if err != nil {
		return diag.FromErr(err)
	}

	rawContent := map[string]any{
		"name":           apiKey.Name,
		"masked_api_key": apiKey.MaskedApiKey,
		"grants":         convertFromGrants(apiKey.Grants),
		"owner_id":       apiKey.OwnerID,
		"last_used_at":   apiKey.LastUsedAt,
		"created_at":     apiKey.CreatedAt,
		"updated_at":     apiKey.UpdatedAt,
	}

	if apiKey.Tags != nil {
		rawContent["tags"] = convertFromTags(apiKey.Tags)
	}

	for k, v := range rawContent {
		if err := d.Set(k, v); err != nil {
			return diag.FromErr(err)
		}
	}

	return nil
}

func CreateApiKey(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)

	body := CreateApiKeyRequestBody{
		Name:   d.Get("name").(string),
		Grants: convertToGrants(d),
	}

	if tags := convertToTags(d); tags != nil {
		body.Tags = tags
	}

	apiKey, err := nullOps.CreateApiKey(&body)
	if err != nil {
		return diag.FromErr(err)
	}

	apiKeyId := strconv.FormatInt(apiKey.ID, 10)

	d.SetId(apiKeyId)
	d.Set("api_key", apiKey.ApiKeyValue)

	return ReadApiKey(ctx, d, m)
}

func UpdateApiKey(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)

	body := PatchApiKeyRequestBody{}

	if d.HasChange("name") {
		body.Name = d.Get("name").(string)
	}

	if d.HasChange("grants") {
		if grants := convertToGrants(d); grants != nil {
			body.Grants = convertToGrants(d)
		}
	}

	if d.HasChange("tags") {
		if tags := convertToTags(d); tags != nil {
			body.Tags = tags
		}
	}

	apiKeyId, err := strconv.ParseInt(d.Id(), 10, 64)
	if err != nil {
		return diag.Errorf("failed to parse API key ID: %v", err)
	}

	err = nullOps.PatchApiKey(apiKeyId, &body)
	if err != nil {
		return diag.FromErr(err)
	}

	return ReadApiKey(ctx, d, m)
}

func DeleteApiKey(_ context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)

	apiKeyId, err := strconv.ParseInt(d.Id(), 10, 64)
	if err != nil {
		return diag.Errorf("failed to parse API key ID: %v", err)
	}

	err = nullOps.DeleteApiKey(apiKeyId)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")

	return nil
}

func convertFromTags(tags []Tag) []map[string]any {
	if tags == nil {
		return nil
	}

	rawTags := make([]map[string]any, len(tags))

	for i, tag := range tags {
		rawTags[i] = map[string]any{
			"key":   tag.Key,
			"value": tag.Value,
		}
	}

	return rawTags
}

func convertToTags(d *schema.ResourceData) []Tag {
	if tagsSet, ok := d.GetOk("tags"); ok {
		tagsList := tagsSet.(*schema.Set).List()
		tags := make([]Tag, len(tagsList))

		for i, t := range tagsList {
			tagMap := t.(map[string]interface{})

			tags[i] = Tag{
				Key:   tagMap["key"].(string),
				Value: tagMap["value"].(string),
			}
		}

		return tags
	}

	return nil
}

func convertFromGrants(grants []ApiKeyGrant) []map[string]any {
	rawGrants := make([]map[string]any, len(grants))

	for i, grant := range grants {
		rawGrants[i] = convertFromGrant(grant)
	}

	return rawGrants
}

func convertFromGrant(grant ApiKeyGrant) map[string]any {
	rawGrant := map[string]any{
		"nrn": grant.NRN,
	}

	if grant.RoleID != nil {
		rawGrant["role_id"] = *grant.RoleID
	}

	if grant.RoleSlug != nil {
		rawGrant["role_slug"] = *grant.RoleSlug
	}

	return rawGrant
}

func convertToGrants(d *schema.ResourceData) []ApiKeyGrant {
	grantsSet := d.Get("grants").(*schema.Set).List()
	grants := make([]ApiKeyGrant, len(grantsSet))

	var preferSlug = true

	for i, g := range grantsSet {
		grantMap := g.(map[string]interface{})
		grants[i] = convertToGrant(grantMap)
		preferSlug = preferSlug && grantMap["role_slug"] != nil
	}

	if preferSlug {
		for i := range grants {
			grants[i].RoleID = nil
		}
	} else {
		for i := range grants {
			grants[i].RoleSlug = nil
		}
	}

	return grants
}

func convertToGrant(grantMap map[string]interface{}) ApiKeyGrant {
	grant := ApiKeyGrant{
		NRN: grantMap["nrn"].(string),
	}

	if roleID, ok := grantMap["role_id"]; ok && roleID != nil {
		roleIDInt64 := int64(roleID.(int))
		grant.RoleID = &roleIDInt64
	}

	if roleSlug, ok := grantMap["role_slug"]; ok && roleSlug != nil {
		roleSlugStr := roleSlug.(string)
		grant.RoleSlug = &roleSlugStr
	}

	return grant
}
