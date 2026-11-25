package nullplatform

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceApplication() *schema.Resource {
	return &schema.Resource{
		Description: "The application resource allows you to manage a nullplatform Application",

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
				Description: "The URL of the repository that holds this application.",
			},
			"status": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "The status of the application. Possible values: [`pending`, `creating`, `updating`, `active`, `inactive`, `failed`].",
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
				DiffSuppressFunc: suppressEquivalentJSON,
			},
			"settings": {
				Type:             schema.TypeString,
				Optional:         true,
				Description:      "JSON string containing settings for the application.",
				DiffSuppressFunc: suppressEquivalentJSON,
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
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Description: "The ID of the application type of this application (see `nullplatform_application_type`). The type must be active and visible in the application's namespace. It is set when the application is created and cannot be changed: a different type is a different application. With a type, `repository_url` may be left out and arrive later from the type's workflow.",
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
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The ID of the snapshot of its type the application is on: its `attributes` pass that snapshot's schema. Empty for an application with no type.",
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
		newApp.Attributes = withoutNulls(attributes)
	}

	app, err := nullOps.CreateApplication(newApp)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(strconv.Itoa(app.Id))
	return ApplicationRead(ctx, d, m)
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
		app.Attributes = mergePatchBetween(held, wanted)
	}

	if err := nullOps.PatchApplication(appId, app); err != nil {
		return diag.FromErr(err)
	}

	return ApplicationRead(ctx, d, m)
}

func ApplicationDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)
	appId := d.Id()

	if err := nullOps.DeleteApplication(appId); err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
