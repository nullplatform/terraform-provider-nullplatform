package nullplatform

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceApplications() *schema.Resource {
	return &schema.Resource{
		Description: "Lists every application of a namespace. The API lists applications one namespace at a time, so to cover an account use `for_each` over the namespaces of `nullplatform_namespaces`.",

		ReadContext: dataSourceApplicationsRead,
		Schema: map[string]*schema.Schema{
			"namespace_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "ID of the namespace whose applications are listed.",
			},
			"applications": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "Every application of the namespace, ordered by ID.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id":                      {Type: schema.TypeInt, Computed: true, Description: "A system-wide unique ID for the application."},
						"name":                    {Type: schema.TypeString, Computed: true, Description: "The name of the application."},
						"slug":                    {Type: schema.TypeString, Computed: true, Description: "The unique slug identifier for the application."},
						"status":                  {Type: schema.TypeString, Computed: true, Description: "The status of the application, e.g. `active`."},
						"namespace_id":            {Type: schema.TypeInt, Computed: true, Description: "The ID of the namespace that owns the application."},
						"nrn":                     {Type: schema.TypeString, Computed: true, Description: "The Nullplatform Resource Name (NRN) for the application."},
						"repository_url":          {Type: schema.TypeString, Computed: true, Description: "The URL of the application's code repository."},
						"repository_app_path":     {Type: schema.TypeString, Computed: true, Description: "The path of the application inside a mono-repo; empty otherwise."},
						"is_mono_repo":            {Type: schema.TypeBool, Computed: true, Description: "Whether the application lives in a mono-repo."},
						"template_id":             {Type: schema.TypeInt, Computed: true, Description: "The ID of the technology template the application was created from."},
						"auto_deploy_on_creation": {Type: schema.TypeBool, Computed: true, Description: "Whether the application deploys automatically when created."},
						"tags":                    {Type: schema.TypeString, Computed: true, Description: "The application's tags as a JSON string; empty when it has none."},
					},
				},
			},
		},
	}
}

func dataSourceApplicationsRead(_ context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)
	namespaceID := strconv.Itoa(d.Get("namespace_id").(int))

	applications, err := nullOps.ListApplications(namespaceID)
	if err != nil {
		return diag.FromErr(err)
	}

	rows := make([]map[string]any, 0, len(applications))
	for _, app := range applications {
		tags := ""
		if app.Tags != nil {
			tagsJSON, err := json.Marshal(app.Tags)
			if err != nil {
				return diag.FromErr(fmt.Errorf("error serializing tags for application %d: %v", app.Id, err))
			}
			tags = string(tagsJSON)
		}
		rows = append(rows, map[string]any{
			"id":                      app.Id,
			"name":                    app.Name,
			"slug":                    app.Slug,
			"status":                  app.Status,
			"namespace_id":            app.NamespaceId,
			"nrn":                     app.Nrn,
			"repository_url":          app.RepositoryUrl,
			"repository_app_path":     app.RepositoryAppPath,
			"is_mono_repo":            app.IsMonoRepo,
			"template_id":             app.TemplateId,
			"auto_deploy_on_creation": app.AutoDeployOnCreation,
			"tags":                    tags,
		})
	}

	if err := d.Set("applications", rows); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(namespaceID)

	return nil
}
