package nullplatform

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceApplication() *schema.Resource {
	return &schema.Resource{
		Description: "The application resource allows you to manage a nullplatform Application. " +
			"Destroying it asks the platform to delete the application (status `deleting`) and waits, up to the `delete` timeout, until the application's workflow has removed what it created. " +
			"An application still being created (`pending_hook`, `pending`, `creating`) or updated (`updating`) cannot be deleted yet: destroy first waits for that to end, within the same timeout. " +
			"One already `deleting` is waited for the same way.",

		CreateContext: ApplicationCreate,
		ReadContext:   ApplicationRead,
		UpdateContext: ApplicationUpdate,
		DeleteContext: ApplicationDelete,

		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				d.Set("id", d.Id())
				return []*schema.ResourceData{d}, nil
			},
		},

		Timeouts: &schema.ResourceTimeout{
			// Create waits only for an application configured for a snapshot of
			// its type other than the last: it cannot be moved before its type's
			// workflow has answered its creation. Any other create is one request.
			Create: schema.DefaultTimeout(10 * time.Minute),
			// Update waits only for an application whose creation its type's
			// workflow has not answered yet: until then every edit is refused.
			// Any other update is one request.
			Update: schema.DefaultTimeout(10 * time.Minute),
			// Delete waits for the platform to finish deleting the application.
			Delete: schema.DefaultTimeout(10 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The name of the application. Maximum length is 60 characters.",
			},
			"namespace_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "The ID of the namespace that owns this application.",
			},
			"repository_url": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "The URL of the repository that holds this application. When not set, it reads what the platform holds: for an application of a type, the one the type's workflow reported when it created the application.",
			},
			"status": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				Description: "The status of the application. Possible values: [`pending_hook`, `pending`, `creating`, `updating`, `active`, `inactive`, `failed`, `deleting`, `deleted`]. " +
					"An application with no type starts as `pending`, and the platform's workflow makes it `creating`, then `active`. " +
					"An application of a type starts as `pending_hook` and its type's workflow makes it `active` (or `failed`); until then the platform refuses every edit of it, and an update waits for it (`update` timeout). " +
					"When the platform has no hook to send for the creation of an application of a type, it creates it `failed`, and the create fails with the platform's message.",
			},
			"slug": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "A namespace-wide unique slug for the application.",
			},
			"template_id": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "The ID of the template that was used to create this application.",
			},
			"auto_deploy_on_creation": {
				Type:        schema.TypeBool,
				Optional:    true,
				Computed:    true,
				ForceNew:    true,
				Description: "`True` if the application must be deployed immediately after being created, `false` otherwise.",
			},
			"repository_app_path": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				ForceNew:    true,
				Description: "The folder where the application is located inside a monorepo.",
			},
			"is_mono_repo": {
				Type:        schema.TypeBool,
				Optional:    true,
				Computed:    true,
				ForceNew:    true,
				Description: "`True` if the application shares the repository with other apps, `false` otherwise.",
			},
			"tags": {
				Type:             schema.TypeString,
				Optional:         true,
				Description:      "JSON string containing tags for the application.",
				DiffSuppressFunc: suppressEquivalentJSONOrNoneHeld,
			},
			"settings": {
				Type:             schema.TypeString,
				Optional:         true,
				Description:      "JSON string containing settings for the application.",
				DiffSuppressFunc: suppressEquivalentJSONOrNoneHeld,
			},
			"messages": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "JSON string containing status messages for the application.",
			},
			"nrn": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The Nullplatform Resource Name (NRN) for the application.",
			},
			"application_type_id": {
				Type:             schema.TypeString,
				Optional:         true,
				Computed:         true,
				ForceNew:         true,
				DiffSuppressFunc: suppressSameID,
				Description: "The ID of the application type of this application (see `nullplatform_application_type`). The type must be active and visible in the application's namespace. " +
					"It is set when the application is created and cannot be changed: a different type is a different application, and the platform does not give the name of a deleted application to a new one. " +
					"When not set, it reads the type the platform holds: an application of a type imported, or managed from before this attribute, keeps its type. " +
					"With a type, `repository_url` may be left out and arrive later from the type's workflow.",
			},
			"attributes": {
				Type:             schema.TypeString,
				Optional:         true,
				Computed:         true,
				DiffSuppressFunc: suppressEquivalentJSONWithoutNulls,
				Description: "JSON string with the values of the fields of the application's type, checked against the type's schema. Only for an application with `application_type_id`. " +
					"When set, it is the whole of them: only what changed is sent, and a key that is no longer here is removed from the application, also one that was set outside Terraform. " +
					"A key written as `null` is a key left out, and a key the platform holds as `null` is removed. " +
					"When not set, it reads what the platform holds (`{}` for an application with no type).",
			},
			"application_type_snapshot_id": {
				Type:             schema.TypeString,
				Optional:         true,
				Computed:         true,
				DiffSuppressFunc: suppressSameID,
				Description: "The ID of the snapshot of its type the application is on: its `attributes` pass that snapshot's schema. Empty for an application with no type. " +
					"A change of the type's name, description or schema cuts a new snapshot and leaves the application on the one it has. " +
					"Set it to move the application: to `nullplatform_application_type.<name>.last_snapshot_id` to keep it on the type's latest schema, or to an earlier snapshot of the type to go back. " +
					"The move is one request together with `attributes`, and it is refused, changing nothing, when the attributes do not pass the destination's schema. " +
					"A new application is created on the type's last snapshot, and the `attributes` it is created with must pass that snapshot's schema; one configured for another snapshot is moved there once its type's workflow has answered its creation, which the create waits for (`create` timeout). " +
					"When not set, the application stays on its snapshot and this reads it.",
			},
		},
	}
}

func ApplicationCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)

	newApp := &Application{
		Name:                 d.Get("name").(string),
		NamespaceId:          d.Get("namespace_id").(int),
		RepositoryUrl:        d.Get("repository_url").(string),
		TemplateId:           d.Get("template_id").(int),
		AutoDeployOnCreation: d.Get("auto_deploy_on_creation").(bool),
		RepositoryAppPath:    d.Get("repository_app_path").(string),
		IsMonoRepo:           d.Get("is_mono_repo").(bool),
	}

	if tagsStr, ok := d.GetOk("tags"); ok {
		var tags map[string]interface{}
		if err := json.Unmarshal([]byte(tagsStr.(string)), &tags); err != nil {
			return diag.FromErr(fmt.Errorf("error parsing tags JSON: %v", err))
		}
		newApp.Tags = tags
	}

	if settingsStr, ok := d.GetOk("settings"); ok {
		var settings map[string]interface{}
		if err := json.Unmarshal([]byte(settingsStr.(string)), &settings); err != nil {
			return diag.FromErr(fmt.Errorf("error parsing settings JSON: %v", err))
		}
		newApp.Settings = settings
	}

	newApp.ApplicationTypeId = d.Get("application_type_id").(string)
	if attributesStr := configuredString(d, "attributes"); attributesStr != "" {
		var attributes map[string]interface{}
		if err := json.Unmarshal([]byte(attributesStr), &attributes); err != nil {
			return diag.FromErr(fmt.Errorf("error parsing attributes JSON: %v", err))
		}
		// None to send is no member at all, as before a type took part.
		if attributes = withoutNulls(attributes); len(attributes) > 0 {
			newApp.Attributes = attributes
		}
	}

	app, err := nullOps.CreateApplication(newApp)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(strconv.Itoa(app.Id))

	// With no hook to send for its creation, the platform stores an
	// application of a type as failed: nothing is creating it.
	if app.Status == "failed" {
		return diag.Errorf("application %s was created as failed: %s", d.Id(), summarizeMessages(app.Messages))
	}

	// A new application is on its type's last snapshot; one configured for
	// another is moved there.
	if wanted := configuredString(d, "application_type_snapshot_id"); wanted != "" && !strings.EqualFold(wanted, app.ApplicationTypeSnapshotId) {
		created, err := waitForApplicationSettled(ctx, nullOps, d.Id(), d.Timeout(schema.TimeoutCreate))
		if err != nil {
			return diag.FromErr(err)
		}
		if created.Status == "failed" {
			return diag.Errorf("application %s could not be moved to snapshot %s of its type: its creation failed: %s",
				d.Id(), wanted, summarizeMessages(created.Messages))
		}
		if err := moveApplicationToSnapshot(nullOps, d.Id(), &Application{}, wanted); err != nil {
			return diag.FromErr(err)
		}
	}

	return ApplicationRead(ctx, d, m)
}

// applicationStatusesInFlight are the statuses of an application its
// workflow has not finished with: one of a type waiting for its type's
// workflow to answer its creation (`pending_hook`), and one with no type that
// the platform's workflow is creating or updating. The platform refuses to
// delete it meanwhile, and, for `pending_hook`, every edit of it.
var applicationStatusesInFlight = []string{"pending_hook", "pending", "creating", "updating"}

// waitForApplicationSettled polls an application until its workflow is done
// with it: its status is none of applicationStatusesInFlight.
func waitForApplicationSettled(ctx context.Context, nullOps NullOps, appId string, timeout time.Duration) (*Application, error) {
	stateConf := &retry.StateChangeConf{
		Pending: applicationStatusesInFlight,
		Target:  []string{"settled"},
		Refresh: func() (interface{}, string, error) {
			app, err := nullOps.GetApplication(appId)
			if err != nil {
				return nil, "", err
			}
			if slices.Contains(applicationStatusesInFlight, app.Status) {
				return app, app.Status, nil
			}
			return app, "settled", nil
		},
		Timeout:    timeout,
		Delay:      0,
		MinTimeout: statusPollInterval,
	}
	raw, err := stateConf.WaitForStateContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("application %s is still being created or updated by its workflow: %w", appId, err)
	}
	return raw.(*Application), nil
}

// moveApplicationToSnapshot sends `edit` with the move to `snapshotID`, and
// checks the application got there. The platform moves an application with an
// edit of it, and naming the destination alone is not one: it would be stored
// and applied by whichever edit came next. So the move always carries
// attributes, empty when none change, which the platform checks against the
// destination: it moves, or it refuses and changes nothing.
func moveApplicationToSnapshot(nullOps NullOps, appId string, edit *Application, snapshotID string) error {
	edit.DesiredApplicationTypeSnapshotId = snapshotID
	if edit.Attributes == nil {
		edit.Attributes = map[string]interface{}{}
	}
	if err := nullOps.PatchApplication(appId, edit); err != nil {
		return err
	}

	moved, err := nullOps.GetApplication(appId)
	if err != nil {
		return err
	}
	if !strings.EqualFold(moved.ApplicationTypeSnapshotId, snapshotID) {
		return fmt.Errorf("application %s was asked to move to snapshot %s of its type and is on %s",
			appId, snapshotID, moved.ApplicationTypeSnapshotId)
	}
	return nil
}

func ApplicationRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)
	appId := d.Id()

	app, err := nullOps.GetApplication(appId)
	if err != nil {
		// Deleted outside Terraform: drop it from state so the plan proposes
		// recreating it instead of failing.
		if _, ok := IsResourceNotFoundError(err); ok {
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}
	if app.Status == "deleted" {
		d.SetId("")
		return nil
	}

	if err := d.Set("name", app.Name); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("namespace_id", app.NamespaceId); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("repository_url", app.RepositoryUrl); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("status", app.Status); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("slug", app.Slug); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("template_id", app.TemplateId); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("auto_deploy_on_creation", app.AutoDeployOnCreation); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("repository_app_path", app.RepositoryAppPath); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("is_mono_repo", app.IsMonoRepo); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("nrn", app.Nrn); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("application_type_id", app.ApplicationTypeId); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("application_type_snapshot_id", app.ApplicationTypeSnapshotId); err != nil {
		return diag.FromErr(err)
	}
	// Always an object: the platform answers {} for an application with no
	// type, and so does an API from before application types (no field).
	attributesJSON := "{}"
	if len(app.Attributes) > 0 {
		encoded, err := json.Marshal(app.Attributes)
		if err != nil {
			return diag.FromErr(fmt.Errorf("error serializing attributes to JSON: %v", err))
		}
		attributesJSON = string(encoded)
	}
	if err := d.Set("attributes", attributesJSON); err != nil {
		return diag.FromErr(err)
	}

	if app.Tags != nil {
		tagsJSON, err := json.Marshal(app.Tags)
		if err != nil {
			return diag.FromErr(fmt.Errorf("error serializing tags to JSON: %v", err))
		}
		if err := d.Set("tags", string(tagsJSON)); err != nil {
			return diag.FromErr(err)
		}
	}

	if app.Settings != nil {
		settingsJSON, err := json.Marshal(app.Settings)
		if err != nil {
			return diag.FromErr(fmt.Errorf("error serializing settings to JSON: %v", err))
		}
		if err := d.Set("settings", string(settingsJSON)); err != nil {
			return diag.FromErr(err)
		}
	}

	if app.Messages != nil {
		messagesJSON, err := json.Marshal(app.Messages)
		if err != nil {
			return diag.FromErr(fmt.Errorf("error serializing messages to JSON: %v", err))
		}
		if err := d.Set("messages", string(messagesJSON)); err != nil {
			return diag.FromErr(err)
		}
	}

	return nil
}

func ApplicationUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)
	appId := d.Id()

	// The platform refuses every edit of an application whose creation its
	// type's workflow has not answered: wait for the answer. The state says
	// so from the refresh that planned this update; any other update sends
	// its request straight away.
	if held, _ := d.GetChange("status"); held.(string) == "pending_hook" {
		created, err := waitForApplicationSettled(ctx, nullOps, appId, d.Timeout(schema.TimeoutUpdate))
		if err != nil {
			return diag.FromErr(err)
		}
		if created.Status == "failed" {
			return diag.Errorf("application %s cannot be updated: its creation failed: %s", appId, summarizeMessages(created.Messages))
		}
	}

	app := &Application{}

	if d.HasChange("name") {
		app.Name = d.Get("name").(string)
	}
	if d.HasChange("repository_url") {
		app.RepositoryUrl = d.Get("repository_url").(string)
	}
	if d.HasChange("status") {
		app.Status = d.Get("status").(string)
	}
	if d.HasChange("template_id") {
		app.TemplateId = d.Get("template_id").(int)
	}
	if d.HasChange("tags") {
		if tagsStr, ok := d.GetOk("tags"); ok {
			var tags map[string]interface{}
			if err := json.Unmarshal([]byte(tagsStr.(string)), &tags); err != nil {
				return diag.FromErr(fmt.Errorf("error parsing tags JSON: %v", err))
			}
			app.Tags = tags
		}
	}
	if d.HasChange("settings") {
		if settingsStr, ok := d.GetOk("settings"); ok {
			var settings map[string]interface{}
			if err := json.Unmarshal([]byte(settingsStr.(string)), &settings); err != nil {
				return diag.FromErr(fmt.Errorf("error parsing settings JSON: %v", err))
			}
			app.Settings = settings
		}
	}

	if d.HasChange("attributes") {
		// The platform takes attributes as a JSON Merge Patch (RFC 7396) of the
		// ones it holds, so what goes is the way from those to the ones wanted.
		was, is := d.GetChange("attributes")
		var held, wanted map[string]interface{}
		// Read wrote the state; one from before attributes existed is empty and
		// holds none.
		_ = json.Unmarshal([]byte(was.(string)), &held)
		if err := json.Unmarshal([]byte(is.(string)), &wanted); err != nil {
			return diag.FromErr(fmt.Errorf("error parsing attributes JSON: %v", err))
		}
		// Nothing to change is no member at all.
		if patch := mergePatchBetween(held, wanted); len(patch) > 0 {
			app.Attributes = patch
		}
	}

	if wanted := configuredString(d, "application_type_snapshot_id"); wanted != "" && d.HasChange("application_type_snapshot_id") {
		if err := moveApplicationToSnapshot(nullOps, appId, app, wanted); err != nil {
			// Refused: the state keeps what the platform has, not what was asked.
			d.Partial(true)
			return diag.FromErr(err)
		}
		return ApplicationRead(ctx, d, m)
	}

	if err := nullOps.PatchApplication(appId, app); err != nil {
		d.Partial(true)
		return diag.FromErr(err)
	}

	return ApplicationRead(ctx, d, m)
}

// ApplicationDelete asks the platform to delete the application and waits for
// it. The platform deletes an application by moving it to `deleting`: its
// workflow (the type's, or the built-in one) then removes what it created and
// leaves it `deleted`, or gone. An application its workflow is still creating
// or updating cannot be moved to `deleting` yet, so that is waited out first.
// One already `deleting` is waited for the same way: its workflow is removing
// it, and a DELETE would take it out from under that workflow.
func ApplicationDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)
	appId := d.Id()

	app, err := nullOps.GetApplication(appId)
	if err != nil {
		if _, gone := IsResourceNotFoundError(err); gone {
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}

	if slices.Contains(applicationStatusesInFlight, app.Status) {
		if app, err = waitForApplicationSettled(ctx, nullOps, appId, d.Timeout(schema.TimeoutDelete)); err != nil {
			return diag.FromErr(err)
		}
	}

	switch app.Status {
	case "deleted":
	case "deleting":
		if err := waitForApplicationDeleted(ctx, nullOps, appId, d.Timeout(schema.TimeoutDelete)); err != nil {
			return diag.FromErr(err)
		}
	default:
		if err := nullOps.PatchApplication(appId, &Application{Status: "deleting"}); err != nil {
			return diag.FromErr(err)
		}
		if err := waitForApplicationDeleted(ctx, nullOps, appId, d.Timeout(schema.TimeoutDelete)); err != nil {
			return diag.FromErr(err)
		}
	}

	d.SetId("")
	return nil
}

// waitForApplicationDeleted polls an application that was moved to `deleting`
// until its workflow leaves it `deleted`, or gone.
func waitForApplicationDeleted(ctx context.Context, nullOps NullOps, appId string, timeout time.Duration) error {
	stateConf := &retry.StateChangeConf{
		Pending: []string{"deleting"},
		Target:  []string{"deleted"},
		Refresh: func() (interface{}, string, error) {
			app, err := nullOps.GetApplication(appId)
			if err != nil {
				if _, gone := IsResourceNotFoundError(err); gone {
					return &Application{}, "deleted", nil
				}
				return nil, "", err
			}
			if app.Status == "failed" {
				return app, app.Status, fmt.Errorf("application %s ended in status %q while it was being deleted: %s",
					appId, app.Status, summarizeMessages(app.Messages))
			}
			return app, app.Status, nil
		},
		Timeout:    timeout,
		Delay:      0,
		MinTimeout: statusPollInterval,
	}
	if _, err := stateConf.WaitForStateContext(ctx); err != nil {
		return fmt.Errorf("application %s was asked to be deleted and its workflow did not finish: %w", appId, err)
	}
	return nil
}
