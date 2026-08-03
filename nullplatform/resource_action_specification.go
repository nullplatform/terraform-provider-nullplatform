package nullplatform

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourceActionSpecification() *schema.Resource {
	return &schema.Resource{
		Description: "The action_specification resource allows you to manage nullplatform Action Specifications",

		CreateContext: ActionSpecificationCreate,
		ReadContext:   ActionSpecificationRead,
		UpdateContext: ActionSpecificationUpdate,
		DeleteContext: ActionSpecificationDelete,

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
				Description: "Name of the action specification",
			},
			"description": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Description of the action specification",
			},
			"slug": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The computed slug for the action specification",
			},
			"last_snapshot_id": {
				Type:     schema.TypeString,
				Computed: true,
				Description: "Newest snapshot id of this action specification. Pin it as a package BOM " +
					"component's resource_revision_id to freeze this exact version into a package.",
			},
			"type": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				ValidateFunc: validation.StringInSlice([]string{
					"custom", "create", "update", "delete", "archive", "unarchive", "diagnose",
				}, false),
				Description: "Type of the action. Must be one of: custom, create, update, delete, " +
					"archive, unarchive, diagnose. On a specification with `use_default_actions`, " +
					"`archive` and `unarchive` are the opt-in that makes " +
					"`nullplatform_service.status = \"archived\"` run as a managed action instead of " +
					"a direct status flip; their content is platform-generated, so omit " +
					"`parameters` and `results` for them.",
			},
			"service_specification_id": {
				Type:         schema.TypeString,
				Optional:     true,
				ForceNew:     true,
				ExactlyOneOf: []string{"service_specification_id", "link_specification_id"},
				Description:  "ID of the associated service specification",
			},
			"link_specification_id": {
				Type:         schema.TypeString,
				Optional:     true,
				ForceNew:     true,
				ExactlyOneOf: []string{"service_specification_id", "link_specification_id"},
				Description:  "ID of the associated link specification",
			},
			// Optional+Computed rather than Required: an `archive`/`unarchive` action
			// on a `use_default_actions` specification is platform-generated from the
			// specification's attributes schema, and sending either field is refused
			// with a 400. Omitting them lets the generated content land in state
			// without a permanent diff (which could not even be applied — PATCHing a
			// default action is refused too). Configurations that set them keep
			// behaving exactly as before.
			"parameters": {
				Type:             schema.TypeString,
				Optional:         true,
				Computed:         true,
				Description:      "JSON string containing the parameters schema and values. Omit it for `archive`/`unarchive` actions on a specification with `use_default_actions`, whose content is platform-generated.",
				DiffSuppressFunc: suppressEquivalentJSON,
			},
			"results": {
				Type:             schema.TypeString,
				Optional:         true,
				Computed:         true,
				Description:      "JSON string containing the expected results schema. Omit it for `archive`/`unarchive` actions on a specification with `use_default_actions`, whose content is platform-generated.",
				DiffSuppressFunc: suppressEquivalentJSON,
			},
			"retryable": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether the action can be retried if the instance is in a failed state",
			},
			"parallelize": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether multiple instances of this action can be executed in parallel. Only applicable to custom type actions",
			},
			"enabled_when": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Condition that must be met for the action to be enabled",
			},
			"icon": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Icon for the action specification",
			},
			"annotations": {
				Type:             schema.TypeString,
				Optional:         true,
				Description:      "JSON string containing annotations for the action specification",
				DiffSuppressFunc: suppressEquivalentJSON,
			},
			"external": {
				Type:             schema.TypeString,
				Optional:         true,
				Description:      "JSON string with the configuration for resolving external context data via the nullplatform agent",
				DiffSuppressFunc: suppressEquivalentJSON,
			},
			"external_resolution": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "JSON string with the status of the external context resolution when the action specification was read",
			},
		},
	}
}

// decodeOptionalJSONObject parses a JSON object out of an Optional+Computed
// string attribute. An empty string means "not configured" and yields a nil map,
// which the `omitempty` tag keeps off the request body entirely — the difference
// that lets the API generate an archive/unarchive action's content instead of
// refusing a caller-supplied one.
func decodeOptionalJSONObject(raw string) (map[string]interface{}, error) {
	if raw == "" {
		return nil, nil
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return nil, err
	}
	return decoded, nil
}

func ActionSpecificationCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)

	// `parameters` and `results` are Optional+Computed, so an omitted attribute
	// reads back as "". Leave the field off the request entirely in that case:
	// the API generates it (default actions) or defaults it to an empty schema.
	parameters, err := decodeOptionalJSONObject(d.Get("parameters").(string))
	if err != nil {
		return diag.FromErr(fmt.Errorf("error parsing parameters JSON: %v", err))
	}

	results, err := decodeOptionalJSONObject(d.Get("results").(string))
	if err != nil {
		return diag.FromErr(fmt.Errorf("error parsing results JSON: %v", err))
	}

	spec := &ActionSpecification{
		Name:                   d.Get("name").(string),
		Description:            d.Get("description").(string),
		Type:                   d.Get("type").(string),
		Parameters:             parameters,
		Results:                results,
		Retryable:              d.Get("retryable").(bool),
		Parallelize:            d.Get("parallelize").(bool),
		EnabledWhen:            d.Get("enabled_when").(string),
		ServiceSpecificationId: d.Get("service_specification_id").(string),
		LinkSpecificationId:    d.Get("link_specification_id").(string),
		Icon:                   d.Get("icon").(string),
	}

	// Handle annotations if provided
	if annotationsStr, ok := d.GetOk("annotations"); ok {
		var annotations map[string]interface{}
		if err := json.Unmarshal([]byte(annotationsStr.(string)), &annotations); err != nil {
			return diag.FromErr(fmt.Errorf("error parsing annotations JSON: %v", err))
		}
		spec.Annotations = annotations
	}

	// Handle external configuration if provided
	if externalStr, ok := d.GetOk("external"); ok {
		var external map[string]interface{}
		if err := json.Unmarshal([]byte(externalStr.(string)), &external); err != nil {
			return diag.FromErr(fmt.Errorf("error parsing external JSON: %v", err))
		}
		spec.External = external
	}

	newSpec, err := nullOps.CreateActionSpecification(spec)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(newSpec.Id)
	return ActionSpecificationRead(ctx, d, m)
}

func ActionSpecificationRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)
	specId := d.Id()

	var parentType, parentId string

	if v := d.Get("service_specification_id").(string); v != "" {
		parentType = "service"
		parentId = v
	} else {
		parentType = "link"
		parentId = d.Get("link_specification_id").(string)
	}

	spec, err := nullOps.GetActionSpecification(specId, parentType, parentId)
	if err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("name", spec.Name); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("description", spec.Description); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("slug", spec.Slug); err != nil {
		return diag.FromErr(err)
	}
	// Best-effort newest snapshot id, for pinning into a package BOM.
	if snapshotID, snapErr := nullOps.GetLatestSnapshotID("action_specification", specId); snapErr == nil {
		if err := d.Set("last_snapshot_id", snapshotID); err != nil {
			return diag.FromErr(err)
		}
	}
	if err := d.Set("type", spec.Type); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("service_specification_id", spec.ServiceSpecificationId); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("link_specification_id", spec.LinkSpecificationId); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("retryable", spec.Retryable); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("parallelize", spec.Parallelize); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("enabled_when", spec.EnabledWhen); err != nil {
		return diag.FromErr(err)
	}

	parametersJSON, err := json.Marshal(spec.Parameters)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error serializing parameters to JSON: %v", err))
	}
	if err := d.Set("parameters", string(parametersJSON)); err != nil {
		return diag.FromErr(err)
	}

	resultsJSON, err := json.Marshal(spec.Results)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error serializing results to JSON: %v", err))
	}
	if err := d.Set("results", string(resultsJSON)); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("icon", spec.Icon); err != nil {
		return diag.FromErr(err)
	}

	if spec.Annotations != nil {
		annotationsJSON, err := json.Marshal(spec.Annotations)
		if err != nil {
			return diag.FromErr(fmt.Errorf("error serializing annotations to JSON: %v", err))
		}
		if err := d.Set("annotations", string(annotationsJSON)); err != nil {
			return diag.FromErr(err)
		}
	}

	if spec.External != nil {
		externalJSON, err := json.Marshal(spec.External)
		if err != nil {
			return diag.FromErr(fmt.Errorf("error serializing external to JSON: %v", err))
		}
		if err := d.Set("external", string(externalJSON)); err != nil {
			return diag.FromErr(err)
		}
	}

	if spec.ExternalResolution != nil {
		externalResolutionJSON, err := json.Marshal(spec.ExternalResolution)
		if err != nil {
			return diag.FromErr(fmt.Errorf("error serializing external_resolution to JSON: %v", err))
		}
		if err := d.Set("external_resolution", string(externalResolutionJSON)); err != nil {
			return diag.FromErr(err)
		}
	}

	return nil
}

func ActionSpecificationUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)
	specId := d.Id()

	// Determine the parent type
	var parentType string
	var parentId string
	if v := d.Get("service_specification_id").(string); v != "" {
		parentType = "service"
		parentId = v
	} else {
		parentType = "link"
		parentId = d.Get("link_specification_id").(string)
	}

	spec := &ActionSpecification{}

	if d.HasChange("name") {
		spec.Name = d.Get("name").(string)
	}

	if d.HasChange("description") {
		spec.Description = d.Get("description").(string)
	}

	if d.HasChange("type") {
		spec.Type = d.Get("type").(string)
	}

	if d.HasChange("parameters") {
		parameters, err := decodeOptionalJSONObject(d.Get("parameters").(string))
		if err != nil {
			return diag.FromErr(fmt.Errorf("error parsing parameters JSON: %v", err))
		}
		spec.Parameters = parameters
	}

	if d.HasChange("results") {
		results, err := decodeOptionalJSONObject(d.Get("results").(string))
		if err != nil {
			return diag.FromErr(fmt.Errorf("error parsing results JSON: %v", err))
		}
		spec.Results = results
	}

	if d.HasChange("retryable") {
		spec.Retryable = d.Get("retryable").(bool)
	}

	if d.HasChange("parallelize") {
		spec.Parallelize = d.Get("parallelize").(bool)
	}

	if d.HasChange("enabled_when") {
		spec.EnabledWhen = d.Get("enabled_when").(string)
	}

	if d.HasChange("icon") {
		spec.Icon = d.Get("icon").(string)
	}

	if d.HasChange("annotations") {
		if annotationsStr, ok := d.GetOk("annotations"); ok {
			var annotations map[string]interface{}
			if err := json.Unmarshal([]byte(annotationsStr.(string)), &annotations); err != nil {
				return diag.FromErr(fmt.Errorf("error parsing annotations JSON: %v", err))
			}
			spec.Annotations = annotations
		} else {
			spec.Annotations = nil
		}
	}

	if d.HasChange("external") {
		if externalStr, ok := d.GetOk("external"); ok {
			var external map[string]interface{}
			if err := json.Unmarshal([]byte(externalStr.(string)), &external); err != nil {
				return diag.FromErr(fmt.Errorf("error parsing external JSON: %v", err))
			}
			spec.External = external
		} else {
			spec.External = nil
		}
	}

	err := nullOps.PatchActionSpecification(specId, spec, parentType, parentId)
	if err != nil {
		return diag.FromErr(err)
	}

	return ActionSpecificationRead(ctx, d, m)
}

func ActionSpecificationDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)
	specId := d.Id()

	// Determine the parent type
	var parentType string
	var parentId string
	if v := d.Get("service_specification_id").(string); v != "" {
		parentType = "service"
		parentId = v
	} else {
		parentType = "link"
		parentId = d.Get("link_specification_id").(string)
	}

	err := nullOps.DeleteActionSpecification(specId, parentType, parentId)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
