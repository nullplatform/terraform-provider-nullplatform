package nullplatform

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourceApplicationType() *schema.Resource {
	return &schema.Resource{
		Description: "The application_type resource manages a nullplatform Application Type: a kind of application an organization defines, " +
			"with the fields its applications carry and the places it can be used.\n\n" +
			"An application type cannot be deleted. `terraform destroy` sets its `status` to `inactive`, which stops new applications " +
			"from taking it, and removes it from the state; the applications that have the type keep it. The type keeps its name, so " +
			"creating another with the same name is refused: bring the existing one back with `terraform import` and set `status = \"active\"`.",

		CreateContext: ApplicationTypeCreate,
		ReadContext:   ApplicationTypeRead,
		UpdateContext: ApplicationTypeUpdate,
		DeleteContext: ApplicationTypeDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		CustomizeDiff: applicationTypeSnapshotDiff,

		Schema: map[string]*schema.Schema{
			"nrn": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.StringIsNotEmpty,
				Description:  "The NRN that owns the application type, for example `organization=1:account=2`. It cannot be changed: a different NRN is a different type, so changing it retires this one and creates another. The retired type keeps its name, so inside the same organization the new one needs a different `name`.",
			},
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringLenBetween(1, 120),
				Description:  "The name of the application type, up to 120 characters. No two types visible in the same organization share a name. Renaming a type does not change its `slug`.",
			},
			"description": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringLenBetween(0, 255),
				Description:  "What the application type is for, up to 255 characters.",
			},
			"status": {
				Type:         schema.TypeString,
				Optional:     true,
				Computed:     true,
				ValidateFunc: validation.StringInSlice([]string{"active", "inactive"}, false),
				Description:  "`active` or `inactive`. Only an active type can be given to a new application; the applications that already have the type keep it either way. The platform creates a type `active` when this is not set.",
			},
			"visible_to": {
				Type:        schema.TypeSet,
				Optional:    true,
				Computed:    true,
				MinItems:    1,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "The NRNs where the application type can be used: an application created at or below one of them may take it. When not set, the platform makes the type visible at its own `nrn`. It cannot be an empty list.",
			},
			"schema": {
				Type:             schema.TypeString,
				Required:         true,
				ValidateFunc:     validateJSONObject,
				DiffSuppressFunc: suppressSameJSONInOrder,
				Description: "The JSON Schema (draft-07, 2019-09 or 2020-12, root `type` `object`) that the `attributes` of an application of this type must pass, as a JSON string. " +
					"The platform shows its properties as a form **in the order they are written**, so that order is kept: the properties are sent in the order written, and reordering them is a change. " +
					"`jsonencode()` writes the keys of every object in alphabetical order; to choose the order, write the schema as JSON with `file()` or a heredoc. " +
					"A change of schema cuts a new snapshot (`last_snapshot_id`); applications stay on the snapshot they are on until they are moved.",
			},
			"slug": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The slug of the application type, made from its name when it is created. It never changes.",
			},
			"last_snapshot_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The ID of the snapshot that holds the current `schema`. New applications are created on it. It changes whenever `schema` does.",
			},
		},
	}
}

// validateJSONObject accepts a string holding one JSON object.
func validateJSONObject(value any, key string) ([]string, []error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal([]byte(value.(string)), &object); err != nil || object == nil {
		return nil, []error{fmt.Errorf("%q must be a JSON object", key)}
	}
	return nil, nil
}

// applicationTypeSnapshotDiff marks last_snapshot_id as unknown when the
// schema changes: the platform cuts a new snapshot for it, and whatever reads
// the id in the same plan must see that it is about to change.
func applicationTypeSnapshotDiff(_ context.Context, d *schema.ResourceDiff, _ any) error {
	if d.Id() == "" {
		return nil
	}
	// HasChange also reports a change of whitespace, which cuts no snapshot.
	if held, planned := d.GetChange("schema"); !sameJSONInOrder(held.(string), planned.(string)) {
		return d.SetNewComputed("last_snapshot_id")
	}
	return nil
}

// setApplicationTypeState writes what the API holds into the state of the
// resource or of the data source. The schema is stored compact, with its keys
// in the order the API answered them.
func setApplicationTypeState(d *schema.ResourceData, applicationType *ApplicationType) error {
	typeSchema, err := compactJSON(applicationType.Schema)
	if err != nil {
		return fmt.Errorf("application type %s has a schema that is not JSON: %w", applicationType.Id, err)
	}

	description := ""
	if applicationType.Description != nil {
		description = *applicationType.Description
	}

	for _, attribute := range []struct {
		key   string
		value any
	}{
		{"nrn", applicationType.Nrn},
		{"name", applicationType.Name},
		{"description", description},
		{"status", applicationType.Status},
		{"visible_to", applicationType.VisibleTo},
		{"schema", typeSchema},
		{"slug", applicationType.Slug},
		{"last_snapshot_id", applicationType.LastSnapshotId},
	} {
		if err := d.Set(attribute.key, attribute.value); err != nil {
			return err
		}
	}
	return nil
}

func stringsOfSet(set *schema.Set) []string {
	values := make([]string, 0, set.Len())
	for _, value := range set.List() {
		values = append(values, value.(string))
	}
	return values
}

func ApplicationTypeCreate(_ context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)

	newType := &ApplicationType{
		Nrn:       d.Get("nrn").(string),
		Name:      d.Get("name").(string),
		Status:    configuredString(d, "status"),
		VisibleTo: stringsOfSet(d.Get("visible_to").(*schema.Set)),
		Schema:    json.RawMessage(d.Get("schema").(string)),
	}
	if description, ok := d.GetOk("description"); ok {
		value := description.(string)
		newType.Description = &value
	}

	created, err := nullOps.CreateApplicationType(newType)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(created.Id)
	return diag.FromErr(setApplicationTypeState(d, created))
}

func ApplicationTypeRead(_ context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)

	// The API answers an id it does not know like one the caller may not see,
	// and a type is never deleted, so there is no "gone" to reconcile here.
	applicationType, err := nullOps.GetApplicationType(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	return diag.FromErr(setApplicationTypeState(d, applicationType))
}

func ApplicationTypeUpdate(_ context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)

	patch := &ApplicationType{}
	if d.HasChange("name") {
		patch.Name = d.Get("name").(string)
	}
	if d.HasChange("description") {
		description := d.Get("description").(string)
		patch.Description = &description
	}
	if d.HasChange("status") {
		patch.Status = d.Get("status").(string)
	}
	if d.HasChange("visible_to") {
		patch.VisibleTo = stringsOfSet(d.Get("visible_to").(*schema.Set))
	}
	if d.HasChange("schema") {
		patch.Schema = json.RawMessage(d.Get("schema").(string))
	}

	updated, err := nullOps.PatchApplicationType(d.Id(), patch)
	if err != nil {
		return diag.FromErr(err)
	}

	return diag.FromErr(setApplicationTypeState(d, updated))
}

// ApplicationTypeDelete retires the type: the API has no delete, because
// applications keep pointing at their type and at its snapshots.
func ApplicationTypeDelete(_ context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)

	if _, err := nullOps.PatchApplicationType(d.Id(), &ApplicationType{Status: "inactive"}); err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
