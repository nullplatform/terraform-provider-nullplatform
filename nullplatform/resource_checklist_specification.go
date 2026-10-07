package nullplatform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourceChecklistSpecification() *schema.Resource {
	return &schema.Resource{
		Description: "Manages a checklist specification: the items an approval action evaluates instead of policies. The " +
			"API stores every edit of `definition` or `description` as a new version with a new id; this resource tracks one " +
			"`name` at one `nrn`, and `current_version_id` is the version approval actions link to. The resource `id` is " +
			"`<nrn>/<name>`.\n\n" +
			"~> **Note:** A new version re-links only the actions linked through " +
			"`nullplatform_approval_action_checklist_specification_association`; actions linked outside Terraform stay on " +
			"their version. Destroying the resource deletes every version, and fails while an approval action is linked to " +
			"any of them.",

		CreateContext: ChecklistSpecificationCreate,
		ReadContext:   ChecklistSpecificationRead,
		UpdateContext: ChecklistSpecificationUpdate,
		DeleteContext: ChecklistSpecificationDelete,
		CustomizeDiff: checklistSpecificationVersionDiff,

		Importer: &schema.ResourceImporter{StateContext: importChecklistSpecification},

		Schema: AddNRNSchema(map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				Description: "The name of the specification. With `nrn`, its identity: changing either replaces the resource " +
					"(add `lifecycle { create_before_destroy = true }` while approval actions are linked to it). A name once " +
					"used at an `nrn`, even deleted, cannot be created there again.",
			},
			"description": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "A description of the specification. Changing it creates a new version.",
			},
			"definition": {
				Type:                  schema.TypeString,
				Required:              true,
				ValidateFunc:          validation.StringIsJSON,
				DiffSuppressFunc:      suppressEquivalentJSON,
				DiffSuppressOnRefresh: true,
				Description: "The specification as a JSON object, usually `jsonencode({ items = [...], execution_trigger = \"explicit\" })`; " +
					"see [Specs and actions](https://docs.nullplatform.com/docs/approvals/checklist-specs) for its format. " +
					"Changing it creates a new version; key order and spacing are not a change.",
			},
			"current_version_id": {
				Type:     schema.TypeString,
				Computed: true,
				Description: "The id (`spec_…`) of the current version: the newest one not deleted. It is what an approval " +
					"action links to, and it changes with every new version.",
			},
			"version": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "The number of the current version.",
			},
		}),
	}
}

// checklistSpecificationVersionDiff plans the current version unknown when the
// apply creates a new one: a changed description, or a definition that is not
// the same JSON spelled otherwise.
func checklistSpecificationVersionDiff(_ context.Context, d *schema.ResourceDiff, _ any) error {
	oldDefinition, newDefinition := d.GetChange("definition")
	if !d.HasChange("description") && suppressEquivalentJSON("definition", oldDefinition.(string), newDefinition.(string), nil) {
		return nil
	}
	return errors.Join(d.SetNewComputed("current_version_id"), d.SetNewComputed("version"))
}

// desiredChecklistSpecification is the description and definition to send. The
// description always goes, so a PATCH never inherits a removed one, and "" goes
// as null: the API's create stores "" as null and a PATCH stores it as given.
func desiredChecklistSpecification(d *schema.ResourceData) *ChecklistSpecification {
	spec := &ChecklistSpecification{Definition: json.RawMessage(d.Get("definition").(string))}
	if description := d.Get("description").(string); description != "" {
		spec.Description = &description
	}
	return spec
}

func checklistSpecificationDescription(spec *ChecklistSpecification) string {
	if spec.Description == nil {
		return ""
	}
	return *spec.Description
}

// currentChecklistSpecificationVersion is the newest version of spec's lineage
// that is not deleted, or nil when every one is.
func currentChecklistSpecificationVersion(spec *ChecklistSpecification) *ChecklistSpecification {
	versions := append([]ChecklistSpecification{*spec}, spec.Versions...)
	var current *ChecklistSpecification
	for i := range versions {
		if versions[i].Status != "deleted" && (current == nil || versions[i].Version > current.Version) {
			current = &versions[i]
		}
	}
	return current
}

func ChecklistSpecificationCreate(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)

	var nrn string
	var err error
	if v, ok := d.GetOk("nrn"); ok {
		nrn = v.(string)
	} else if nrn, err = ConstructNRNFromComponents(d, nullOps); err != nil {
		return diag.Errorf("error constructing NRN: %v", err)
	}
	spec := desiredChecklistSpecification(d)
	spec.Nrn, spec.Name = nrn, d.Get("name").(string)

	// A used name, even deleted, is refused: never retried, never adopted.
	created, err := nullOps.CreateChecklistSpecification(spec)
	if err != nil {
		return diag.FromErr(err)
	}
	// The specification exists from here: the state comes from the create's
	// answer, and a read that fails warns, for the next refresh to complete.
	diags := setChecklistSpecificationVersion(d, created)
	for _, read := range ChecklistSpecificationRead(ctx, d, m) {
		if read.Severity == diag.Error {
			read.Severity = diag.Warning
			read.Summary = fmt.Sprintf("checklist specification %s was created, but reading it back failed: %s", d.Id(), read.Summary)
		}
		diags = append(diags, read)
	}
	return diags
}

// ChecklistSpecificationRead follows the lineage of the current version to its
// newest live version; with none left, or the version gone, the
// specification leaves the state.
func ChecklistSpecificationRead(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	versionID := d.Get("current_version_id").(string)
	if !strings.HasPrefix(versionID, "spec_") {
		return diag.Errorf("checklist specification %q has no version id in its state (%q); import it again", d.Id(), versionID)
	}
	spec, err := m.(NullOps).GetChecklistSpecification(versionID)
	if errors.Is(err, errChecklistSpecificationNotFound) {
		d.SetId("")
		return nil
	}
	if err != nil {
		return diag.FromErr(err)
	}
	current := currentChecklistSpecificationVersion(spec)
	if current == nil {
		d.SetId("")
		return nil
	}
	return setChecklistSpecificationVersion(d, current)
}

// setChecklistSpecificationVersion puts a version in the state. The API
// answers the definition in its own key order: the state keeps the
// configuration's spelling while it is the same JSON.
func setChecklistSpecificationVersion(d *schema.ResourceData, version *ChecklistSpecification) diag.Diagnostics {
	d.SetId(version.Nrn + "/" + version.Name)
	definition := string(version.Definition)
	if written := d.Get("definition").(string); suppressEquivalentJSON("definition", written, definition, d) {
		definition = written
	}
	var diags diag.Diagnostics
	for key, value := range map[string]any{
		"nrn": version.Nrn, "name": version.Name, "description": checklistSpecificationDescription(version),
		"definition": definition, "current_version_id": version.Id, "version": version.Version,
	} {
		diags = append(diags, diag.FromErr(d.Set(key, value))...)
	}
	return diags
}

// ChecklistSpecificationUpdate creates the next version, unless the lineage's
// newest live version already is the desired one (a PATCH whose answer was
// lost): then it adopts that version, because the API numbers every PATCH
// max+1 and a retry would create another.
func ChecklistSpecificationUpdate(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)
	// On an error the state keeps the version the API still has as current,
	// not the configuration that failed.
	d.Partial(true)

	versionID, _ := d.GetChange("current_version_id")
	desired := desiredChecklistSpecification(d)
	spec, err := nullOps.GetChecklistSpecification(versionID.(string))
	if err != nil {
		return diag.FromErr(err)
	}
	next := currentChecklistSpecificationVersion(spec)
	if next == nil {
		// A PATCH would bring a deleted lineage back (the API does not look at
		// the status): the name stays deleted.
		return diag.Errorf("every version of checklist specification %q was deleted outside Terraform; remove it from the "+
			"state (terraform state rm) or import another name", d.Id())
	}
	if checklistSpecificationDescription(next) != checklistSpecificationDescription(desired) ||
		!suppressEquivalentJSON("definition", string(next.Definition), string(desired.Definition), d) {
		if next, err = nullOps.PatchChecklistSpecification(versionID.(string), desired); err != nil {
			return diag.FromErr(err)
		}
	}

	d.Partial(false)
	return append(diag.FromErr(d.Set("current_version_id", next.Id)), ChecklistSpecificationRead(ctx, d, m)...)
}

// ChecklistSpecificationDelete deletes every version or none: while an approval
// action is linked to any version it refuses before the first DELETE,
// and it never forces the API's unlinking (?force=true) nor moves actions.
func ChecklistSpecificationDelete(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)

	versionID := d.Get("current_version_id").(string)
	if !strings.HasPrefix(versionID, "spec_") {
		return diag.Errorf("checklist specification %q has no version id in its state (%q); import it again", d.Id(), versionID)
	}
	spec, err := nullOps.GetChecklistSpecification(versionID)
	if err != nil {
		return diag.FromErr(err)
	}
	// A read without its lineage still names the version it read: never a
	// destroy that deletes nothing.
	versions := spec.Versions
	if len(versions) == 0 {
		versions = []ChecklistSpecification{*spec}
	}
	// The lineage carries each version's count; a version's own read carries
	// its actions, cut at 50.
	var linked []string
	more := 0
	for _, version := range versions {
		// Both skips only save reads: a version without actions adds none, and
		// the version already read needs no second read.
		if version.AssociatedActionsCount == 0 {
			continue
		}
		read := spec
		if version.Id != spec.Id {
			if read, err = nullOps.GetChecklistSpecification(version.Id); err != nil {
				return diag.FromErr(err)
			}
		}
		for _, action := range read.AssociatedActions {
			linked = append(linked, strconv.Itoa(action.Id))
		}
		more += read.AssociatedActionsCount - len(read.AssociatedActions)
	}
	if len(linked) > 0 {
		if more > 0 {
			linked[len(linked)-1] += fmt.Sprintf(" and %d more", more)
		}
		return diag.Errorf("cannot delete checklist specification %q: Checklist specification is in use by one or more approval "+
			"actions (approval actions %s). Unlink them first; to rename a linked specification, add lifecycle { "+
			"create_before_destroy = true }", d.Id(), strings.Join(linked, ", "))
	}

	for i := len(versions) - 1; i >= 0; i-- {
		if versions[i].Status == "deleted" {
			continue
		}
		if err := nullOps.DeleteChecklistSpecification(versions[i].Id); err != nil {
			return diag.FromErr(err)
		}
	}
	d.SetId("")
	return nil
}

// importChecklistSpecification takes the id of any version (spec_…) or
// "<nrn>/<name>"; either way the Read follows the lineage to its current
// version. An NRN has no "/", so the name is everything after the first one.
func importChecklistSpecification(_ context.Context, d *schema.ResourceData, m any) ([]*schema.ResourceData, error) {
	id := d.Id()
	if strings.HasPrefix(id, "spec_") {
		// A version that does not exist leaves the Read nothing: Terraform's
		// "Cannot import non-existent remote object".
		return []*schema.ResourceData{d}, d.Set("current_version_id", id)
	}
	nrn, name, ok := strings.Cut(id, "/")
	if !ok {
		return nil, fmt.Errorf(`checklist specification import id %q is neither a version id (spec_…) nor "<nrn>/<name>"`, id)
	}
	specs, err := m.(NullOps).ListChecklistSpecifications(nrn)
	if err != nil {
		return nil, err
	}
	for _, spec := range specs {
		if spec.Name == name {
			d.SetId(spec.Nrn + "/" + spec.Name)
			return []*schema.ResourceData{d}, d.Set("current_version_id", spec.Id)
		}
	}
	return nil, fmt.Errorf("cannot import checklist specification %q: %s has no live version named %q", id, nrn, name)
}
